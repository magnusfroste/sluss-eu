package evals

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

func TestBuildFrontierReportGroupsModelsByTaskClass(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("default snapshot: %v", err)
	}

	ds := &Dataset{
		Version: "frontier-test-v1",
		Cases: []Case{
			{
				ID:            "sum_001",
				Name:          "short summary",
				TaskType:      "summarization",
				RiskLevel:     "low",
				Prompt:        "Summarize this status update in one sentence.",
				ExpectedRoute: ExpectedRoute{Tier: "cheap"},
			},
			{
				ID:            "sum_002",
				Name:          "meeting digest",
				TaskType:      "summarization",
				RiskLevel:     "low",
				Prompt:        "Summarize these meeting notes for executives.",
				ExpectedRoute: ExpectedRoute{Tier: "balanced"},
			},
			{
				ID:            "sec_001",
				Name:          "auth review",
				TaskType:      "security_review",
				RiskLevel:     "high",
				Prompt:        "Review this auth flow for security issues.",
				ExpectedRoute: ExpectedRoute{Tier: "premium"},
			},
		},
	}
	results := []CaseResult{
		{
			Case:          ds.Cases[0],
			Pass:          true,
			SelectedModel: "balanced-coder",
		},
		{
			Case:          ds.Cases[1],
			Pass:          true,
			SelectedModel: "balanced-coder",
		},
		{
			Case:          ds.Cases[2],
			Pass:          true,
			SelectedModel: "premium-reasoning",
		},
	}

	report := buildFrontierReport(ds, results, snap)
	if len(report.TaskClasses) != 2 {
		t.Fatalf("task classes = %d, want 2", len(report.TaskClasses))
	}

	summarization := report.TaskClasses[0]
	if summarization.TaskClass != "security_review" && summarization.TaskClass != "summarization" {
		t.Fatalf("unexpected first task class %q", summarization.TaskClass)
	}
	if report.TaskClasses[0].TaskClass != "security_review" || report.TaskClasses[1].TaskClass != "summarization" {
		t.Fatalf("task class order = [%s %s], want [security_review summarization]",
			report.TaskClasses[0].TaskClass, report.TaskClasses[1].TaskClass)
	}
	summarization = report.TaskClasses[1]
	if summarization.CaseCount != 2 {
		t.Fatalf("summarization case count = %d, want 2", summarization.CaseCount)
	}
	if len(summarization.Models) != 3 {
		t.Fatalf("summarization models = %d, want 3", len(summarization.Models))
	}
	if summarization.Models[0].ModelID != "cheap-general" {
		t.Fatalf("first summarization model = %q, want cheap-general", summarization.Models[0].ModelID)
	}
	if summarization.Models[1].ModelID != "balanced-coder" {
		t.Fatalf("second summarization model = %q, want balanced-coder", summarization.Models[1].ModelID)
	}
	if !summarization.Models[0].OnFrontier {
		t.Fatalf("cheap-general should be on the frontier")
	}
	if !summarization.Models[1].OnFrontier {
		t.Fatalf("balanced-coder should be on the frontier")
	}
	if summarization.Models[2].OnFrontier {
		t.Fatalf("premium-reasoning should be dominated for summarization")
	}
	if summarization.Models[1].EvalSamples != 2 {
		t.Fatalf("balanced-coder eval samples = %d, want 2", summarization.Models[1].EvalSamples)
	}
	if summarization.Models[1].FrontierQuality <= summarization.Models[0].FrontierQuality {
		t.Fatalf("balanced frontier quality = %.4f, want > cheap frontier quality %.4f",
			summarization.Models[1].FrontierQuality, summarization.Models[0].FrontierQuality)
	}
	if len(summarization.Recommendations) == 0 {
		t.Fatal("expected summarization recommendations")
	}
	if summarization.Recommendations[0].Kind != "best_value" {
		t.Fatalf("first recommendation kind = %q, want best_value", summarization.Recommendations[0].Kind)
	}
	if summarization.Recommendations[0].ModelID != "balanced-coder" {
		t.Fatalf("best_value recommendation = %q, want balanced-coder", summarization.Recommendations[0].ModelID)
	}
}

func TestBuildFrontierRecommendationsHandleEmptySingleAndTies(t *testing.T) {
	empty := recommendTaskClass(TaskClassFrontier{})
	if len(empty) != 0 {
		t.Fatalf("empty recommendations = %d, want 0", len(empty))
	}

	single := recommendTaskClass(TaskClassFrontier{
		TaskClass: "summarization",
		Models: []FrontierModel{
			{
				ModelID:                      "cheap-general",
				AverageEstimatedCostMicroUSD: 1200,
				FrontierQuality:              0.78,
				OnFrontier:                   true,
			},
		},
	})
	if len(single) != 1 {
		t.Fatalf("single recommendations = %d, want 1", len(single))
	}
	if single[0].Kind != "only_option" {
		t.Fatalf("single recommendation kind = %q, want only_option", single[0].Kind)
	}

	tied := recommendTaskClass(TaskClassFrontier{
		TaskClass: "summarization",
		Models: []FrontierModel{
			{
				ModelID:                      "alpha",
				AverageEstimatedCostMicroUSD: 1000,
				FrontierQuality:              0.80,
				OnFrontier:                   true,
			},
			{
				ModelID:                      "beta",
				AverageEstimatedCostMicroUSD: 1000,
				FrontierQuality:              0.80,
				OnFrontier:                   true,
			},
		},
	})
	if len(tied) == 0 {
		t.Fatal("expected tie recommendation")
	}
	if tied[0].ModelID != "alpha" {
		t.Fatalf("tied recommendation model = %q, want alpha", tied[0].ModelID)
	}
	if len(tied[0].TiedModelIDs) != 2 {
		t.Fatalf("tied recommendation tied ids = %d, want 2", len(tied[0].TiedModelIDs))
	}
}

func TestFormatReportIncludesFrontierSection(t *testing.T) {
	report := Report{
		Total:  2,
		Passed: 2,
		Frontier: FrontierReport{
			TaskClasses: []TaskClassFrontier{
				{
					TaskClass: "summarization",
					CaseCount: 2,
					Models: []FrontierModel{
						{
							ModelID:                      "cheap-general",
							Tier:                         "cheap",
							AverageEstimatedCostMicroUSD: 1200,
							RegistryQualityPrior:         0.78,
							FrontierQuality:              0.78,
							OnFrontier:                   true,
						},
					},
					Recommendations: []FrontierRecommendation{
						{
							Kind:    "only_option",
							ModelID: "cheap-general",
							Reason:  "only frontier model",
						},
					},
				},
			},
		},
	}

	out := FormatReport(report)
	for _, want := range []string{"Cost-quality frontier", "summarization", "cheap-general", "only_option"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, out)
		}
	}
}
