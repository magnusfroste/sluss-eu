package spend

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/magnusfroste/sluss/internal/cost"
	"github.com/magnusfroste/sluss/internal/registry"
)

// SimRequest is one request to simulate: its token usage, the risk level it was
// classified at, and the model it was actually routed to.
type SimRequest struct {
	InputTokens  int64
	OutputTokens int64
	RiskLevel    string
	RoutedModel  registry.Model
}

// DefaultRiskWeights weights realized savings by risk level. Savings on low-risk
// requests count fully; savings won by routing risky tasks to cheaper models are
// discounted, because aggressive downgrading there carries quality/safety risk.
func DefaultRiskWeights() map[string]float64 {
	return map[string]float64{
		"low":      1.0,
		"medium":   0.85,
		"high":     0.5,
		"critical": 0.2,
	}
}

// defaultRiskWeight is used for an unknown/empty risk level — conservative.
const defaultRiskWeight = 0.5

// Simulator computes what-if spend against a premium baseline model: what every
// request would have cost on Baseline, versus where it was actually routed.
type Simulator struct {
	// Baseline is the premium model every request is priced against.
	Baseline registry.Model
	// RiskWeights overrides DefaultRiskWeights when non-nil.
	RiskWeights map[string]float64
}

// SimResult is the outcome of a simulation. Monetary fields are in micro-USD.
type SimResult struct {
	Requests                    int
	BaselinePremiumMicroUSD     int64
	ActualMicroUSD              int64
	SavingsMicroUSD             int64
	SavingsPercent              float64
	RiskAdjustedSavingsMicroUSD int64
}

func (s Simulator) weight(risk string) float64 {
	weights := s.RiskWeights
	if weights == nil {
		weights = DefaultRiskWeights()
	}
	if v, ok := weights[strings.ToLower(strings.TrimSpace(risk))]; ok {
		return v
	}
	return defaultRiskWeight
}

// Run simulates the requests, returning baseline (all-premium) spend, actual
// routed spend, raw savings and risk-adjusted savings. It errors if a cost
// estimate fails (e.g. missing cost metadata on the baseline or a routed model).
func (s Simulator) Run(requests []SimRequest) (SimResult, error) {
	var res SimResult
	var riskAdjusted float64

	for i, r := range requests {
		usage := cost.TokenUsage{InputTokens: r.InputTokens, OutputTokens: r.OutputTokens}

		base, err := cost.EstimateModel(s.Baseline, usage)
		if err != nil {
			return SimResult{}, fmt.Errorf("baseline estimate (request %d): %w", i, err)
		}
		actual, err := cost.EstimateModel(r.RoutedModel, usage)
		if err != nil {
			return SimResult{}, fmt.Errorf("routed estimate (request %d): %w", i, err)
		}

		res.Requests++
		res.BaselinePremiumMicroUSD += base.TotalMicroUSD
		res.ActualMicroUSD += actual.TotalMicroUSD

		saving := base.TotalMicroUSD - actual.TotalMicroUSD
		riskAdjusted += float64(saving) * s.weight(r.RiskLevel)
	}

	res.SavingsMicroUSD = res.BaselinePremiumMicroUSD - res.ActualMicroUSD
	res.RiskAdjustedSavingsMicroUSD = int64(math.Round(riskAdjusted))
	if res.BaselinePremiumMicroUSD > 0 {
		res.SavingsPercent = float64(res.SavingsMicroUSD) / float64(res.BaselinePremiumMicroUSD) * 100
	}
	return res, nil
}

// Summary writes a human-readable report of the simulation.
func (r SimResult) Summary(w io.Writer) {
	fmt.Fprintf(w, "Requests:                %d\n", r.Requests)
	fmt.Fprintf(w, "Baseline (all premium):  $%s\n", cost.FormatMicroUSD(r.BaselinePremiumMicroUSD))
	fmt.Fprintf(w, "Actual (routed):         $%s\n", cost.FormatMicroUSD(r.ActualMicroUSD))
	fmt.Fprintf(w, "Savings:                 $%s (%.1f%%)\n", cost.FormatMicroUSD(r.SavingsMicroUSD), r.SavingsPercent)
	fmt.Fprintf(w, "Risk-adjusted savings:   $%s\n", cost.FormatMicroUSD(r.RiskAdjustedSavingsMicroUSD))
}
