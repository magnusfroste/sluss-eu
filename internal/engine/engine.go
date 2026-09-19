package engine

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// conservativeConfidenceThreshold is the task-classification confidence below
// which conservative mode treats a request as uncertain (ISSUE-060).
const conservativeConfidenceThreshold = 0.5

// residencyBlockCode is the block code for a request that could not route to any
// provider satisfying the policy's data-residency/compliance tag requirements
// (ISSUE-084). A first-class, audited 403 block — not a generic no_route.
const residencyBlockCode = "residency_no_compliant_provider"

var (
	ErrNoRoute       = errors.New("engine: no route found")
	ErrBlocked       = errors.New("engine: request blocked by policy")
	ErrDisabled      = errors.New("engine: routing disabled")
	ErrModelNotFound = errors.New("engine: model not found in registry")
)

// Engine is the stateless routing decision engine.
// It always reads the active snapshot from the store so hot-reloads are transparent.
type Engine struct {
	Registry *registry.Store
	Weights  Weights
	// conservative is a global safety lever (ISSUE-060). When set, uncertain
	// (low-confidence) classifications are routed at a raised minimum tier so an
	// incident never silently downgrades ambiguous traffic to cheap models.
	conservative atomic.Bool
	// Quality is an optional measured quality source (evals/outcomes). When set,
	// it overrides the registry's static quality priors in scoring so routing
	// prefers the cheapest model measured "good enough" per task class. Nil =
	// static priors (default). Set once at startup.
	Quality QualitySource
}

// New creates an Engine backed by the given registry store, using default weights.
func New(store *registry.Store) *Engine {
	return &Engine{
		Registry: store,
		Weights:  DefaultWeights(),
	}
}

// SetConservative toggles global conservative mode at runtime (e.g. during an
// incident). Safe for concurrent use.
func (e *Engine) SetConservative(on bool) { e.conservative.Store(on) }

// Conservative reports whether global conservative mode is on.
func (e *Engine) Conservative() bool { return e.conservative.Load() }

// conservativeForJob reports whether this request should be routed cautiously:
// conservative mode is on and the task classification is uncertain (unknown or
// below the confidence threshold).
func (e *Engine) conservativeForJob(job *router.JobDescriptor) bool {
	if !e.Conservative() {
		return false
	}
	if job.TaskType == router.TaskUnknownHighRisk {
		return true
	}
	return job.TaskConfidence > 0 && job.TaskConfidence < conservativeConfidenceThreshold
}

