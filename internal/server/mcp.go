package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/magnusfroste/sluss/internal/buildinfo"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/mcp"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/registry"
)

// MCPName and MCPVersion identify the server in the MCP initialize handshake.
const (
	MCPName    = "sluss"
	MCPVersion = "0.1.0"
)

// MCPOptions carries the (already-wired) dependencies the read-only MCP tools
// wrap. Everything is optional except Engine, which route_explain needs; a tool
// whose dependency is nil returns an explanatory error rather than panicking.
type MCPOptions struct {
	Engine      *engine.Engine
	PolicyCache *policy.Cache
	// Dashboard is reused verbatim by savings_report via buildDashboardData, so
	// the MCP savings figures match the dashboard exactly (no duplicated math).
	Dashboard  DashboardOptions
	History    *history.Store
	RequestLog *eventlog.RequestLogTracker
	Health     *health.Tracker
	// Roster, when set, is the SQLite-backed source of truth (ISSUE-073) read by
	// get_roster. Nil falls back to the live registry snapshot.
	Roster providercfg.RosterStore
	// Errors, when set, is the in-memory ring of recent upstream failures read by
	// recent_errors. Nil disables that tool.
	Errors *eventlog.ErrorRing
	// ProbeClient makes the outbound HTTP request for provider_probe. Nil uses a
	// default client with a short timeout. Injected in tests.
	ProbeClient *http.Client
	// KeyManager provisions DB-backed keys for the write tools (mint/revoke).
	KeyManager *apikeys.Manager
	// Auditor records write-tool actions (mint/revoke/demo-link) in the audit trail.
	Auditor audit.Sink
	// PublicURL builds the shareable demo link URL in create_demo_link.
	PublicURL string
	// Profile is the explicit regime profile (ROUTER_PROFILE) for incident_report
	// framing; empty derives from the active policy version.
	Profile string
	// WriteEnabled gates the mutating tools (mint_key, revoke_key,
	// create_demo_link). Off by default so a deployment's MCP surface stays
	// read-only unless ROUTER_MCP_WRITE_ENABLED is set. Read tools (usage) are
	// always available. Every mutation is still admin-gated and audited.
	WriteEnabled bool
	Version      string
}

