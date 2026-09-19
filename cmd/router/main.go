package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/budget"
	"github.com/magnusfroste/sluss/internal/council"
	"github.com/magnusfroste/sluss/internal/decisioncache"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/evals"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/experiment"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/outcomes"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/pricing"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/retention"
	"github.com/magnusfroste/sluss/internal/server"
	"github.com/magnusfroste/sluss/internal/spend"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("LOG_LEVEL")),
	}))

	// Security audit trail (ISSUE-044): structured logs plus an in-memory ring
	// buffer for in-process retrieval. Wired into the key store before any keys
	// are added so seed mutations are captured too. With a data dir, entries are
	// also appended to the tamper-evident hash chain (ISSUE-075) so the trail
	// can be exported and verified offline (cmd/audit-verify).
	var auditChain *audit.ChainSink
	if dir := strings.TrimSpace(os.Getenv("ROUTER_DATA_DIR")); dir != "" {
		chain, err := audit.NewChainSink(filepath.Join(dir, "audit-chain.jsonl"), logger)
		if err != nil {
			logger.Error("audit chain unavailable", "err", err)
		} else {
			auditChain = chain
			logger.Info("tamper-evident audit chain enabled", "file", chain.Path())
		}
	}
	auditMemory := audit.NewMemorySink(0)
	auditSinks := []audit.Sink{&audit.LogSink{Logger: logger}, auditMemory}
	if auditChain != nil {
		auditSinks = append(auditSinks, auditChain)
	}
	auditSink := audit.MultiSink(auditSinks...)

	keyStore := auth.NewInMemoryKeyStore()
	keyStore.SetAuditor(auditSink)
	if k := strings.TrimSpace(os.Getenv("LOCAL_API_KEY")); k != "" {
		keyStore.Add(k, &tenant.Tenant{
			ID:      "tn_local",
			Project: "prj_local",
			KeyID:   "key_local",
			Scopes:  auth.AllScopes(),
		})
		logger.Info("seeded local api key", "tenant", "tn_local")
	}

	mockURL := os.Getenv("MOCK_PROVIDER_URL")
	if mockURL == "" {
		mockURL = "http://localhost:18080"
	}

	mock := &provider.MockAdapter{
		BaseURL: mockURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}

	// Provider selection (ISSUE: OpenRouter integration). When OPENROUTER_API_KEY
	// is set, route through OpenRouter (a real OpenAI-compatible provider) via the
	// existing OpenAIAdapter; otherwise fall back to the in-process mock so the
	// build/dev path needs no credentials.
	var (
		snap            *registry.Snapshot
		primaryProvider provider.Adapter
		adapters        map[string]provider.Adapter
		snapErr         error
	)
	// Admin-editable roster (ISSUE-073): the roster — provider connections and
	// routable models (each carrying a tier, the router's USP) — is owned by the
	// durable SQLite store (history.db) so there is ONE source of truth. Keys stay
	// in env, referenced by name (never in the DB — the CISO posture). The store
	// is opened EARLY, before the registry is built, so the roster can seed the
	// registry Definition.
	//
	// On first run it seeds the built-in OpenRouter connection + three tier models
	// as data (nothing hardcoded); an existing models.json/providers.json roster
	// (ISSUE-072) is imported once and the files renamed ".imported". With no data
	// dir the registry falls back to the in-memory seed (current dev behaviour).
	dataDir := strings.TrimSpace(os.Getenv("ROUTER_DATA_DIR"))
	providersFile, modelsFile := "", ""
	var historyStore *history.Store
	if dataDir != "" {
		providersFile = filepath.Join(dataDir, "providers.json")
		modelsFile = filepath.Join(dataDir, "models.json")
		// Durable SQLite history + roster (ISSUE-070/073) — the v1.0 storage
		// decision (DECISION_LOG 2026-07-04). One file under ROUTER_DATA_DIR.
		historyPath := filepath.Join(dataDir, "history.db")
		if hs, err := history.Open(historyPath); err != nil {
			logger.Warn("could not open history db; roster falls back to in-memory seed", "err", err, "file", historyPath)
		} else {
			historyStore = hs
			logger.Info("durable history enabled", "file", historyPath)
		}
	}

	// Roster source of truth: the DB. Seed the built-ins (or migrate a prior
	// file-based roster) on an empty DB, then load it to build the registry.
	var customProviders []providercfg.Provider
	var customModels []providercfg.Model
	if historyStore != nil {
		if migrated, seeded, err := providercfg.MigrateAndSeedDB(historyStore, providersFile, modelsFile); err != nil {
			logger.Warn("could not prepare roster in sqlite", "err", err)
		} else {
			if migrated {
				logger.Info("imported models.json/providers.json roster into sqlite; renamed files .imported")
			}
			if seeded {
				logger.Info("seeded built-in roster (openrouter + three tiers) into sqlite")
			}
		}
		var lerr error
		if customProviders, lerr = historyStore.LoadRosterProviders(); lerr != nil {
			logger.Warn("could not load roster providers from sqlite", "err", lerr)
		}
		if customModels, lerr = historyStore.LoadRosterModels(); lerr != nil {
			logger.Warn("could not load roster models from sqlite", "err", lerr)
		}
	}

	if orKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); orKey != "" {
		// Per-tier model roster: swap which OpenRouter model fills each tier via
		// env (e.g. ROUTER_MODEL_PREMIUM=z-ai/glm-4.6), no code change needed.
		overrides := map[registry.Tier]string{
			registry.TierCheap:    strings.TrimSpace(os.Getenv("ROUTER_MODEL_CHEAP")),
			registry.TierBalanced: strings.TrimSpace(os.Getenv("ROUTER_MODEL_BALANCED")),
			registry.TierPremium:  strings.TrimSpace(os.Getenv("ROUTER_MODEL_PREMIUM")),
		}
		// ROUTER_MODEL_<TIER> is now a legacy convenience (edit the Models page
		// instead). Since the DB owns the roster, write any override through to it
		// so the persisted roster and routing agree — no UI/routing mismatch
		// (backlog-roster-override-deprecation). With no DB it still applies
		// in-memory via ApplyProviderModelOverrides below.
		if historyStore != nil {
			strOverrides := map[string]string{
				string(registry.TierCheap):    overrides[registry.TierCheap],
				string(registry.TierBalanced): overrides[registry.TierBalanced],
				string(registry.TierPremium):  overrides[registry.TierPremium],
			}
			if n, err := providercfg.ApplyTierOverridesToDB(historyStore, strOverrides); err != nil {
				logger.Warn("roster override write-through failed", "err", err)
			} else if n > 0 {
				logger.Info("ROUTER_MODEL_* override written through to sqlite roster (source of truth)", "models_updated", n)
				if refreshed, lerr := historyStore.LoadRosterModels(); lerr == nil {
					customModels = refreshed
				}
			}
		}
		// Build the registry entirely from config providers + models. With no data
		// dir, fall back to the in-memory seed so the router still runs. This makes
		// the built-in tiers plain data (re-tierable from the Models page).
		effProviders, effModels := customProviders, customModels
		if len(effProviders) == 0 {
			effProviders = providercfg.SeedProviders()
		}
		if len(effModels) == 0 {
			effModels = providercfg.SeedModels()
		}
		regProvs, regModels := providercfg.RegistryEntries(effProviders, effModels)
		def := registry.Definition{
			RegistryVersion: "registry-roster-2026-07-04",
			CreatedAt:       time.Now().UTC(),
			Providers:       regProvs,
			Models:          regModels,
		}
		def = registry.ApplyProviderModelOverrides(def, overrides)
		// Sync live per-model prices from OpenRouter's catalog (best-effort) so
		// estimates/savings/scoring stay honest without manual upkeep. Matches by
		// slug — also prices any roster-overridden OpenRouter slug. Disable with
		// ROUTER_PRICING_SYNC=false.
		if strings.ToLower(strings.TrimSpace(os.Getenv("ROUTER_PRICING_SYNC"))) != "false" {
			pctx, pcancel := context.WithTimeout(context.Background(), 10*time.Second)
			if prices, perr := pricing.FetchOpenRouter(pctx, "https://openrouter.ai/api/v1", orKey, &http.Client{Timeout: 10 * time.Second}); perr != nil {
				logger.Warn("pricing sync failed; using existing prices", "err", perr)
			} else {
				// Scope to OpenRouter's own models — custom providers keep their
				// admin-set (negotiated) prices even if a slug collides.
				logger.Info("synced pricing from OpenRouter", "models_updated", pricing.ApplyToDefinition(&def, prices, "openrouter"), "catalog_size", len(prices))
			}
			pcancel()
		}
		snap, snapErr = registry.NewSnapshot(def)
		for tier, slug := range overrides {
			if slug != "" {
				logger.Info("model roster override", "tier", string(tier), "model", slug)
			}
		}
		// The OpenRouter connection carries attribution headers; source its base
		// URL from the (seeded, admin-editable) connection when present.
		orBaseURL := "https://openrouter.ai/api/v1"
		for _, cp := range effProviders {
			if cp.ID == "openrouter" && cp.BaseURL != "" {
				orBaseURL = cp.BaseURL
			}
		}
		orAdapter := &provider.OpenAIAdapter{
			BaseURL: orBaseURL,
			APIKey:  orKey,
			// OpenRouter attribution (optional but recommended for app ranking).
			Referer:    envDefault("OPENROUTER_REFERER", "https://github.com/magnusfroste/sluss"),
			Title:      envDefault("OPENROUTER_TITLE", "tokenizer"),
			Client:     &http.Client{Timeout: 60 * time.Second},
			Timeout:    60 * time.Second,
			ProviderID: "openrouter",
			Logger:     logger,
		}
		primaryProvider = orAdapter
		adapters = map[string]provider.Adapter{"openrouter": orAdapter}
		// One OpenAI-compatible adapter per provider connection whose key is
		// present (openrouter handled above with its attribution headers).
		for _, cp := range effProviders {
			if cp.ID == "openrouter" {
				continue
			}
			if key := strings.TrimSpace(os.Getenv(cp.KeyEnv)); key != "" {
				adapters[cp.ID] = &provider.OpenAIAdapter{
					BaseURL: cp.BaseURL, APIKey: key,
					Client:     &http.Client{Timeout: 60 * time.Second},
					Timeout:    60 * time.Second,
					ProviderID: cp.ID,
					Logger:     logger,
				}
				logger.Info("custom provider enabled", "id", cp.ID, "base_url", cp.BaseURL)
			} else {
				logger.Warn("custom provider missing key; unroutable", "id", cp.ID, "key_env", cp.KeyEnv)
			}
		}
		logger.Info("using OpenRouter provider", "base_url", "https://openrouter.ai/api/v1")
	} else {
		// Build from the default definition so dev/mock can also carry provider
		// compliance tags (ROUTER_PROVIDER_TAGS="anthropic=local,eu-resident;
		// openai=cloud") — lets residency routing be demonstrated without real
		// providers (the CISO demo, ISSUE-082). Format: id=tag,tag;id2=tag.
		def := registry.DefaultDefinition()
		applyProviderTags(&def, os.Getenv("ROUTER_PROVIDER_TAGS"), logger)
		snap, snapErr = registry.NewSnapshot(def)
		primaryProvider = mock
		// In local dev the mock adapter serves all providers.
		adapters = map[string]provider.Adapter{
			"openai":    mock,
			"anthropic": mock,
		}
		logger.Info("using mock provider", "mock_provider", mockURL)
	}
	if snapErr != nil {
		logger.Error("failed to build registry", "err", snapErr)
		os.Exit(1)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		logger.Error("failed to create registry store", "err", err)
		os.Exit(1)
	}
	policyCache, err := loadRuntimePolicyCache(snap, os.Getenv("ROUTER_POLICY_PATH"))
	if err != nil {
		logger.Error("failed to build policy cache", "err", err)
		os.Exit(1)
	}
	policyCache.SetAuditor(auditSink)
	// No-YAML rule editing (ISSUE-093): a console ruleset that was live before
	// the restart is re-activated from the DB, so the CISO's rules survive a
	// redeploy. On failure the env-configured baseline stays in place.
	if historyStore != nil {
		if v, err := server.ApplyStoredConsolePolicy(policyCache, snap, historyStore); err != nil {
			logger.Warn("stored console policy failed to re-activate; baseline stays", "err", err)
		} else if v != "" {
			logger.Info("console policy re-activated from db", "policy_version", v)
		}
	}
	shadowPolicyCache, err := loadOptionalRuntimePolicyCache(snap, os.Getenv("ROUTER_SHADOW_POLICY_PATH"))
	if err != nil {
		logger.Error("failed to build shadow policy cache", "err", err)
		os.Exit(1)
	}
	// Live policy A/B (Fas 4): serve an experiment policy variant to a
	// deterministic share of traffic. Off unless a variant path is set.
	experimentPolicyCache, err := loadOptionalRuntimePolicyCache(snap, os.Getenv("ROUTER_EXPERIMENT_POLICY_PATH"))
	if err != nil {
		logger.Error("failed to build experiment policy cache", "err", err)
		os.Exit(1)
	}
	experimentCfg := experiment.Config{
		Percentage: parseIntEnv(os.Getenv("ROUTER_EXPERIMENT_PERCENTAGE"), 0),
		Salt:       strings.TrimSpace(os.Getenv("ROUTER_EXPERIMENT_SALT")),
	}
	if experimentPolicyCache != nil && experimentCfg.Enabled() {
		logger.Info("live policy experiment enabled", "treatment_percentage", experimentCfg.Percentage)
	}
	eng := engine.New(store)
	// Global conservative mode (ISSUE-060): incident lever that routes uncertain
	// classifications at a raised minimum tier. See docs/07-operations/runbook.md.
	if parseBoolEnv(os.Getenv("ROUTER_CONSERVATIVE_MODE")) {
		eng.SetConservative(true)
		logger.Info("global conservative mode enabled")
	}
	// Eval-driven scoring (borrowed from Kilo's "Efficient" tier): when an eval
	// report is provided, feed measured pass rates into scoring so routing prefers
	// the cheapest model measured good enough per task class. Default: static priors.
	if p := strings.TrimSpace(os.Getenv("ROUTER_MEASURED_QUALITY_REPORT")); p != "" {
		if rep, err := evals.LoadReport(p); err != nil {
			logger.Warn("measured-quality report load failed; using static quality priors", "path", p, "err", err)
		} else {
			mq := evals.NewMeasuredQuality(rep.Frontier, parseIntEnv(os.Getenv("ROUTER_MEASURED_QUALITY_MIN_SAMPLES"), 1))
			eng.Quality = mq
			logger.Info("measured-quality routing enabled", "path", p, "task_classes", mq.TaskClasses())
		}
	}

	// Bandit routing (Fas 4): learn per-(task, model) which model actually
	// delivers from realized outcomes and blend it into scoring via engine.Quality.
	// UCB1 is deterministic (no RNG), so the fast path stays reproducible. When on,
	// it supersedes the static measured-quality source. Off by default.
	var banditRouter *bandit.Bandit
	if parseBoolEnv(os.Getenv("ROUTER_BANDIT_ENABLED")) {
		banditRouter = bandit.New(parseFloatEnv(os.Getenv("ROUTER_BANDIT_EXPLORATION"), bandit.DefaultExploration))
		// Warm-start from the measured eval report when available so the bandit is
		// not cold before it gathers online data.
		if p := strings.TrimSpace(os.Getenv("ROUTER_MEASURED_QUALITY_REPORT")); p != "" {
			if rep, err := evals.LoadReport(p); err == nil {
				for _, tc := range rep.Frontier.TaskClasses {
					for _, m := range tc.Models {
						banditRouter.Seed(tc.TaskClass, m.ModelID, m.EvalSamples, m.EvalPassed)
					}
				}
			}
		}
		eng.Quality = banditRouter
		logger.Info("bandit routing enabled", "exploration", parseFloatEnv(os.Getenv("ROUTER_BANDIT_EXPLORATION"), bandit.DefaultExploration), "seeded_arms", banditRouter.Arms())
	}

	// Observability: health tracker (with circuit breaker), spend tracker, event
	// queue. The breaker opens a provider's circuit after a run of consecutive
	// failures and excludes it from routing for a cooldown, forcing fallback to a
	// healthy provider. Threshold 0 disables breaking (rolling-window only).
	cbThreshold := parseIntEnv(os.Getenv("ROUTER_CIRCUIT_BREAKER_THRESHOLD"), health.DefaultTripThreshold)
	cbCooldown := time.Duration(parseIntEnv(os.Getenv("ROUTER_CIRCUIT_BREAKER_COOLDOWN_SECONDS"), int(health.DefaultCooldown/time.Second))) * time.Second
	if strings.TrimSpace(os.Getenv("ROUTER_CIRCUIT_BREAKER_THRESHOLD")) == "0" {
		cbThreshold = 0
	}
	healthTracker := health.NewWithConfig(cbThreshold, cbCooldown)
	logger.Info("provider circuit breaker configured", "trip_threshold", cbThreshold, "cooldown", cbCooldown.String())
	spendTracker := spend.New()
	// Spend/savings persistence (ISSUE-067/070): with durable history (opened
	// early, above) the SQLite DB is the sink and the legacy spend.json is
	// imported once. Without history (no data dir or open failed) fall back to
	// periodic spend.json snapshots so savings still survive restarts.
	spendFile := ""
	if dataDir != "" {
		if historyStore != nil {
			legacySpend := filepath.Join(dataDir, "spend.json")
			if legacy, err := spend.LoadJSON(legacySpend); err == nil && len(legacy.Models) > 0 {
				if err := historyStore.ImportSpendSnapshot(legacy); err != nil {
					logger.Warn("legacy spend import failed", "err", err)
				} else if err := os.Rename(legacySpend, legacySpend+".imported"); err == nil {
					logger.Info("imported legacy spend.json into history", "renamed_to", legacySpend+".imported")
				}
			}
		} else {
			spendFile = filepath.Join(dataDir, "spend.json")
			if legacy, err := spend.LoadJSON(spendFile); err != nil {
				logger.Warn("could not load persisted spend", "err", err, "file", spendFile)
			} else {
				spendTracker.Restore(legacy)
				logger.Info("loaded persisted spend", "file", spendFile)
			}
		}
	}
	outcomeStore := outcomes.NewStore()
	eventQueue := eventlog.NewQueue(0)
	comparisonTracker := eventlog.NewComparisonTracker(0)
	requestLog := eventlog.NewRequestLogTracker(0)
	// In-memory ring of recent upstream failures for live debugging via the
	// recent_errors MCP tool (ISSUE-090).
	errorRing := eventlog.NewErrorRing(0)

	// Budget caps (ISSUE-051): a ledger accrues spend from the event queue and an
	// evaluator checks it on the request path. Caps are opt-in; ROUTER_BUDGET_USD
	// sets a default per-tenant cap for local dev.
	budgetCaps := budget.NewCaps()
	budgetLedger := budget.NewLedger()
	if usd := parseIntEnv(os.Getenv("ROUTER_BUDGET_USD"), 0); usd > 0 {
		budgetCaps.SetTenant("tn_local", budget.Cap{
			LimitMicroUSD: int64(usd) * 1_000_000,
			Action:        budget.ActionDowngrade,
		})
	}
	budgetEvaluator := budget.NewEvaluator(budgetCaps, budgetLedger)

	// Route decision cache (ISSUE-052): low-risk decisions reused within a short
	// TTL. Disabled when ROUTER_DECISION_CACHE_TTL_SECONDS is 0.
	decisionCache := decisioncache.New(
		time.Duration(parseIntEnv(os.Getenv("ROUTER_DECISION_CACHE_TTL_SECONDS"), 60))*time.Second,
		0,
	)

	// Build the fan-out event handler: logging + metrics + spend + budget ledger
	// + shadow comparison tracking.
	combinedHandler := buildEventHandler(logger, spendTracker, budgetLedger, comparisonTracker, requestLog, historyStore, errorRing)

	// Start the queue worker in the background.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go eventQueue.Run(workerCtx, combinedHandler, logger)

	// Periodically flush spend aggregates so savings survive a crash/redeploy.
	if spendFile != "" {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := spendTracker.SaveJSON(spendFile); err != nil {
						logger.Warn("spend flush failed", "err", err)
					}
				case <-workerCtx.Done():
					return
				}
			}
		}()
	}

	// Retention/privacy settings (ISSUE-045). Prompt logging is off unless
	// ROUTER_PROMPT_LOGGING is explicitly enabled.
	retentionSettings := retention.NewSettings(
		parseIntEnv(os.Getenv("ROUTER_RETENTION_DAYS"), retention.DefaultRetentionDays),
		parseBoolEnv(os.Getenv("ROUTER_PROMPT_LOGGING")),
	)
	if historyStore != nil {
		historyStore.StartRetention(workerCtx, retentionSettings.RetentionDays(""), 24*time.Hour, logger)
	}

	// Roster store for the admin pages (Models/Providers CRUD → SQLite). Kept as
	// an interface so a nil *history.Store doesn't become a non-nil interface;
	// nil here makes both pages read-only (no data dir).
	var rosterStore providercfg.RosterStore
	if historyStore != nil {
		rosterStore = historyStore
	}

	// API-key provisioning (ISSUE-079): with a data dir, DB-backed keys per
	// department. Load active keys into the (cache-fast) in-memory keystore at
	// boot, on top of the env bootstrap key so a fresh deploy is never locked out.
	var keyManager *apikeys.Manager
	if historyStore != nil {
		keyManager = apikeys.NewManager(historyStore, keyStore, auditSink)
		if n, err := keyManager.LoadIntoKeystore(); err != nil {
			logger.Warn("could not load db api keys", "err", err)
		} else if n > 0 {
			logger.Info("db-backed api keys loaded", "count", n)
		}
	}

	// Premium-tier pricing for the dashboard "saved vs all-premium" baseline.
	premInMicros, premOutMicros, premBaselineModel := premiumPricing(snap)
	// Green receipt: per-model energy estimates + grid intensity for the
	// dashboard's estimated energy/CO2e savings line.
	energyByModel, premiumWh := energyEstimates(snap)
	gridCO2e := float64(parseIntEnv(os.Getenv("ROUTER_GRID_CO2E_G_PER_KWH"), 400))

	// Council mode (Fusion-inspired): task classes that deliberate across a panel
	// of models on the slow path. Off unless ROUTER_COUNCIL_TASKS lists task types.
	councilTasks := council.ParseTasks(os.Getenv("ROUTER_COUNCIL_TASKS"))
	if len(councilTasks) > 0 {
		logger.Info("council mode enabled", "tasks", council.SortedTasks(councilTasks), "panel_size", parseIntEnv(os.Getenv("ROUTER_COUNCIL_SIZE"), 3))
	}

	handler := server.New(server.Config{
		Logger:                     logger,
		KeyStore:                   keyStore,
		Provider:                   primaryProvider,
		ContextPipelineEnabled:     parseBoolEnv(os.Getenv("ROUTER_CONTEXT_PIPELINE_ENABLED")),
		PromptAdapter:              buildPromptAdapter(parseBoolEnv(os.Getenv("ROUTER_PROMPT_ADAPTER_ENABLED"))),
		Engine:                     eng,
		Adapters:                   adapters,
		PolicyCache:                policyCache,
		PolicyBaselinePath:         strings.TrimSpace(os.Getenv("ROUTER_POLICY_PATH")),
		ShadowPolicyCache:          shadowPolicyCache,
		HealthTracker:              healthTracker,
		SpendTracker:               spendTracker,
		ComparisonTracker:          comparisonTracker,
		RequestLog:                 requestLog,
		History:                    historyStore,
		EventQueue:                 eventQueue,
		RegistryVersion:            snap.RegistryVersion(),
		OutcomeStore:               outcomeStore,
		Auditor:                    auditSink,
		Retention:                  retentionSettings,
		Budget:                     budgetEvaluator,
		DecisionCache:              decisionCache,
		EnergyWhPerMTokByModel:     energyByModel,
		PremiumEnergyWhPerMTok:     premiumWh,
		GridCO2eGramsPerKWh:        gridCO2e,
		PremiumInputMicrosPerMTok:  premInMicros,
		PremiumOutputMicrosPerMTok: premOutMicros,
		PremiumBaselineModel:       premBaselineModel,
		DashboardPassword:          strings.TrimSpace(os.Getenv("ROUTER_DASHBOARD_PASSWORD")),
		Roster:                     rosterStore,
		ErrorRing:                  errorRing,
		MCPEnabled:                 parseBoolEnv(os.Getenv("ROUTER_MCP_ENABLED")),
		MCPWriteEnabled:            parseBoolEnv(os.Getenv("ROUTER_MCP_WRITE_ENABLED")),
		CouncilTasks:               councilTasks,
		CouncilSize:                parseIntEnv(os.Getenv("ROUTER_COUNCIL_SIZE"), 3),
		ExperimentPolicyCache:      experimentPolicyCache,
		Experiment:                 experimentCfg,
		Bandit:                     banditRouter,
		AuditChain:                 auditChain,
		AuditMemory:                auditMemory,
		ConservativeMode:           parseBoolEnv(os.Getenv("ROUTER_CONSERVATIVE_MODE")),
		BudgetUSD:                  float64(parseIntEnv(os.Getenv("ROUTER_BUDGET_USD"), 0)),
		KeyManager:                 keyManager,
		Profile:                    strings.TrimSpace(os.Getenv("ROUTER_PROFILE")),
		PublicURL:                  strings.TrimSpace(os.Getenv("ROUTER_PUBLIC_URL")),
	})
	// Named-user console (accountability): seed an initial admin from env when
	// the user table is empty. The env dashboard password remains break-glass.
	if err := server.SeedAdminUser(historyStore,
		strings.TrimSpace(os.Getenv("ROUTER_ADMIN_USER")), os.Getenv("ROUTER_ADMIN_PASSWORD")); err != nil {
		logger.Warn("could not seed admin user", "err", err)
	} else if strings.TrimSpace(os.Getenv("ROUTER_ADMIN_USER")) != "" {
		logger.Info("named-user console enabled", "seed_user", os.Getenv("ROUTER_ADMIN_USER"))
	}
	if parseBoolEnv(os.Getenv("ROUTER_MCP_ENABLED")) {
		logger.Info("read-only MCP surface enabled", "path", "/mcp")
	}

	addr := os.Getenv("ROUTER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("router starting", "addr", addr, "mock_provider", mockURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	workerCancel() // drain event queue gracefully
	if historyStore != nil {
		if err := historyStore.Close(); err != nil {
			logger.Warn("history close failed", "err", err)
		}
	}
	if spendFile != "" {
		if err := spendTracker.SaveJSON(spendFile); err != nil {
			logger.Error("final spend flush failed", "err", err)
		} else {
			logger.Info("persisted spend on shutdown", "file", spendFile)
		}
	}
}

