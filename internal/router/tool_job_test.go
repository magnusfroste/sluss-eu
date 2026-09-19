package router

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

// ISSUE-111 end-to-end through the descriptor: an agent request declaring a
// destructive tool must surface tool_risk on the JobDescriptor, which is what
// policy gates on before any provider is called.
func TestJobDescriptorCarriesToolRisk(t *testing.T) {
	req := openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "clean up the stale test accounts"}},
		Tools: []any{
			map[string]any{"type": "function", "function": map[string]any{
				"name": "search_accounts", "description": "Find accounts"}},
			map[string]any{"type": "function", "function": map[string]any{
				"name": "delete_account", "description": "Permanently remove an account"}},
		},
	}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})

	if !job.RequiresToolUse {
		t.Fatal("requires_tool_use should be set")
	}
	if job.ToolRisk != "destructive" {
		t.Fatalf("tool_risk = %q, want destructive", job.ToolRisk)
	}
	if len(job.ToolsDeclared) != 2 {
		t.Fatalf("tools_declared = %v, want both names", job.ToolsDeclared)
	}
	// The descriptor is evidence: it records capability, never arguments.
	m := job.SafeLogFields()
	if m["tool_risk"] != "destructive" {
		t.Fatalf("safe log fields missing tool_risk: %v", m["tool_risk"])
	}
}

// A normal chat prompt must stay untouched — no tools, no tool risk.
func TestJobDescriptorNoToolsNoRisk(t *testing.T) {
	req := openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "summarise this"}},
	}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})
	if job.ToolRisk != "" || len(job.ToolsDeclared) != 0 {
		t.Fatalf("expected no tool risk, got %q %v", job.ToolRisk, job.ToolsDeclared)
	}
}