// NewMCPServer builds an mcp.Server with the read-only tools registered, plus
// the write tools (mint_key/revoke_key/create_demo_link) when opts.WriteEnabled.
// Shared by the HTTP mount (internal/server) and the stdio binary (cmd/mcp) so
// the tool set and its logic live in exactly one place.
func NewMCPServer(opts MCPOptions) *mcp.Server {
	s := mcp.NewServer(MCPName, MCPVersion)
	s.Register(mcp.Tool{
		Name: "route_explain",
		Description: "Dry-run a prompt through the routing engine (no provider call) and " +
			"explain the decision: task_type, risk, tier, selected_model, provider_model_id, " +
			"reasons, fallbacks and estimated cost. Answers \"why did this route to premium?\".",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "The user prompt to classify and route.",
				},
				"messages": map[string]any{
					"type":        "array",
					"description": "Optional OpenAI-style chat messages; overrides prompt when set.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"role":    map[string]any{"type": "string"},
							"content": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
		Handler: opts.routeExplain,
	})
	s.Register(mcp.Tool{
		Name: "savings_report",
		Description: "Summarise savings and the green receipt (reuses the dashboard data): " +
			"saved USD, saved percent, spend, tokens, estimated energy/CO2e saved, and route distribution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"window": map[string]any{
					"type":        "string",
					"description": "\"24h\", \"72h\", \"7d\" or \"30d\" for that period only; omit (or \"all\") for cumulative all-time. Same vocabulary as incident_report, so a daily brief can ask both tools for the same period.",
				},
			},
		},
		Handler: opts.savingsReport,
	})
	s.Register(mcp.Tool{
		Name:        "recent_requests",
		Description: "Recent request-log rows: time, task, risk, model, provider, tokens and estimated cost. No raw prompts are stored.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"n": map[string]any{
					"type":        "integer",
					"description": "How many recent rows to return (default 20, max 200).",
				},
				"task_filter": map[string]any{
					"type":        "string",
					"description": "Optional task_type to filter by (exact match).",
				},
			},
		},
		Handler: opts.recentRequests,
	})
	s.Register(mcp.Tool{
		Name:        "get_roster",
		Description: "The current roster: tier -> model with provider, provider_model_id, per-Mtok price and enabled flag, plus provider connections.",
		InputSchema: map[string]any{"type": "object"},
		Handler:     opts.getRoster,
	})
	s.Register(mcp.Tool{
		Name:        "provider_health",
		Description: "Provider health scores in [0,1] and a status label (healthy/degraded/down) per provider connection.",
		InputSchema: map[string]any{"type": "object"},
		Handler:     opts.providerHealth,
	})
	s.Register(mcp.Tool{
		Name: "server_info",
		Description: "Diagnostics: registry version, roster source, provider/model counts and the active runtime flags " +
			"(conservative mode, pricing sync, decision-cache TTL, budget). Answers \"is prod running the config I think?\".",
		InputSchema: map[string]any{"type": "object"},
		Handler:     opts.serverInfo,
	})
	s.Register(mcp.Tool{
		Name: "explain_request",
		Description: "Look up a past decision by its request ID (the x-router-request-id header/log value): " +
			"routed model/tier/provider, tokens and cost. No raw prompt is stored (masking).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"request_id": map[string]any{"type": "string", "description": "The x-router-request-id to look up."},
			},
			"required": []any{"request_id"},
		},
		Handler: opts.explainRequest,
	})
	s.Register(mcp.Tool{
		Name: "get_policy",
		Description: "The active routing policy for a scope (default: global): policy version, registry version, " +
			"rule count and settings. Answers \"is prod running the policy I think?\".",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Optional tenant scope; empty = global."},
			},
		},
		Handler: opts.getPolicy,
	})
	s.Register(mcp.Tool{
		Name: "recent_errors",
		Description: "Recent UPSTREAM failures (in-memory, since last restart): time, provider, model, error_code " +
			"(provider_5xx/provider_timeout/provider_auth_error/...), attempt index and latency. No prompt text. " +
			"Answers \"why is the demo 502ing?\" without shell access to logs.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"n": map[string]any{"type": "integer", "description": "How many recent failures (default 20, max 200)."},
			},
		},
		Handler: opts.recentErrors,
	})
	s.Register(mcp.Tool{
		Name: "provider_probe",
		Description: "Live-check a CONFIGURED provider FROM THE ROUTER'S OWN NETWORK: GET <base_url>/models (or a tiny " +
			"completion) and report status, latency and any error. Answers \"can the router actually reach this endpoint?\" — " +
			"the network reality a client elsewhere can't see. Only configured providers can be probed (no arbitrary URLs).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"provider_id": map[string]any{"type": "string", "description": "Provider connection id to probe (see get_roster)."},
				"mode":        map[string]any{"type": "string", "description": "\"models\" (default, GET /models) or \"chat\" (a 1-token completion using its provider_model_id)."},
			},
			"required": []any{"provider_id"},
		},
		Handler: opts.providerProbe,
	})

	if opts.History != nil {
		s.Register(mcp.Tool{
			Name: "incident_report",
			Description: "Incident evidence for a time window (NIS2 24h early warning / 72h notification): blocked " +
				"requests by code/classification/task, sensitive prompts and where they went (local vs cloud), active " +
				"policy version. Counts and classifications only — never prompt content. Not a legal attestation.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"window": map[string]any{"type": "string", "description": "\"24h\", \"72h\" (default) or \"7d\"."},
				},
			},
			Handler: opts.incidentReport,
		})
	}

	// --- outreach: create + measure (ISSUE-091) -----------------------------
	// Read tools (usage) are always available; write tools are gated behind
	// WriteEnabled so a deployment stays read-only unless it opts in.
	s.Register(mcp.Tool{
		Name: "key_usage",
		Description: "Has a prospect tested? Realized activity for a key's tenant from the durable log: " +
			"created_at, revoked, request_count, tokens, last_activity and a used flag. Mint per-prospect keys " +
			"with a unique tenant to make this attributable to one prospect.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"key_id": map[string]any{"type": "string", "description": "The key id from mint_key / get keys."}},
			"required":   []any{"key_id"},
		},
		Handler: opts.keyUsage,
	})
	s.Register(mcp.Tool{
		Name: "demo_link_usage",
		Description: "Who has tried the demo: every NAMED link with label, opens and last-open time, plus the legacy " +
			"shared link's counters — the per-prospect follow-up signal.",
		InputSchema: map[string]any{"type": "object"},
		Handler:     opts.demoLinkUsage,
	})
	if opts.WriteEnabled {
		s.Register(mcp.Tool{
			Name: "mint_key",
			Description: "Create a per-prospect API key. Returns the plaintext secret ONCE plus its key_id. " +
				"Pass a unique tenant per prospect so key_usage is attributable. (Write tool; audited.)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tenant":  map[string]any{"type": "string", "description": "Unique tenant id for this prospect (required)."},
					"project": map[string]any{"type": "string", "description": "Optional project/department label."},
					"role":    map[string]any{"type": "string", "description": "Optional role (default user)."},
					"scopes":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional scopes; empty = full access."},
				},
				"required": []any{"tenant"},
			},
			Handler: opts.mintKey,
		})
		s.Register(mcp.Tool{
			Name:        "revoke_key",
			Description: "Revoke a key by id (e.g. when a trial ends). (Write tool; audited.)",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"key_id": map[string]any{"type": "string", "description": "The key id to revoke."}},
				"required":   []any{"key_id"},
			},
			Handler: opts.revokeKey,
		})
		s.Register(mcp.Tool{
			Name: "create_demo_link",
			Description: "Create a NAMED per-prospect demo link (pass label, e.g. the company) and return its full URL — " +
				"opens are counted per link so follow-up knows who tried. Without a label it rotates the legacy shared link. (Write tool; audited.)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"label": map[string]any{"type": "string", "description": "Prospect label (company/person). Empty = rotate the legacy shared link."},
				},
			},
			Handler: opts.createDemoLink,
		})
	}
	return s
}