func loadRuntimePolicyCache(snapshot *registry.Snapshot, path string) (*policy.Cache, error) {
	return policy.NewRuntimeCache(snapshot, strings.TrimSpace(path))
}

func loadOptionalRuntimePolicyCache(snapshot *registry.Snapshot, path string) (*policy.Cache, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	return policy.NewRuntimeCache(snapshot, path)
}

func buildPromptAdapter(enabled bool) *provider.PromptAdapter {
	if !enabled {
		return nil
	}
	return &provider.PromptAdapter{
		Enabled: true,
		ModelProfiles: map[string]string{
			"cheap-general":     "cheap",
			"balanced-coder":    "balanced",
			"premium-reasoning": "premium",
		},
		Rules: []provider.PromptAdapterRule{
			{
				Name: "cheap-system-cost-aware",
				Match: provider.PromptAdapterMatch{
					Profiles: []string{"cheap"},
				},
				Mutation: provider.SystemPromptMutation{
					Suffix: "\n\nPrefer concise answers and avoid unnecessary reasoning traces.",
				},
			},
			{
				Name: "premium-system-depth",
				Match: provider.PromptAdapterMatch{
					Profiles: []string{"premium"},
				},
				Mutation: provider.SystemPromptMutation{
					Prefix: "Use careful, high-assurance reasoning for this task.\n\n",
				},
			},
		},
	}
}

