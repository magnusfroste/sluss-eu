package classifier_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestClassifyTaskGoldenCases(t *testing.T) {
	tests := []struct {
		name       string
		messages   []openai.Message
		wantTask   router.TaskType
		wantSignal string
	}{
		{
			name: "commit message diff is trivial git",
			messages: []openai.Message{{
				Role: "user",
				Content: "Write a commit message for this diff:\n" +
					"diff --git a/internal/router/job.go b/internal/router/job.go\n" +
					"@@ -1 +1 @@\n-old\n+new",
			}},
			wantTask:   router.TaskTrivialGit,
			wantSignal: "git_keyword",
		},
		{
			name:       "short general question is simple chat",
			messages:   []openai.Message{{Role: "user", Content: "What is the difference between latency and throughput?"}},
			wantTask:   router.TaskSimpleChat,
			wantSignal: "short_prompt",
		},
		{
			name:       "summarize request is summarization",
			messages:   []openai.Message{{Role: "user", Content: "Summarize this incident log into three bullets."}},
			wantTask:   router.TaskSummarization,
			wantSignal: "summarize_keyword",
		},
		{
			name: "small code edit is simple code edit",
			messages: []openai.Message{{
				Role:    "user",
				Content: "Update internal/router/job.go to rename `buildJob` to `newJob`.",
			}},
			wantTask:   router.TaskSimpleCodeEdit,
			wantSignal: "edit_keyword",
		},
		{
			name: "stack trace auth path is hard code debugging",
			messages: []openai.Message{{
				Role: "user",
				Content: "Fix this auth crash in src/auth/session.ts:\n" +
					"TypeError: cannot read properties of undefined\n" +
					"    at handler (/app/src/auth/session.ts:10:5)",
			}},
			wantTask:   router.TaskHardCodeDebugging,
			wantSignal: "stack_trace",
		},
		{
			name:       "prose race condition debugging is hard code debugging",
			messages:   []openai.Message{{Role: "user", Content: "Debug this hard race condition deadlock in my concurrent Go code, stack trace attached."}},
			wantTask:   router.TaskHardCodeDebugging,
			wantSignal: "debug_keyword",
		},
		{
			name:       "prose segfault debugging is hard code debugging",
			messages:   []openai.Message{{Role: "user", Content: "My service keeps crashing with a segfault and a nil pointer dereference; help me debug it."}},
			wantTask:   router.TaskHardCodeDebugging,
			wantSignal: "debug_keyword",
		},
		{
			name: "migration sql is database migration",
			messages: []openai.Message{{
				Role: "user",
				Content: "Create db/migrations/004_add_users.sql with:\n" +
					"```sql\nALTER TABLE users ADD COLUMN deleted_at timestamptz;\n```",
			}},
			wantTask:   router.TaskDatabaseMigration,
			wantSignal: "database_migration_keyword",
		},
		{
			name:       "xss secret security review is security review",
			messages:   []openai.Message{{Role: "user", Content: "Security review this login form for XSS and secret leakage."}},
			wantTask:   router.TaskSecurityReview,
			wantSignal: "security_review_keyword",
		},
		{
			name:       "unknown production secret pii hint is high risk unknown",
			messages:   []openai.Message{{Role: "user", Content: "The production customer PII includes an API key. What now?"}},
			wantTask:   router.TaskUnknownHighRisk,
			wantSignal: "low_task_confidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := classifier.ExtractFromMessages(tt.messages, classifier.RequestHints{})
			got := classifier.ClassifyTask(features, tt.messages)

			if got.TaskType != string(tt.wantTask) {
				t.Fatalf("task type mismatch: got %q want %q; classification=%+v features=%+v", got.TaskType, tt.wantTask, got, features)
			}
			if got.Confidence < 0 || got.Confidence > 1 {
				t.Fatalf("confidence out of range: %+v", got)
			}
			if got.Confidence == 0 {
				t.Fatalf("expected non-zero confidence: %+v", got)
			}
			if len(got.Signals) == 0 {
				t.Fatalf("expected signals: %+v", got)
			}
			assertSignal(t, got.Signals, tt.wantSignal)
		})
	}
}

