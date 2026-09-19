package classifier_test

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestClassifyRisk(t *testing.T) {
	tests := []struct {
		name            string
		prompt          string
		taskType        router.TaskType
		clientRiskHint  router.RiskLevel
		wantAtLeastRisk router.RiskLevel
		wantRisk        router.RiskLevel
		wantSensitivity router.Sensitivity
	}{
		{
			name:            "auth path and code edit is high",
			prompt:          "Edit src/auth/session.ts and fix the session validation code.",
			taskType:        router.TaskSimpleCodeEdit,
			wantAtLeastRisk: router.RiskHigh,
			wantSensitivity: router.SensitivitySourceCode,
		},
		{
			name:            "stripe checkout payment is high",
			prompt:          "Implement the Stripe checkout payment flow and update billing refunds.",
			taskType:        router.TaskSimpleCodeEdit,
			wantAtLeastRisk: router.RiskHigh,
			wantSensitivity: router.SensitivitySourceCode,
		},
		{
			name:            "sql migration rollback is high",
			prompt:          "Create db/migrations/20260519_add_customer.sql with ALTER TABLE and a rollback plan.",
			taskType:        router.TaskDatabaseMigration,
			wantRisk:        router.RiskHigh,
			wantSensitivity: router.SensitivitySourceCode,
		},
		{
			name:            "xss security review is high",
			prompt:          "Do a security review for XSS and csrf vulnerability in the login form.",
			taskType:        router.TaskSecurityReview,
			wantAtLeastRisk: router.RiskHigh,
			wantSensitivity: router.SensitivitySourceCode,
		},
		{
			name:            "production outage with secret and exploit is critical",
			prompt:          "Production outage: exploited secret token leak in auth. Urgent hotfix ASAP.",
			taskType:        router.TaskSecurityReview,
			wantRisk:        router.RiskCritical,
			wantSensitivity: router.SensitivitySecretsPossible,
		},
		{
			name:            "pii raises risk and sensitivity",
			prompt:          "Summarize customer data with email, phone, address, and personnummer fields.",
			taskType:        router.TaskSummarization,
			wantAtLeastRisk: router.RiskMedium,
			wantSensitivity: router.SensitivityPII,
		},
		{
			name:            "low client risk hint cannot lower auth payment risk",
			prompt:          "Low risk please: change auth checkout payment handling in src/auth/session.ts.",
			taskType:        router.TaskSimpleCodeEdit,
			clientRiskHint:  router.RiskLow,
			wantAtLeastRisk: router.RiskHigh,
			wantSensitivity: router.SensitivitySourceCode,
		},
		{
			name:            "unknown high risk is never low",
			prompt:          "Classify this unclear request.",
			taskType:        router.TaskUnknownHighRisk,
			wantAtLeastRisk: router.RiskHigh,
			wantSensitivity: router.SensitivityNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := classifier.ExtractFromMessages([]openai.Message{{Role: "user", Content: tt.prompt}}, classifier.RequestHints{})
			got := classifier.ClassifyRisk(features, string(tt.taskType), string(tt.clientRiskHint))

			if tt.wantRisk != "" && got.RiskLevel != string(tt.wantRisk) {
				t.Fatalf("RiskLevel = %q, want %q; result=%+v features=%+v", got.RiskLevel, tt.wantRisk, got, features)
			}
			if tt.wantAtLeastRisk != "" && riskRankForTest(got.RiskLevel) < riskRankForTest(string(tt.wantAtLeastRisk)) {
				t.Fatalf("RiskLevel = %q, want at least %q; result=%+v features=%+v", got.RiskLevel, tt.wantAtLeastRisk, got, features)
			}
			if got.Sensitivity != string(tt.wantSensitivity) {
				t.Fatalf("Sensitivity = %q, want %q; result=%+v features=%+v", got.Sensitivity, tt.wantSensitivity, got, features)
			}
			assertNoRawPromptText(t, got.Reasons, tt.prompt)
			assertNoRawPromptText(t, got.Signals, tt.prompt)
		})
	}
}

func TestClassifyRiskClientHintMayOnlyEscalate(t *testing.T) {
	features := classifier.ExtractFromMessages([]openai.Message{{Role: "user", Content: "Say hello."}}, classifier.RequestHints{})
	got := classifier.ClassifyRisk(features, string(router.TaskSimpleChat), string(router.RiskCritical))

	if got.RiskLevel != string(router.RiskCritical) {
		t.Fatalf("RiskLevel = %q, want %q", got.RiskLevel, router.RiskCritical)
	}
	assertContainsReason(t, got.Reasons, "client_risk_hint")
}

func assertNoRawPromptText(t *testing.T, reasons []string, prompt string) {
	t.Helper()
	for _, reason := range reasons {
		if strings.Contains(prompt, reason) {
			t.Fatalf("reason %q looks prompt-derived; reasons=%#v prompt=%q", reason, reasons, prompt)
		}
		for _, raw := range []string{"src/auth/session.ts", "Stripe", "ALTER TABLE", "Production outage", "personnummer"} {
			if strings.Contains(reason, raw) {
				t.Fatalf("reason %q contains raw prompt text", reason)
			}
		}
	}
}

func assertContainsReason(t *testing.T, reasons []string, want string) {
	t.Helper()
	for _, reason := range reasons {
		if reason == want {
			return
		}
	}
	t.Fatalf("expected reason %q in %#v", want, reasons)
}

func riskRankForTest(level string) int {
	switch level {
	case string(router.RiskCritical):
		return 4
	case string(router.RiskHigh):
		return 3
	case string(router.RiskMedium):
		return 2
	case string(router.RiskLow):
		return 1
	default:
		return 0
	}
}
