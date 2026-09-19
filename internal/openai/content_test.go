package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

// String content — the common case — is untouched and round-trips as a string.
func TestMessageStringContentRoundTrip(t *testing.T) {
	in := `{"role":"user","content":"hello there"}`
	var m Message
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Content != "hello there" || m.ContentParts != nil || m.NonTextParts {
		t.Fatalf("string content mishandled: %+v", m)
	}
	out, _ := json.Marshal(m)
	if !strings.Contains(string(out), `"content":"hello there"`) {
		t.Fatalf("marshal should emit string content: %s", out)
	}
}

// Array-form text-only content (several SDKs send this even for plain chat):
// the text parts are concatenated into Content — the classifier sees the full
// document — and the ORIGINAL array is re-emitted to the provider.
func TestMessageArrayTextContent(t *testing.T) {
	in := `{"role":"user","content":[{"type":"text","text":"Summarise the contract:"},{"type":"text","text":"personnummer 811218-9876"}]}`
	var m Message
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Content != "Summarise the contract:\npersonnummer 811218-9876" {
		t.Fatalf("text parts not concatenated: %q", m.Content)
	}
	if m.NonTextParts {
		t.Fatal("text-only parts must not raise the attachment flag")
	}
	out, _ := json.Marshal(m)
	if !strings.Contains(string(out), `"type":"text"`) || strings.Contains(string(out), `"content":"Summarise`) {
		t.Fatalf("marshal should re-emit the original array: %s", out)
	}
}

// A non-text part (image) raises the attachment flag; text parts still feed
// Content; the raw array (incl. the image payload) reaches the provider.
func TestMessageArrayWithImagePart(t *testing.T) {
	in := `{"role":"user","content":[{"type":"text","text":"what is in this scan?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}`
	var m Message
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !m.NonTextParts {
		t.Fatal("image part should set NonTextParts")
	}
	if m.Content != "what is in this scan?" {
		t.Fatalf("text part lost: %q", m.Content)
	}
	out, _ := json.Marshal(m)
	if !strings.Contains(string(out), "image_url") || !strings.Contains(string(out), "base64,AAAA") {
		t.Fatalf("image payload must round-trip to the provider: %s", out)
	}
}

// Invalid content shapes are rejected with a clean error, not silently dropped.
func TestMessageInvalidContent(t *testing.T) {
	for _, in := range []string{
		`{"role":"user","content":42}`,
		`{"role":"user","content":[42]}`,
	} {
		var m Message
		if err := json.Unmarshal([]byte(in), &m); err == nil {
			t.Fatalf("invalid content should error: %s", in)
		}
	}
	// Absent/null content is fine (assistant tool-call messages).
	var m Message
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":null}`), &m); err != nil {
		t.Fatalf("null content should be accepted: %v", err)
	}
}

// Tool fields survive the custom (un)marshal.
func TestMessageToolFieldsRoundTrip(t *testing.T) {
	in := `{"role":"tool","content":"result","tool_call_id":"tc1","name":"lookup"}`
	var m Message
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.ToolCallID != "tc1" || m.Name != "lookup" {
		t.Fatalf("tool fields lost: %+v", m)
	}
	out, _ := json.Marshal(m)
	for _, want := range []string{`"tool_call_id":"tc1"`, `"name":"lookup"`, `"content":"result"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("marshal missing %q: %s", want, out)
		}
	}
}