func TestClassifyTaskSpecificRiskWinsOverGenericCodeEdit(t *testing.T) {
	messages := []openai.Message{{
		Role: "user",
		Content: "Update db/migrations/007_accounts.sql:\n" +
			"```sql\nALTER TABLE accounts ADD COLUMN plan text;\n```",
	}}
	features := classifier.ExtractFromMessages(messages, classifier.RequestHints{})

	got := classifier.ClassifyTask(features, messages)

	if got.TaskType != string(router.TaskDatabaseMigration) {
		t.Fatalf("expected migration to win over generic edit, got %+v", got)
	}
	assertSignal(t, got.Signals, "database_migration_keyword")
}

// TestClassifyTask_Issue066_IntentAwareSecurity verifies that documentation /
// explanation intent over a security topic does not over-escalate to
// security_review, while genuine security-review requests are unaffected.
func TestClassifyTask_Issue066_IntentAwareSecurity(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantTask router.TaskType
	}{
		// Genuine security work — must stay security_review.
		{"explicit security review", "Security review this login form for XSS and secret leakage", router.TaskSecurityReview},
		{"threat model", "Threat model the new SSO integration", router.TaskSecurityReview},
		{"check for csrf vuln", "Check this auth handler for a CSRF vulnerability", router.TaskSecurityReview},
		// Documentation/explanation about a security topic — must NOT be security_review.
		{"document security modules", "Update the documentation for the security review and audit log modules", router.TaskSummarization},
		{"explain csrf protection", "Explain what CSRF protection we have in the gateway", router.TaskSummarization},
		{"changelog for vuln scanner", "Write a changelog entry for the vulnerability scanner", router.TaskSummarization},
		{"document scanning function", "Document this function that scans for vulnerabilities and exploits", router.TaskSummarization},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := []openai.Message{{Role: "user", Content: tt.content}}
			features := classifier.ExtractFromMessages(messages, classifier.RequestHints{})
			got := classifier.ClassifyTask(features, messages)
			if got.TaskType != string(tt.wantTask) {
				t.Fatalf("task=%q want %q for %q (signals=%v)", got.TaskType, tt.wantTask, tt.content, got.Signals)
			}
		})
	}
}

// TestClassifyTask_CodeGenAndDocNoun covers two dogfooding findings: code
// generation from scratch must not fall to simple_chat, and the noun "document"
// (HTML document) must not trip documentation intent.
func TestClassifyTask_CodeGenAndDocNoun(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantTask router.TaskType
	}{
		// Code generation → a code task (not simple_chat).
		{"snake with document noun", "create an HTML snake game and respond with only the raw HTML document", router.TaskSimpleCodeEdit},
		{"snake plain", "create an HTML snake game with canvas and arrow keys", router.TaskSimpleCodeEdit},
		{"python script", "write a python script that sorts a list of numbers", router.TaskSimpleCodeEdit},
		{"rest api", "build a REST API in Go with three endpoints for users", router.TaskSimpleCodeEdit},
		// Non-code "write" must NOT become a code task.
		{"blog post", "write a blog post about remote work", router.TaskSimpleChat},
		{"poem", "write a poem about the sea", router.TaskSimpleChat},
		// Genuine doc intent (verb form) still routes as summarization.
		{"document this function", "document this function with a docstring", router.TaskSummarization},
		// Code-with-clear-intent must not fall through to unknown_high_risk/premium.
		{"review dockerfile", "review my dockerfile for best practices", router.TaskSimpleCodeEdit},
		{"convert go to rust", "convert this go code to rust", router.TaskSimpleCodeEdit},
		// Questions about code must NOT be pushed to premium (no ambiguous noun → cheap).
		{"regex question stays chat", "what does this regex do", router.TaskSimpleChat},
		{"function noun stays chat", "what is the function of the liver", router.TaskSimpleChat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := []openai.Message{{Role: "user", Content: tt.content}}
			features := classifier.ExtractFromMessages(messages, classifier.RequestHints{})
			got := classifier.ClassifyTask(features, messages)
			if got.TaskType != string(tt.wantTask) {
				t.Fatalf("task=%q want %q for %q", got.TaskType, tt.wantTask, tt.content)
			}
		})
	}
}

func assertSignal(t *testing.T, signals []string, want string) {
	t.Helper()
	for _, signal := range signals {
		if signal == want {
			return
		}
	}
	t.Fatalf("expected signal %q in %#v", want, signals)
}
