package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/router"
)

func sampleFrontier() FrontierReport {
	return FrontierReport{TaskClasses: []TaskClassFrontier{
		{
			TaskClass: "simple_code_edit",
			Models: []FrontierModel{
				{ModelID: "cheap-general", EvalSamples: 5, EvalPassed: 5, EvalPassRate: 1.0},
				{ModelID: "balanced-coder", EvalSamples: 3, EvalPassed: 2, EvalPassRate: 0.667},
				{ModelID: "barely-tested", EvalSamples: 1, EvalPassed: 1, EvalPassRate: 1.0},
			},
		},
	}}
}

func TestMeasuredQualityLookup(t *testing.T) {
	mq := NewMeasuredQuality(sampleFrontier(), 2) // require >= 2 samples

	if s, ok := mq.Quality(router.TaskSimpleCodeEdit, "cheap-general"); !ok || s != 1.0 {
		t.Errorf("cheap-general = %v,%v want 1.0,true", s, ok)
	}
	if s, ok := mq.Quality(router.TaskSimpleCodeEdit, "balanced-coder"); !ok || s != 0.667 {
		t.Errorf("balanced-coder = %v,%v want 0.667,true", s, ok)
	}
	// barely-tested has only 1 sample (< minSamples) → excluded.
	if _, ok := mq.Quality(router.TaskSimpleCodeEdit, "barely-tested"); ok {
		t.Error("under-sampled model should be excluded")
	}
	// unknown task/model → false.
	if _, ok := mq.Quality(router.TaskSecurityReview, "cheap-general"); ok {
		t.Error("unknown task should return false")
	}
	if mq.TaskClasses() != 1 {
		t.Errorf("task classes = %d, want 1", mq.TaskClasses())
	}
}

func TestMeasuredQualityNilSafe(t *testing.T) {
	var mq *MeasuredQuality
	if _, ok := mq.Quality(router.TaskSimpleChat, "x"); ok {
		t.Error("nil MeasuredQuality should return false")
	}
	if mq.TaskClasses() != 0 {
		t.Error("nil MeasuredQuality TaskClasses should be 0")
	}
}

func TestLoadReportRoundTrip(t *testing.T) {
	report := Report{Total: 2, Frontier: sampleFrontier()}
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	raw, _ := json.Marshal(report)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := LoadReport(path)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}
	if len(got.Frontier.TaskClasses) != 1 {
		t.Fatalf("loaded frontier task classes = %d, want 1", len(got.Frontier.TaskClasses))
	}
	mq := NewMeasuredQuality(got.Frontier, 1)
	if s, ok := mq.Quality(router.TaskSimpleCodeEdit, "cheap-general"); !ok || s != 1.0 {
		t.Errorf("round-tripped quality = %v,%v", s, ok)
	}
}

func TestLoadReportMissingFile(t *testing.T) {
	if _, err := LoadReport("does-not-exist.json"); err == nil {
		t.Fatal("expected error for missing report")
	}
}
