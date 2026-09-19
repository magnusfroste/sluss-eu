package router

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

// Golden case for ISSUE-077: a request whose prompt CONTAINS a personnummer is
// classified sensitivity=pii with escalated risk and the detected types on the
// JobDescriptor — the signal policy needs to block or force residency.
func TestNewJobDescriptorEscalatesOnDetectedPersonnummer(t *testing.T) {
	job := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_pii",
		Request: &openai.ChatRequest{
			Model: "auto",
			Messages: []openai.Message{
				{Role: "user", Content: "Sammanfatta ärendet för kund 811218-9876 kortfattat"},
			},
		},
	})

	if job.Sensitivity != SensitivityPII {
		t.Fatalf("sensitivity = %q, want pii", job.Sensitivity)
	}
	if job.RiskLevel != RiskMedium && job.RiskLevel != RiskHigh && job.RiskLevel != RiskCritical {
		t.Fatalf("risk = %q, want >= medium", job.RiskLevel)
	}
	if len(job.PIITypes) != 1 || job.PIITypes[0] != "personnummer" {
		t.Fatalf("pii types = %v, want [personnummer]", job.PIITypes)
	}
}

// The same prompt without the personnummer stays non-PII — the detector, not
// the surrounding words, drives the escalation.
func TestNewJobDescriptorCleanPromptStaysNonPII(t *testing.T) {
	job := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_clean",
		Request: &openai.ChatRequest{
			Model: "auto",
			Messages: []openai.Message{
				{Role: "user", Content: "Sammanfatta ärendet för kunden kortfattat"},
			},
		},
	})
	if job.Sensitivity == SensitivityPII {
		t.Fatalf("clean prompt classified as pii: %+v", job)
	}
	if len(job.PIITypes) != 0 {
		t.Fatalf("pii types = %v, want none", job.PIITypes)
	}
}
