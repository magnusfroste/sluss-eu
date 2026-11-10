// Package answercheck reads a model's answer with the same fixed detectors
// that read the question, and names what the answer holds that the question
// did not (ISSUE-152): a key or token, or a personal identifier such as a
// personal identity number, an e-mail address, a phone number, an address,
// a card number or an IBAN.
//
// It flags; it never blocks or changes the answer. A flag is evidence for
// the request log, the inspector and the audit chain: "the model returned a
// key nobody gave it" is what a CISO wants to know about, whether it came
// from the model's training data, a tool result or a prompt injection.
//
// Only detectors that find values are used, not topic words, so an answer
// that explains what a password is does not raise a flag; an answer that
// contains one does. What the question already held is not flagged: a user
// who pastes their own e-mail address and gets it back is not a finding.
package answercheck

import (
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/pii"
	"github.com/magnusfroste/sluss/internal/secrets"
)

// MaxAnswerBytes bounds how much of an answer is read, so a very long answer
// costs a bounded amount of time. The detectors themselves cap at 64 KiB.
const MaxAnswerBytes = 256 * 1024

// Check returns the flags for an answer, sorted: "secret:<kind>" for a key or
// token (kind as the masking rules name it, e.g. api_key, aws_secret) and
// "pii:<type>" for a personal identifier (personnummer, email, phone,
// address, card, iban). Nil means nothing new.
func Check(question []openai.Message, answer string) []string {
	if strings.TrimSpace(answer) == "" {
		return nil
	}
	if len(answer) > MaxAnswerBytes {
		answer = answer[:MaxAnswerBytes]
	}
	asked := questionText(question)
	have := map[string]bool{}
	for _, k := range secrets.Detect(asked) {
		have["secret:"+k] = true
	}
	for _, f := range pii.Detect(asked) {
		have["pii:"+string(f.Type)] = true
	}

	var flags []string
	seen := map[string]bool{}
	add := func(f string) {
		if !have[f] && !seen[f] {
			seen[f] = true
			flags = append(flags, f)
		}
	}
	for _, k := range secrets.Detect(answer) {
		add("secret:" + k)
	}
	for _, f := range pii.Detect(answer) {
		// A name in an answer is ordinary: every answer about a book, a
		// law or a company has one. Names are for classifying and masking
		// the question, not for flagging the answer.
		if f.Type == pii.TypeName {
			continue
		}
		add("pii:" + string(f.Type))
	}
	sort.Strings(flags)
	return flags
}

// Classes reduces flags to the router's data classes, for a short summary:
// "secrets_possible" and "pii".
func Classes(flags []string) []string {
	var out []string
	sec, p := false, false
	for _, f := range flags {
		switch {
		case strings.HasPrefix(f, "secret:"):
			sec = true
		case strings.HasPrefix(f, "pii:"):
			p = true
		}
	}
	if sec {
		out = append(out, "secrets_possible")
	}
	if p {
		out = append(out, "pii")
	}
	return out
}

func questionText(messages []openai.Message) string {
	var b strings.Builder
	for _, m := range messages {
		// Earlier assistant turns are part of what the person already saw;
		// a value that was in an earlier answer and comes back is not new.
		if m.Content == "" {
			continue
		}
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

// Collector gathers the text of a streamed answer from its SSE chunks,
// bounded, so the check runs once the stream has ended and never delays it.
type Collector struct {
	b    strings.Builder
	full bool
}

// Add reads one chunk's JSON payload ({"choices":[{"delta":{"content":…}}]})
// and keeps its content. Anything it cannot read is ignored.
func (c *Collector) Add(data []byte) {
	if c == nil || c.full || len(data) == 0 {
		return
	}
	content := deltaContent(data)
	if content == "" {
		return
	}
	if c.b.Len()+len(content) > MaxAnswerBytes {
		content = content[:MaxAnswerBytes-c.b.Len()]
		c.full = true
	}
	c.b.WriteString(content)
}

// Text is what has been collected.
func (c *Collector) Text() string {
	if c == nil {
		return ""
	}
	return c.b.String()
}
