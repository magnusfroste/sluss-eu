package engine_test

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// makeStore builds a registry store from DefaultDefinition and panics on error.
func makeStore(t *testing.T) *registry.Store {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func makeEngine(t *testing.T) *engine.Engine {
	t.Helper()
	return engine.New(makeStore(t))
}

func simpleJob(task router.TaskType, risk router.RiskLevel) *router.JobDescriptor {
	return &router.JobDescriptor{
		RequestID:               "req_test",
		TaskType:                task,
		RiskLevel:               risk,
		Sensitivity:             router.SensitivityNone,
		PromptTokensEstimate:    200,
		MaxOutputTokensEstimate: 400,
		LatencyPreference:       router.LatencyBalanced,
		QualityPreference:       router.QualityBalanced,
		BudgetPreference:        router.BudgetNormal,
		RouterMode:              router.RouterModeAuto,
	}
}

// --- FilterCandidates ---

func TestFilterCandidates_AllEnabledForSimpleTask(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	res := engine.FilterCandidates(job, policy.Route{}, snap, engine.FullyHealthy, false)
	if len(res.Candidates) == 0 {
		t.Fatal("expected at least one candidate for simple chat")
	}
}

func TestFilterCandidates_UnhealthyProviderExcluded(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	health := engine.StaticHealth{"openai": 0.0, "anthropic": 1.0}
	res := engine.FilterCandidates(job, policy.Route{}, snap, health, false)
	for _, m := range res.Candidates {
		if m.ProviderID == "openai" {
			t.Fatalf("provider openai should be excluded (health 0.0), but model %s passed", m.ID)
		}
	}
}

func TestFilterCandidates_SecurityReviewRequiresPremium(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSecurityReview, router.RiskHigh)
	res := engine.FilterCandidates(job, policy.Route{}, snap, engine.FullyHealthy, false)
	for _, m := range res.Candidates {
		if m.Tier != registry.TierPremium {
			t.Errorf("model %s (tier=%s) should be excluded for security_review", m.ID, m.Tier)
		}
	}
}

func TestFilterCandidates_StreamingExcludesNonStreamingModels(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	res := engine.FilterCandidates(job, policy.Route{}, snap, engine.FullyHealthy, true)
	for _, m := range res.Candidates {
		if !m.Capabilities.Streaming {
			t.Errorf("model %s lacks streaming but passed the streaming filter", m.ID)
		}
	}
}

func TestFilterCandidates_PolicyDeniedProvider(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	constraints := &policy.Constraints{DeniedProviders: []string{"openai"}}
	route := policy.Route{Constraints: constraints}
	res := engine.FilterCandidates(job, route, snap, engine.FullyHealthy, false)
	for _, m := range res.Candidates {
		if m.ProviderID == "openai" {
			t.Fatalf("openai model %s should be excluded by policy denied_providers", m.ID)
		}
	}
}

func TestFilterCandidates_PolicyAllowedModels(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	constraints := &policy.Constraints{AllowedModels: []string{"cheap-general"}}
	route := policy.Route{Constraints: constraints}
	res := engine.FilterCandidates(job, route, snap, engine.FullyHealthy, false)
	if len(res.Candidates) != 1 || res.Candidates[0].ID != "cheap-general" {
		t.Fatalf("expected only cheap-general, got %d candidates", len(res.Candidates))
	}
}

func TestFilterCandidates_RouterModeCheapExcludesPremium(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	job.RouterMode = router.RouterModeCheap
	res := engine.FilterCandidates(job, policy.Route{}, snap, engine.FullyHealthy, false)
	for _, m := range res.Candidates {
		if m.Tier == registry.TierPremium {
			t.Errorf("premium model %s should be excluded with router_mode=cheap", m.ID)
		}
	}
}

// --- ScoreCandidates ---

func TestScoreCandidates_SortedDescending(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskHardCodeDebugging, router.RiskMedium)
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	for i := 1; i < len(scored); i++ {
		if scored[i].Score > scored[i-1].Score {
			t.Fatalf("scores not sorted descending at index %d: %.4f > %.4f", i, scored[i].Score, scored[i-1].Score)
		}
	}
}

func TestScoreCandidates_HighRiskPrefersPremium(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskUnknownHighRisk, router.RiskHigh)
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	if len(scored) == 0 {
		t.Skip("no scored candidates")
	}
	if scored[0].Model.Tier != registry.TierPremium {
		t.Errorf("expected premium model as top pick for unknown_high_risk, got tier=%s model=%s",
			scored[0].Model.Tier, scored[0].Model.ID)
	}
}

