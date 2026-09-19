package server

import (
	"strings"
	"testing"
)

func TestReportCarriesRegimeProfile(t *testing.T) {
	r := BuildComplianceReport(ReportOptions{Profile: "dora"})
	if r.Profile != "dora" {
		t.Fatalf("profile=%q", r.Profile)
	}
	md := RenderComplianceMarkdown(r)
	for _, want := range []string{"DORA", "Regime profile", "not a legal attestation"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q", want)
		}
	}
	// Derivation: nis2 pack version → nis2 wording without explicit key.
	r2 := BuildComplianceReport(ReportOptions{})
	if r2.Profile != "eu" {
		t.Fatalf("empty should derive eu, got %q", r2.Profile)
	}
}
