package classifier_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

// ISSUE-167: five things the benchmark found. The examples here are new, not
// taken from benchmark/corpus.jsonl, so the benchmark still measures rules
// that were not tuned to it.
func TestBenchmarkFindings(t *testing.T) {
	classes := func(text string) map[string]bool {
		job := router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{
			Model:    "auto",
			Messages: []openai.Message{{Role: "user", Content: text}},
		}})
		out := map[string]bool{}
		for _, s := range job.AllSensitivities() {
			out[s] = true
		}
		return out
	}
	cases := []struct {
		name, text string
		want       string // a class that must be found; "" means none at all
	}{
		// 1. A word about personal data is not personal data.
		{"email as a word", "Draft an email inviting everyone to the kickoff.", ""},
		{"phone as a word", "Skriv en skylt: telefontid 9–15, phone support closed on Fridays.", ""},
		{"address as a word", "How should I address the board in the opening paragraph?", ""},
		{"an e-mail value still counts", "Draft an email to karin.ek@example.org about the kickoff.", "pii"},
		{"a phone value still counts", "Ring kunden på 073-555 12 34 om leveransen.", "pii"},

		// 2. A password label behind an underscore.
		{"prefixed password key", "Varför startar inte tjänsten?\nredis_password=Kv7nP2xLm9", "secrets_possible"},
		{"prefixed token key", "smtp_auth_token: 9f8e7d6c5b4a3f2e1d0c", "secrets_possible"},

		// 3. A street name in two words.
		{"two-word street", "Skicka provet till Ida Holm, Stora gatan 12, 411 05 Göteborg.", "pii"},
		{"two-word street, vägen", "Fakturaadress: Gamla vägen 3B, 903 30 Umeå.", "pii"},
		{"a floor is not a street", "Mötet är på plan 3, rum 12. Ta med 2 500 kr i kassan.", ""},

		// 4. One SQL statement without a fence.
		{"one-line select", "SELECT name, total FROM invoices_2024 WHERE total > 900;\nWhy is this slow?", "source_code"},
		{"one-line update", "UPDATE members SET active = 0 WHERE last_seen < '2023-01-01'", "source_code"},
		{"select in prose", "Please select a date from the list below and reply before Friday.", ""},
		{"update in prose", "Update the team on the plan and set a new meeting.", ""},

		// 5. A word about a code topic is not code, unless the task is code.
		{"an invoice letter", "Faktura #2291 på 4 800 kr är obetald; invoice reminder, write it politely.", ""},
		{"a subscription letter", "Write a friendly note that the gym subscription renews in May.", ""},
		{"architecture prose", "Is it a smell that our billing service asks the catalogue twice per order?", ""},
	}
	for _, c := range cases {
		got := classes(c.text)
		if c.want == "" && len(got) > 0 {
			t.Errorf("%s: flagged %v, want nothing", c.name, got)
		}
		if c.want != "" && !got[c.want] {
			t.Errorf("%s: got %v, want %s", c.name, got, c.want)
		}
	}
}
