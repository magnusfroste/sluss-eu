package evals_test

import (
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

const regressionPath = "../../evals/regression-cases.yaml"

// TestRegressionSuite_NoMisrouting asserts that EVERY confirmed-incident case
// routes exactly as required. A single failure here fails CI (ISSUE-041).
func TestRegressionSuite_NoMisrouting(t *testing.T) {
	rn, err := evals.NewRunner()
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	ds, err := evals.LoadDataset(filepath.Clean(regressionPath))
	if err != nil {
		t.Fatalf("load regression cases: %v", err)
	}

	report, err := rn.Run(ds)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, res := range report.Results {
		if !res.Pass {
			t.Errorf("REGRESSION: %s (%s): %s", res.Case.ID, res.Case.Name, res.Reason)
		}
	}
	if report.Passed != report.Total {
		t.Fatalf("regression suite must be 100%%: %d/%d passed", report.Passed, report.Total)
	}
}
