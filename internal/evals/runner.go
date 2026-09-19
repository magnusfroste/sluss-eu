package evals

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// Runner executes eval cases against the routing engine.
type Runner struct {
	Engine   *engine.Engine
	Snapshot *registry.Snapshot
}

// NewRunner builds a Runner backed by the default registry.
func NewRunner() (*Runner, error) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		return nil, err
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		return nil, err
	}
	return &Runner{Engine: engine.New(store), Snapshot: snap}, nil
}

// CaseResult is the outcome of a single eval case.
type CaseResult struct {
	Case             Case
	Pass             bool
	ClassifiedTask   string
	SelectedModel    string
	SelectedTier     string
	ExpectedTier     string
	EstimatedCostUSD float64
	RoutingMicros    int64
	PolicyVersion    string
	Blocked          bool
	BlockCode        string
	BlockReason      string
	Reason           string // failure reason when !Pass
	decision         engine.RouteDecision
}

// Report aggregates results across the whole dataset.
type Report struct {
	Total          int
	Passed         int
	Results        []CaseResult
	TotalCostUSD   float64
	MeanRoutingMic float64
	Frontier       FrontierReport
}

// CaseComparison is the per-case output of an offline two-policy simulation.
type CaseComparison struct {
	Case       Case                      `json:"case"`
	Primary    CaseResult                `json:"primary"`
	Secondary  CaseResult                `json:"secondary"`
	Comparison engine.DecisionComparison `json:"comparison"`
}

// ComparisonReport aggregates an offline decision diff for the same dataset
// under two policy variants.
type ComparisonReport struct {
	Total                      int              `json:"total"`
	PrimaryPassed              int              `json:"primary_passed"`
	SecondaryPassed            int              `json:"secondary_passed"`
	PrimaryTotalCostUSD        float64          `json:"primary_total_cost_usd"`
	SecondaryTotalCostUSD      float64          `json:"secondary_total_cost_usd"`
	PrimaryMeanRoutingMic      float64          `json:"primary_mean_routing_mic"`
	SecondaryMeanRoutingMic    float64          `json:"secondary_mean_routing_mic"`
	ChangedCount               int              `json:"changed_count"`
	RouteChangedCount          int              `json:"route_changed_count"`
	FallbackChangedCount       int              `json:"fallback_changed_count"`
	TimeoutChangedCount        int              `json:"timeout_changed_count"`
	VerifierChangedCount       int              `json:"verifier_changed_count"`
	PolicyVersionChangedCount  int              `json:"policy_version_changed_count"`
	CostChangedCount           int              `json:"cost_changed_count"`
	EstimatedCostDeltaMicroUSD int64            `json:"estimated_cost_delta_microusd"`
	EstimatedCostDeltaUSD      float64          `json:"estimated_cost_delta_usd"`
	Comparisons                []CaseComparison `json:"comparisons"`
}

// PassRate returns the fraction of cases that routed as expected.
func (r Report) PassRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Passed) / float64(r.Total)
}

// PrimaryPassRate returns the pass rate for the first policy in a comparison.
func (r ComparisonReport) PrimaryPassRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.PrimaryPassed) / float64(r.Total)
}

// SecondaryPassRate returns the pass rate for the second policy in a comparison.
func (r ComparisonReport) SecondaryPassRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.SecondaryPassed) / float64(r.Total)
}

// Run executes every case in the dataset and returns a Report.
func (rn *Runner) Run(ds *Dataset) (Report, error) {
	return rn.RunWithPolicy(ds, nil)
}

// RunWithPolicy executes every case in the dataset under the supplied compiled
// policy. A nil policy means "engine defaults only".
func (rn *Runner) RunWithPolicy(ds *Dataset, pol *policy.CompiledPolicy) (Report, error) {
	report := Report{Total: len(ds.Cases)}
	var totalMicros int64
	for _, c := range ds.Cases {
		res := rn.runCase(c, pol)
		if res.Pass {
			report.Passed++
		}
		report.TotalCostUSD += res.EstimatedCostUSD
		totalMicros += res.RoutingMicros
		report.Results = append(report.Results, res)
	}
	if report.Total > 0 {
		report.MeanRoutingMic = float64(totalMicros) / float64(report.Total)
	}
	report.Frontier = buildFrontierReport(ds, report.Results, rn.Snapshot)
	return report, nil
}

