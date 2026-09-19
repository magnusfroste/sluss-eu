// Package pricing syncs live model prices from a provider catalog into the
// registry, so per-model cost stays honest without manual upkeep. OpenRouter's
// public /models API returns per-token USD prices per model slug; we map those
// onto registry models by ProviderModelID. Realized cost always comes from
// actual usage — this only keeps the *estimates*, savings baseline and scoring
// cost term current.
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/registry"
)

// Price is per-model cost in micros (µUSD) per million tokens.
type Price struct {
	InputMicrosPerMTok  int64
	OutputMicrosPerMTok int64
}

// FetchOpenRouter fetches the OpenRouter model catalog and returns a map of
// model slug -> price. baseURL is the OpenRouter API root (…/api/v1); apiKey is
// optional (the catalog is public). The returned map is keyed by the same slug
// used as ProviderModelID (e.g. "openai/gpt-4o-mini").
func FetchOpenRouter(ctx context.Context, baseURL, apiKey string, client *http.Client) (map[string]Price, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pricing: build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pricing: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pricing: %s returned %d", url, resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("pricing: decode: %w", err)
	}
	return parseCatalog(body.Data), nil
}

// parseCatalog is the pure part (testable without HTTP).
func parseCatalog(data []struct {
	ID      string `json:"id"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}) map[string]Price {
	out := make(map[string]Price, len(data))
	for _, m := range data {
		in := perTokenToMicrosPerMTok(m.Pricing.Prompt)
		outp := perTokenToMicrosPerMTok(m.Pricing.Completion)
		if in == 0 && outp == 0 {
			continue // free or unpriced (e.g. some :free variants) — skip
		}
		out[m.ID] = Price{InputMicrosPerMTok: in, OutputMicrosPerMTok: outp}
	}
	return out
}

// perTokenToMicrosPerMTok converts an OpenRouter per-token USD string
// (e.g. "0.00000015") into micros (µUSD) per million tokens (e.g. 150000).
func perTokenToMicrosPerMTok(s string) int64 {
	usdPerToken, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || usdPerToken <= 0 {
		return 0
	}
	// $/token * 1e6 tokens = $/Mtok; * 1e6 = micros/Mtok  →  * 1e12 total.
	return int64(usdPerToken*1e12 + 0.5)
}

// ApplyToDefinition updates model Cost from the price map, matched by
// ProviderModelID, but ONLY for models whose ProviderID is in syncProviders
// (the catalog the prices came from, e.g. "openrouter"). This is deliberate:
// a custom provider is a different endpoint that may charge a different
// (negotiated) rate for the same model slug, so its admin-set price must stand
// even when the slug collides with the catalog. Returns how many were updated.
func ApplyToDefinition(def *registry.Definition, prices map[string]Price, syncProviders ...string) int {
	if def == nil || len(prices) == 0 {
		return 0
	}
	synced := map[string]bool{}
	for _, id := range syncProviders {
		synced[id] = true
	}
	n := 0
	for i := range def.Models {
		if len(synced) > 0 && !synced[def.Models[i].ProviderID] {
			continue // only the synced catalog's own models
		}
		if p, ok := prices[def.Models[i].ProviderModelID]; ok {
			def.Models[i].Cost.Currency = "USD"
			def.Models[i].Cost.InputMicrosPerMillionToken = p.InputMicrosPerMTok
			def.Models[i].Cost.OutputMicrosPerMillionToken = p.OutputMicrosPerMTok
			n++
		}
	}
	return n
}
