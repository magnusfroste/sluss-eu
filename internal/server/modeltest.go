package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/provider"
)

// modelTestTimeout bounds the live test call. Generous enough for a cold local
// GPU to answer one token, short enough that the button never feels hung.
const modelTestTimeout = 30 * time.Second

// modelTestResult is the JSON the Test button renders inline.
type modelTestResult struct {
	OK         bool   `json:"ok"`
	ModelID    string `json:"model_id"`
	ProviderID string `json:"provider_id"`
	Slug       string `json:"slug"`
	URL        string `json:"url,omitempty"`
	Status     int    `json:"status,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
	Diagnosis  string `json:"diagnosis,omitempty"`
}

// ModelTestHandler live-tests ONE model row: a 1-token chat completion with the
// row's exact slug against its provider, from the router's own network. This is
// the save-time guard against the classic slug mistake (`zai/glm-4.6` instead of
// the provider's own `glm-4.6`) that otherwise surfaces as a 400 in production.
// Only configured models are testable (no arbitrary URLs — same SSRF stance as
// provider_probe).
func ModelTestHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := strings.TrimSpace(r.FormValue("id"))
		w.Header().Set("Content-Type", "application/json")
		if id == "" {
			writeModelTest(w, http.StatusBadRequest, modelTestResult{Error: "missing model id"})
			return
		}
		res, status := opts.runModelTest(r, id)
		writeModelTest(w, status, res)
	}
}

func writeModelTest(w http.ResponseWriter, status int, res modelTestResult) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(res)
}

// runModelTest resolves the row (slug + provider connection) and makes the call.
func (o ModelsOptions) runModelTest(r *http.Request, id string) (modelTestResult, int) {
	res := modelTestResult{ModelID: id}

	providerID, slug, reasoning, found := o.modelConn(id)
	if !found {
		res.Error = "unknown model id"
		return res, http.StatusNotFound
	}
	res.ProviderID, res.Slug = providerID, slug

	baseURL, keyEnv, ok := o.providerConnByID(providerID)
	if !ok || baseURL == "" {
		res.Error = "no provider connection (base_url) for " + providerID
		return res, http.StatusOK
	}
	chatURL, err := provider.ChatCompletionsURL(baseURL)
	if err != nil {
		res.Error = "invalid base_url: " + err.Error()
		return res, http.StatusOK
	}
	res.URL = chatURL

	// The exact request a routed call would make, minimized: the row's slug,
	// one token, no stream. Reasoning-capable rows test with thinking off so
	// the test is fast and still proves the slug + auth + endpoint.
	payload := map[string]any{
		"model":      slug,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
		"stream":     false,
	}
	if reasoning {
		payload["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		return res, http.StatusOK
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(os.Getenv(keyEnv)); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	client := o.ProbeClient
	if client == nil {
		client = &http.Client{Timeout: modelTestTimeout}
	}
	start := time.Now()
	resp, err := client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		res.Diagnosis = "connection failed from the router — DNS, egress firewall, or the endpoint is unreachable from this host."
		return res, http.StatusOK
	}
	defer resp.Body.Close()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	res.Status = resp.StatusCode
	res.OK = resp.StatusCode < 400
	if !res.OK {
		res.Error = strings.TrimSpace(string(snippet))
		res.Diagnosis = modelTestDiagnosis(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Server"))
	}
	return res, http.StatusOK
}

// modelTestDiagnosis adds the slug-specific hint on top of the generic probe
// diagnosis: a 400/404 on a chat call with a known-reachable provider is almost
// always a wrong model slug.
func modelTestDiagnosis(status int, contentType, server string) string {
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return "the provider rejected the model slug — it must be exactly what the provider's own /models endpoint returns (e.g. \"glm-4.6\", not \"zai/glm-4.6\")."
	}
	return probeDiagnosis(status, contentType, server)
}

// modelConn resolves a model id to its provider id, provider-side slug and
// reasoning flag — roster (source of truth) first, live registry as fallback so
// read-only rows are testable too.
func (o ModelsOptions) modelConn(id string) (providerID, slug string, reasoning, ok bool) {
	if o.Roster != nil {
		if ms, err := o.Roster.LoadRosterModels(); err == nil {
			for _, m := range ms {
				if m.ID == id {
					return m.ProviderID, m.ProviderModelID, m.ReasoningCapable, true
				}
			}
		}
	}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			if m, found := snap.Model(id); found {
				return m.ProviderID, m.ProviderModelID, m.ReasoningCapable, true
			}
		}
	}
	return "", "", false, false
}

// providerConnByID resolves a provider id to its base_url and key env var name,
// roster first, then the live registry.
func (o ModelsOptions) providerConnByID(id string) (baseURL, keyEnv string, ok bool) {
	if o.Roster != nil {
		if ps, err := o.Roster.LoadRosterProviders(); err == nil {
			for _, p := range ps {
				if p.ID == id {
					return p.BaseURL, p.KeyEnv, true
				}
			}
		}
	}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				if p.ID == id {
					return p.BaseURL, p.AuthSecretRef, true
				}
			}
		}
	}
	return "", "", false
}

// providerTestResult is what the per-provider Test button renders (ISSUE-134):
// every model routed to this connection, live-tested with its own slug, plus
// what the provider itself says it serves — so a model renamed or unloaded on
// a lab box shows up as "registered here, not served there" instead of as an
// error in the chat.
type providerTestResult struct {
	ProviderID  string            `json:"provider_id"`
	Models      []modelTestResult `json:"models"`
	Served      []string          `json:"served,omitempty"`
	ServedError string            `json:"served_error,omitempty"`
	Missing     []string          `json:"missing,omitempty"` // registered slugs the provider does not list
}

// ProviderTestHandler tests one provider connection: GET <base>/models for the
// served list, then a 1-token completion per model routed to it.
func ProviderTestHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := strings.TrimSpace(r.FormValue("id"))
		w.Header().Set("Content-Type", "application/json")
		baseURL, keyEnv, ok := opts.providerConnByID(id)
		if id == "" || !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(providerTestResult{ProviderID: id, ServedError: "unknown provider id"})
			return
		}
		res := providerTestResult{ProviderID: id}
		res.Served, res.ServedError = opts.servedModels(r, baseURL, keyEnv)
		served := map[string]bool{}
		for _, s := range res.Served {
			served[s] = true
		}
		for _, mid := range opts.modelsOfProvider(id) {
			mr, _ := opts.runModelTest(r, mid)
			res.Models = append(res.Models, mr)
			if len(res.Served) > 0 && mr.Slug != "" && !served[mr.Slug] {
				res.Missing = append(res.Missing, mr.Slug)
			}
		}
		_ = json.NewEncoder(w).Encode(res)
	}
}

// modelsOfProvider lists the model ids routed to a provider connection.
func (o ModelsOptions) modelsOfProvider(providerID string) []string {
	var ids []string
	seen := map[string]bool{}
	if o.Roster != nil {
		if ms, err := o.Roster.LoadRosterModels(); err == nil {
			for _, m := range ms {
				if m.ProviderID == providerID && !seen[m.ID] {
					ids, seen[m.ID] = append(ids, m.ID), true
				}
			}
		}
	}
	if len(ids) == 0 && o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, m := range snap.ModelsForProvider(providerID) {
				if !seen[m.ID] {
					ids, seen[m.ID] = append(ids, m.ID), true
				}
			}
		}
	}
	return ids
}

// servedModels asks the provider which models it serves (OpenAI-style
// GET <base>/models). Errors are returned as text for the panel, never fatal.
func (o ModelsOptions) servedModels(r *http.Request, baseURL, keyEnv string) ([]string, string) {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err.Error()
	}
	if key := strings.TrimSpace(os.Getenv(keyEnv)); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := o.ProbeClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "could not reach " + u + ": " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, u + " answered HTTP " + itoa(resp.StatusCode)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, u + " did not return a model list"
	}
	var ids []string
	for _, d := range out.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, ""
}

// ModelSlugHandler points a routed model at another slug the provider serves
// (ISSUE-134): the "Use this" button in a provider test. Only a slug the
// provider's own /models lists is accepted, so a typo cannot be saved; the
// edit is applied live and audited like any model save.
func ModelSlugHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, slug := strings.TrimSpace(r.FormValue("id")), strings.TrimSpace(r.FormValue("slug"))
		w.Header().Set("Content-Type", "application/json")
		fail := func(code int, msg string) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		}
		if opts.Roster == nil {
			fail(http.StatusConflict, "roster is read-only (no data dir configured)")
			return
		}
		ms, err := opts.Roster.LoadRosterModels()
		if err != nil {
			fail(http.StatusInternalServerError, err.Error())
			return
		}
		for _, m := range ms {
			if m.ID != id {
				continue
			}
			baseURL, keyEnv, ok := opts.providerConnByID(m.ProviderID)
			if !ok {
				fail(http.StatusConflict, "no provider connection for "+m.ProviderID)
				return
			}
			served, serr := opts.servedModels(r, baseURL, keyEnv)
			if serr != "" {
				fail(http.StatusBadGateway, serr)
				return
			}
			listed := false
			for _, s := range served {
				listed = listed || s == slug
			}
			if !listed {
				fail(http.StatusBadRequest, "the provider does not serve "+slug)
				return
			}
			m.ProviderModelID = slug
			if err := opts.Roster.UpsertRosterModel(m); err != nil {
				fail(http.StatusInternalServerError, err.Error())
				return
			}
			okMsg, errMsg := applyRosterEdit(r.Context(), opts.Reloader, AdminUserFromContext(r.Context()), "Model "+m.ID+" now uses "+slug)
			_ = json.NewEncoder(w).Encode(map[string]string{"ok": okMsg, "error": errMsg})
			return
		}
		fail(http.StatusNotFound, "unknown model id")
	}
}