// ComparePolicies runs the same dataset under two policies and returns a
// deterministic diff report. Nil policies are allowed so callers can compare a
// policy variant against the engine's default behavior without provider calls.
func (rn *Runner) ComparePolicies(ds *Dataset, primary, secondary *policy.CompiledPolicy) (ComparisonReport, error) {
	primaryReport, err := rn.RunWithPolicy(ds, primary)
	if err != nil {
		return ComparisonReport{}, err
	}
	secondaryReport, err := rn.RunWithPolicy(ds, secondary)
	if err != nil {
		return ComparisonReport{}, err
	}

	report := ComparisonReport{
		Total:                   len(ds.Cases),
		PrimaryPassed:           primaryReport.Passed,
		SecondaryPassed:         secondaryReport.Passed,
		PrimaryTotalCostUSD:     primaryReport.TotalCostUSD,
		SecondaryTotalCostUSD:   secondaryReport.TotalCostUSD,
		PrimaryMeanRoutingMic:   primaryReport.MeanRoutingMic,
		SecondaryMeanRoutingMic: secondaryReport.MeanRoutingMic,
	}
	for i := range primaryReport.Results {
		comparison := engine.CompareDecisions(primaryReport.Results[i].decision, secondaryReport.Results[i].decision)
		if comparison.Changed {
			report.ChangedCount++
		}
		if comparison.RouteChanged {
			report.RouteChangedCount++
		}
		if comparison.FallbackChanged {
			report.FallbackChangedCount++
		}
		if comparison.TimeoutChanged {
			report.TimeoutChangedCount++
		}
		if comparison.RequiresVerifierChanged {
			report.VerifierChangedCount++
		}
		if comparison.PolicyVersionChanged {
			report.PolicyVersionChangedCount++
		}
		if comparison.CostChanged {
			report.CostChangedCount++
		}
		report.EstimatedCostDeltaMicroUSD += comparison.EstimatedCostDeltaMicroUSD
		report.Comparisons = append(report.Comparisons, CaseComparison{
			Case:       primaryReport.Results[i].Case,
			Primary:    primaryReport.Results[i],
			Secondary:  secondaryReport.Results[i],
			Comparison: comparison,
		})
	}
	report.EstimatedCostDeltaUSD = float64(report.EstimatedCostDeltaMicroUSD) / 1_000_000
	return report, nil
}

func (rn *Runner) runCase(c Case, pol *policy.CompiledPolicy) CaseResult {
	req := &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: c.PromptText()}},
		Metadata: caseMetadata(c),
	}
	if c.ExplicitModel != "" {
		req.Model = c.ExplicitModel
	}

	job := router.NewJobDescriptor(router.JobDescriptorInput{
		RequestID: "eval_" + c.ID,
		Auth: router.AuthTenantContext{
			TenantID:  c.TenantID,
			ProjectID: c.ProjectID,
		},
		Request: req,
	})

	start := time.Now()
	dec, err := rn.Engine.Decide(job, pol, engine.FullyHealthy, false)
	micros := time.Since(start).Microseconds()

	res := CaseResult{
		Case:           c,
		ClassifiedTask: string(job.TaskType),
		ExpectedTier:   c.ExpectedRoute.Tier,
		RoutingMicros:  micros,
		PolicyVersion:  dec.PolicyVersion,
		Blocked:        dec.Blocked,
		BlockCode:      dec.BlockCode,
		BlockReason:    dec.BlockReason,
		decision:       dec,
	}
	if err != nil && !errors.Is(err, engine.ErrBlocked) {
		res.Reason = fmt.Sprintf("engine error: %v", err)
		return res
	}

	res.SelectedModel = dec.SelectedModel
	res.EstimatedCostUSD = dec.EstimatedCostUSD
	if model, ok := rn.Snapshot.Model(dec.SelectedModel); ok {
		res.SelectedTier = string(model.Tier)
	}
	if errors.Is(err, engine.ErrBlocked) {
		res.Reason = fmt.Sprintf("blocked: %s", blockedLabel(dec))
		return res
	}

	// Model pin takes precedence when specified.
	if c.ExpectedRoute.Model != "" {
		res.Pass = dec.SelectedModel == c.ExpectedRoute.Model
		if !res.Pass {
			res.Reason = fmt.Sprintf("expected model %s, got %s", c.ExpectedRoute.Model, dec.SelectedModel)
		}
		return res
	}

	res.Pass = res.SelectedTier == c.ExpectedRoute.Tier
	if !res.Pass {
		res.Reason = fmt.Sprintf("expected tier %s, got %s (model %s, classified %s)",
			c.ExpectedRoute.Tier, res.SelectedTier, dec.SelectedModel, res.ClassifiedTask)
	}
	return res
}

// FormatReport renders a human-readable text report.
func FormatReport(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Eval report: %d/%d passed (%.1f%%)\n", r.Passed, r.Total, r.PassRate()*100)
	fmt.Fprintf(&b, "Total estimated cost: $%.6f  ·  mean routing: %.1fµs\n", r.TotalCostUSD, r.MeanRoutingMic)

	// Group failures for readability.
	var failures []CaseResult
	for _, res := range r.Results {
		if !res.Pass {
			failures = append(failures, res)
		}
	}
	if len(failures) == 0 {
		b.WriteString("All cases routed as expected.\n")
		b.WriteString(formatFrontierReport(r.Frontier))
		return b.String()
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Case.ID < failures[j].Case.ID })
	fmt.Fprintf(&b, "\nFailures (%d):\n", len(failures))
	for _, f := range failures {
		fmt.Fprintf(&b, "  - %s (%s): %s\n", f.Case.ID, f.Case.Name, f.Reason)
	}
	b.WriteString(formatFrontierReport(r.Frontier))
	return b.String()
}

