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
