package classifier

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

// A prompt CONTAINING a personnummer (without talking about PII) must produce
// PII findings, the "pii" sensitivity hint, and risk escalation with a
// per-type reason — the content-level gap ISSUE-077 closes.
func TestPIIContentEscalatesWithoutKeywords(t *testing.T) {
	f := ExtractFromMessages([]openai.Message{
		{Role: "user", Content: "Skriv ett välkomstbrev till kunden 811218-9876 tack"},
	}, RequestHints{})

	if len(f.PIIFindings) != 1 || string(f.PIIFindings[0].Type) != "personnummer" {
		t.Fatalf("PIIFindings = %+v, want one personnummer finding", f.PIIFindings)
	}
	if !hasSensitivity(f.SensitivityHints, "pii") {
		t.Fatalf("sensitivity hints = %v, want to include pii", f.SensitivityHints)
	}

	risk := ClassifyRisk(f, "simple_chat", "")
	if risk.Sensitivity != SensitivityPII {
		t.Fatalf("sensitivity = %q, want pii", risk.Sensitivity)
	}
	if riskRank(risk.RiskLevel) < riskRank(RiskMedium) {
		t.Fatalf("risk = %q, want >= medium", risk.RiskLevel)
	}
	if !containsReason(risk.Reasons, "pii_detected:personnummer") {
		t.Fatalf("reasons = %v, want pii_detected:personnummer", risk.Reasons)
	}
}

// Findings from several messages merge (counts summed per type).
func TestPIIFindingsMergeAcrossMessages(t *testing.T) {
	f := ExtractFromMessages([]openai.Message{
		{Role: "user", Content: "kund 811218-9876"},
		{Role: "user", Content: "och 19811218-9876 samt anna@example.se"},
	}, RequestHints{})

	var pnr, email int
	for _, finding := range f.PIIFindings {
		switch string(finding.Type) {
		case "personnummer":
			pnr = finding.Count
		case "email":
			email = finding.Count
		}
	}
	if pnr != 2 || email != 1 {
		t.Fatalf("findings = %+v, want personnummer=2 email=1", f.PIIFindings)
	}
}

// A clean prompt produces no findings and no PII escalation.
func TestNoPIIFindingsOnCleanPrompt(t *testing.T) {
	f := ExtractFromMessages([]openai.Message{
		{Role: "user", Content: "Förklara skillnaden mellan slice och array i Go"},
	}, RequestHints{})
	if len(f.PIIFindings) != 0 {
		t.Fatalf("PIIFindings = %+v, want none", f.PIIFindings)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