// FormatComparisonReport renders a deterministic text report for two-policy
// offline simulations. The labels are presentation-only and do not affect the
// JSON shape used by other callers.
func FormatComparisonReport(r ComparisonReport, primaryLabel, secondaryLabel string) string {
	if primaryLabel == "" {
		primaryLabel = "primary"
	}
	if secondaryLabel == "" {
		secondaryLabel = "secondary"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Policy comparison: %d/%d cases changed\n", r.ChangedCount, r.Total)
	fmt.Fprintf(&b, "%s: %d/%d passed (%.1f%%)  ·  total estimated cost: $%.6f  ·  mean routing: %.1fµs\n",
		primaryLabel, r.PrimaryPassed, r.Total, r.PrimaryPassRate()*100, r.PrimaryTotalCostUSD, r.PrimaryMeanRoutingMic)
	fmt.Fprintf(&b, "%s: %d/%d passed (%.1f%%)  ·  total estimated cost: $%.6f  ·  mean routing: %.1fµs\n",
		secondaryLabel, r.SecondaryPassed, r.Total, r.SecondaryPassRate()*100, r.SecondaryTotalCostUSD, r.SecondaryMeanRoutingMic)
	fmt.Fprintf(&b, "Diff counts: route=%d fallback=%d timeout=%d verifier=%d policy_version=%d cost=%d\n",
		r.RouteChangedCount, r.FallbackChangedCount, r.TimeoutChangedCount, r.VerifierChangedCount, r.PolicyVersionChangedCount, r.CostChangedCount)
	fmt.Fprintf(&b, "Estimated cost delta (%s-%s): %+.6f USD\n", secondaryLabel, primaryLabel, r.EstimatedCostDeltaUSD)

	var changed []CaseComparison
	for _, comparison := range r.Comparisons {
		if comparison.Comparison.Changed {
			changed = append(changed, comparison)
		}
	}
	if len(changed) == 0 {
		b.WriteString("No decision differences.\n")
		return b.String()
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Case.ID < changed[j].Case.ID })
	fmt.Fprintf(&b, "\nDifferences (%d):\n", len(changed))
	for _, changedCase := range changed {
		fmt.Fprintf(&b, "  - %s (%s): %s\n",
			changedCase.Case.ID,
			changedCase.Case.Name,
			formatCaseComparison(changedCase),
		)
	}
	return b.String()
}

func caseMetadata(c Case) map[string]any {
	metadata := cloneMetadata(c.Metadata)
	if c.RouterMode == "" {
		return metadata
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["router_mode"] = c.RouterMode
	return metadata
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func formatCaseComparison(comparison CaseComparison) string {
	var parts []string
	if comparison.Comparison.RouteChanged {
		parts = append(parts, fmt.Sprintf("route %s -> %s",
			decisionLabel(comparison.Primary.decision),
			decisionLabel(comparison.Secondary.decision)))
	}
	if comparison.Comparison.FallbackChanged {
		parts = append(parts, fmt.Sprintf("fallbacks %s -> %s",
			fallbackLabel(comparison.Primary.decision.Fallbacks),
			fallbackLabel(comparison.Secondary.decision.Fallbacks)))
	}
	if comparison.Comparison.TimeoutChanged {
		parts = append(parts, fmt.Sprintf("timeout %dms -> %dms",
			comparison.Primary.decision.TimeoutMS,
			comparison.Secondary.decision.TimeoutMS))
	}
	if comparison.Comparison.RequiresVerifierChanged {
		parts = append(parts, fmt.Sprintf("verifier %t -> %t",
			comparison.Primary.decision.RequiresVerifier,
			comparison.Secondary.decision.RequiresVerifier))
	}
	if comparison.Comparison.PolicyVersionChanged {
		parts = append(parts, fmt.Sprintf("policy_version %q -> %q",
			comparison.Primary.PolicyVersion,
			comparison.Secondary.PolicyVersion))
	}
	if comparison.Comparison.CostChanged {
		parts = append(parts, fmt.Sprintf("cost $%.6f -> $%.6f (%+.6f USD)",
			comparison.Primary.EstimatedCostUSD,
			comparison.Secondary.EstimatedCostUSD,
			comparison.Comparison.EstimatedCostDeltaUSD))
	}
	if len(parts) == 0 {
		return "decision unchanged"
	}
	return strings.Join(parts, "; ")
}

func decisionLabel(dec engine.RouteDecision) string {
	if dec.Blocked {
		return "BLOCKED[" + blockedLabel(dec) + "]"
	}
	if dec.SelectedModel == "" {
		return "<none>"
	}
	return dec.SelectedModel
}

func blockedLabel(dec engine.RouteDecision) string {
	if dec.BlockCode != "" && dec.BlockReason != "" {
		return dec.BlockCode + ": " + dec.BlockReason
	}
	if dec.BlockCode != "" {
		return dec.BlockCode
	}
	if dec.BlockReason != "" {
		return dec.BlockReason
	}
	return "blocked"
}

func fallbackLabel(fallbacks []engine.FallbackEntry) string {
	if len(fallbacks) == 0 {
		return "<none>"
	}
	labels := make([]string, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		labels = append(labels, fallback.ModelID)
	}
	return strings.Join(labels, ",")
}