func TestScoreCandidates_RouterModeCheapPrefersCheap(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSummarization, router.RiskLow)
	job.RouterMode = router.RouterModeCheap
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	if len(scored) == 0 {
		t.Skip("no scored candidates")
	}
	if scored[0].Model.Tier == registry.TierPremium {
		t.Errorf("cheap mode should not pick premium as top model, got %s", scored[0].Model.ID)
	}
}

func TestScoreCandidates_UnhealthyProviderScoresLower(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	// Make anthropic (premium-reasoning) unhealthy.
	health := engine.StaticHealth{"anthropic": 0.2}
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, health, engine.DefaultWeights())
	for i, sc := range scored {
		if sc.Model.ProviderID == "anthropic" && len(scored) > 1 {
			// unhealthy provider should not be top pick when alternatives exist
			if i == 0 {
				t.Errorf("unhealthy anthropic model %s should not be top pick", sc.Model.ID)
			}
		}
	}
}

// --- BuildFallbackChain ---

func TestBuildFallbackChain_MaxTwoFallbacks(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	if len(scored) == 0 {
		t.Skip("no scored candidates")
	}
	primary := scored[0].Model
	fallbacks := engine.BuildFallbackChain(primary, scored, job, policy.Route{})
	if len(fallbacks) > 2 {
		t.Errorf("expected max 2 fallbacks, got %d", len(fallbacks))
	}
}

func TestBuildFallbackChain_HighRiskNoDowngrade(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskUnknownHighRisk, router.RiskHigh)
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	if len(scored) == 0 {
		t.Skip("no scored candidates")
	}
	primary := scored[0].Model
	fallbacks := engine.BuildFallbackChain(primary, scored, job, policy.Route{})
	for _, fb := range fallbacks {
		m, ok := snap.Model(fb.ModelID)
		if !ok {
			t.Fatalf("fallback model %s not in registry", fb.ModelID)
		}
		if engine.TierOrdinal(m.Tier) < engine.TierOrdinal(primary.Tier) {
			t.Errorf("fallback %s (tier=%s) is lower than primary %s (tier=%s) for high-risk task",
				fb.ModelID, m.Tier, primary.ID, primary.Tier)
		}
	}
}

func TestBuildFallbackChain_PrimaryNotInFallbacks(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	minTier := engine.MinimumTierForTask(job, policy.Route{})
	candidates := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	scored := engine.ScoreCandidates(candidates, job, minTier, engine.FullyHealthy, engine.DefaultWeights())
	if len(scored) == 0 {
		t.Skip("no scored candidates")
	}
	primary := scored[0].Model
	fallbacks := engine.BuildFallbackChain(primary, scored, job, policy.Route{})
	for _, fb := range fallbacks {
		if fb.ModelID == primary.ID {
			t.Errorf("primary %s should not appear in its own fallback chain", primary.ID)
		}
	}
}

// --- Engine.Decide ---

func TestDecide_SimpleChat(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.SelectedModel == "" {
		t.Fatal("expected non-empty SelectedModel")
	}
	if dec.SelectedProvider == "" {
		t.Fatal("expected non-empty SelectedProvider")
	}
}

func TestDecide_SecurityReviewSelectsPremium(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSecurityReview, router.RiskHigh)
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	snap, _ := registry.DefaultSnapshot()
	m, ok := snap.Model(dec.SelectedModel)
	if !ok {
		t.Fatalf("selected model %s not in registry", dec.SelectedModel)
	}
	if m.Tier != registry.TierPremium {
		t.Errorf("security_review should route to premium, got tier=%s model=%s", m.Tier, m.ID)
	}
	if !dec.RequiresVerifier {
		t.Error("security_review should require verifier")
	}
}

