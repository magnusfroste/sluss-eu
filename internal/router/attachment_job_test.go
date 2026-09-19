package router

import (
	"encoding/json"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

// The pitch objection, as a test (ISSUE-102): an "attached document" arrives
// as array-form content — the text parts ARE the prompt and are classified
// (PII found inside the document), and a non-text part raises requires_vision
// so policy can confine what rules cannot read.
func TestAttachedDocumentIsClassified(t *testing.T) {
	payload := `{
		"model": "auto",
		"messages": [{"role":"user","content":[
			{"type":"text","text":"Summarise the attached contract"},
			{"type":"text","text":"EMPLOYMENT AGREEMENT for Anna, personnummer 811218-9876, salary per payroll appendix"}
		]}]
	}`
	var req openai.ChatRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("array-form content must decode: %v", err)
	}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})
	if job.Sensitivity != SensitivityPII {
		t.Fatalf("document text should classify as pii, got %q", job.Sensitivity)
	}
	if len(job.PIITypes) == 0 || job.PIITypes[0] != "personnummer" {
		t.Fatalf("personnummer inside the document should be detected: %v", job.PIITypes)
	}
	if job.RequiresVision {
		t.Fatal("text-only parts must not require vision")
	}
}

func TestImageAttachmentRequiresVision(t *testing.T) {
	payload := `{
		"model": "auto",
		"messages": [{"role":"user","content":[
			{"type":"text","text":"what does this invoice scan say?"},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}
		]}]
	}`
	var req openai.ChatRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})
	if !job.RequiresVision {
		t.Fatal("an image part must set requires_vision — the policy hook for unreadable content")
	}
}
