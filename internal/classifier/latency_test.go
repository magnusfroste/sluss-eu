package classifier

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

const (
	featureExtractionLatencySamples = 240
	featureExtractionWarmupSamples  = 24
	featureExtractionP95Budget      = 20 * time.Millisecond
)

type featureLatencyFixture struct {
	name          string
	request       openai.ChatRequest
	rawSentinels  []string
	assertSignals func(*testing.T, Features)
}

var featureExtractionBenchmarkSink Features

func TestFeatureExtractionLatency(t *testing.T) {
	fixtures := featureExtractionLatencyFixtures()
	if len(fixtures) == 0 {
		t.Fatal("expected deterministic feature extraction fixtures")
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name+"_signals", func(t *testing.T) {
			got := ExtractFromRequest(fixture.request)
			fixture.assertSignals(t, got)
			assertFeatureOutputHasNoRawPromptLeak(t, got, fixture.rawSentinels)
		})
	}
	if raceDetectorEnabled {
		t.Skip("wall-clock latency guard is measured without the race detector; signal coverage above still runs under -race")
	}

	durations := make([]time.Duration, 0, len(fixtures)*featureExtractionLatencySamples)
	for _, fixture := range fixtures {
		for i := 0; i < featureExtractionWarmupSamples; i++ {
			got := ExtractFromRequest(fixture.request)
			fixture.assertSignals(t, got)
		}

		startLen := len(durations)
		for i := 0; i < featureExtractionLatencySamples; i++ {
			start := time.Now()
			got := ExtractFromRequest(fixture.request)
			durations = append(durations, time.Since(start))
			fixture.assertSignals(t, got)
			assertFeatureOutputHasNoRawPromptLeak(t, got, fixture.rawSentinels)
		}
		t.Logf("fixture=%s samples=%d approximate_p95=%s", fixture.name, featureExtractionLatencySamples, approximateP95(durations[startLen:]))
	}

	p95 := approximateP95(durations)
	t.Logf("feature extraction samples=%d fixtures=%d approximate_p95=%s budget_lt=%s", len(durations), len(fixtures), p95, featureExtractionP95Budget)
	if p95 >= featureExtractionP95Budget {
		t.Fatalf("feature extraction approximate p95 %s exceeded documented <20ms budget from low-latency architecture and latency budget docs", p95)
	}
}

func BenchmarkFeatureExtraction(b *testing.B) {
	fixtures := featureExtractionLatencyFixtures()
	for _, fixture := range fixtures {
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				got := ExtractFromRequest(fixture.request)
				featureExtractionBenchmarkSink = got
			}
		})
	}
}