func TestDecide_BlockedByPolicy(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	job.TenantID = "blocked-tenant"

	snap, _ := registry.DefaultSnapshot()
	store, _ := registry.NewStore(snap)
	pol := mustCompilePolicy(t, &policy.Policy{
		Version: "v1",
		Rules: []policy.Rule{
			{
				ID: "block-tenant",
				When: policy.When{
					Tenant: &policy.EnumMatch{Values: []string{"blocked-tenant"}},
				},
				Route: policy.Route{
					Block: &policy.Block{Code: "tenant_blocked", Reason: "tenant not allowed", Status: 403},
				},
			},
		},
	}, store)

	_, err := eng.Decide(job, pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestDecide_ExplicitModelPinned(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	model := "cheap-general"
	job.ExplicitModel = &model
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.SelectedModel != "cheap-general" {
		t.Errorf("expected cheap-general, got %s", dec.SelectedModel)
	}
}

// policyWithRoute compiles a match-all policy carrying the given route so tests
// can exercise constraints through the full Decide path.
func policyWithRoute(t *testing.T, route policy.Route) *policy.CompiledPolicy {
	t.Helper()
	store := makeStore(t)
	p := &policy.Policy{
		Version: "test-constraints",
		Rules:   []policy.Rule{{ID: "all", When: policy.When{Empty: true}, Route: route}},
	}
	return mustCompilePolicy(t, p, store)
}

func explicitJob(model string) *router.JobDescriptor {
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	job.ExplicitModel = &model
	return job
}

func assertBlocked(t *testing.T, dec engine.RouteDecision, err error) {
	t.Helper()
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got err=%v decision=%+v", err, dec)
	}
	if !dec.Blocked || dec.BlockStatus != 403 {
		t.Fatalf("expected blocked 403 decision, got %+v", dec)
	}
	if dec.SelectedModel != "" {
		t.Fatalf("blocked decision must not select a model, got %q", dec.SelectedModel)
	}
}

// An explicit client model on the project denylist must be blocked — an override
// can never bypass a denylist.
func TestDecide_ExplicitModelDeniedByModelList(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{Constraints: &policy.Constraints{DeniedModels: []string{"cheap-general"}}})
	dec, err := eng.Decide(explicitJob("cheap-general"), pol, engine.FullyHealthy, false)
	assertBlocked(t, dec, err)
}

// An explicit client model whose provider is denied must be blocked.
func TestDecide_ExplicitModelDeniedByProviderList(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{Constraints: &policy.Constraints{DeniedProviders: []string{"openai"}}})
	dec, err := eng.Decide(explicitJob("cheap-general"), pol, engine.FullyHealthy, false) // cheap-general is an openai model
	assertBlocked(t, dec, err)
}

// An explicit client model outside the allowlist must be blocked.
func TestDecide_ExplicitModelNotInAllowlist(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{Constraints: &policy.Constraints{AllowedModels: []string{"premium-reasoning"}}})
	dec, err := eng.Decide(explicitJob("cheap-general"), pol, engine.FullyHealthy, false)
	assertBlocked(t, dec, err)
}

// An explicit client model that satisfies the allowlist is honored.
func TestDecide_ExplicitModelAllowedPassesThrough(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{Constraints: &policy.Constraints{AllowedModels: []string{"premium-reasoning"}}})
	dec, err := eng.Decide(explicitJob("premium-reasoning"), pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.Blocked || dec.SelectedModel != "premium-reasoning" {
		t.Fatalf("expected premium-reasoning honored, got %+v", dec)
	}
}

// router_mode=disabled must not become a way to bypass a denylist.
func TestDecide_DisabledModeDeniedModelBlocked(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{Constraints: &policy.Constraints{DeniedModels: []string{"cheap-general"}}})
	job := explicitJob("cheap-general")
	job.RouterMode = router.RouterModeDisabled
	dec, err := eng.Decide(job, pol, engine.FullyHealthy, false)
	assertBlocked(t, dec, err)
}

// Even a policy force.model cannot select a denied model — deny is the floor.
func TestDecide_ForceModelCannotBypassDeny(t *testing.T) {
	eng := makeEngine(t)
	pol := policyWithRoute(t, policy.Route{
		Force:       &policy.Force{Model: "cheap-general"},
		Constraints: &policy.Constraints{DeniedModels: []string{"cheap-general"}},
	})
	dec, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false)
	assertBlocked(t, dec, err)
}

func TestDecide_DisabledModeWithoutExplicitModel(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	job.RouterMode = router.RouterModeDisabled
	_, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestDecide_FallbacksPresent(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	// The default registry has 3 models; after selecting primary there should be fallbacks.
	if len(dec.Fallbacks) == 0 {
		t.Error("expected at least one fallback entry")
	}
}

func TestDecide_TimeoutDefaulted(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.TimeoutMS <= 0 {
		t.Errorf("expected positive timeout, got %d", dec.TimeoutMS)
	}
}

func TestDecide_PolicyVersionPropagated(t *testing.T) {
	eng := makeEngine(t)
	job := simpleJob(router.TaskSimpleChat, router.RiskLow)

	snap, _ := registry.DefaultSnapshot()
	store, _ := registry.NewStore(snap)
	pol := mustCompilePolicy(t, &policy.Policy{Version: "test-v42"}, store)

	dec, err := eng.Decide(job, pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.PolicyVersion != "test-v42" {
		t.Errorf("expected policy version test-v42, got %q", dec.PolicyVersion)
	}
}

func mustCompilePolicy(t *testing.T, p *policy.Policy, store *registry.Store) *policy.CompiledPolicy {
	t.Helper()
	snap, err := store.Active()
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := policy.Compile(p, snap)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}
