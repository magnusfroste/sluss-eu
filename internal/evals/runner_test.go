package evals_test

import (
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

const datasetPath = "../../evals/dataset-v1.yaml"

// minPassRate is the floor the golden dataset must clear. It guards against
// routing regressions while tolerating known classifier limitations. Raise it
// as the classifier improves.
const minPassRate = 0.85

// minCases enforces the ISSUE-037 requirement of at least 50 eval cases.
const minCases = 50

func loadRunner(t *testing.T) (*evals.Runner, *evals.Dataset) {
	t.Helper()
	rn, err := evals.NewRunner()
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	ds, err := evals.LoadDataset(filepath.Clean(datasetPath))
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	return rn, ds
}

func TestDataset_HasAtLeast50Cases(t *testing.T) {
	_, ds := loadRunner(t)
	if len(ds.Cases) < minCases {
		t.Fatalf("dataset has %d cases, need at least %d", len(ds.Cases), minCases)
	}
}

func TestDataset_NoSecretsAndValid(t *testing.T) {
	// LoadDataset already runs Validate (which includes the secret scan); this
	// test documents that contract explicitly.
	_, ds := loadRunner(t)
	if err := ds.Validate(); err != nil {
		t.Fatalf("dataset validation failed: %v", err)
	}
}

func TestEvalSmoke_MeetsPassRate(t *testing.T) {
	rn, ds := loadRunner(t)
	report, err := rn.Run(ds)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	t.Log("\n" + evals.FormatReport(report))
	if report.PassRate() < minPassRate {
		t.Fatalf("eval pass rate %.1f%% below threshold %.1f%%",
			report.PassRate()*100, minPassRate*100)
	}
}

func TestEvalSmoke_RoutingIsFast(t *testing.T) {
	rn, ds := loadRunner(t)
	report, err := rn.Run(ds)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Routing must stay well under the 100ms p95 budget even on the largest
	// long-context cases. Mean is a coarse but cheap guard.
	if report.MeanRoutingMic > 100_000 {
		t.Fatalf("mean routing %.0fµs exceeds 100ms budget", report.MeanRoutingMic)
	}
}
