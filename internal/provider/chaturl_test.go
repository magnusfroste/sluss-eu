package provider

import "testing"

func TestChatCompletionsURLVersionSegments(t *testing.T) {
	cases := map[string]string{
		// z.ai coding: version is /v4, must NOT get /v1 appended (the bug).
		"https://api.z.ai/api/coding/paas/v4":  "https://api.z.ai/api/coding/paas/v4/chat/completions",
		"https://api.z.ai/api/coding/paas/v4/": "https://api.z.ai/api/coding/paas/v4/chat/completions",
		// OpenAI / OpenRouter / DGX: /v1 stays correct.
		"https://api.openai.com/v1":    "https://api.openai.com/v1/chat/completions",
		"https://openrouter.ai/api/v1": "https://openrouter.ai/api/v1/chat/completions",
		"https://dgx1.privai.se/v1":    "https://dgx1.privai.se/v1/chat/completions",
		// No version segment → default appends /v1/chat/completions.
		"https://provider.example.com": "https://provider.example.com/v1/chat/completions",
		// Already a full path → left as-is.
		"https://x.example.com/v1/chat/completions": "https://x.example.com/v1/chat/completions",
	}
	for in, want := range cases {
		got, err := ChatCompletionsURL(in)
		if err != nil {
			t.Errorf("ChatCompletionsURL(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ChatCompletionsURL(%q) = %q, want %q", in, got, want)
		}
	}
}