func buildEventHandler(logger *slog.Logger, spendTracker *spend.Tracker, budgetLedger *budget.Ledger, comparisonTracker *eventlog.ComparisonTracker, requestLog *eventlog.RequestLogTracker, historyStore *history.Store, errorRing *eventlog.ErrorRing) eventlog.Handler {
	handlers := []eventlog.Handler{
		&eventlog.LoggingHandler{Logger: logger},
		spendTracker,
		budgetLedger,
	}
	if comparisonTracker != nil {
		handlers = append(handlers, comparisonTracker)
	}
	if requestLog != nil {
		handlers = append(handlers, requestLog)
	}
	if historyStore != nil {
		handlers = append(handlers, historyStore)
	}
	if errorRing != nil {
		handlers = append(handlers, errorRing)
	}
	return eventlog.MultiHandler(handlers...)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseBoolEnv(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// tierEnergyWhPerMTok holds order-of-magnitude energy estimates per million
// tokens by tier, derived from published per-query figures (a frontier-class
// response of ~1k tokens ≈ 0.3–3 Wh; small models roughly a tenth of that).
// Deliberately rough — the dashboard labels the result as an estimate; the
// point is the ratio between tiers, which mirrors the cost counterfactual.
var tierEnergyWhPerMTok = map[registry.Tier]float64{
	registry.TierCheap:    150,
	registry.TierBalanced: 700,
	registry.TierPremium:  1500,
}

// energyEstimates maps each enabled model to its tier's estimated Wh per
// million tokens and returns the premium baseline for the green receipt.
func energyEstimates(snap *registry.Snapshot) (map[string]float64, float64) {
	byModel := make(map[string]float64)
	for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}) {
		if wh, ok := tierEnergyWhPerMTok[m.Tier]; ok {
			byModel[m.ID] = wh
		}
	}
	return byModel, tierEnergyWhPerMTok[registry.TierPremium]
}

