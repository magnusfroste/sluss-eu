package answercheck

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func q(s string) []openai.Message { return []openai.Message{{Role: "user", Content: s}} }

// A key built at run time, so no scanner takes this file for a leak.
var fakeOpenAIKey = "sk-proj-" + strings.Repeat("a", 20) + strings.Repeat("B", 20)

func TestFlagsWhatTheAnswerAddsOnly(t *testing.T) {
	cases := []struct {
		name, question, answer string
		want                   string
	}{
		{"key the question never had", "How do I call the API?", "Use this: OPENAI_API_KEY=" + fakeOpenAIKey, "secret:api_key"},
		{"personnummer in the answer", "Who is the customer in case 4471?", "The customer is 811218-9876, Anna Svensson.", "pii:personnummer"},
		{"e-mail and phone in the answer", "How do I reach support?", "Mail anna.svensson@example.com or call 070-123 45 67.", "pii:email,pii:phone"},
		{"the user's own e-mail coming back is not new", "My address is anna.svensson@example.com, write a signature.", "Best regards, Anna — anna.svensson@example.com", ""},
		{"explaining what a password is, without one", "What makes a good password?", "A good password is long, unique and kept in a password manager.", ""},
		{"harmless", "Write a haiku about routers.", "Packets find their way / quiet lights blink in the rack / the router hums on", ""},
	}
	for _, c := range cases {
		got := strings.Join(Check(q(c.question), c.answer), ",")
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestClasses(t *testing.T) {
	if got := strings.Join(Classes([]string{"pii:email", "secret:aws_secret", "pii:phone"}), ","); got != "secrets_possible,pii" {
		t.Fatalf("Classes = %q", got)
	}
	if Classes(nil) != nil {
		t.Fatal("no flags, no classes")
	}
}

func TestCollectorReadsStreamChunks(t *testing.T) {
	var c Collector
	c.Add([]byte(`{"choices":[{"delta":{"role":"assistant","content":"Mail anna.svensson@"}}]}`))
	c.Add([]byte(`{"choices":[{"delta":{"content":"example.com today."}}]}`))
	c.Add([]byte(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3}}`))
	c.Add([]byte(`not json`))
	if c.Text() != "Mail anna.svensson@example.com today." {
		t.Fatalf("collected %q", c.Text())
	}
	if got := strings.Join(Check(q("How do I reach Anna?"), c.Text()), ","); got != "pii:email" {
		t.Fatalf("an address split across chunks must still be found: %q", got)
	}
}
