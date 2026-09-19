package sdk

import (
	"net/http"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestBuildProducesCanonicalKeys(t *testing.T) {
	md := New().
		Project("prj_1").
		Tenant("tn_1").
		Task(TaskHardCodeDebugging).
		Risk(RiskHigh).
		Sensitivity(SensitivitySourceCode).
		Latency(LatencyQuality).
		Quality(QualityHigh).
		Budget(BudgetHigh).
		Mode(ModePremium).
		RequiresJSONSchema(true).
		Build()

	want := map[string]any{
		"project_id":           "prj_1",
		"tenant_id":            "tn_1",
		"task_type":            "hard_code_debugging",
		"risk_level":           "high",
		"sensitivity":          "source_code",
		"latency_preference":   "quality",
		"quality_preference":   "high",
		"budget_preference":    "high",
		"router_mode":          "premium",
		"requires_json_schema": true,
	}
	for k, v := range want {
		if md[k] != v {
			t.Errorf("metadata[%q] = %v, want %v", k, md[k], v)
		}
	}
}

func TestEmptyBuilderProducesEmptyMetadata(t *testing.T) {
	if got := New().Build(); len(got) != 0 {
		t.Errorf("empty builder should produce no metadata, got %v", got)
	}
}

// TestMetadataRoundTripsThroughRouter is the anti-drift guard: hints built by
// the SDK must be understood by the real router. project/task/risk are the
// headline acceptance criteria (ISSUE-048).
func TestMetadataRoundTripsThroughRouter(t *testing.T) {
	md := New().
		Project("prj_sdk").
		Task(TaskSecurityReview).
		Risk(RiskHigh).
		Latency(LatencyFast).
		Build()

	req := &openai.ChatRequest{
		Model:    "auto",
		Metadata: md,
		Messages: []openai.Message{{Role: "user", Content: "review this auth change"}},
	}
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: req})

	if job.ProjectIDHint != "prj_sdk" {
		t.Errorf("ProjectIDHint = %q, want prj_sdk", job.ProjectIDHint)
	}
	if job.TaskTypeHint != TaskSecurityReview {
		t.Errorf("TaskTypeHint = %q, want %q", job.TaskTypeHint, TaskSecurityReview)
	}
	if job.RiskLevelHint != RiskHigh {
		t.Errorf("RiskLevelHint = %q, want %q", job.RiskLevelHint, RiskHigh)
	}
	if job.LatencyPreference != LatencyFast {
		t.Errorf("LatencyPreference = %q, want %q", job.LatencyPreference, LatencyFast)
	}
}

// TestHeadersRoundTripThroughRouter verifies the X-Router-* header form is also
// understood by the router.
func TestHeadersRoundTripThroughRouter(t *testing.T) {
	h := New().Task(TaskTrivialGit).Risk(RiskLow).Project("prj_hdr").Headers()

	// Sanity: canonical header names present.
	if h.Get("X-Router-Task-Type") != "trivial_git" {
		t.Fatalf("header X-Router-Task-Type = %q", h.Get("X-Router-Task-Type"))
	}

	req := &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "git status"}},
	}
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: req, Headers: h})

	if job.TaskTypeHint != TaskTrivialGit {
		t.Errorf("TaskTypeHint from header = %q, want %q", job.TaskTypeHint, TaskTrivialGit)
	}
	if job.RiskLevelHint != RiskLow {
		t.Errorf("RiskLevelHint from header = %q, want %q", job.RiskLevelHint, RiskLow)
	}
	if job.ProjectIDHint != "prj_hdr" {
		t.Errorf("ProjectIDHint from header = %q, want prj_hdr", job.ProjectIDHint)
	}
}

func TestApplyIsBackwardCompatible(t *testing.T) {
	// Existing metadata is preserved; SDK hints are added.
	req := &openai.ChatRequest{Metadata: map[string]any{"task_type": "summarization", "custom": 1}}
	New().Task(TaskHardCodeDebugging).Risk(RiskHigh).Apply(req)

	if req.Metadata["task_type"] != "summarization" {
		t.Errorf("Apply must not overwrite existing key, got %v", req.Metadata["task_type"])
	}
	if req.Metadata["custom"] != 1 {
		t.Errorf("Apply must preserve unrelated keys, got %v", req.Metadata["custom"])
	}
	if req.Metadata["risk_level"] != "high" {
		t.Errorf("Apply should add new hint, got %v", req.Metadata["risk_level"])
	}
}

func TestApplyNilAndEmptyAreSafe(t *testing.T) {
	New().Task(TaskTrivialGit).Apply(nil) // must not panic

	req := &openai.ChatRequest{} // nil metadata
	New().Apply(req)             // empty builder
	if len(req.Metadata) != 0 {
		t.Errorf("empty builder should leave metadata empty, got %v", req.Metadata)
	}
}

func TestSetEscapeHatch(t *testing.T) {
	md := New().Set("experimental_flag", "x").Build()
	if md["experimental_flag"] != "x" {
		t.Errorf("Set should carry arbitrary metadata, got %v", md)
	}
	// Escape-hatch values are metadata-only, never headers.
	if h := (New().Set("experimental_flag", "x").Headers()); len(h) != 0 {
		t.Errorf("escape-hatch value should not produce headers, got %v", http.Header(h))
	}
}
