package engine

import (
	"math"
	"slices"
)

// DecisionComparison captures deterministic routing-relevant differences
// between two route decisions so offline evals and future shadow-routing
// reporting can share the same contract.
type DecisionComparison struct {
	Primary                    RouteDecision `json:"primary"`
	Secondary                  RouteDecision `json:"secondary"`
	Changed                    bool          `json:"changed"`
	RouteChanged               bool          `json:"route_changed"`
	FallbackChanged            bool          `json:"fallback_changed"`
	TimeoutChanged             bool          `json:"timeout_changed"`
	RequiresVerifierChanged    bool          `json:"requires_verifier_changed"`
	PolicyVersionChanged       bool          `json:"policy_version_changed"`
	CostChanged                bool          `json:"cost_changed"`
	EstimatedCostDeltaMicroUSD int64         `json:"estimated_cost_delta_microusd"`
	EstimatedCostDeltaUSD      float64       `json:"estimated_cost_delta_usd"`
}

// CompareDecisions reports stable, structured diffs between two decisions while
// intentionally ignoring explanation text so logs and reports stay deterministic.
func CompareDecisions(primary, secondary RouteDecision) DecisionComparison {
	deltaMicroUSD := estimatedCostMicroUSD(secondary.EstimatedCostUSD) - estimatedCostMicroUSD(primary.EstimatedCostUSD)
	comparison := DecisionComparison{
		Primary:                    primary,
		Secondary:                  secondary,
		RouteChanged:               routeChanged(primary, secondary),
		FallbackChanged:            !slices.Equal(primary.Fallbacks, secondary.Fallbacks),
		TimeoutChanged:             primary.TimeoutMS != secondary.TimeoutMS,
		RequiresVerifierChanged:    primary.RequiresVerifier != secondary.RequiresVerifier,
		PolicyVersionChanged:       primary.PolicyVersion != secondary.PolicyVersion,
		CostChanged:                deltaMicroUSD != 0,
		EstimatedCostDeltaMicroUSD: deltaMicroUSD,
		EstimatedCostDeltaUSD:      float64(deltaMicroUSD) / 1_000_000,
	}
	comparison.Changed = comparison.RouteChanged ||
		comparison.FallbackChanged ||
		comparison.TimeoutChanged ||
		comparison.RequiresVerifierChanged ||
		comparison.CostChanged
	return comparison
}

func routeChanged(primary, secondary RouteDecision) bool {
	return primary.Blocked != secondary.Blocked ||
		primary.BlockCode != secondary.BlockCode ||
		primary.BlockReason != secondary.BlockReason ||
		primary.BlockStatus != secondary.BlockStatus ||
		primary.SelectedModel != secondary.SelectedModel ||
		primary.SelectedProvider != secondary.SelectedProvider ||
		primary.ProviderModelID != secondary.ProviderModelID
}

func estimatedCostMicroUSD(costUSD float64) int64 {
	return int64(math.Round(costUSD * 1_000_000))
}
