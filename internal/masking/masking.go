// Package masking replaces personal identifiers and secrets in a question
// with placeholders before it is sent to a cloud model, and puts the values
// back in the answer before the person sees it (ISSUE-151, "mask and send").
//
// Only what the detectors find with certainty is masked: personal identity
// numbers, e-mail addresses, phone numbers, IBANs, card numbers, postal
// addresses and keys or tokens. A name in free text is not: the rule that
// chose "mask and send" accepts that, and the manual says so.
//
// The values live only in this Masker, in memory, for one request. Nothing
// here logs or stores a value; Counts and Summary report types and numbers.
package masking

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/pii"
	"github.com/magnusfroste/sluss/internal/secrets"
)

// Instruction is added as a system message so the model keeps placeholders
// exactly as written; otherwise it might reformat "[EMAIL_1]" and the value
// could not be put back.
const Instruction = "Some values in this conversation were replaced with placeholders such as [EMAIL_1] or [PERSONNUMMER_1]. Keep every placeholder exactly as written, including the brackets, wherever you refer to that value. Do not ask what they stand for."

var labels = map[pii.Type]string{
	pii.TypePersonnummer: "PERSONNUMMER",
	pii.TypeEmail:        "EMAIL",
	pii.TypePhone:        "PHONE",
	pii.TypeIBAN:         "IBAN",
	pii.TypeCard:         "CARD",
	pii.TypeAddress:      "ADDRESS",
}

// rePlaceholder finds the placeholders this package writes.
var rePlaceholder = regexp.MustCompile(`\[(?:PERSONNUMMER|EMAIL|PHONE|IBAN|CARD|ADDRESS|SECRET)_\d+\]`)

// maxPlaceholder is the longest placeholder, for the stream restorer's hold-back.
const maxPlaceholder = len("[PERSONNUMMER_9999]")

// Masker holds one request's mapping. The same value always gets the same
// placeholder, in every message of the conversation.
type Masker struct {
	byValue       map[string]string // label+"\x00"+value → placeholder
	byPlaceholder map[string]string // placeholder → value
	next          map[string]int    // label → last number used
	counts        map[string]int    // type → values masked
}

// New returns an empty Masker.
func New() *Masker {
	return &Masker{byValue: map[string]string{}, byPlaceholder: map[string]string{}, next: map[string]int{}, counts: map[string]int{}}
}

func (m *Masker) placeholder(label, typ, value string) string {
	key := label + "\x00" + value
	if p, ok := m.byValue[key]; ok {
		return p
	}
	m.next[label]++
	p := fmt.Sprintf("[%s_%d]", label, m.next[label])
	m.byValue[key] = p
	m.byPlaceholder[p] = value
	m.counts[typ]++
	return p
}

// Text masks one string: secrets first (a key may hold digit runs that look
// like a phone number), then personal identifiers.
func (m *Masker) Text(s string) string {
	if s == "" {
		return s
	}
	s, _ = secrets.MaskWith(s, func(kind, value string) string {
		return m.placeholder("SECRET", "secret", value)
	})
	spans := pii.Find(s)
	if len(spans) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		// Never mask inside a placeholder this pass already wrote.
		if inPlaceholder(s, sp.Start) {
			continue
		}
		b.WriteString(s[last:sp.Start])
		b.WriteString(m.placeholder(labels[sp.Type], string(sp.Type), s[sp.Start:sp.End]))
		last = sp.End
	}
	b.WriteString(s[last:])
	return b.String()
}

func inPlaceholder(s string, at int) bool {
	open := strings.LastIndex(s[:at], "[")
	if open < 0 {
		return false
	}
	close := strings.Index(s[open:], "]")
	return close >= 0 && open+close >= at && rePlaceholder.MatchString(s[open:open+close+1])
}

// Messages returns a masked copy of the conversation: every role (an earlier
// answer may hold a value too), the text parts of array-form content, and the
// string fields of tool calls. The originals are not changed.
func (m *Masker) Messages(in []openai.Message) []openai.Message {
	out := make([]openai.Message, len(in))
	for i, msg := range in {
		msg.Content = m.Text(msg.Content)
		if len(msg.ContentParts) > 0 {
			msg.ContentParts = m.contentParts(msg.ContentParts)
		}
		if len(msg.ToolCalls) > 0 {
			calls := make([]any, len(msg.ToolCalls))
			for j, c := range msg.ToolCalls {
				calls[j] = m.anyValue(c)
			}
			msg.ToolCalls = calls
		}
		out[i] = msg
	}
	return out
}

func (m *Masker) contentParts(raw json.RawMessage) json.RawMessage {
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		// Unreadable parts are dropped rather than sent unmasked; Content
		// already carries the text.
		return nil
	}
	for _, p := range parts {
		if t, _ := p["type"].(string); t == "text" {
			if txt, ok := p["text"].(string); ok {
				p["text"] = m.Text(txt)
			}
		}
	}
	b, err := json.Marshal(parts)
	if err != nil {
		return nil
	}
	return b
}

// anyValue masks every string inside a decoded JSON value.
func (m *Masker) anyValue(v any) any {
	switch x := v.(type) {
	case string:
		return m.Text(x)
	case map[string]any:
		for k, val := range x {
			x[k] = m.anyValue(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = m.anyValue(val)
		}
		return x
	}
	return v
}

// Masked reports whether anything was replaced.
func (m *Masker) Masked() bool { return m != nil && len(m.byPlaceholder) > 0 }

// Counts is how many distinct values of each type were masked.
func (m *Masker) Counts() map[string]int {
	out := map[string]int{}
	if m == nil {
		return out
	}
	for k, v := range m.counts {
		out[k] = v
	}
	return out
}

// Summary is "email:1,personnummer:2", sorted, for headers, reasons and the
// audit chain. Types and numbers only.
func (m *Masker) Summary() string {
	c := m.Counts()
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", k, c[k])
	}
	return strings.Join(parts, ",")
}

// Restore puts the values back. A placeholder the model invented is left as
// it is.
func (m *Masker) Restore(s string) string {
	if m == nil || len(m.byPlaceholder) == 0 || !strings.Contains(s, "[") {
		return s
	}
	return rePlaceholder.ReplaceAllStringFunc(s, func(p string) string {
		if v, ok := m.byPlaceholder[p]; ok {
			return v
		}
		return p
	})
}

// Restorer restores placeholders in a streamed answer, where one placeholder
// may arrive split across two chunks: it holds back a trailing "[…" until it
// is complete or can no longer be one.
type Restorer struct {
	m       *Masker
	pending string
}

// NewRestorer returns a stream restorer for m.
func (m *Masker) NewRestorer() *Restorer { return &Restorer{m: m} }

// Add takes the next piece of the answer and returns what can be shown now.
func (r *Restorer) Add(piece string) string {
	buf := r.pending + piece
	r.pending = ""
	if i := strings.LastIndex(buf, "["); i >= 0 && !strings.Contains(buf[i:], "]") && len(buf)-i < maxPlaceholder {
		r.pending = buf[i:]
		buf = buf[:i]
	}
	return r.m.Restore(buf)
}

// Flush returns whatever is still held back, at the end of the stream.
func (r *Restorer) Flush() string {
	out := r.m.Restore(r.pending)
	r.pending = ""
	return out
}
