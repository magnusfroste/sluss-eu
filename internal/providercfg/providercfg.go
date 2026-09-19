// Package providercfg is the admin-editable, file-persisted configuration of
// the router's provider connections and routable models. It is split into two
// concerns (ISSUE-072), mirroring LiteLLM's "LLM Credentials" vs "Models":
//
//   - Provider (connection) → providers.json: { id, name, base_url, key_env }.
//     A reusable OpenAI-compatible endpoint. Holds only non-secret config — the
//     API key is referenced by env var name (KeyEnv) and resolved from the
//     environment at startup, never stored here (the CISO posture).
//   - Model (routable) → models.json: { id, provider_id, provider_model_id,
//     tier, prices, enabled }. References a provider connection by provider_id.
//
// The three built-in tiers (cheap-general / balanced-coder / premium-reasoning)
// and the OpenRouter connection are seeded as data on first run, so nothing is
// hardcoded and everything is re-tierable from the Models admin page.
package providercfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/registry"
)

// Provider is a reusable connection to one OpenAI-compatible endpoint. It has
// no embedded models — models reference it by ID (see Model.ProviderID).
type Provider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	KeyEnv  string `json:"key_env"`
	// ComplianceTags are operator-set residency/compliance facts (e.g.
	// "eu-resident", "dpa-signed") that policy can require/deny (ISSUE-078).
	// Inherited by the provider's models.
	ComplianceTags []string `json:"compliance_tags,omitempty"`
}

// Model is one routable model: it names a provider connection, the concrete
// provider-side model slug, its routing tier, and per-Mtok USD price.
type Model struct {
	ID               string  `json:"id"`
	ProviderID       string  `json:"provider_id"`
	ProviderModelID  string  `json:"provider_model_id"`
	Tier             string  `json:"tier"`
	InputUSDPerMTok  float64 `json:"input_usd_per_mtok"`
	OutputUSDPerMTok float64 `json:"output_usd_per_mtok"`
	Enabled          bool    `json:"enabled"`
	// ComplianceTags add to the provider's tags for this model (never remove).
	ComplianceTags []string `json:"compliance_tags,omitempty"`
	// ReasoningCapable: the model has a switchable thinking mode the router
	// controls per request (enable_thinking on/off by task). See registry.Model.
	ReasoningCapable bool `json:"reasoning_capable,omitempty"`
}

// ParseTags parses a comma-separated compliance-tag string (admin form input)
// into a normalized slice: trimmed, lowercased, empties dropped. Returns nil
// for blank input.
func ParseTags(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Load reads the providers (connections) config file. A missing file yields an
// empty list with no error (first run).
func Load(path string) ([]Provider, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("providercfg: read %s: %w", path, err)
	}
	var ps []Provider
	if err := json.Unmarshal(b, &ps); err != nil {
		return nil, fmt.Errorf("providercfg: parse %s: %w", path, err)
	}
	return ps, nil
}

// LoadModels reads the models config file. A missing file yields an empty list
// with no error (first run).
func LoadModels(path string) ([]Model, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("providercfg: read %s: %w", path, err)
	}
	var ms []Model
	if err := json.Unmarshal(b, &ms); err != nil {
		return nil, fmt.Errorf("providercfg: parse %s: %w", path, err)
	}
	return ms, nil
}

// Save writes the providers config atomically (temp file + rename).
func Save(path string, ps []Provider) error { return writeJSON(path, ps) }

// SaveModels writes the models config atomically (temp file + rename).
func SaveModels(path string, ms []Model) error { return writeJSON(path, ms) }