// ---- explain_request ----

type explainRequestArgs struct {
	RequestID string `json:"request_id"`
}

func (o MCPOptions) explainRequest(_ context.Context, raw json.RawMessage) (any, error) {
	var args explainRequestArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	id := strings.TrimSpace(args.RequestID)
	if id == "" {
		return nil, errors.New("provide a request_id")
	}
	if o.History == nil {
		return nil, errors.New("request history not configured (set ROUTER_DATA_DIR)")
	}
	rec, ok := o.History.ByRequestID(id)
	if !ok {
		return map[string]any{
			"found":      false,
			"request_id": id,
			"note":       "no logged request with that id — it may be outside the retention window. No raw prompt is ever stored (masking).",
		}, nil
	}
	return map[string]any{
		"found":   true,
		"request": rec,
		"note":    "decision row from the durable log; no raw prompt is stored (masking).",
	}, nil
}

// ---- get_policy ----

type getPolicyArgs struct {
	TenantID string `json:"tenant_id"`
}

func (o MCPOptions) getPolicy(_ context.Context, raw json.RawMessage) (any, error) {
	if o.PolicyCache == nil {
		return nil, errors.New("policy cache not configured")
	}
	var args getPolicyArgs
	_ = json.Unmarshal(raw, &args) // arguments are optional
	scope := policy.Scope{TenantID: strings.TrimSpace(args.TenantID)}
	p, ok := o.PolicyCache.Active(scope)
	if !ok {
		return map[string]any{"found": false, "note": "no active policy for that scope (built-in defaults in effect)"}, nil
	}
	return map[string]any{
		"found":            true,
		"scope_tenant":     scope.TenantID,
		"policy_version":   p.Version(),
		"registry_version": p.RegistryVersion(),
		"rule_count":       p.RuleCount(),
		"settings":         p.Settings(),
	}, nil
}

// ---- recent_errors ----

type recentErrorsArgs struct {
	N int `json:"n"`
}

func (o MCPOptions) recentErrors(_ context.Context, raw json.RawMessage) (any, error) {
	if o.Errors == nil {
		return nil, errors.New("error ring not configured")
	}
	var args recentErrorsArgs
	_ = json.Unmarshal(raw, &args)
	n := args.N
	if n <= 0 {
		n = 20
	}
	if n > 200 {
		n = 200
	}
	rows := o.Errors.Recent(n)
	return map[string]any{
		"count":  len(rows),
		"errors": rows,
		"note":   "upstream failures since the last restart (in-memory). No prompt text is stored.",
	}, nil
}

// ---- provider_probe ----

type providerProbeArgs struct {
	ProviderID string `json:"provider_id"`
	Mode       string `json:"mode"`
}

