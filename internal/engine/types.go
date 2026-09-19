// Package engine is the stateless routing decision engine. It filters model
// candidates, scores them, builds a fallback chain and returns a RouteDecision
// — all without calling any provider or LLM.
package engine

import "github.com/magnusfroste/sluss/internal/registry"

// RouteDecision is the output of the routing engine for a single request.
type RouteDecision struct {
	RouteID          string          `json:"route_id"`
	SelectedModel    string          `json:"selected_model"`    // registry model ID
	SelectedProvider string          `json:"selected_provider"` // registry provider ID
	ProviderModelID  string          `json:"provider_model_id"` // ID sent to the provider
	Fallbacks        []FallbackEntry `json:"fallbacks"`
	TimeoutMS        int             `json:"timeout_ms"`
	RequiresVerifier bool            `json:"requires_verifier"`
	DecisionReasons  []string        `json:"decision_reasons"`
	PolicyVersion    string          `json:"policy_version"`
	EstimatedCostUSD float64         `json:"estimated_cost_usd"`
	Blocked          bool            `json:"blocked,omitempty"`
	BlockCode        string          `json:"block_code,omitempty"`
	BlockReason      string          `json:"block_reason,omitempty"`
	BlockStatus      int             `json:"block_status,omitempty"`
}

// FallbackEntry is one entry in the ordered fallback chain.
type FallbackEntry struct {
	ModelID         string `json:"model_id"`
	ProviderID      string `json:"provider_id"`
	ProviderModelID string `json:"provider_model_id"`
}

// ScoredCandidate holds a model with its computed score and scoring detail.
type ScoredCandidate struct {
	Model   registry.Model
	Score   float64
	Reasons []string
}

// Weights controls the scoring formula coefficients.
// All weights should be positive; cost and latency are subtracted.
type Weights struct {
	Quality    float64
	Capability float64
	Health     float64
	Cost       float64
	Latency    float64
}

// DefaultWeights returns the baseline scoring weights from the architecture spec.
func DefaultWeights() Weights {
	return Weights{
		Quality:    0.35,
		Capability: 0.25,
		Health:     0.20,
		Cost:       0.15,
		Latency:    0.05,
	}
}

// HealthSnapshot provides per-provider health scores in [0.0, 1.0].
// Implementations must be safe for concurrent reads.
type HealthSnapshot interface {
	ProviderHealth(providerID string) float64
}

// StaticHealth is a fixed map-based HealthSnapshot. A nil value returns 1.0 for all providers.
type StaticHealth map[string]float64

func (h StaticHealth) ProviderHealth(providerID string) float64 {
	if h == nil {
		return 1.0
	}
	if v, ok := h[providerID]; ok {
		return v
	}
	return 1.0
}

// FullyHealthy is a HealthSnapshot where every provider reports 1.0.
var FullyHealthy HealthSnapshot = StaticHealth(nil)
