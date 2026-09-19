package classifier_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

// Golden cases for the richer sensitivity classes (ISSUE-093): rule-based
// detection of financial / health / legal / security_classified material, in
// English and Swedish, plus the ranking invariants that keep existing deployed
// policies (matching pii / secrets_possible) authoritative.
func TestRicherSensitivityClasses(t *testing.T) {
	tests := []struct {
		name            string
		prompt          string
		wantSensitivity router.Sensitivity
		wantAtLeastRisk router.RiskLevel
	}{
		{
			name:            "financial EN — balance sheet and payroll",
			prompt:          "Summarize the attached balance sheet and payroll data before the Q3 board meeting.",
			wantSensitivity: router.SensitivityFinancial,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "financial SV — bokslut och lönelista",
			prompt:          "Sammanfatta bokslutet och lönelistan inför styrelsemötet.",
			wantSensitivity: router.SensitivityFinancial,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "health EN — patient diagnosis and treatment plan",
			prompt:          "Draft a referral letter describing the patient's diagnosis and treatment plan.",
			wantSensitivity: router.SensitivityHealth,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "health SV — patientjournal och sjukskrivning",
			prompt:          "Sammanfatta patientjournalen och sjukskrivningen till försäkringskassan.",
			wantSensitivity: router.SensitivityHealth,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "legal EN — NDA and settlement agreement",
			prompt:          "Review this NDA and the settlement agreement and list the risks.",
			wantSensitivity: router.SensitivityLegal,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "legal SV — sekretessavtal och stämningsansökan",
			prompt:          "Granska sekretessavtalet och sammanfatta stämningsansökan.",
			wantSensitivity: router.SensitivityLegal,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "security classified EN — classified document",
			prompt:          "Translate this classified document, marked NATO restricted.",
			wantSensitivity: router.SensitivitySecurityClassified,
			wantAtLeastRisk: router.RiskHigh,
		},
		{
			name:            "security classified SV — hemligstämplat, rikets säkerhet",
			prompt:          "Dokumentet är hemligstämplat och rör rikets säkerhet.",
			wantSensitivity: router.SensitivitySecurityClassified,
			wantAtLeastRisk: router.RiskHigh,
		},
		// Ranking invariants: a new class must never displace what deployed
		// policies already match on.
		{
			name:            "health with a personnummer stays pii",
			prompt:          "Sammanfatta patientjournalen för personen med personnummer 811218-9876.",
			wantSensitivity: router.SensitivityPII,
			wantAtLeastRisk: router.RiskMedium,
		},
		{
			name:            "financial with a possible secret stays secrets_possible",
			prompt:          "The balance sheet export script has the api key hardcoded — rotate the secret.",
			wantSensitivity: router.SensitivitySecretsPossible,
			wantAtLeastRisk: router.RiskHigh,
		},
		{
			name:            "plain prompt stays none",
			prompt:          "Write a haiku about routers.",
			wantSensitivity: router.SensitivityNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := classifier.ExtractFromMessages([]openai.Message{{Role: "user", Content: tt.prompt}}, classifier.RequestHints{})
			got := classifier.ClassifyRisk(features, "summarization", "")
			if got.Sensitivity != string(tt.wantSensitivity) {
				t.Fatalf("Sensitivity = %q, want %q; result=%+v features=%+v", got.Sensitivity, tt.wantSensitivity, got, features)
			}
			if tt.wantAtLeastRisk != "" && riskRankForTest(got.RiskLevel) < riskRankForTest(string(tt.wantAtLeastRisk)) {
				t.Fatalf("RiskLevel = %q, want at least %q; result=%+v", got.RiskLevel, tt.wantAtLeastRisk, got)
			}
		})
	}
}