// providerProbe makes a live call to a configured provider from the router's own
// network so an operator can tell reachability apart from a routing/config bug.
// Only configured providers are probed (base_url comes from the roster/registry,
// never from the caller) — no arbitrary-URL SSRF surface.
func (o MCPOptions) providerProbe(ctx context.Context, raw json.RawMessage) (any, error) {
	var args providerProbeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	id := strings.TrimSpace(args.ProviderID)
	if id == "" {
		return nil, errors.New("provide a provider_id (see get_roster)")
	}
	baseURL, keyEnv, ok := o.providerConn(id)
	if !ok {
		return nil, fmt.Errorf("unknown provider_id %q (see get_roster)", id)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("provider %q has no base_url", id)
	}

	client := o.ProbeClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	apiKey := strings.TrimSpace(os.Getenv(keyEnv))

	// Derive the exact endpoints a real request would hit (any base_url shape).
	chatURL, urlErr := provider.ChatCompletionsURL(baseURL)
	if urlErr != nil {
		return nil, fmt.Errorf("provider %q base_url invalid: %w", id, urlErr)
	}
	modelsURL := strings.TrimSuffix(chatURL, "/chat/completions") + "/models"

	mode := strings.TrimSpace(args.Mode)
	if mode == "" {
		mode = "models"
	}

	result := map[string]any{
		"provider_id": id,
		"base_url":    baseURL,
		"mode":        mode,
		"key_present": apiKey != "",
	}

	var (
		httpReq *http.Request
		err     error
	)
	switch mode {
	case "models":
		result["url"] = modelsURL
		httpReq, err = http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	case "chat":
		pmid := o.providerModelIDFor(id)
		if pmid == "" {
			return nil, fmt.Errorf("no routable model found for provider %q to probe with chat mode", id)
		}
		result["provider_model_id"] = pmid
		result["url"] = chatURL
		body, _ := json.Marshal(map[string]any{
			"model":      pmid,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			"max_tokens": 1,
			"stream":     false,
		})
		httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
		if httpReq != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}
	default:
		return nil, fmt.Errorf("mode must be \"models\" or \"chat\", got %q", mode)
	}
	if err != nil {
		return nil, fmt.Errorf("build probe request: %w", err)
	}
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	result["latency_ms"] = time.Since(start).Milliseconds()
	if err != nil {
		result["ok"] = false
		result["error"] = err.Error()
		result["diagnosis"] = "connection failed from the router — DNS, egress firewall, or the endpoint is unreachable from this host (not a routing/config bug)."
		return result, nil
	}
	defer resp.Body.Close()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
	ct := resp.Header.Get("Content-Type")
	result["status"] = resp.StatusCode
	result["content_type"] = ct
	result["server"] = resp.Header.Get("Server")
	result["ok"] = resp.StatusCode < 400
	result["body_snippet"] = string(bytes.TrimSpace(snippet))
	result["diagnosis"] = probeDiagnosis(resp.StatusCode, ct, resp.Header.Get("Server"))
	return result, nil
}

// probeDiagnosis turns a probe result into a plain-language hint.
func probeDiagnosis(status int, contentType, server string) string {
	htmlBody := strings.Contains(strings.ToLower(contentType), "html")
	switch {
	case status < 400:
		return "reachable and healthy from the router."
	case status == 401 || status == 403:
		return "reached the endpoint, but auth was rejected — check the API key env var."
	case status == 429:
		return "reached the endpoint, but it is rate-limiting the router."
	case status >= 500 && htmlBody && strings.Contains(strings.ToLower(server), "cloudflare"):
		return "a Cloudflare error page (not the model server) — the origin behind Cloudflare is down/unreachable, or Cloudflare is blocking this host's IP. Allowlist the router's egress IP or point at the origin directly."
	case status >= 500:
		return "the endpoint returned a server error — likely the model server is down/overloaded (e.g. OOM)."
	default:
		return "the endpoint returned a client error."
	}
}

// providerConn resolves a provider id to its base_url and key env, from the
// roster (source of truth) or the live registry snapshot.
func (o MCPOptions) providerConn(id string) (baseURL, keyEnv string, ok bool) {
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

// providerModelIDFor returns a provider-side model slug for the given provider,
// preferring an enabled roster model, for the chat probe mode.
func (o MCPOptions) providerModelIDFor(providerID string) string {
	if o.Roster != nil {
		if ms, err := o.Roster.LoadRosterModels(); err == nil {
			for _, m := range ms {
				if m.ProviderID == providerID && m.Enabled {
					return m.ProviderModelID
				}
			}
		}
	}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, m := range snap.ModelsForProvider(providerID) {
				if m.ProviderModelID != "" {
					return m.ProviderModelID
				}
			}
		}
	}
	return ""
}

// ---- incident_report ----

type incidentReportArgs struct {
	Window string `json:"window"`
}

func (o MCPOptions) incidentReport(_ context.Context, raw json.RawMessage) (any, error) {
	if o.History == nil {
		return nil, errors.New("incident report needs a data dir (ROUTER_DATA_DIR)")
	}
	var args incidentReportArgs
	_ = json.Unmarshal(raw, &args)
	rep := BuildIncidentReport(IncidentReportOptions{
		History: o.History, Engine: o.Engine, Cache: o.PolicyCache, Profile: o.Profile,
	}, strings.TrimSpace(args.Window), time.Now())
	return map[string]any{
		"report":   rep,
		"markdown": RenderIncidentMarkdown(rep),
		"note":     "counts and classifications only — no prompt content. Evidence for a report, not a legal attestation.",
	}, nil
}

// ---- outreach: create + measure (ISSUE-091) ----

type mintKeyArgs struct {
	Tenant  string   `json:"tenant"`
	Project string   `json:"project"`
	Role    string   `json:"role"`
	Scopes  []string `json:"scopes"`
}

