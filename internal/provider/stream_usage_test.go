package provider

import "testing"

func TestParseSSEDataLineExtractsUsage(t *testing.T) {
	// A normal content chunk: no usage.
	c, ok := parseSSEDataLine(`data: {"choices":[{"delta":{"content":"hi"}}]}`)
	if !ok || c.InputTokens != 0 || c.OutputTokens != 0 {
		t.Fatalf("content chunk should carry no usage: %+v", c)
	}
	// The final usage chunk (include_usage): tokens captured.
	c, ok = parseSSEDataLine(`data: {"choices":[],"usage":{"prompt_tokens":21,"completion_tokens":361}}`)
	if !ok || c.InputTokens != 21 || c.OutputTokens != 361 {
		t.Fatalf("usage chunk not parsed: %+v", c)
	}
	// [DONE] terminator.
	if c, ok := parseSSEDataLine("data: [DONE]"); !ok || !c.Done {
		t.Fatalf("expected done marker, got %+v", c)
	}
}
