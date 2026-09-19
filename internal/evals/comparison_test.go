package evals_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

const (
	policySimDatasetPath = "../../evals/policy-sim-dataset.yaml"
	policySimAPath       = "../../evals/policy-sim-a.yaml"
	policySimBPath       = "../../evals/policy-sim-b.yaml"
)

func TestComparePoliciesReportsDeterministicDiffs(t *testing.T) {
	runner, err := evals.NewRunner()
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	dataset, err := evals.LoadDataset(filepath.Clean(policySimDatasetPath))
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	policyA, err := evals.LoadCompiledPolicy(filepath.Clean(policySimAPath), runner.Snapshot)
	if err != nil {
		t.Fatalf("load policy A: %v", err)
	}
	policyB, err := evals.LoadCompiledPolicy(filepath.Clean(policySimBPath), runner.Snapshot)
	if err != nil {
		t.Fatalf("load policy B: %v", err)
	}

	report, err := runner.ComparePolicies(dataset, policyA, policyB)
	if err != nil {
		t.Fatalf("compare policies: %v", err)
	}
	if report.Total != 3 {
		t.Fatalf("total = %d, want 3", report.Total)
	}
	if report.ChangedCount != 2 {
		t.Fatalf("changed count = %d, want 2", report.ChangedCount)
	}
	if report.RouteChangedCount != 2 {
		t.Fatalf("route changed count = %d, want 2", report.RouteChangedCount)
	}
	if report.CostChangedCount != 2 {
		t.Fatalf("cost changed count = %d, want 2", report.CostChangedCount)
	}
	if report.PolicyVersionChangedCount != 3 {
		t.Fatalf("policy version changed count = %d, want 3", report.PolicyVersionChangedCount)
	}
	if report.EstimatedCostDeltaMicroUSD <= 0 {
		t.Fatalf("expected positive total cost delta, got %d", report.EstimatedCostDeltaMicroUSD)
	}

	first := report.Comparisons[0]
	if first.Primary.SelectedModel != "balanced-coder" {
		t.Fatalf("primary selected model = %q, want balanced-coder", first.Primary.SelectedModel)
	}
	if first.Secondary.SelectedModel != "premium-reasoning" {
		t.Fatalf("secondary selected model = %q, want premium-reasoning", first.Secondary.SelectedModel)
	}

	third := report.Comparisons[2]
	if !third.Secondary.Blocked {
		t.Fatalf("expected third case to be blocked under policy B")
	}
	if !strings.Contains(evals.FormatComparisonReport(report, "policy-a", "policy-b"), "sim_003") {
		t.Fatalf("formatted comparison report should include changed case details")
	}
}