func (o MCPOptions) mintKey(ctx context.Context, raw json.RawMessage) (any, error) {
	if o.KeyManager == nil || !o.KeyManager.Enabled() {
		return nil, errors.New("key provisioning disabled (set ROUTER_DATA_DIR)")
	}
	var args mintKeyArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Tenant) == "" {
		return nil, errors.New("tenant is required (use a unique tenant per prospect)")
	}
	plaintext, rec, err := o.KeyManager.Mint(apikeys.MintRequest{
		TenantID:  strings.TrimSpace(args.Tenant),
		ProjectID: strings.TrimSpace(args.Project),
		Role:      strings.TrimSpace(args.Role),
		Scopes:    args.Scopes,
		Actor:     "mcp-agent",
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"key_id":     rec.KeyID,
		"api_key":    plaintext, // shown ONCE — the agent must deliver it now
		"tenant":     rec.TenantID,
		"project":    rec.ProjectID,
		"created_at": rec.CreatedAt,
		"note":       "the api_key is shown once and never stored in plaintext — deliver it now.",
	}, nil
}

type keyIDArgs struct {
	KeyID string `json:"key_id"`
}

func (o MCPOptions) revokeKey(_ context.Context, raw json.RawMessage) (any, error) {
	if o.KeyManager == nil || !o.KeyManager.Enabled() {
		return nil, errors.New("key provisioning disabled (set ROUTER_DATA_DIR)")
	}
	var args keyIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	id := strings.TrimSpace(args.KeyID)
	if id == "" {
		return nil, errors.New("key_id is required")
	}
	ok, err := o.KeyManager.Revoke(id, "mcp-agent")
	if err != nil {
		return nil, err
	}
	return map[string]any{"key_id": id, "revoked": ok}, nil
}

func (o MCPOptions) keyUsage(_ context.Context, raw json.RawMessage) (any, error) {
	if o.KeyManager == nil || !o.KeyManager.Enabled() {
		return nil, errors.New("key provisioning disabled (set ROUTER_DATA_DIR)")
	}
	if o.History == nil {
		return nil, errors.New("usage requires the durable history (set ROUTER_DATA_DIR)")
	}
	var args keyIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	id := strings.TrimSpace(args.KeyID)
	if id == "" {
		return nil, errors.New("key_id is required")
	}
	keys, err := o.KeyManager.List()
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.KeyID != id {
			continue
		}
		usage := o.History.UsageForTenant(k.TenantID)
		return map[string]any{
			"found":         true,
			"key_id":        k.KeyID,
			"tenant":        k.TenantID,
			"project":       k.ProjectID,
			"revoked":       k.Revoked(),
			"created_at":    k.CreatedAt,
			"used":          usage.Requests > 0,
			"request_count": usage.Requests,
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"last_activity": usage.LastActivity,
			"note":          "usage is aggregated over the key's tenant — mint one tenant per prospect for a clean signal.",
		}, nil
	}
	return map[string]any{"found": false, "key_id": id}, nil
}

func (o MCPOptions) createDemoLink(ctx context.Context, raw json.RawMessage) (any, error) {
	if o.History == nil {
		return nil, errors.New("demo link requires the durable history (set ROUTER_DATA_DIR)")
	}
	var args struct {
		Label string `json:"label"`
	}
	_ = json.Unmarshal(raw, &args)
	tok, err := GenerateDemoShareToken()
	if err != nil {
		return nil, err
	}
	label := strings.TrimSpace(args.Label)
	if label != "" {
		// Named per-prospect link (ISSUE-089): opens are attributed to the label.
		if err := o.History.CreateDemoLink(tok, label, time.Now()); err != nil {
			return nil, err
		}
		audit.Record(ctx, o.Auditor, audit.Entry{Action: actionDemoAccess, Actor: "mcp-agent", Target: "named-link", Reason: "created: " + label})
	} else {
		if err := SetDemoShareToken(o.History, tok); err != nil {
			return nil, err
		}
		audit.Record(ctx, o.Auditor, audit.Entry{Action: actionDemoAccess, Actor: "mcp-agent", Target: "shared-link", Reason: "rotated"})
	}
	url := DemoShareURL(o.PublicURL, tok)
	out := map[string]any{"url": url, "enabled": true, "label": label}
	if strings.TrimSpace(o.PublicURL) == "" {
		out["note"] = "ROUTER_PUBLIC_URL is unset — url is relative; set it for a full shareable link."
	}
	return out, nil
}

func (o MCPOptions) demoLinkUsage(_ context.Context, _ json.RawMessage) (any, error) {
	if o.History == nil {
		return nil, errors.New("demo link usage requires the durable history (set ROUTER_DATA_DIR)")
	}
	links, err := o.History.ListDemoLinks()
	if err != nil {
		return nil, err
	}
	type linkOut struct {
		Label    string `json:"label"`
		Active   bool   `json:"active"`
		Opens    int64  `json:"opens"`
		LastOpen string `json:"last_open,omitempty"`
		Created  string `json:"created_at"`
	}
	named := make([]linkOut, 0, len(links))
	for _, l := range links {
		lo := linkOut{Label: l.Label, Active: l.Active(), Opens: l.Opens, Created: l.CreatedAt.Format(time.RFC3339)}
		if !l.LastOpenAt.IsZero() {
			lo.LastOpen = l.LastOpenAt.Format(time.RFC3339)
		}
		named = append(named, lo)
	}
	return map[string]any{
		"named_links": named,
		"legacy":      LoadDemoLinkUsage(o.History),
		"note":        "named links attribute opens per prospect; tokens are never returned here.",
	}, nil
}

