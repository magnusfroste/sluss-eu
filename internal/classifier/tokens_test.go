package classifier

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestPromptEstimateCeilCharsOverFour(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "one char", text: "a", want: 1},
		{name: "four chars", text: "abcd", want: 1},
		{name: "five chars", text: "abcde", want: 2},
		{name: "eight chars", text: "abcdefgh", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateChatRequestTokens(&openai.ChatRequest{
				Messages: []openai.Message{{Role: "user", Content: tt.text}},
			})

			if got.CharCount != len(tt.text) {
				t.Fatalf("CharCount = %d, want %d", got.CharCount, len(tt.text))
			}
			if got.PromptTokensEstimate != tt.want {
				t.Fatalf("PromptTokensEstimate = %d, want %d", got.PromptTokensEstimate, tt.want)
			}
		})
	}
}

func TestPromptEstimateAggregatesRelevantMessages(t *testing.T) {
	got := EstimateChatRequestTokens(&openai.ChatRequest{
		Messages: []openai.Message{
			{Role: "system", Content: "abcd"},
			{Role: "developer", Content: "abcde"},
			{Role: "user", Content: "abcdefgh"},
			{Role: "assistant", Content: "a"},
			{Role: "tool", Content: "tool"},
			{Role: "unknown", Content: strings.Repeat("x", 100)},
		},
	})

	if got.CharCount != 22 {
		t.Fatalf("CharCount = %d, want 22", got.CharCount)
	}
	if got.PromptTokensEstimate != 6 {
		t.Fatalf("PromptTokensEstimate = %d, want 6", got.PromptTokensEstimate)
	}
}

func TestMaxTokensSource(t *testing.T) {
	maxTokens := 123
	got := EstimateChatRequestTokens(&openai.ChatRequest{
		MaxTokens: &maxTokens,
	})

	if got.MaxOutputTokensEstimate != maxTokens {
		t.Fatalf("MaxOutputTokensEstimate = %d, want %d", got.MaxOutputTokensEstimate, maxTokens)
	}
	if got.MaxOutputTokensSource != OutputTokenSourceMaxTokens {
		t.Fatalf("MaxOutputTokensSource = %q, want %q", got.MaxOutputTokensSource, OutputTokenSourceMaxTokens)
	}
}

func TestMaxCompletionTokensHasPriority(t *testing.T) {
	maxTokens := 123
	maxCompletionTokens := 456
	req := struct {
		Messages            []openai.Message
		MaxTokens           *int
		MaxCompletionTokens *int
	}{
		Messages:            []openai.Message{{Role: "user", Content: "abcd"}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
	}

	got := estimateChatRequestTokens(&req)

	if got.MaxOutputTokensEstimate != maxCompletionTokens {
		t.Fatalf("MaxOutputTokensEstimate = %d, want %d", got.MaxOutputTokensEstimate, maxCompletionTokens)
	}
	if got.MaxOutputTokensSource != OutputTokenSourceMaxCompletionTokens {
		t.Fatalf("MaxOutputTokensSource = %q, want %q", got.MaxOutputTokensSource, OutputTokenSourceMaxCompletionTokens)
	}
}

func TestDefaultOutputTokenSource(t *testing.T) {
	got := EstimateChatRequestTokens(&openai.ChatRequest{})

	if got.MaxOutputTokensEstimate != DefaultMaxOutputTokensEstimate {
		t.Fatalf("MaxOutputTokensEstimate = %d, want %d", got.MaxOutputTokensEstimate, DefaultMaxOutputTokensEstimate)
	}
	if got.MaxOutputTokensSource != OutputTokenSourceDefault {
		t.Fatalf("MaxOutputTokensSource = %q, want %q", got.MaxOutputTokensSource, OutputTokenSourceDefault)
	}
}

func TestNilAndEmptyRequests(t *testing.T) {
	tests := []struct {
		name string
		req  *openai.ChatRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty request", req: &openai.ChatRequest{}},
		{name: "empty messages", req: &openai.ChatRequest{Messages: []openai.Message{}}},
		{name: "empty content", req: &openai.ChatRequest{Messages: []openai.Message{{Role: "user"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateChatRequestTokens(tt.req)

			if got.CharCount != 0 {
				t.Fatalf("CharCount = %d, want 0", got.CharCount)
			}
			if got.PromptTokensEstimate != 0 {
				t.Fatalf("PromptTokensEstimate = %d, want 0", got.PromptTokensEstimate)
			}
			if got.MaxOutputTokensSource != OutputTokenSourceDefault {
				t.Fatalf("MaxOutputTokensSource = %q, want %q", got.MaxOutputTokensSource, OutputTokenSourceDefault)
			}
		})
	}
}

func BenchmarkEstimateChatRequestTokensShort(b *testing.B) {
	benchmarkEstimateChatRequestTokens(b, strings.Repeat("a", 64))
}

func BenchmarkEstimateChatRequestTokensMedium(b *testing.B) {
	benchmarkEstimateChatRequestTokens(b, strings.Repeat("a", 8*1024))
}

func BenchmarkEstimateChatRequestTokensLarge(b *testing.B) {
	benchmarkEstimateChatRequestTokens(b, strings.Repeat("a", 256*1024))
}

func benchmarkEstimateChatRequestTokens(b *testing.B, prompt string) {
	req := &openai.ChatRequest{
		Messages: []openai.Message{
			{Role: "system", Content: "You are a deterministic router classifier."},
			{Role: "user", Content: prompt},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimate := EstimateChatRequestTokens(req)
		if estimate.PromptTokensEstimate == 0 {
			b.Fatal("expected non-zero prompt token estimate")
		}
	}
}
