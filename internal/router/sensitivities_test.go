package router

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

// A prompt holding several data classes keeps all of them (ISSUE-158), the
// strongest first, while Sensitivity stays the strongest one.
func TestJobKeepsEveryDataClass(t *testing.T) {
	req := openai.ChatRequest{Model: "auto", Messages: []openai.Message{{Role: "user",
		Content: "HEMLIG enligt säkerhetsskyddslagen. Deploy fails with AKIAQ3EGTRWKX5VL7P2N for customer 811218-9876"}}}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})
	if job.Sensitivity != SensitivitySecretsPossible {
		t.Fatalf("primary = %q, want secrets_possible", job.Sensitivity)
	}
	want := []Sensitivity{SensitivitySecretsPossible, SensitivityPII, SensitivitySecurityClassified}
	got := job.Sensitivities
	if len(got) < len(want) {
		t.Fatalf("sensitivities = %v, want at least %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("sensitivities = %v, want %v first", got, want)
		}
	}
}

// Words about code topics ("invoice" reads as payment) do not add source_code
// to the list; only real code does.
func TestDataClassesNoCodeFromTopicWords(t *testing.T) {
	req := openai.ChatRequest{Model: "auto", Messages: []openai.Message{{Role: "user",
		Content: "Summarise the complaint from customer 811218-9876 about a late invoice"}}}
	job := NewJobDescriptor(JobDescriptorInput{Request: &req})
	for _, s := range job.Sensitivities {
		if s == SensitivitySourceCode {
			t.Fatalf("a customer letter is not code: %v", job.Sensitivities)
		}
	}
	req.Messages[0].Content = "Fix this payment handler:\n```go\nfunc charge(c Card) error { return nil }\n```"
	job = NewJobDescriptor(JobDescriptorInput{Request: &req})
	found := false
	for _, s := range job.Sensitivities {
		found = found || s == SensitivitySourceCode
	}
	if !found {
		t.Fatalf("real code must still be source_code: %v", job.Sensitivities)
	}
}
