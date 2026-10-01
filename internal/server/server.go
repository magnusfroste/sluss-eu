// Package server wires the HTTP router. It exposes a single New() that
// returns an http.Handler with all middleware and routes registered.
package server

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/budget"
	"github.com/magnusfroste/sluss/internal/contextproc"
	"github.com/magnusfroste/sluss/internal/decisioncache"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/experiment"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/outcomes"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/regime"
	"github.com/magnusfroste/sluss/internal/retention"
	"github.com/magnusfroste/sluss/internal/spend"
)

type Config struct {
	Logger                 *slog.Logger
	KeyStore               auth.KeyStore
	Provider               provider.Adapter // legacy: used when Engine is nil
	ContextPipeline        *contextproc.Pipeline
	ContextPipelineEnabled bool
	PromptAdapter          *provider.PromptAdapter
	Readiness              []ReadyzChecker

	// Routing engine (Sprint 05). Optional — if nil, Provider is used directly.
	Engine   *engine.Engine
	Adapters map[string]provider.Adapter // provider ID → adapter
	// RosterReloader, when set, applies Models/Providers edits live
	// (ISSUE-115) and owns the live adapter set. Nil → edits need a restart.
	RosterReloader    *RosterReloader
	PolicyCache       *policy.Cache
	ShadowPolicyCache *policy.Cache
	// PolicyBaselinePath is the ROUTER_POLICY_PATH value (verbatim; "" = built-in
	// default) — the baseline the policy console's rollback returns to (ISSUE-093).
	PolicyBaselinePath string

	// Observability (Sprint 06). All optional.
	HealthTracker     *health.Tracker
	SpendTracker      *spend.Tracker
	EventQueue        *eventlog.Queue
	ComparisonTracker *eventlog.ComparisonTracker
	RequestLog        *eventlog.RequestLogTracker
	History           *history.Store // durable SQLite history (ISSUE-070); optional
	RegistryVersion   string         // shown on dashboard

	// Feedback (Sprint 07). Optional.
	OutcomeStore *outcomes.Store

	// Security (Sprint 08). Optional audit trail for blocked requests and
	// control-plane changes (ISSUE-044).
	Auditor audit.Sink

	// Retention/privacy settings (ISSUE-045). Optional; gates prompt logging.
	Retention *retention.Settings

	// Budget caps (ISSUE-051). Optional; blocks or downgrades over-budget scopes.
	Budget *budget.Evaluator

	// Route decision cache (ISSUE-052). Optional; caches low-risk decisions.
	DecisionCache *decisioncache.Cache

	// Premium-tier per-token pricing (micros per million tokens) for the
	// dashboard "saved vs all-premium" baseline. Optional.
	PremiumInputMicrosPerMTok  int64
	PremiumOutputMicrosPerMTok int64
	PremiumBaselineModel       string

	// Green receipt (estimated energy/CO2e saved). Optional; zero disables.
	EnergyWhPerMTokByModel map[string]float64
	PremiumEnergyWhPerMTok float64
	GridCO2eGramsPerKWh    float64

	// DashboardPassword, when set, gates the dashboard behind browser-friendly
	// HTTP Basic Auth instead of the Bearer + admin-role gate. Lets you open the
	// dashboard URL directly and log in.
	DashboardPassword string

	// Roster, when set (a data dir is configured), is the SQLite-backed source of
	// truth for the roster — provider connections and routable models (ISSUE-073).
	// It enables CRUD on the Models and Providers pages, persisted to history.db.
	// Nil (no data dir) → both pages are read-only over the live registry.
	Roster providercfg.RosterStore

	// ErrorRing, when set, is the in-memory ring of recent upstream failures
	// surfaced by the recent_errors MCP tool for live debugging.
	ErrorRing *eventlog.ErrorRing

	// MCPEnabled mounts the read-only MCP surface at POST /mcp (ISSUE-074), gated
	// behind the same Bearer API key or dashboard password as the rest. Off by
	// default; enable with ROUTER_MCP_ENABLED. Requires Engine.
	MCPEnabled bool

	// MCPWriteEnabled additionally exposes the mutating MCP tools (mint_key,
	// revoke_key, create_demo_link) so an agent can drive prospect outreach.
	// Off by default (ROUTER_MCP_WRITE_ENABLED) — the surface stays read-only
	// unless opted in. Still admin-gated and audited.
	MCPWriteEnabled bool

	// Council mode (Fusion-inspired). CouncilTasks is the set of task classes that
	// deliberate across a panel of the top-N routed models (non-streaming only)
	// and return the consensus answer; empty disables it. CouncilSize caps the
	// panel (default 3). Slow path only.
	CouncilTasks map[string]bool
	CouncilSize  int

	// Live policy A/B (Fas 4). ExperimentPolicyCache holds the variant policy;
	// Experiment configures the traffic split. When enabled, a deterministic share
	// of traffic (bucketed by tenant+project) is served the variant and tagged with
	// its arm. Disabled by default.
	ExperimentPolicyCache *policy.Cache
	Experiment            experiment.Config

	// Bandit (Fas 4), when set, receives per-(task, model) realized rewards from
	// the request path so the engine can learn online. Optional.
	Bandit *bandit.Bandit

	// AuditChain, when set (a data dir is configured), is the tamper-evident
	// hash-chained audit log (ISSUE-075). Mounts GET /router/audit/export behind
	// the admin gate so the chain can be handed to an auditor and verified
	// offline with cmd/audit-verify.
	AuditChain *audit.ChainSink

	// Compliance report inputs (ISSUE-076). AuditMemory is the bounded in-memory
	// audit window used for event breakdowns; ConservativeMode/BudgetUSD are the
	// env-derived control facts shown under Kontrollstatus.
	AuditMemory      *audit.MemorySink
	ConservativeMode bool
	BudgetUSD        float64

	// KeyManager, when set (a data dir is configured), provisions DB-backed API
	// keys per department (ISSUE-079). Mounts the admin-gated /router/keys API.
	KeyManager *apikeys.Manager

	// PublicURL (ROUTER_PUBLIC_URL), when set, is the site's public base URL used
	// for the landing page's canonical/OpenGraph tags and the sitemap. Empty is
	// fine — links degrade to relative and canonical is omitted.
	PublicURL string

	// Profile (ROUTER_PROFILE) is the explicit regime profile key (nis2 | dora |
	// gdpr | eu) flavouring the control report and public label. Empty derives it
	// from the active policy pack (ISSUE-094).
	Profile string
}

