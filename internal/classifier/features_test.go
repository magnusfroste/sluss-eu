package classifier

import (
	"fmt"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestExtractFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []openai.Message
		hints    RequestHints
		assert   func(t *testing.T, got Features)
	}{
		{
			name:     "markdown code block sets code signals",
			messages: []openai.Message{{Role: "user", Content: "Fix this:\n```go\nfunc main() {}\n```"}},
			assert: func(t *testing.T, got Features) {
				if !got.HasCodeBlock || got.CodeBlockCount != 1 || !got.RequiresCode {
					t.Fatalf("expected one code block and requires_code, got %+v", got)
				}
				assertContains(t, got.SensitivityHints, "source_code")
			},
		},
		{
			name:     "inline code count excludes fenced code",
			messages: []openai.Message{{Role: "user", Content: "Use `make test` after ```js\nconst x = 1\n``` and inspect `err`."}},
			assert: func(t *testing.T, got Features) {
				if !got.HasInlineCode || got.InlineCodeCount != 2 || got.CodeBlockCount != 1 {
					t.Fatalf("expected two inline code spans and one block, got %+v", got)
				}
			},
		},
		{
			name:     "go stack trace",
			messages: []openai.Message{{Role: "user", Content: "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n\tinternal/router/foo.go:42 +0x39"}},
			assert: func(t *testing.T, got Features) {
				if !got.HasStackTrace || got.StackTraceCount < 2 || !got.RequiresCode {
					t.Fatalf("expected go stack trace, got %+v", got)
				}
			},
		},
		{
			name:     "javascript stack trace",
			messages: []openai.Message{{Role: "user", Content: "TypeError: no\n    at handler (/app/src/auth/session.ts:10:5)\n    at /app/server.js:4:1"}},
			assert: func(t *testing.T, got Features) {
				if !got.HasStackTrace || got.StackTraceCount < 2 {
					t.Fatalf("expected js stack trace, got %+v", got)
				}
			},
		},
		{
			name:     "python stack trace",
			messages: []openai.Message{{Role: "user", Content: "Traceback (most recent call last):\n  File \"app.py\", line 7, in <module>\n    main()"}},
			assert: func(t *testing.T, got Features) {
				if !got.HasStackTrace || got.StackTraceCount < 2 {
					t.Fatalf("expected python stack trace, got %+v", got)
				}
			},
		},
		{
			name:     "filepath extraction preserves first path form and dedupes",
			messages: []openai.Message{{Role: "user", Content: "Touch src/Auth/session.ts, src/auth/session.ts and internal/router/route.go plus package.json."}},
			assert: func(t *testing.T, got Features) {
				assertEqualSlices(t, got.FilesTouched, []string{"src/Auth/session.ts", "internal/router/route.go", "package.json"})
			},
		},
		{
			name:     "sql migration keywords from prompt and path",
			messages: []openai.Message{{Role: "user", Content: "Create db/migrations/001_init.sql with ALTER TABLE users and rollback plan for schema changes."}},
			assert: func(t *testing.T, got Features) {
				assertContains(t, got.FilesTouched, "db/migrations/001_init.sql")
				assertContains(t, got.Keywords, "sql")
				assertContains(t, got.Keywords, "migration")
			},
		},
		{
			name:     "auth payment and security keywords",
			messages: []openai.Message{{Role: "user", Content: "Review auth session JWT checkout Stripe refund flow for csrf vulnerability and secret leakage."}},
			assert: func(t *testing.T, got Features) {
				for _, keyword := range []string{"auth", "payment", "security", "secret"} {
					assertContains(t, got.Keywords, keyword)
				}
				for _, hint := range []string{"auth", "payment", "security", "secrets_possible"} {
					assertContains(t, got.SensitivityHints, hint)
				}
			},
		},
		{
			name: "tool and response format request hints",
			hints: RequestHints{
				Tools:          []any{map[string]any{"type": "function"}},
				ResponseFormat: map[string]any{"type": "json_schema"},
			},
			messages: []openai.Message{{Role: "user", Content: "Classify the request."}},
			assert: func(t *testing.T, got Features) {
				if !got.RequiresToolUse || !got.RequiresJSONSchema {
					t.Fatalf("expected tool and json schema requirements, got %+v", got)
				}
				assertContains(t, got.Keywords, "tool_use")
				assertContains(t, got.Keywords, "json_schema")
			},
		},
		{
			name: "non schema response format stays non json schema",
			hints: RequestHints{
				ResponseFormat: map[string]any{"type": "text"},
			},
			messages: []openai.Message{{Role: "user", Content: "Say hello."}},
			assert: func(t *testing.T, got Features) {
				if got.RequiresJSONSchema {
					t.Fatalf("expected text response_format not to require JSON schema, got %+v", got)
				}
				assertNotContains(t, got.Keywords, "json_schema")
			},
		},
		{
			name:     "prompt json schema hint",
			messages: []openai.Message{{Role: "user", Content: "Return JSON matching schema with structured output only."}},
			assert: func(t *testing.T, got Features) {
				if !got.RequiresJSONSchema {
					t.Fatalf("expected json schema requirement, got %+v", got)
				}
				assertContains(t, got.Keywords, "json_schema")
			},
		},
		{
			name:     "vision and large context hints",
			messages: []openai.Message{{Role: "user", Content: "Analyze this screenshot and the whole repository with long context."}},
			assert: func(t *testing.T, got Features) {
				if !got.RequiresVision || !got.RequiresLargeContext {
					t.Fatalf("expected vision and large context hints, got %+v", got)
				}
			},
		},
		{
			name:     "keyword and file caps dedupe deterministically",
			messages: []openai.Message{{Role: "user", Content: cappedPrompt()}},
			assert: func(t *testing.T, got Features) {
				if len(got.FilesTouched) != MaxFilesTouched {
					t.Fatalf("expected file cap %d, got %d: %#v", MaxFilesTouched, len(got.FilesTouched), got.FilesTouched)
				}
				if len(got.Keywords) > MaxKeywords {
					t.Fatalf("keywords exceeded cap %d: %#v", MaxKeywords, got.Keywords)
				}
				assertContains(t, got.Keywords, "auth")
				assertContains(t, got.Keywords, "payment")
				assertContains(t, got.Keywords, "security")
			},
		},
		{
			name:     "safe output has no raw prompt leakage",
			messages: []openai.Message{{Role: "user", Content: "The private phrase AlphaBetaUnique must never leave. Fix src/auth/session.ts and return JSON."}},
			assert: func(t *testing.T, got Features) {
				serialized := fmt.Sprintf("%+v", got)
				if strings.Contains(serialized, "AlphaBetaUnique") || strings.Contains(serialized, "private phrase") {
					t.Fatalf("raw prompt leaked into features: %s", serialized)
				}
				assertContains(t, got.FilesTouched, "src/auth/session.ts")
				assertContains(t, got.Keywords, "json_schema")
			},
		},
		{
			name:     "prose debugging terms set code requirement without a stack trace",
			messages: []openai.Message{{Role: "user", Content: "Debug this hard race condition deadlock in my concurrent Go code, stack trace attached."}},
			assert: func(t *testing.T, got Features) {
				if got.HasStackTrace {
					t.Fatalf("expected no literal stack trace from prose, got %+v", got)
				}
				if !got.RequiresCode {
					t.Fatalf("expected prose debugging terms to require code, got %+v", got)
				}
				assertContains(t, got.Keywords, "debug")
				assertContains(t, got.Keywords, "code")
				assertContains(t, got.SensitivityHints, "source_code")
			},
		},
		{
			name:     "individual debugging symptoms require code",
			messages: []openai.Message{{Role: "user", Content: "My service keeps crashing with a segfault and a nil pointer dereference at startup."}},
			assert: func(t *testing.T, got Features) {
				if !got.RequiresCode {
					t.Fatalf("expected debugging symptoms to require code, got %+v", got)
				}
				assertContains(t, got.Keywords, "debug")
			},
		},
		{
			name:     "coding intent prose requires code",
			messages: []openai.Message{{Role: "user", Content: "The goroutine deadlocks because the mutex is not thread-safe; help me fix the compile error."}},
			assert: func(t *testing.T, got Features) {
				if !got.RequiresCode {
					t.Fatalf("expected coding intent prose to require code, got %+v", got)
				}
				assertContains(t, got.Keywords, "code")
			},
		},
		{
			name:     "non code prose stays non code",
			messages: []openai.Message{{Role: "user", Content: "Can you recommend a good restaurant downtown for dinner tonight?"}},
			assert: func(t *testing.T, got Features) {
				assertNotContains(t, got.Keywords, "code")
				assertNotContains(t, got.Keywords, "debug")
				if got.RequiresCode {
					t.Fatalf("expected benign prose to stay non-code, got %+v", got)
				}
			},
		},
		{
			name:     "single word keywords do not match inside product names",
			messages: []openai.Message{{Role: "user", Content: "Explain the tokenizer routing overview without changing code."}},
			assert: func(t *testing.T, got Features) {
				assertNotContains(t, got.Keywords, "auth")
				assertNotContains(t, got.Keywords, "secret")
				if got.RequiresCode {
					t.Fatalf("expected no code requirement from benign product wording, got %+v", got)
				}
			},
		},
		{
			name:     "generic update wording is not sql",
			messages: []openai.Message{{Role: "user", Content: "Can you update me on the launch plan?"}},
			assert: func(t *testing.T, got Features) {
				assertNotContains(t, got.Keywords, "sql")
				assertNotContains(t, got.Keywords, "migration")
				if got.RequiresCode {
					t.Fatalf("expected benign update wording to stay non-code, got %+v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFromMessages(tt.messages, tt.hints)
			tt.assert(t, got)
		})
	}
}

func TestExtractFromRequestUsesChatRequestFields(t *testing.T) {
	got := ExtractFromRequest(openai.ChatRequest{
		Model:          "test-model",
		Messages:       []openai.Message{{Role: "user", Content: "Use auth flow and return JSON."}},
		Tools:          []any{map[string]any{"type": "function", "name": "lookup"}},
		ResponseFormat: map[string]any{"type": "json_schema"},
	})

	if !got.RequiresToolUse || !got.RequiresJSONSchema {
		t.Fatalf("expected request fields and prompt signals to drive features, got %+v", got)
	}
	assertContains(t, got.Keywords, "auth")
	assertContains(t, got.Keywords, "json_schema")
	assertContains(t, got.Keywords, "tool_use")
}

func cappedPrompt() string {
	var b strings.Builder
	b.WriteString("auth payment security vulnerability jwt checkout stripe csrf ")
	for i := 0; i < MaxFilesTouched+8; i++ {
		fmt.Fprintf(&b, "src/pkg/file_%02d.go ", i)
	}
	b.WriteString("AUTH auth payment security")
	return b.String()
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %q in %#v", want, values)
}

func assertNotContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			t.Fatalf("did not expect %q in %#v", want, values)
		}
	}
}

func assertEqualSlices(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length mismatch: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice mismatch: got %#v want %#v", got, want)
		}
	}
}
