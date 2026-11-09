package masking

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/pii"
)

var fakeGitHub = "ghp_" + strings.Repeat("A", 36)

func TestMaskAndRestoreRoundTrip(t *testing.T) {
	m := New()
	in := "Customer 811218-2392 (anna.svensson@example.com, 070-123 45 67) paid from SE45 5000 0000 0583 9825 7466. Token " + fakeGitHub + ". Mail anna.svensson@example.com again."
	out := m.Text(in)
	for _, leak := range []string{"811218-2392", "anna.svensson@example.com", "070-123 45 67", "SE45", fakeGitHub} {
		if strings.Contains(out, leak) {
			t.Fatalf("masked text still holds %q: %s", leak, out)
		}
	}
	if strings.Count(out, "[EMAIL_1]") != 2 {
		t.Fatalf("the same value must get the same placeholder: %s", out)
	}
	if got := m.Restore(out); got != in {
		t.Fatalf("restore did not round-trip:\n got %s\nwant %s", got, in)
	}
	if s := m.Summary(); s != "email:1,iban:1,personnummer:1,phone:1,secret:1" {
		t.Fatalf("summary = %q", s)
	}
	if strings.Contains(m.Summary(), "@") {
		t.Fatal("the summary must never carry a value")
	}
}

func TestMaskWholeLongText(t *testing.T) {
	m := New()
	in := strings.Repeat("filler text ", 10000) + " contact anna.svensson@example.com"
	if len(in) < 64*1024 {
		t.Fatal("test text should be longer than the detectors' classification limit")
	}
	if out := m.Text(in); strings.Contains(out, "anna.svensson@example.com") {
		t.Fatal("masking must scan the whole text, not stop at 64 KiB")
	}
}

func TestMaskMessagesEveryRoleContentPartsAndToolCalls(t *testing.T) {
	var parts json.RawMessage = json.RawMessage(`[{"type":"text","text":"call 070-123 45 67"},{"type":"image_url","image_url":{"url":"data:x"}}]`)
	in := []openai.Message{
		{Role: "user", Content: "my number is 811218-2392"},
		{Role: "assistant", Content: "Noted, 811218-2392."},
		{Role: "user", Content: "call 070-123 45 67", ContentParts: parts},
		{Role: "assistant", ToolCalls: []any{map[string]any{"function": map[string]any{"arguments": `{"to":"anna.svensson@example.com"}`}}}},
	}
	m := New()
	out := m.Messages(in)
	b, _ := json.Marshal(out)
	for _, leak := range []string{"811218-2392", "070-123 45 67", "anna.svensson@example.com"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("masked conversation still holds %q: %s", leak, b)
		}
	}
	if !strings.Contains(string(b), `"image_url"`) {
		t.Fatal("non-text parts must be kept")
	}
	if in[0].Content != "my number is 811218-2392" {
		t.Fatal("the original messages must not be changed")
	}
}

func TestRestorerHandlesSplitPlaceholders(t *testing.T) {
	m := New()
	_ = m.Text("mail anna.svensson@example.com or 811218-2392")
	r := m.NewRestorer()
	var got strings.Builder
	for _, piece := range []string{"Write to [EM", "AIL_1] and file it under [PERSON", "NUMMER_1]. Done [x"} {
		got.WriteString(r.Add(piece))
	}
	got.WriteString(r.Flush())
	want := "Write to anna.svensson@example.com and file it under 811218-2392. Done [x"
	if got.String() != want {
		t.Fatalf("got  %q\nwant %q", got.String(), want)
	}
}

func TestUnknownPlaceholderIsLeft(t *testing.T) {
	m := New()
	_ = m.Text("mail anna.svensson@example.com")
	if got := m.Restore("see [EMAIL_7] and [EMAIL_1]"); got != "see [EMAIL_7] and anna.svensson@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestFindMatchesDetect(t *testing.T) {
	text := "811218-2392, anna.svensson@example.com, 070-123 45 67, Storgatan 12, 111 22 Stockholm"
	types := map[pii.Type]bool{}
	for _, sp := range pii.Find(text) {
		types[sp.Type] = true
	}
	for _, f := range pii.Detect(text) {
		if !types[f.Type] {
			t.Errorf("Detect found %s but Find did not", f.Type)
		}
	}
}
