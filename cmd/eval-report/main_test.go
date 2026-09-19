package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

const datasetPath = "../../evals/dataset-v1.yaml"
const (
	policySimDatasetPath = "../../evals/policy-sim-dataset.yaml"
	policySimAPath       = "../../evals/policy-sim-a.yaml"
	policySimBPath       = "../../evals/policy-sim-b.yaml"
)

func TestGenerateRunsDataset(t *testing.T) {
	report, err := generate(datasetPath)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if report.Total < 50 {
		t.Errorf("expected at least 50 cases, got %d", report.Total)
	}
	if len(report.Results) != report.Total {
		t.Errorf("results = %d, want %d", len(report.Results), report.Total)
	}
	if len(report.Frontier.TaskClasses) == 0 {
		t.Fatal("expected frontier task classes in report")
	}
}

func TestWriteReportProducesArtifacts(t *testing.T) {
	report, err := generate(datasetPath)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	dir := t.TempDir()
	if err := writeReport(report, filepath.Join(dir, "eval-report")); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	jsonPath := filepath.Join(dir, "eval-report", "report.json")
	txtPath := filepath.Join(dir, "eval-report", "report.txt")

	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var parsed evals.Report
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("report.json is not valid JSON: %v", err)
	}
	if parsed.Total != report.Total {
		t.Errorf("round-tripped total = %d, want %d", parsed.Total, report.Total)
	}
	if len(parsed.Frontier.TaskClasses) == 0 {
		t.Fatal("expected frontier task classes in report.json")
	}

	if info, err := os.Stat(txtPath); err != nil || info.Size() == 0 {
		t.Errorf("report.txt missing or empty: err=%v", err)
	}
	rawText, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read text report: %v", err)
	}
	if !strings.Contains(string(rawText), "Cost-quality frontier") {
		t.Fatalf("report.txt missing frontier section:\n%s", string(rawText))
	}
}

func TestGenerateMissingDatasetErrors(t *testing.T) {
	if _, err := generate("does-not-exist.yaml"); err == nil {
		t.Fatal("expected error for missing dataset")
	}
}

func TestGenerateComparisonRunsPolicies(t *testing.T) {
	report, err := generateComparison(policySimDatasetPath, policySimAPath, policySimBPath)
	if err != nil {
		t.Fatalf("generateComparison: %v", err)
	}
	if report.Total != 3 {
		t.Fatalf("comparison total = %d, want 3", report.Total)
	}
	if report.ChangedCount != 2 {
		t.Fatalf("comparison changed count = %d, want 2", report.ChangedCount)
	}
}

func TestWriteComparisonReportProducesArtifacts(t *testing.T) {
	report, err := generateComparison(policySimDatasetPath, policySimAPath, policySimBPath)
	if err != nil {
		t.Fatalf("generateComparison: %v", err)
	}
	dir := t.TempDir()
	if err := writeComparisonReport(report, filepath.Join(dir, "eval-report"), "policy-sim-a", "policy-sim-b"); err != nil {
		t.Fatalf("writeComparisonReport: %v", err)
	}

	jsonPath := filepath.Join(dir, "eval-report", "comparison.json")
	txtPath := filepath.Join(dir, "eval-report", "comparison.txt")

	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read comparison json: %v", err)
	}
	var parsed evals.ComparisonReport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("comparison.json is not valid JSON: %v", err)
	}
	if parsed.Total != report.Total {
		t.Errorf("round-tripped total = %d, want %d", parsed.Total, report.Total)
	}

	if info, err := os.Stat(txtPath); err != nil || info.Size() == 0 {
		t.Errorf("comparison.txt missing or empty: err=%v", err)
	}
	rawText, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read comparison text: %v", err)
	}
	if !strings.Contains(string(rawText), "policy-sim-a") || !strings.Contains(string(rawText), "policy-sim-b") {
		t.Fatalf("comparison.txt missing policy labels:\n%s", string(rawText))
	}
}