// premiumPricing returns the premium-tier model's input/output price (micros per
// million tokens) for the dashboard's all-premium savings baseline. Returns 0,0
// when no premium model exists in the snapshot.
func premiumPricing(snap *registry.Snapshot) (inMicros, outMicros int64, name string) {
	best := int64(-1)
	for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}) {
		if m.Tier != registry.TierPremium {
			continue
		}
		total := m.Cost.InputMicrosPerMillionToken + m.Cost.OutputMicrosPerMillionToken
		if total > best {
			best = total
			inMicros = m.Cost.InputMicrosPerMillionToken
			outMicros = m.Cost.OutputMicrosPerMillionToken
			if name = m.ProviderModelID; name == "" {
				name = m.ID
			}
		}
	}
	return inMicros, outMicros, name
}

func parseIntEnv(s string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
		return v
	}
	return fallback
}

// applyProviderTags overlays operator-set compliance tags onto the default
// registry providers from ROUTER_PROVIDER_TAGS ("id=tag,tag;id2=tag"). Used in
// dev/mock so residency routing can be demonstrated without real providers.
// Models inherit their provider's tags in NewSnapshot.
func applyProviderTags(def *registry.Definition, spec string, logger *slog.Logger) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return
	}
	tagsByProvider := map[string][]string{}
	for _, entry := range strings.Split(spec, ";") {
		entry = strings.TrimSpace(entry)
		id, rest, ok := strings.Cut(entry, "=")
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			continue
		}
		var tags []string
		for _, t := range strings.Split(rest, ",") {
			if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
				tags = append(tags, t)
			}
		}
		tagsByProvider[id] = tags
	}
	applied := 0
	for i := range def.Providers {
		if tags, ok := tagsByProvider[def.Providers[i].ID]; ok {
			def.Providers[i].ComplianceTags = tags
			applied++
		}
	}
	if applied > 0 {
		logger.Info("applied provider compliance tags (dev/mock)", "providers", applied)
	}
}

func parseFloatEnv(s string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil && v > 0 {
		return v
	}
	return fallback
}