// ---- server_info ----

func (o MCPOptions) serverInfo(_ context.Context, _ json.RawMessage) (any, error) {
	flags := map[string]any{
		// conservative mode can be toggled at runtime, so read it live from the engine.
		"conservative_mode":          false,
		"pricing_sync":               strings.ToLower(strings.TrimSpace(os.Getenv("ROUTER_PRICING_SYNC"))) != "false",
		"decision_cache_ttl_seconds": envInt("ROUTER_DECISION_CACHE_TTL_SECONDS", 60),
		"budget_usd":                 envInt("ROUTER_BUDGET_USD", 0),
		"data_dir_set":               strings.TrimSpace(os.Getenv("ROUTER_DATA_DIR")) != "",
	}
	if o.Engine != nil {
		flags["conservative_mode"] = o.Engine.Conservative()
	}
	rosterSource := "registry"
	if o.Roster != nil {
		rosterSource = "sqlite"
	}
	registryVersion, providers, enabledModels := o.Version, 0, 0
	if o.Engine != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			registryVersion = snap.RegistryVersion()
			providers = len(snap.Providers())
			enabledModels = len(snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}))
		}
	}
	return map[string]any{
		"name":             MCPName,
		"mcp_version":      MCPVersion,
		"build_commit":     buildinfo.FullCommit(),
		"build_time":       buildinfo.BuiltAt(),
		"registry_version": registryVersion,
		"roster_source":    rosterSource,
		"providers":        providers,
		"enabled_models":   enabledModels,
		"flags":            flags,
	}, nil
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ---- route_explain ----

type routeExplainArgs struct {
	Prompt   string           `json:"prompt"`
	Messages []openai.Message `json:"messages"`
}

type routeExplainResult struct {
	TaskType        string                 `json:"task_type"`
	TaskConfidence  float64                `json:"task_confidence"`
	Risk            string                 `json:"risk"`
	Sensitivity     string                 `json:"sensitivity"`
	Tier            string                 `json:"tier"`
	SelectedModel   string                 `json:"selected_model"`
	ProviderModelID string                 `json:"provider_model_id"`
	Provider        string                 `json:"provider"`
	Reasons         []string               `json:"reasons"`
	Fallbacks       []engine.FallbackEntry `json:"fallbacks"`
	EstCostUSD      float64                `json:"est_cost_usd"`
	PolicyVersion   string                 `json:"policy_version,omitempty"`
	// Signals is the classification detail — the "why did it read it this way"
	// that turns "it routed wrong" into "aha, it picked up the wrong signal".
	Signals     routeExplainSignals `json:"signals"`
	Blocked     bool                `json:"blocked,omitempty"`
	BlockReason string              `json:"block_reason,omitempty"`
}

type routeExplainSignals struct {
	PromptTokensEstimate int      `json:"prompt_tokens_estimate"`
	RequiresReasoning    bool     `json:"requires_reasoning"`
	RequiresCode         bool     `json:"requires_code"`
	RequiresToolUse      bool     `json:"requires_tool_use"`
	RequiresJSONSchema   bool     `json:"requires_json_schema"`
	RequiresLargeContext bool     `json:"requires_large_context"`
	RequiresVision       bool     `json:"requires_vision"`
	Conservative         bool     `json:"conservative"`
	Keywords             []string `json:"keywords,omitempty"`
	FilesTouched         []string `json:"files_touched,omitempty"`
}