// Decide computes a RouteDecision for the given job.
//
//   - pol may be nil (no policy constraints applied).
//   - health may be nil (all providers assumed fully healthy).
//   - streaming indicates whether the upstream request will use server-sent events.
//
// Decide never calls any provider, LLM, or external service.
func (e *Engine) Decide(
	job *router.JobDescriptor,
	pol *policy.CompiledPolicy,
	health HealthSnapshot,
	streaming bool,
) (RouteDecision, error) {
	snap, err := e.Registry.Active()
	if err != nil {
		return RouteDecision{}, fmt.Errorf("engine: registry unavailable: %w", err)
	}
	if health == nil {
		health = FullyHealthy
	}

	// RouterMode=disabled skips routing entirely.
	if job.RouterMode == router.RouterModeDisabled {
		return decideDisabled(job, pol, snap)
	}

	// Conservative mode (ISSUE-060): mark uncertain requests for cautious
	// routing before filtering/scoring read the flag off the job.
	job.Conservative = e.conservativeForJob(job)

	// Evaluate compiled policy.
	eval := evaluatePolicy(pol, job)
	if eval.Blocked {
		b := eval.Route.Block
		dec := RouteDecision{
			Blocked:         true,
			BlockReason:     b.Reason,
			DecisionReasons: eval.Explanations,
			PolicyVersion:   eval.PolicyVersion,
			BlockStatus:     403,
		}
		if b.Code != "" {
			dec.BlockCode = b.Code
		}
		if b.Status != 0 {
			dec.BlockStatus = b.Status
		}
		return dec, ErrBlocked
	}

	// Explicit model in request (client override).
	if job.ExplicitModel != nil {
		return decidePinned(*job.ExplicitModel, "explicit model in request", job, eval, snap)
	}

	// Policy force.model pins the model.
	if eval.Route.Force != nil && eval.Route.Force.Model != "" {
		return decidePinned(eval.Route.Force.Model, "policy force.model", job, eval, snap)
	}

	// Filter candidates.
	filterRes := FilterCandidates(job, eval.Route, snap, health, streaming)
	if len(filterRes.Candidates) == 0 {
		// A residency/compliance requirement that leaves no compliant provider is
		// a first-class, audited policy block (403) — not a generic gateway error
		// — so the audit chain, blocked count, and compliance report reflect it
		// (ISSUE-084). Any other empty result stays a no_route.
		if filterRes.ResidencyExcluded {
			reason := "no provider satisfies the required data-residency/compliance tags"
			return RouteDecision{
				Blocked:         true,
				BlockCode:       residencyBlockCode,
				BlockReason:     reason,
				BlockStatus:     403,
				DecisionReasons: append(append(eval.Explanations, reason), filterRes.ResidencyReasons...),
				PolicyVersion:   eval.PolicyVersion,
			}, ErrBlocked
		}
		return RouteDecision{}, fmt.Errorf("%w: no models pass filters (task=%s risk=%s streaming=%v excluded=%d)",
			ErrNoRoute, job.TaskType, job.RiskLevel, streaming, len(filterRes.Excluded))
	}

	// Score and rank.
	minTier := MinimumTierForTask(job, eval.Route)
	scored := ScoreCandidates(filterRes.Candidates, job, minTier, health, e.Weights, e.Quality)
	primary := scored[0].Model

	// Build fallback chain.
	fallbacks := BuildFallbackChain(primary, scored, job, eval.Route)

	// Cost estimate.
	costUSD := estimateCostMicroUSD(primary, job) / 1_000_000

	return RouteDecision{
		RouteID:          routeID(job, primary.ID),
		SelectedModel:    primary.ID,
		SelectedProvider: primary.ProviderID,
		ProviderModelID:  primary.ProviderModelID,
		Fallbacks:        fallbacks,
		TimeoutMS:        resolveTimeout(pol, eval.Route, job),
		RequiresVerifier: resolveVerifier(job, eval.Route),
		DecisionReasons:  buildReasons(job, eval, filterRes, scored, primary, fallbacks),
		PolicyVersion:    eval.PolicyVersion,
		EstimatedCostUSD: costUSD,
	}, nil
}

func decideDisabled(job *router.JobDescriptor, pol *policy.CompiledPolicy, snap *registry.Snapshot) (RouteDecision, error) {
	if job.ExplicitModel == nil {
		return RouteDecision{}, fmt.Errorf("%w: router_mode=disabled requires an explicit model field", ErrDisabled)
	}
	eval := evaluatePolicy(pol, job)

	// Disabled mode uses the client model as-is, but the project's allow/deny
	// lists still apply — disabling routing must not become a way to bypass a
	// denylist. Resolve the model when known (to check its provider); otherwise
	// enforce the model-name lists against the raw reference.
	pinned := registry.Model{ID: *job.ExplicitModel, ProviderModelID: *job.ExplicitModel}
	if m, ok := snap.Model(*job.ExplicitModel); ok {
		pinned = m
	}
	if reason, _ := providerModelConstraintReason(pinned, eval.Route.Constraints); reason != "" {
		return RouteDecision{
			Blocked:         true,
			BlockCode:       "provider_not_allowed",
			BlockReason:     fmt.Sprintf("router_mode=disabled model %q rejected: %s", *job.ExplicitModel, reason),
			BlockStatus:     403,
			DecisionReasons: append(eval.Explanations, fmt.Sprintf("router_mode=disabled model %q blocked: %s", *job.ExplicitModel, reason)),
			PolicyVersion:   eval.PolicyVersion,
		}, ErrBlocked
	}

	return RouteDecision{
		RouteID:         routeID(job, *job.ExplicitModel),
		SelectedModel:   *job.ExplicitModel,
		ProviderModelID: *job.ExplicitModel,
		DecisionReasons: append(eval.Explanations, "routing disabled, using client model as-is"),
		PolicyVersion:   eval.PolicyVersion,
	}, nil
}

