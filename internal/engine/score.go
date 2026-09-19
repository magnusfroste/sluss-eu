package engine

import (
	"fmt"
	"sort"

	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// ScoreCandidates ranks filtered candidates using the scoring formula from the
// architecture spec. Returns candidates sorted descending by score.
//
// Formula:
//
//	score = quality_weight  * predicted_quality
//	      + capability_weight * capability_match
//	      + health_weight    * provider_health
//	      - cost_weight      * normalized_cost
//	      - latency_weight   * normalized_latency
//	      - risk_penalty
func ScoreCandidates(
	candidates []registry.Model,
	job *router.JobDescriptor,
	minTier registry.Tier,
	health HealthSnapshot,
	weights Weights,
	quality ...QualitySource,
) []ScoredCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if health == nil {
		health = FullyHealthy
	}
	var q QualitySource
	if len(quality) > 0 {
		q = quality[0]
	}

	// Two-pass: compute maxima for per-candidate normalization.
	var maxCost, maxLatency float64
	for _, m := range candidates {
		if c := estimateCostMicroUSD(m, job); c > maxCost {
			maxCost = c
		}
		if l := float64(m.Latency.P95FirstTokenMS); l > maxLatency {
			maxLatency = l
		}
	}
	if maxCost == 0 {
		maxCost = 1
	}
	if maxLatency == 0 {
		maxLatency = 1
	}

	// Apply router_mode adjustments once, shared across all candidates.
	w := applyRouterModeWeights(weights, job.RouterMode)

	scored := make([]ScoredCandidate, 0, len(candidates))
	for _, model := range candidates {
		sc := scoreOne(model, job, minTier, health, w, maxCost, maxLatency, q)
		scored = append(scored, sc)
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	return scored
}

// QualitySource supplies a measured predicted-quality score in [0,1] for a
// (task, model) pair — e.g. eval pass-rates or outcome acceptance. When present
// and it has a value, it overrides the registry's static quality prior, so
// routing prefers the cheapest model measured "good enough" per task class.
// Optional: a nil source leaves scoring on the static priors (default behavior).
type QualitySource interface {
	Quality(task router.TaskType, modelID string) (float64, bool)
}

func scoreOne(
	model registry.Model,
	job *router.JobDescriptor,
	minTier registry.Tier,
	health HealthSnapshot,
	w Weights,
	maxCost, maxLatency float64,
	q QualitySource,
) ScoredCandidate {
	quality := qualityScore(model, job.TaskType, q)
	capability := capabilityMatchScore(model.Tier, minTier)
	healthVal := health.ProviderHealth(model.ProviderID)
	normalizedCost := estimateCostMicroUSD(model, job) / maxCost
	normalizedLatency := float64(model.Latency.P95FirstTokenMS) / maxLatency
	penalty := riskPenalty(model.Tier, job.RiskLevel)

	score := w.Quality*quality +
		w.Capability*capability +
		w.Health*healthVal -
		w.Cost*normalizedCost -
		w.Latency*normalizedLatency -
		penalty

	reason := fmt.Sprintf(
		"quality=%.2f capability=%.2f health=%.2f cost_norm=%.2f latency_norm=%.2f penalty=%.2f → score=%.4f",
		quality, capability, healthVal, normalizedCost, normalizedLatency, penalty, score,
	)
	return ScoredCandidate{Model: model, Score: score, Reasons: []string{reason}}
}

func qualityScore(model registry.Model, task router.TaskType, q QualitySource) float64 {
	// Measured quality (evals/outcomes) wins over the static registry prior.
	if q != nil {
		if s, ok := q.Quality(task, model.ID); ok {
			return s
		}
	}
	if s, ok := model.QualityScores[string(task)]; ok {
		return s
	}
	switch model.Tier {
	case registry.TierCheap:
		return 0.55
	case registry.TierBalanced:
		return 0.70
	case registry.TierPremium:
		return 0.85
	default:
		return 0.60
	}
}

func capabilityMatchScore(modelTier, minTier registry.Tier) float64 {
	diff := TierOrdinal(modelTier) - TierOrdinal(minTier)
	switch {
	case diff < 0:
		return 0.5 // under-provisioned — filtered out already, safety fallback
	case diff == 0:
		return 1.0
	default:
		return max(0.85, 1.0-float64(diff)*0.05) // slight penalty for over-provisioning
	}
}

func riskPenalty(modelTier registry.Tier, risk router.RiskLevel) float64 {
	switch risk {
	case router.RiskCritical:
		switch modelTier {
		case registry.TierCheap:
			return 0.40
		case registry.TierBalanced:
			return 0.15
		}
	case router.RiskHigh:
		switch modelTier {
		case registry.TierCheap:
			return 0.30
		case registry.TierBalanced:
			return 0.05
		}
	case router.RiskMedium:
		if modelTier == registry.TierCheap {
			return 0.05
		}
	}
	return 0
}

// estimateCostMicroUSD returns a rough input+output cost estimate in micro-USD.
func estimateCostMicroUSD(model registry.Model, job *router.JobDescriptor) float64 {
	if !model.Cost.Available() {
		return 0
	}
	input := float64(job.PromptTokensEstimate) * float64(model.Cost.InputMicrosPerMillionToken) / 1_000_000
	output := float64(job.MaxOutputTokensEstimate) * float64(model.Cost.OutputMicrosPerMillionToken) / 1_000_000
	return input + output
}

func applyRouterModeWeights(w Weights, mode router.RouterMode) Weights {
	switch mode {
	case router.RouterModeCheap:
		w.Cost += 0.15
		w.Quality -= 0.10
		w.Latency += 0.05
	case router.RouterModePremium:
		w.Quality += 0.15
		w.Cost -= 0.10
	}
	return w
}