func (o MCPOptions) routeExplain(_ context.Context, raw json.RawMessage) (any, error) {
	if o.Engine == nil {
		return nil, errors.New("routing engine not configured")
	}
	var args routeExplainArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	msgs := args.Messages
	if len(msgs) == 0 {
		if strings.TrimSpace(args.Prompt) == "" {
			return nil, errors.New("provide a prompt or messages")
		}
		msgs = []openai.Message{{Role: "user", Content: args.Prompt}}
	}
	req := &openai.ChatRequest{Messages: msgs}

	dec, job, err := explainRoute(o.Engine, o.PolicyCache, explainInput{
		RequestID: "mcp-route-explain",
		Request:   req,
	})
	res := routeExplainResult{
		TaskType:        string(job.TaskType),
		TaskConfidence:  job.TaskConfidence,
		Risk:            string(job.RiskLevel),
		Sensitivity:     string(job.Sensitivity),
		SelectedModel:   dec.SelectedModel,
		ProviderModelID: dec.ProviderModelID,
		Provider:        dec.SelectedProvider,
		Reasons:         dec.DecisionReasons,
		Fallbacks:       dec.Fallbacks,
		EstCostUSD:      dec.EstimatedCostUSD,
		PolicyVersion:   dec.PolicyVersion,
		Tier:            o.tierFor(dec.SelectedModel),
		Signals: routeExplainSignals{
			PromptTokensEstimate: job.PromptTokensEstimate,
			RequiresReasoning:    job.RequiresReasoning,
			RequiresCode:         job.RequiresCode,
			RequiresToolUse:      job.RequiresToolUse,
			RequiresJSONSchema:   job.RequiresJSONSchema,
			RequiresLargeContext: job.RequiresLargeContext,
			RequiresVision:       job.RequiresVision,
			Conservative:         job.Conservative,
			Keywords:             job.Keywords,
			FilesTouched:         job.FilesTouched,
		},
	}
	if err != nil {
		if errors.Is(err, engine.ErrBlocked) {
			res.Blocked = true
			if dec.BlockReason != "" {
				res.BlockReason = dec.BlockReason
			} else {
				res.BlockReason = err.Error()
			}
			return res, nil
		}
		return nil, err
	}
	return res, nil
}

// tierFor looks up the routing tier of a selected model from the live snapshot.
func (o MCPOptions) tierFor(modelID string) string {
	if o.Engine == nil || modelID == "" {
		return ""
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return ""
	}
	if m, ok := snap.Model(modelID); ok {
		return string(m.Tier)
	}
	return ""
}

// ---- savings_report ----

// savingsWindows are the windows savings_report accepts. They mirror
// incident_report's vocabulary so a reporting agent can ask both tools for the
// same period ("what happened, and what did it cost/save").
var savingsWindows = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"72h": 72 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