func decidePinned(
	modelRef string,
	source string,
	job *router.JobDescriptor,
	eval policy.Evaluation,
	snap *registry.Snapshot,
) (RouteDecision, error) {
	model, ok := snap.Model(modelRef)
	if !ok {
		// Fall back to searching by ProviderModelID.
		found := findByProviderModelID(snap, modelRef)
		if found == nil {
			return RouteDecision{}, fmt.Errorf("%w: %s %q not found in registry", ErrModelNotFound, source, modelRef)
		}
		model = *found
	}

	// A pinned model (explicit client model, policy force.model, or disabled
	// mode) must still satisfy the project's provider/model allow and deny
	// lists. An override can never bypass a denylist — deny is the security
	// floor, so a violating pin is blocked rather than honored.
	if reason, residency := providerModelConstraintReason(model, eval.Route.Constraints); reason != "" {
		code := "provider_not_allowed"
		if residency {
			code = residencyBlockCode
		}
		return RouteDecision{
			Blocked:         true,
			BlockCode:       code,
			BlockReason:     fmt.Sprintf("%s %q rejected: %s", source, modelRef, reason),
			BlockStatus:     403,
			DecisionReasons: append(eval.Explanations, fmt.Sprintf("%s %q blocked: %s", source, modelRef, reason)),
			PolicyVersion:   eval.PolicyVersion,
		}, ErrBlocked
	}

	return RouteDecision{
		RouteID:          routeID(job, model.ID),
		SelectedModel:    model.ID,
		SelectedProvider: model.ProviderID,
		ProviderModelID:  model.ProviderModelID,
		DecisionReasons:  append(eval.Explanations, fmt.Sprintf("%s: %q", source, modelRef)),
		PolicyVersion:    eval.PolicyVersion,
	}, nil
}

func findByProviderModelID(snap *registry.Snapshot, providerModelID string) *registry.Model {
	for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}) {
		if m.ProviderModelID == providerModelID {
			return &m
		}
	}
	return nil
}

func evaluatePolicy(pol *policy.CompiledPolicy, job *router.JobDescriptor) policy.Evaluation {
	if pol == nil {
		return policy.Evaluation{}
	}
	return pol.Evaluate(policy.EvaluationInput{
		TenantID:             job.TenantID,
		ProjectID:            job.ProjectID,
		TaskType:             string(job.TaskType),
		RiskLevel:            string(job.RiskLevel),
		Sensitivity:          string(job.Sensitivity),
		PromptTokensEstimate: job.PromptTokensEstimate,
		Keywords:             append([]string(nil), job.Keywords...),
		FilesTouched:         append([]string(nil), job.FilesTouched...),
		RequiresToolUse:      job.RequiresToolUse,
		RequiresJSONSchema:   job.RequiresJSONSchema,
		RequiresVision:       job.RequiresVision,
		RouterMode:           string(job.RouterMode),
		ToolRisk:             job.ToolRisk,
		ToolsDeclared:        append([]string(nil), job.ToolsDeclared...),
	})
}

func resolveTimeout(pol *policy.CompiledPolicy, route policy.Route, job *router.JobDescriptor) int {
	if f := route.Force; f != nil && f.TimeoutMS != nil {
		return *f.TimeoutMS
	}
	if d := route.Defaults; d != nil && d.TimeoutMS != nil {
		return *d.TimeoutMS
	}
	if pol != nil && pol.Settings().DefaultTimeoutMS > 0 {
		return pol.Settings().DefaultTimeoutMS
	}
	if job.RequiresLargeContext {
		return 60_000
	}
	return 30_000
}

func resolveVerifier(job *router.JobDescriptor, route policy.Route) bool {
	if f := route.Force; f != nil && f.Verifier != nil {
		return *f.Verifier
	}
	if d := route.Defaults; d != nil && d.Verifier != nil {
		return *d.Verifier
	}
	return job.TaskType == router.TaskSecurityReview || job.TaskType == router.TaskDatabaseMigration
}

func buildReasons(
	job *router.JobDescriptor,
	eval policy.Evaluation,
	filterRes FilterResult,
	scored []ScoredCandidate,
	primary registry.Model,
	fallbacks []FallbackEntry,
) []string {
	reasons := make([]string, 0, 6)
	reasons = append(reasons, fmt.Sprintf("task=%s risk=%s sensitivity=%s tokens≈%d",
		job.TaskType, job.RiskLevel, job.Sensitivity, job.PromptTokensEstimate))
	reasons = append(reasons, filterRes.Reasons...)
	reasons = append(reasons, eval.Explanations...)
	if len(scored) > 0 {
		reasons = append(reasons, fmt.Sprintf("selected %s (tier=%s provider=%s score=%.4f)",
			primary.ID, primary.Tier, primary.ProviderID, scored[0].Score))
		reasons = append(reasons, scored[0].Reasons...)
	}
	reasons = append(reasons, FallbackChainReason(primary, fallbacks))
	return reasons
}

func routeID(job *router.JobDescriptor, modelID string) string {
	if job.RequestID != "" {
		return fmt.Sprintf("route_%s_%s", job.RequestID, modelID)
	}
	return fmt.Sprintf("route_%s", modelID)
}