func featureExtractionLatencyFixtures() []featureLatencyFixture {
	return []featureLatencyFixture{
		{
			name: "short_prompt",
			request: openai.ChatRequest{
				Model:    "auto",
				Messages: []openai.Message{{Role: "user", Content: "Summarize this launch note in two bullets. SentinelShortPromptNeverLog"}},
			},
			rawSentinels: []string{"SentinelShortPromptNeverLog", "Summarize this launch note"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if got.RequiresCode || got.RequiresToolUse || got.RequiresJSONSchema || got.RequiresLargeContext {
					t.Fatalf("expected short prompt to stay lightweight, got %+v", got)
				}
			},
		},
		{
			name: "medium_code_prompt",
			request: openai.ChatRequest{
				Model: "auto",
				Messages: []openai.Message{{Role: "user", Content: strings.Join([]string{
					"Fix src/router/decision.go and internal/classifier/features.go. SentinelMediumCodeNeverLog",
					"```go",
					"func chooseModel(task string) string {",
					"    if task == \"simple_code_edit\" {",
					"        return \"fast-code\"",
					"    }",
					"    return \"balanced\"",
					"}",
					"```",
					"Keep the change deterministic and update the test.",
				}, "\n")}},
			},
			rawSentinels: []string{"SentinelMediumCodeNeverLog", "chooseModel"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if !got.HasCodeBlock || !got.RequiresCode {
					t.Fatalf("expected medium code prompt to require code, got %+v", got)
				}
				assertContains(t, got.FilesTouched, "src/router/decision.go")
				assertContains(t, got.FilesTouched, "internal/classifier/features.go")
				assertContains(t, got.SensitivityHints, "source_code")
			},
		},
		{
			name: "large_code_prompt",
			request: openai.ChatRequest{
				Model:    "auto",
				Messages: []openai.Message{{Role: "user", Content: largeCodePromptForLatency()}},
			},
			rawSentinels: []string{"SentinelLargeCodeNeverLog", "generatedFunction099"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if !got.RequiresCode || !got.RequiresLargeContext {
					t.Fatalf("expected large code prompt to require code and large context, got %+v", got)
				}
				if got.CodeBlockCount == 0 {
					t.Fatalf("expected large fixture code block, got %+v", got)
				}
			},
		},
		{
			name: "stack_trace",
			request: openai.ChatRequest{
				Model: "auto",
				Messages: []openai.Message{{Role: "user", Content: strings.Join([]string{
					"Debug this production panic. SentinelStackTraceNeverLog",
					"panic: nil pointer dereference",
					"goroutine 1 [running]:",
					"github.com/example/app/internal/payment.(*Handler).ServeHTTP(0x0)",
					"\tinternal/payment/handler.go:87 +0x39",
					"main.main()",
					"\tcmd/api/main.go:42 +0x91",
				}, "\n")}},
			},
			rawSentinels: []string{"SentinelStackTraceNeverLog", "nil pointer dereference"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if !got.HasStackTrace || !got.RequiresCode {
					t.Fatalf("expected stack trace code signals, got %+v", got)
				}
				assertContains(t, got.Keywords, "payment")
				assertContains(t, got.Keywords, "production")
			},
		},
		{
			name: "sql_migration",
			request: openai.ChatRequest{
				Model: "auto",
				Messages: []openai.Message{{Role: "user", Content: strings.Join([]string{
					"Create db/migrations/20260519_add_billing_events.sql. SentinelSQLMigrationNeverLog",
					"Keep payment ledger behavior compatible with Stripe checkout reconciliation.",
					"ALTER TABLE billing_events ADD COLUMN checkout_session_id text;",
					"CREATE INDEX billing_events_checkout_idx ON billing_events(checkout_session_id);",
					"Include a rollback migration plan.",
				}, "\n")}},
			},
			rawSentinels: []string{"SentinelSQLMigrationNeverLog", "checkout_session_id"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if !got.RequiresCode {
					t.Fatalf("expected SQL migration to require code, got %+v", got)
				}
				assertContains(t, got.FilesTouched, "db/migrations/20260519_add_billing_events.sql")
				assertContains(t, got.Keywords, "sql")
				assertContains(t, got.Keywords, "migration")
				assertContains(t, got.Keywords, "payment")
			},
		},
		{
			name: "auth_payment_security_keywords",
			request: openai.ChatRequest{
				Model:    "auto",
				Messages: []openai.Message{{Role: "user", Content: "Review auth session JWT checkout Stripe refund flow for csrf vulnerability and secret leakage. SentinelRiskKeywordsNeverLog"}},
			},
			rawSentinels: []string{"SentinelRiskKeywordsNeverLog", "secret leakage"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				for _, keyword := range []string{"auth", "payment", "security", "secret"} {
					assertContains(t, got.Keywords, keyword)
				}
				for _, hint := range []string{"auth", "payment", "security", "secrets_possible"} {
					assertContains(t, got.SensitivityHints, hint)
				}
			},
		},
		{
			name: "tool_json_schema_request",
			request: openai.ChatRequest{
				Model:    "auto",
				Messages: []openai.Message{{Role: "user", Content: "Use the lookup tool and return JSON matching schema. SentinelToolSchemaNeverLog"}},
				Tools: []any{map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":        "lookup_customer_status",
						"description": "Local deterministic test double metadata only.",
					},
				}},
				ResponseFormat: map[string]any{
					"type": "json_schema",
					"json_schema": map[string]any{
						"name": "routing_signals",
						"schema": map[string]any{
							"type":       "object",
							"properties": map[string]any{"status": map[string]any{"type": "string"}},
						},
					},
				},
			},
			rawSentinels: []string{"SentinelToolSchemaNeverLog", "lookup tool"},
			assertSignals: func(t *testing.T, got Features) {
				t.Helper()
				if !got.RequiresToolUse || !got.RequiresJSONSchema {
					t.Fatalf("expected tool and JSON schema requirements, got %+v", got)
				}
				assertContains(t, got.Keywords, "tool_use")
				assertContains(t, got.Keywords, "json_schema")
			},
		},
	}
}

func approximateP95(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	idx := (95*len(sorted) + 99) / 100
	if idx <= 0 {
		idx = 1
	}
	return sorted[idx-1]
}

func assertFeatureOutputHasNoRawPromptLeak(t *testing.T, got Features, rawSentinels []string) {
	t.Helper()
	serialized := fmt.Sprintf("%+v", got)
	for _, sentinel := range rawSentinels {
		if strings.Contains(serialized, sentinel) {
			t.Fatalf("feature output leaked raw prompt sentinel %q: %s", sentinel, serialized)
		}
	}
}

func largeCodePromptForLatency() string {
	var b strings.Builder
	b.WriteString("Refactor internal/router/job.go and src/auth/session.ts. SentinelLargeCodeNeverLog\n")
	b.WriteString("```ts\n")
	for i := 0; i < 360; i++ {
		fmt.Fprintf(&b, "export function generatedFunction%03d(input: string): string { return input.trim() + \"-%03d\" }\n", i, i)
	}
	b.WriteString("```\n")
	b.WriteString("Keep repository behavior deterministic across the whole codebase and long context.")
	return b.String()
}