func New(cfg Config) http.Handler {
	mux := http.NewServeMux()

	// Public landing page + SEO/AEO surface at the bare root. Everything
	// operational stays behind the dashboard gate; this is the only crawlable
	// page. favicon/robots/sitemap/llms are public too.
	// Regime profile (ISSUE-094): explicit ROUTER_PROFILE wins, else derived from
	// the active policy pack — flavours the public label and the control report.
	activePolicyVersion := ""
	if cfg.PolicyCache != nil {
		if p, ok := cfg.PolicyCache.Active(policy.Scope{}); ok {
			activePolicyVersion = p.Version()
		}
	}
	prof := regime.Resolve(cfg.Profile, activePolicyVersion)
	landingOpts := LandingOptions{PublicURL: cfg.PublicURL, ProfileLabel: prof.LandingLbl}
	mux.HandleFunc("GET /{$}", LandingHandler(landingOpts))
	// Public "Connect" catalog — how to point clients/agents/IDEs at the router.
	mux.HandleFunc("GET /connect", ConnectHandler(landingOpts))
	mux.HandleFunc("GET /favicon.svg", FaviconHandler())
	mux.HandleFunc("GET /favicon.ico", FaviconICOHandler())
	mux.HandleFunc("GET /robots.txt", RobotsHandler(landingOpts))
	mux.HandleFunc("GET /sitemap.xml", SitemapHandler(landingOpts))
	mux.HandleFunc("GET /llms.txt", LLMSHandler(landingOpts))

	mux.HandleFunc("GET /healthz", HealthzHandler())
	mux.HandleFunc("GET /readyz", ReadyzHandler(cfg.Readiness...))

	// Prometheus metrics.
	mux.Handle("GET /metrics", promhttp.Handler())

	chat := ChatCompletionsHandler(cfg.Provider, ChatOptions{
		ContextPipeline:        cfg.ContextPipeline,
		ContextPipelineEnabled: cfg.ContextPipelineEnabled,
		PromptAdapter:          cfg.PromptAdapter,
		Logger:                 cfg.Logger,
		Engine:                 cfg.Engine,
		Adapters:               cfg.Adapters,
		AdapterSource:          cfg.adapterSource(),
		PolicyCache:            cfg.PolicyCache,
		ShadowPolicyCache:      cfg.ShadowPolicyCache,
		HealthTracker:          cfg.HealthTracker,
		EventQueue:             cfg.EventQueue,
		Auditor:                cfg.Auditor,
		Retention:              cfg.Retention,
		Budget:                 cfg.Budget,
		DecisionCache:          cfg.DecisionCache,
		RegistryVersion:        cfg.RegistryVersion,
		CouncilTasks:           cfg.CouncilTasks,
		CouncilSize:            cfg.CouncilSize,
		ExperimentPolicyCache:  cfg.ExperimentPolicyCache,
		Experiment:             cfg.Experiment,
		Bandit:                 cfg.Bandit,
	})
	mux.Handle("POST /v1/chat/completions",
		auth.Middleware(cfg.KeyStore)(auth.RequireScope(auth.ScopeChatCompletions)(chat)))

	// Anthropic Messages API shim (POST /v1/messages): translates to/from the
	// OpenAI-shaped path so Anthropic-native clients (Claude Code, Codex, the
	// Anthropic SDKs) can point at the router too. Same auth/scope as chat.
	mux.Handle("POST /v1/messages",
		auth.Middleware(cfg.KeyStore)(auth.RequireScope(auth.ScopeChatCompletions)(AnthropicMessagesHandler(chat))))

	// OpenAI-compatible model discovery. Requires a valid key (like OpenAI) but
	// no granular scope — listing models is read-only and benign.
	if cfg.Engine != nil {
		mux.Handle("GET /v1/models",
			auth.Middleware(cfg.KeyStore)(AnthropicModelsNegotiator(cfg.Engine, ModelsHandler(cfg.Engine))))
	}

	if cfg.Engine != nil {
		decision := DecisionHandler(DecisionOptions{
			Engine:      cfg.Engine,
			PolicyCache: cfg.PolicyCache,
			Logger:      cfg.Logger,
		})
		mux.Handle("POST /router/decision",
			auth.Middleware(cfg.KeyStore)(auth.RequireScope(auth.ScopeRouterDecision)(decision)))
	}

	// Outcome feedback API (ISSUE-039).
	if cfg.OutcomeStore != nil {
		outcome := OutcomeHandler(OutcomeOptions{Store: cfg.OutcomeStore, Logger: cfg.Logger})
		mux.Handle("POST /router/outcomes",
			auth.Middleware(cfg.KeyStore)(auth.RequireScope(auth.ScopeRouterOutcomes)(outcome)))
	}

	// Dashboard (no auth — read-only aggregated stats).
	htmlH, dataH := DashboardHandler(DashboardOptions{
		Spend:                      cfg.SpendTracker,
		Health:                     cfg.HealthTracker,
		Outcomes:                   cfg.OutcomeStore,
		Comparisons:                cfg.ComparisonTracker,
		RequestLog:                 cfg.RequestLog,
		History:                    cfg.History,
		Logger:                     cfg.Logger,
		Version:                    cfg.RegistryVersion,
		PremiumInputMicrosPerMTok:  cfg.PremiumInputMicrosPerMTok,
		PremiumOutputMicrosPerMTok: cfg.PremiumOutputMicrosPerMTok,
		PremiumBaselineModel:       cfg.PremiumBaselineModel,
		EnergyWhPerMTokByModel:     cfg.EnergyWhPerMTokByModel,
		PremiumEnergyWhPerMTok:     cfg.PremiumEnergyWhPerMTok,
		GridCO2eGramsPerKWh:        cfg.GridCO2eGramsPerKWh,
		Engine:                     cfg.Engine,
		AuditMemory:                cfg.AuditMemory,
		Bandit:                     cfg.Bandit,
		ConservativeMode:           cfg.ConservativeMode,
	})
	// Admin console gate. With a data dir (History) we use named-user sessions
	// (login page + break-glass env password + Bearer admin), so console actions
	// are attributable to a person in the audit trail. Without a data dir, keep
	// the legacy Basic Auth / Bearer gate.
	var dashGuard func(http.HandlerFunc) http.Handler
	// demoGuard protects the live demo chat. It admits the least-privileged
	// demo-link role in addition to admins; only the /chat routes use it.
	var demoGuard func(http.HandlerFunc) http.Handler
	if cfg.History != nil {
		adminAuth := &AdminAuth{Sessions: NewSessionStore(cfg.History), Store: cfg.History,
			BreakGlass: cfg.DashboardPassword, KeyStore: cfg.KeyStore}
		mux.HandleFunc("GET /router/login", adminAuth.LoginHandler())
		mux.HandleFunc("POST /router/login", adminAuth.LoginHandler())
		mux.HandleFunc("POST /router/logout", adminAuth.LogoutHandler())
		mux.HandleFunc("GET /router/logout", adminAuth.LogoutHandler())
		dashGuard = adminAuth.Require
		demoGuard = adminAuth.RequireDemo
		// Shareable CISO demo link: /demo?token=<token> starts a demo-only session.
		mux.HandleFunc("GET /demo", DemoEntryHandler(adminAuth, cfg.History, cfg.Auditor))
		// Named-user admin page (audited actor = the logged-in user).
		mux.Handle("GET /router/users", dashGuard(UsersPageHandler(cfg.History, cfg.Auditor)))
		mux.Handle("POST /router/users", dashGuard(UsersAddHandler(cfg.History, cfg.Auditor)))
		mux.Handle("POST /router/users/disable", dashGuard(UsersDisableHandler(cfg.History, cfg.Auditor)))
	} else {
		dashGuard = func(h http.HandlerFunc) http.Handler {
			if cfg.DashboardPassword != "" {
				return dashboardBasicAuth(cfg.DashboardPassword, http.HandlerFunc(h))
			}
			return auth.Middleware(cfg.KeyStore)(auth.RequireRole(auth.RoleAdmin)(http.HandlerFunc(h)))
		}
		demoGuard = dashGuard
	}
	mux.Handle("GET /router/dashboard", dashGuard(htmlH))
	mux.Handle("GET /router/dashboard/data", dashGuard(dataH))

	// Policy console (ISSUE-093): view the active policy + dry-run a prompt
	// (which rule fires, model/tier/egress or a fail-closed block). No provider
	// call. The "firewall for data egress" the CISO sees and tests.
	if cfg.Engine != nil {
		mux.Handle("GET /router/policy", dashGuard(PolicyPageHandler(PolicyPageOptions{
			Engine: cfg.Engine, Cache: cfg.PolicyCache, Version: cfg.RegistryVersion,
			Retention: cfg.Retention, History: cfg.History, ResetAvailable: cfg.History != nil,
		})))
		mux.Handle("POST /router/policy/dryrun", dashGuard(PolicyDryRunHandler(cfg.Engine, cfg.PolicyCache)))
		// No-YAML rule editing (ISSUE-093 pt 1): firewall-style rules authored in
		// the console, hot-activated (audited), with rollback to the baseline.
		if cfg.History != nil && cfg.PolicyCache != nil {
			ruleOpts := PolicyRulesOptions{
				Engine: cfg.Engine, Cache: cfg.PolicyCache, History: cfg.History,
				Auditor: cfg.Auditor, BaselinePath: cfg.PolicyBaselinePath,
			}
			mux.Handle("POST /router/policy/rules", dashGuard(PolicyRuleAddHandler(ruleOpts)))
			mux.Handle("POST /router/policy/rules/delete", dashGuard(PolicyRuleDeleteHandler(ruleOpts)))
			mux.Handle("POST /router/policy/activate", dashGuard(PolicyActivateHandler(ruleOpts)))
			mux.Handle("POST /router/policy/rollback", dashGuard(PolicyRollbackHandler(ruleOpts)))
		}
		// Audited demo-data reset (ISSUE-096): clears operational data only; the
		// reset itself is recorded in the audit chain.
		if cfg.History != nil {
			mux.Handle("POST /router/data/reset", dashGuard(DataResetHandler(DataResetOptions{
				History: cfg.History, Spend: cfg.SpendTracker, RequestLog: cfg.RequestLog,
				Comparisons: cfg.ComparisonTracker, Errors: cfg.ErrorRing, Auditor: cfg.Auditor,
			})))
			// Audited per-tenant erasure (GDPR art. 17).
			mux.Handle("POST /router/data/erase-tenant", dashGuard(TenantEraseHandler(TenantEraseOptions{
				History: cfg.History, Spend: cfg.SpendTracker, KeyManager: cfg.KeyManager, Auditor: cfg.Auditor,
			})))
		}
	}

	// Compliance report (ISSUE-076): readable control report (markdown or
	// ?format=json) assembled from existing sources only. Admin-gated.
	chainPath := ""
	if cfg.AuditChain != nil {
		chainPath = cfg.AuditChain.Path()
	}
	// Shadow-AI gap report (ISSUE-095): monitor-mode numbers — what the shadow
	// compliance pack WOULD have done with sensitive prompts. The CISO hook.
	mux.Handle("GET /router/gap-report", dashGuard(GapReportHandler(GapReportOptions{
		Comparisons: cfg.ComparisonTracker,
		Engine:      cfg.Engine,
		ShadowCache: cfg.ShadowPolicyCache,
		Profile:     cfg.Profile,
	})))

	// Incident evidence pack (ISSUE-095 follow-up): windowed blocked/sensitive
	// traffic evidence for the NIS2 24h/72h reporting deadlines.
	if cfg.History != nil {
		mux.Handle("GET /router/incident-report", dashGuard(IncidentReportHandler(IncidentReportOptions{
			History: cfg.History, Engine: cfg.Engine, Cache: cfg.PolicyCache, Profile: cfg.Profile,
		})))
	}

	mux.Handle("GET /router/compliance/report", dashGuard(ComplianceReportHandler(ReportOptions{
		Profile: cfg.Profile,
		Dashboard: DashboardOptions{
			Spend: cfg.SpendTracker, Health: cfg.HealthTracker, Outcomes: cfg.OutcomeStore,
			History: cfg.History, Version: cfg.RegistryVersion,
			PremiumInputMicrosPerMTok:  cfg.PremiumInputMicrosPerMTok,
			PremiumOutputMicrosPerMTok: cfg.PremiumOutputMicrosPerMTok,
			PremiumBaselineModel:       cfg.PremiumBaselineModel,
			EnergyWhPerMTokByModel:     cfg.EnergyWhPerMTokByModel,
			PremiumEnergyWhPerMTok:     cfg.PremiumEnergyWhPerMTok,
			GridCO2eGramsPerKWh:        cfg.GridCO2eGramsPerKWh,
		},
		Engine:           cfg.Engine,
		PolicyCache:      cfg.PolicyCache,
		Health:           cfg.HealthTracker,
		AuditMemory:      cfg.AuditMemory,
		ConservativeMode: cfg.ConservativeMode,
		BudgetUSD:        cfg.BudgetUSD,
		AuditChainPath:   chainPath,
	})))

	// API-key provisioning (ISSUE-079): admin-gated mint/list/revoke of
	// DB-backed keys per department. Available only with a data dir (KeyManager).
	if cfg.KeyManager != nil && cfg.KeyManager.Enabled() {
		// Admin page (ISSUE-080) + JSON API (ISSUE-079). The page lives at
		// /router/keys; the JSON list moves to /router/keys/data (dashboard
		// convention). Browser forms POST to /router/keys and /router/keys/revoke
		// (no DELETE verb / JSON body from a form); the programmatic JSON mint
		// (POST /router/keys/mint) and DELETE revoke stay for API clients.
		mux.Handle("GET /router/keys", dashGuard(KeysPageHandler(cfg.KeyManager)))
		mux.Handle("GET /router/keys/data", dashGuard(KeysListHandler(cfg.KeyManager)))
		mux.Handle("POST /router/keys", dashGuard(KeysMintFormHandler(cfg.KeyManager)))
		mux.Handle("POST /router/keys/revoke", dashGuard(KeysRevokeFormHandler(cfg.KeyManager)))
		mux.Handle("POST /router/keys/mint", dashGuard(KeysMintHandler(cfg.KeyManager)))
		mux.Handle("DELETE /router/keys/{key_id}", dashGuard(KeysRevokeHandler(cfg.KeyManager)))
	}

	// Tamper-evident audit-chain export (ISSUE-075): streams the hash-chained
	// JSONL so an auditor can verify it offline (cmd/audit-verify). Admin-gated
	// like the rest of the control plane; entries carry no prompt text.
	if cfg.AuditChain != nil {
		mux.Handle("GET /router/audit/export", dashGuard(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("Content-Disposition", `attachment; filename="audit-chain.jsonl"`)
			if err := cfg.AuditChain.ExportTo(w); err != nil && cfg.Logger != nil {
				cfg.Logger.Error("audit chain export failed", "err", err)
			}
		}))
	}

	// Full request log on its own page (the dashboard keeps stats/insights).
	logH := LogPageHandler(LogOptions{History: cfg.History, RequestLog: cfg.RequestLog, Logger: cfg.Logger, Version: cfg.RegistryVersion, Engine: cfg.Engine, Cache: cfg.PolicyCache})
	mux.Handle("GET /router/log", dashGuard(logH))

	// Models admin page (ISSUE-072): the primary roster — routable models with
	// tier/price/enabled. With a models config path it gets CRUD (add/edit/
	// re-tier/delete), persisted to the data volume; changes apply on restart.
	if cfg.Engine != nil {
		modelOpts := ModelsOptions{Engine: cfg.Engine, Logger: cfg.Logger,
			Version: cfg.RegistryVersion, Roster: cfg.Roster, Reloader: cfg.RosterReloader}
		mux.Handle("GET /router/models", dashGuard(ModelsPageHandler(modelOpts)))
		// Per-row live test (ISSUE-098): works read-only too (registry fallback),
		// so it is registered regardless of roster CRUD.
		mux.Handle("POST /router/models/test", dashGuard(ModelTestHandler(modelOpts)))
		if cfg.Roster != nil {
			mux.Handle("POST /router/models", dashGuard(ModelsAddHandler(modelOpts)))
			mux.Handle("POST /router/models/delete", dashGuard(ModelsDeleteHandler(modelOpts)))
		}
	}

	// Providers admin page: reusable connections (base_url + key_env), key status
	// (by env var name, never the value) and health. With a config path it also
	// gets CRUD (add/delete), persisted to the data volume.
	if cfg.Engine != nil {
		provOpts := ProvidersOptions{Engine: cfg.Engine, Health: cfg.HealthTracker, Logger: cfg.Logger,
			Version: cfg.RegistryVersion, Roster: cfg.Roster,
			Cache: cfg.PolicyCache, Auditor: cfg.Auditor, Reloader: cfg.RosterReloader}
		mux.Handle("GET /router/providers", dashGuard(ProvidersPageHandler(provOpts)))
		if cfg.Roster != nil {
			mux.Handle("POST /router/providers", dashGuard(ProvidersAddHandler(provOpts)))
			mux.Handle("POST /router/providers/delete", dashGuard(ProvidersDeleteHandler(provOpts)))
			// Risk register (ISSUE-093 pt 4): curated compliance tags per provider.
			mux.Handle("POST /router/providers/tags", dashGuard(ProvidersTagsHandler(provOpts)))
		}
	}

	// "Seeing is believing" demo (needs the routing engine + a browser-friendly
	// gate). The page and its send endpoint sit behind the same guard as the
	// dashboard; the send endpoint injects a demo tenant so no bearer key is
	// exposed in the browser, then delegates to the real chat handler.
	if cfg.Engine != nil {
		demoChat := demoTenantInjector(chat)
		mux.Handle("GET /chat", demoGuard(DemoChatPageHandler()))
		mux.Handle("POST /chat/send", demoGuard(demoChat.ServeHTTP))
		// Demo chat sessions persist server-side in SQLite (when a data dir is
		// set) so they survive restarts/redeploys; the browser falls back to
		// localStorage otherwise.
		getS, putS := DemoSessionsHandlers(cfg.History)
		mux.Handle("GET /chat/sessions", demoGuard(getS))
		mux.Handle("PUT /chat/sessions", demoGuard(putS))

		// Admin-managed demo quick prompts (the chips): served to the chat page
		// and CRUD'd from the Demo-prompts admin page. Seeded in the DB on boot.
		SeedQuickPrompts(cfg.History)
		mux.Handle("GET /chat/quickprompts", demoGuard(QuickPromptsHandler(cfg.History)))
		// Model picker data (ISSUE-098): admin-gated — the picker is a control-
		// panel feature; demo-link guests always route with Auto.
		mux.Handle("GET /chat/models", dashGuard(ChatModelsHandler(cfg.Engine)))
		mux.Handle("GET /router/prompts", dashGuard(PromptsPageHandler(cfg.History, cfg.PublicURL)))
		mux.Handle("POST /router/prompts", dashGuard(PromptsAddHandler(cfg.History)))
		mux.Handle("POST /router/prompts/delete", dashGuard(PromptsDeleteHandler(cfg.History)))
		mux.Handle("POST /router/prompts/reset", dashGuard(PromptsResetHandler(cfg.History)))
		mux.Handle("POST /router/prompts/sharelink", dashGuard(PromptsShareLinkHandler(cfg.History, cfg.Auditor)))
		// Named per-prospect demo links (ISSUE-089).
		mux.Handle("POST /router/demolinks", dashGuard(DemoLinksCreateHandler(cfg.History, cfg.Auditor)))
		mux.Handle("POST /router/demolinks/disable", dashGuard(DemoLinksDisableHandler(cfg.History, cfg.Auditor)))
	}

	// Read-only MCP surface (ISSUE-074): agents introspect routing decisions and
	// query spend/roster/health conversationally. Gated behind the same auth as
	// the API; off unless explicitly enabled. Never touches the fast path.
	if cfg.MCPEnabled && cfg.Engine != nil {
		mcpSrv := NewMCPServer(MCPOptions{
			Engine:      cfg.Engine,
			PolicyCache: cfg.PolicyCache,
			Dashboard: DashboardOptions{
				Spend:                      cfg.SpendTracker,
				Health:                     cfg.HealthTracker,
				Outcomes:                   cfg.OutcomeStore,
				Comparisons:                cfg.ComparisonTracker,
				RequestLog:                 cfg.RequestLog,
				History:                    cfg.History,
				Logger:                     cfg.Logger,
				Version:                    cfg.RegistryVersion,
				PremiumInputMicrosPerMTok:  cfg.PremiumInputMicrosPerMTok,
				PremiumOutputMicrosPerMTok: cfg.PremiumOutputMicrosPerMTok,
				PremiumBaselineModel:       cfg.PremiumBaselineModel,
				EnergyWhPerMTokByModel:     cfg.EnergyWhPerMTokByModel,
				PremiumEnergyWhPerMTok:     cfg.PremiumEnergyWhPerMTok,
				GridCO2eGramsPerKWh:        cfg.GridCO2eGramsPerKWh,
			},
			History:      cfg.History,
			RequestLog:   cfg.RequestLog,
			Health:       cfg.HealthTracker,
			Roster:       cfg.Roster,
			Errors:       cfg.ErrorRing,
			KeyManager:   cfg.KeyManager,
			Auditor:      cfg.Auditor,
			PublicURL:    cfg.PublicURL,
			Profile:      cfg.Profile,
			WriteEnabled: cfg.MCPWriteEnabled,
			Version:      cfg.RegistryVersion,
		})
		mux.Handle("/mcp", mcpAuth(cfg.KeyStore, cfg.DashboardPassword)(mcpSrv.HTTPHandler()))
	}

	return middleware.RequestID(middleware.Logger(cfg.Logger)(mux))
}

// adapterSource returns the live adapter accessor when roster hot-reload is
// wired (ISSUE-115); nil keeps the static Adapters map.
func (cfg Config) adapterSource() func() map[string]provider.Adapter {
	if cfg.RosterReloader == nil {
		return nil
	}
	return cfg.RosterReloader.Adapters
}