func (o MCPOptions) savingsReport(_ context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		Window string `json:"window"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &args)
	}
	window := strings.TrimSpace(strings.ToLower(args.Window))
	dur, windowed := savingsWindows[window]
	if !windowed {
		// Unknown or empty window keeps the historical all-time behaviour.
		window = "all"
	}

	d := buildDashboardData(o.Dashboard, "")
	out := map[string]any{
		"window":           window,
		"total_requests":   d.TotalRequests,
		"total_cost_usd":   d.TotalCostUSD,
		"savings":          d.Savings,
		"green":            d.Green,
		"route_by_model":   d.RoutesByModel,
		"registry_version": d.Version,
	}
	if !windowed {
		return out, nil
	}

	// Windowed: re-aggregate from the durable log and recompute the same
	// counterfactuals over that slice only. Requires the SQLite store; without
	// it we fall back to all-time rather than reporting a wrong number.
	store := o.Dashboard.History
	if store == nil {
		out["window"] = "all"
		out["window_note"] = "windowed savings need the durable request log; returned all-time instead"
		return out, nil
	}
	from := time.Now().Add(-dur)
	rows := store.ByModelSince(from)
	var reqs int64
	var cost float64
	for _, r := range rows {
		reqs += r.Requests
		cost += r.CostUSD
	}
	out["window_from"] = from.UTC().Format(time.RFC3339)
	out["total_requests"] = reqs
	out["total_cost_usd"] = cost
	out["route_by_model"] = rows
	out["savings"] = computeSavings(rows, cost, o.Dashboard.PremiumInputMicrosPerMTok,
		o.Dashboard.PremiumOutputMicrosPerMTok, o.Dashboard.PremiumBaselineModel)
	out["green"] = computeGreen(rows, o.Dashboard.EnergyWhPerMTokByModel,
		o.Dashboard.PremiumEnergyWhPerMTok, o.Dashboard.GridCO2eGramsPerKWh)
	return out, nil
}

// ---- recent_requests ----

type recentRequestsArgs struct {
	N          int    `json:"n"`
	TaskFilter string `json:"task_filter"`
}

func (o MCPOptions) recentRequests(_ context.Context, raw json.RawMessage) (any, error) {
	var args recentRequestsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	n := args.N
	if n <= 0 {
		n = 20
	}
	if n > 200 {
		n = 200
	}

	var rows []eventlog.RequestLogRecord
	switch {
	case o.History != nil:
		rows = o.History.Recent(n)
	case o.RequestLog != nil:
		rows = o.RequestLog.Recent(n)
	default:
		return nil, errors.New("no request log configured")
	}
	if f := strings.TrimSpace(args.TaskFilter); f != "" {
		filtered := rows[:0:0]
		for _, r := range rows {
			if r.TaskType == f {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	return map[string]any{"count": len(rows), "requests": rows}, nil
}

// ---- get_roster ----

type rosterModelOut struct {
	ID               string  `json:"id"`
	Tier             string  `json:"tier"`
	ProviderID       string  `json:"provider_id"`
	ProviderModelID  string  `json:"provider_model_id"`
	InputUSDPerMTok  float64 `json:"input_usd_per_mtok"`
	OutputUSDPerMTok float64 `json:"output_usd_per_mtok"`
	Enabled          bool    `json:"enabled"`
	ReasoningCapable bool    `json:"reasoning_capable"`
}

type rosterProviderOut struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	KeyEnv     string `json:"key_env,omitempty"`
	KeyPresent bool   `json:"key_present"`
}

func (o MCPOptions) getRoster(_ context.Context, _ json.RawMessage) (any, error) {
	// Prefer the SQLite roster (source of truth, ISSUE-073); fall back to the
	// live registry snapshot when no data dir is configured.
	if o.Roster != nil {
		models, err := o.Roster.LoadRosterModels()
		if err != nil {
			return nil, fmt.Errorf("load roster models: %w", err)
		}
		provs, err := o.Roster.LoadRosterProviders()
		if err != nil {
			return nil, fmt.Errorf("load roster providers: %w", err)
		}
		outModels := make([]rosterModelOut, 0, len(models))
		for _, m := range models {
			outModels = append(outModels, rosterModelOut{
				ID: m.ID, Tier: m.Tier, ProviderID: m.ProviderID,
				ProviderModelID: m.ProviderModelID,
				InputUSDPerMTok: m.InputUSDPerMTok, OutputUSDPerMTok: m.OutputUSDPerMTok,
				Enabled: m.Enabled, ReasoningCapable: m.ReasoningCapable,
			})
		}
		outProvs := make([]rosterProviderOut, 0, len(provs))
		for _, p := range provs {
			outProvs = append(outProvs, rosterProviderOut{
				ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, KeyEnv: p.KeyEnv,
				KeyPresent: p.KeyEnv != "" && strings.TrimSpace(os.Getenv(p.KeyEnv)) != "",
			})
		}
		sortRoster(outModels)
		return map[string]any{"source": "sqlite", "models": outModels, "providers": outProvs}, nil
	}
	return o.rosterFromSnapshot()
}

func (o MCPOptions) rosterFromSnapshot() (any, error) {
	if o.Engine == nil {
		return nil, errors.New("no roster or registry available")
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return nil, fmt.Errorf("registry snapshot: %w", err)
	}
	models := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	outModels := make([]rosterModelOut, 0, len(models))
	for _, m := range models {
		outModels = append(outModels, rosterModelOut{
			ID: m.ID, Tier: string(m.Tier), ProviderID: m.ProviderID,
			ProviderModelID:  m.ProviderModelID,
			InputUSDPerMTok:  float64(m.Cost.InputMicrosPerMillionToken) / 1e6,
			OutputUSDPerMTok: float64(m.Cost.OutputMicrosPerMillionToken) / 1e6,
			Enabled:          m.Enabled, ReasoningCapable: m.ReasoningCapable,
		})
	}
	outProvs := make([]rosterProviderOut, 0)
	for _, p := range snap.Providers() {
		outProvs = append(outProvs, rosterProviderOut{ID: p.ID, Name: p.Name, BaseURL: p.BaseURL})
	}
	sortRoster(outModels)
	return map[string]any{"source": "registry", "models": outModels, "providers": outProvs}, nil
}

// sortRoster orders models by tier (cheap, balanced, premium, then anything
// else) so the roster reads top-down like the Models page.
func sortRoster(models []rosterModelOut) {
	rank := map[string]int{"cheap": 0, "balanced": 1, "premium": 2}
	sort.SliceStable(models, func(i, j int) bool {
		ri, oki := rank[models[i].Tier]
		rj, okj := rank[models[j].Tier]
		if !oki {
			ri = 99
		}
		if !okj {
			rj = 99
		}
		if ri != rj {
			return ri < rj
		}
		return models[i].ID < models[j].ID
	})
}

// ---- provider_health ----

func (o MCPOptions) providerHealth(_ context.Context, _ json.RawMessage) (any, error) {
	if o.Health == nil {
		return nil, errors.New("provider health not tracked")
	}
	type row struct {
		Provider string  `json:"provider"`
		Score    float64 `json:"health_score"`
		Status   string  `json:"status"`
	}
	scores := o.Health.Providers()
	rows := make([]row, 0, len(scores))
	for id, score := range scores {
		rows = append(rows, row{Provider: id, Score: score, Status: healthStatus(score)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Provider < rows[j].Provider })
	return map[string]any{"count": len(rows), "providers": rows}, nil
}

func healthStatus(score float64) string {
	switch {
	case score >= 0.9:
		return "healthy"
	case score >= 0.5:
		return "degraded"
	default:
		return "down"
	}
}