func writeJSON(path string, v any) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("providercfg: create dir: %w", err)
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("providercfg: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// Upsert inserts or replaces a provider by ID and returns the new list.
func Upsert(ps []Provider, p Provider) []Provider {
	for i := range ps {
		if ps[i].ID == p.ID {
			ps[i] = p
			return ps
		}
	}
	return append(ps, p)
}

// Delete removes a provider by ID.
func Delete(ps []Provider, id string) []Provider {
	out := ps[:0]
	for _, p := range ps {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return out
}

// UpsertModel inserts or replaces a model by ID and returns the new list.
func UpsertModel(ms []Model, m Model) []Model {
	for i := range ms {
		if ms[i].ID == m.ID {
			ms[i] = m
			return ms
		}
	}
	return append(ms, m)
}

// DeleteModel removes a model by ID.
func DeleteModel(ms []Model, id string) []Model {
	out := ms[:0]
	for _, m := range ms {
		if m.ID != id {
			out = append(out, m)
		}
	}
	return out
}

// Validate checks required fields of a provider connection and normalizes the ID.
func (p *Provider) Validate() error {
	p.ID = slug(p.ID)
	if p.ID == "" {
		return fmt.Errorf("provider id is required")
	}
	if p.Name == "" {
		p.Name = p.ID
	}
	if !strings.HasPrefix(p.BaseURL, "http") {
		return fmt.Errorf("base_url must be an http(s) URL")
	}
	if p.KeyEnv == "" {
		return fmt.Errorf("key_env (env var name holding the API key) is required")
	}
	return nil
}

// Validate checks required fields of a model and normalizes the ID.
func (m *Model) Validate() error {
	m.ID = slug(m.ID)
	if m.ID == "" {
		return fmt.Errorf("model id is required")
	}
	if m.ProviderID == "" {
		return fmt.Errorf("a provider connection must be selected")
	}
	if m.ProviderModelID == "" {
		return fmt.Errorf("provider_model_id (the provider-side model slug) is required")
	}
	switch m.Tier {
	case "cheap", "balanced", "premium":
	default:
		return fmt.Errorf("tier must be cheap|balanced|premium")
	}
	return nil
}

// RegistryEntries converts the config providers + models into registry Provider
// and Model entries to build a Definition. Only models whose provider's KeyEnv
// is set in the environment are Enabled (routable); a missing key leaves the
// provider listed but its models unroutable.
//
// Built-in models (matched by ID against the OpenRouter seed) inherit that
// seed's rich capability / quality / context / latency metadata so routing
// behavior is unchanged when only tier or price is edited. Custom models get
// tier-derived generic metadata.
func RegistryEntries(providers []Provider, models []Model) ([]registry.Provider, []registry.Model) {
	keySet := map[string]bool{}
	var provs []registry.Provider
	for _, p := range providers {
		keySet[p.ID] = os.Getenv(p.KeyEnv) != ""
		provs = append(provs, registry.Provider{
			ID: p.ID, Name: p.Name, Status: registry.ProviderStatusActive,
			BaseURL: p.BaseURL, AuthSecretRef: p.KeyEnv,
			ComplianceTags: append([]string(nil), p.ComplianceTags...),
		})
	}
	templates := builtinTemplates()
	var regModels []registry.Model
	for _, m := range models {
		rm, isBuiltin := templates[m.ID]
		if !isBuiltin {
			rm = registry.Model{
				Capabilities:        registry.Capabilities{Chat: true, Streaming: true, ToolCalls: true, JSONSchema: true},
				ContextWindowTokens: 128000,
				Latency:             registry.LatencyMetadata{P50FirstTokenMS: 600, P95FirstTokenMS: 1600},
				QualityScores:       tierQuality(m.Tier),
			}
		}
		rm.ID = m.ID
		rm.ProviderID = m.ProviderID
		rm.ProviderModelID = m.ProviderModelID
		rm.Tier = registry.Tier(m.Tier)
		rm.Cost = registry.CostMetadata{
			Currency:                    "USD",
			InputMicrosPerMillionToken:  int64(m.InputUSDPerMTok * 1e6),
			OutputMicrosPerMillionToken: int64(m.OutputUSDPerMTok * 1e6),
		}
		rm.ComplianceTags = append([]string(nil), m.ComplianceTags...)
		rm.ReasoningCapable = m.ReasoningCapable
		// Routable only when the admin enabled it AND its provider's key is present.
		rm.Enabled = m.Enabled && keySet[m.ProviderID]
		regModels = append(regModels, rm)
	}
	return provs, regModels
}

// builtinTemplates indexes the OpenRouter seed models by ID for metadata reuse.
func builtinTemplates() map[string]registry.Model {
	out := map[string]registry.Model{}
	for _, m := range registry.OpenRouterDefinition().Models {
		out[m.ID] = m
	}
	return out
}

// SeedProviders is the built-in provider connection (OpenRouter) seeded on first
// run, so the roster is data rather than hardcoded.
func SeedProviders() []Provider {
	def := registry.OpenRouterDefinition()
	var out []Provider
	for _, p := range def.Providers {
		out = append(out, Provider{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, KeyEnv: p.AuthSecretRef})
	}
	return out
}

// SeedModels are the three built-in tier models seeded on first run. Tier and
// price are data here so they can be edited/re-tiered from the Models page.
func SeedModels() []Model {
	def := registry.OpenRouterDefinition()
	var out []Model
	for _, m := range def.Models {
		out = append(out, Model{
			ID:               m.ID,
			ProviderID:       m.ProviderID,
			ProviderModelID:  m.ProviderModelID,
			Tier:             string(m.Tier),
			InputUSDPerMTok:  float64(m.Cost.InputMicrosPerMillionToken) / 1e6,
			OutputUSDPerMTok: float64(m.Cost.OutputMicrosPerMillionToken) / 1e6,
			Enabled:          true,
		})
	}
	return out
}

func tierQuality(tier string) map[string]float64 {
	switch tier {
	case "premium":
		return map[string]float64{"hard_code_debugging": 0.85, "security_review": 0.85, "simple_code_edit": 0.9}
	case "balanced":
		return map[string]float64{"simple_code_edit": 0.82, "hard_code_debugging": 0.66}
	default:
		return map[string]float64{"simple_code_edit": 0.7, "summarization": 0.76}
	}
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '.':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// SortByID returns the providers sorted by ID (stable UI ordering).
func SortByID(ps []Provider) []Provider {
	out := append([]Provider(nil), ps...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SortModels returns models sorted by provider then ID (stable UI ordering).
func SortModels(ms []Model) []Model {
	out := append([]Model(nil), ms...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderID != out[j].ProviderID {
			return out[i].ProviderID < out[j].ProviderID
		}
		return out[i].ID < out[j].ID
	})
	return out
}
