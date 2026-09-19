package engine

import (
	"fmt"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// FilterResult holds filtered candidates and per-model exclusion reasons.
type FilterResult struct {
	Candidates []registry.Model
	Excluded   map[string]string // model ID → human-readable reason
	Reasons    []string          // positive filtering notes
	// ResidencyExcluded is set when at least one otherwise-eligible model was
	// excluded specifically by a residency/compliance-tag constraint. When the
	// candidate set ends up empty, this distinguishes a compliance block (403,
	// audited) from a generic no_route (ISSUE-084).
	ResidencyExcluded bool
	// ResidencyReasons collects those tag exclusion reasons for explainability.
	ResidencyReasons []string
}

// minHealthThreshold is the minimum provider health score to be eligible.
const minHealthThreshold = 0.1

// FilterCandidates applies hard capability, policy, and health filters to the
// registry. Models that fail any filter are excluded. The function never calls
// a provider or LLM.
func FilterCandidates(
	job *router.JobDescriptor,
	route policy.Route,
	snapshot *registry.Snapshot,
	health HealthSnapshot,
	streaming bool,
) FilterResult {
	if health == nil {
		health = FullyHealthy
	}
	result := FilterResult{Excluded: make(map[string]string)}

	required := requiredCapabilities(job, streaming)
	policyCaps := capabilitiesFromPolicy(route.Constraints)
	merged := mergeCapabilities(required, policyCaps)

	candidates := snapshot.EnabledModelsWithCapabilities(merged)
	minTier := MinimumTierForTask(job, route)

	for _, model := range candidates {
		if reason, residency := hardFilter(model, job, route, health, minTier); reason != "" {
			result.Excluded[model.ID] = reason
			if residency {
				result.ResidencyExcluded = true
				result.ResidencyReasons = append(result.ResidencyReasons, reason)
			}
			continue
		}
		result.Candidates = append(result.Candidates, model)
	}

	result.Reasons = append(result.Reasons, fmt.Sprintf(
		"%d candidate(s) after filtering (chat=%v streaming=%v tools=%v vision=%v long_context=%v min_tier=%s)",
		len(result.Candidates), merged.Chat, merged.Streaming, merged.ToolCalls, merged.Vision, merged.LongContext, minTier,
	))
	return result
}

// hardFilter returns a non-empty exclusion reason when the model fails any hard
// filter, plus whether that reason was a residency/compliance-tag one (so the
// engine can turn a "no compliant provider" outcome into an audited block rather
// than a generic no_route — ISSUE-084).
func hardFilter(
	model registry.Model,
	job *router.JobDescriptor,
	route policy.Route,
	health HealthSnapshot,
	minTier registry.Tier,
) (reason string, residency bool) {
	// Minimum tier for task / risk combination.
	if !TierAtLeast(model.Tier, minTier) {
		return fmt.Sprintf("tier %s below minimum %s for task=%s risk=%s", model.Tier, minTier, job.TaskType, job.RiskLevel), false
	}

	// Provider health.
	if h := health.ProviderHealth(model.ProviderID); h < minHealthThreshold {
		return fmt.Sprintf("provider %s health %.2f below threshold", model.ProviderID, h), false
	}

	// Policy constraints.
	if c := route.Constraints; c != nil {
		if r, res := providerModelConstraintReason(model, c); r != "" {
			return r, res
		}
		if c.MaxLatencyMS != nil && model.Latency.P95FirstTokenMS > 0 && model.Latency.P95FirstTokenMS > *c.MaxLatencyMS {
			return fmt.Sprintf("p95 latency %dms exceeds policy max %dms", model.Latency.P95FirstTokenMS, *c.MaxLatencyMS), false
		}
		for _, denied := range c.DenyCapabilities {
			switch denied {
			case policy.CapStreaming:
				if model.Capabilities.Streaming {
					return "policy denies streaming capability", false
				}
			case policy.CapToolUse:
				if model.Capabilities.ToolCalls {
					return "policy denies tool_use capability", false
				}
			case policy.CapVision:
				if model.Capabilities.Vision {
					return "policy denies vision capability", false
				}
			case policy.CapLongContext:
				if model.Capabilities.LongContext {
					return "policy denies long_context capability", false
				}
			}
		}
	}

	// Policy force narrows to one model or provider.
	if f := route.Force; f != nil {
		if f.Model != "" && model.ID != f.Model && model.ProviderModelID != f.Model {
			return fmt.Sprintf("policy forces model %s", f.Model), false
		}
		if f.Provider != "" && model.ProviderID != f.Provider {
			return fmt.Sprintf("policy forces provider %s", f.Provider), false
		}
		if f.ModelProfile != "" && string(model.Tier) != string(f.ModelProfile) {
			return fmt.Sprintf("policy forces profile %s", f.ModelProfile), false
		}
	}

	// RouterMode hard filters.
	if job.RouterMode == router.RouterModeCheap && minTier != registry.TierPremium {
		if model.Tier == registry.TierPremium {
			return "router_mode=cheap excludes premium tier", false
		}
	}

	return "", false
}

// MinimumTierForTask returns the lowest acceptable model tier for the job after
// applying any policy profile force and, when the job is flagged conservative,
// a raised floor (ISSUE-060).
func MinimumTierForTask(job *router.JobDescriptor, route policy.Route) registry.Tier {
	base := baseMinimumTier(job.TaskType, job.RiskLevel, route)
	if job.Conservative {
		// Conservative mode never lowers the safety level: uncertain requests
		// route at least at balanced (policy/task forces above this still win).
		return higherTier(base, registry.TierBalanced)
	}
	return base
}

// BaselineTier returns the router's default minimum tier for a task/risk with no
// policy overrides — the floor the fast path applies before any policy force.
// Exposed for offline analysis (policy recommendations); not used on the hot
// path, which goes through MinimumTierForTask with the live route.
func BaselineTier(task router.TaskType, risk router.RiskLevel) registry.Tier {
	return baseMinimumTier(task, risk, policy.Route{})
}

func baseMinimumTier(task router.TaskType, risk router.RiskLevel, route policy.Route) registry.Tier {
	if f := route.Force; f != nil && f.ModelProfile != "" {
		switch f.ModelProfile {
		case policy.ProfilePremium:
			return registry.TierPremium
		case policy.ProfileBalanced:
			return registry.TierBalanced
		case policy.ProfileCheap:
			return registry.TierCheap
		}
	}
	switch task {
	case router.TaskSecurityReview, router.TaskDatabaseMigration, router.TaskUnknownHighRisk:
		return registry.TierPremium
	case router.TaskHardCodeDebugging, router.TaskLongContextAnalysis:
		return registry.TierBalanced
	default:
		if risk == router.RiskCritical || risk == router.RiskHigh {
			return registry.TierBalanced
		}
		return registry.TierCheap
	}
}

// higherTier returns the more capable of two tiers.
func higherTier(a, b registry.Tier) registry.Tier {
	if TierOrdinal(b) > TierOrdinal(a) {
		return b
	}
	return a
}

// TierAtLeast reports whether tier meets or exceeds minimum.
func TierAtLeast(tier, minimum registry.Tier) bool {
	return TierOrdinal(tier) >= TierOrdinal(minimum)
}

// TierOrdinal maps a tier to a sortable integer (higher = more capable).
func TierOrdinal(tier registry.Tier) int {
	switch tier {
	case registry.TierCheap:
		return 0
	case registry.TierBalanced:
		return 1
	case registry.TierPremium:
		return 2
	default:
		return -1
	}
}

func requiredCapabilities(job *router.JobDescriptor, streaming bool) registry.Capabilities {
	return registry.Capabilities{
		Chat:        true,
		Streaming:   streaming,
		ToolCalls:   job.RequiresToolUse,
		JSONSchema:  job.RequiresJSONSchema,
		Vision:      job.RequiresVision,
		LongContext: job.RequiresLargeContext,
	}
}

func capabilitiesFromPolicy(c *policy.Constraints) registry.Capabilities {
	if c == nil {
		return registry.Capabilities{}
	}
	var caps registry.Capabilities
	for _, cap := range c.RequireCapabilities {
		switch cap {
		case policy.CapStreaming:
			caps.Streaming = true
		case policy.CapToolUse:
			caps.ToolCalls = true
		case policy.CapJSONSchema:
			caps.JSONSchema = true
		case policy.CapVision:
			caps.Vision = true
		case policy.CapLongContext:
			caps.LongContext = true
		}
	}
	return caps
}

func mergeCapabilities(a, b registry.Capabilities) registry.Capabilities {
	return registry.Capabilities{
		Chat:        a.Chat || b.Chat,
		Streaming:   a.Streaming || b.Streaming,
		ToolCalls:   a.ToolCalls || b.ToolCalls,
		JSONSchema:  a.JSONSchema || b.JSONSchema,
		Vision:      a.Vision || b.Vision,
		LongContext: a.LongContext || b.LongContext,
	}
}

// providerModelConstraintReason returns a non-empty exclusion reason if the
// model violates the policy's provider/model allow or deny lists OR its
// residency/compliance tag requirements, plus whether that reason was a
// residency/compliance-tag one. It is the single source of truth for these
// checks, shared by candidate filtering and by pinned-model decisions (explicit
// client model, policy force.model, disabled mode) — so no override path can
// ever bypass a project's denylist, allowlist, or residency requirement.
func providerModelConstraintReason(model registry.Model, c *policy.Constraints) (reason string, residency bool) {
	if c == nil {
		return "", false
	}
	if len(c.AllowedProviders) > 0 && !containsStr(c.AllowedProviders, model.ProviderID) {
		return fmt.Sprintf("provider %s not in allowed_providers", model.ProviderID), false
	}
	if containsStr(c.DeniedProviders, model.ProviderID) {
		return fmt.Sprintf("provider %s in denied_providers", model.ProviderID), false
	}
	if len(c.AllowedModels) > 0 && !containsStr(c.AllowedModels, model.ID) {
		return fmt.Sprintf("model %s not in allowed_models", model.ID), false
	}
	if containsStr(c.DeniedModels, model.ID) {
		return fmt.Sprintf("model %s in denied_models", model.ID), false
	}
	if r := providerTagConstraintReason(model, c); r != "" {
		return r, true
	}
	return "", false
}

// providerTagConstraintReason returns a non-empty reason when the model fails
// the policy's residency/compliance tag requirements (ISSUE-078). The model's
// effective tags are the union of its own and its provider's, materialized at
// snapshot build.
func providerTagConstraintReason(model registry.Model, c *policy.Constraints) string {
	if c == nil {
		return ""
	}
	for _, tag := range c.RequireProviderTags {
		if !containsStr(model.ComplianceTags, tag) {
			return fmt.Sprintf("provider %s/model %s missing required compliance tag %q", model.ProviderID, model.ID, tag)
		}
	}
	for _, tag := range c.DenyProviderTags {
		if containsStr(model.ComplianceTags, tag) {
			return fmt.Sprintf("provider %s/model %s has denied compliance tag %q", model.ProviderID, model.ID, tag)
		}
	}
	return ""
}

func containsStr(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}
