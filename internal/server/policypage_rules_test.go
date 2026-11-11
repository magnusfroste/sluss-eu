package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
)

// ISSUE-116: the policy console lists the active rules in plain words, with
// the rules first, then the test, and the rest under Advanced.
func TestPolicyPageShowsActiveRulesAndOrder(t *testing.T) {
	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewRuntimeCache(snap, "builtin:nis2-baseline")
	if err != nil {
		t.Fatal(err)
	}
	h := PolicyPageHandler(PolicyPageOptions{Cache: cache})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/policy", nil))
	body := rec.Body.String()
	// ISSUE-150: one list in plain words, then the test, then everything
	// else folded away. The engine's codes are not what a person reads.
	if !strings.Contains(body, "data is personal data") || !strings.Contains(body, "Keep on own hardware") {
		t.Fatal("active rules not rendered in plain words")
	}
	rules := strings.Index(body, "<h2>Your rules</h2>")
	test := strings.Index(body, "<h2>Test a sentence</h2>")
	adv := strings.Index(body, `<details class="ds-adv" data-adv="policy">`)
	packs := strings.Index(body, "<h2>Rule packs</h2>")
	if !(rules > 0 && rules < test && test < adv && adv < packs) {
		t.Fatalf("section order wrong: rules=%d test=%d advanced=%d packs=%d", rules, test, adv, packs)
	}
	// Nothing is shown twice: each rule's condition appears once in a cell.
	if n := strings.Count(body, "<td>data is personal data"); n != 1 {
		t.Fatalf("the personal-data rule is shown %d times", n)
	}
	// Codes stay out of the visible list; they remain in tooltips.
	visible := body[rules:test]
	for _, code := range []string{">data is pii", "secrets_possible<", "fail-closed"} {
		if strings.Contains(visible, code) {
			t.Errorf("the rule list shows %q", code)
		}
	}
}

// ISSUE-150: the outcome of a rule in the words of the three outcomes, and a
// rule that masks is said to mask (it used to read "route by scoring").
func TestPlainRuleWords(t *testing.T) {
	for _, c := range []struct{ kind, then, want string }{
		{"block", "BLOCK (fail-closed) — classified_blocked", "Block"},
		{"require", "only models tagged local", "Keep on own hardware"},
		{"require", "only models tagged local + on-prem", "Keep on own hardware"},
		{"require", "only models tagged eu-resident", "Only models tagged eu-resident"},
		{"mask", "mask and send", "Mask and send"},
		{"default", "default balanced tier", "Any allowed model"},
	} {
		if got := plainThen(c.kind, c.then); got != c.want {
			t.Errorf("plainThen(%q, %q) = %q, want %q", c.kind, c.then, got, c.want)
		}
	}
	if got := plainRule("data is pii or secrets_possible"); got != "data is personal data or secrets and keys" {
		t.Errorf("plainRule: %q", got)
	}
	if got := plainRule("agent can destructive or external"); got != "agent can delete things or send things out" {
		t.Errorf("plainRule agent: %q", got)
	}
	r := ConsoleRule{Sensitivities: []string{"health", "pii"}, Action: "mask"}
	if r.PlainWhen() != "data is health or personal data" || r.PlainThen() != "Mask and send" {
		t.Errorf("console rule: %q / %q", r.PlainWhen(), r.PlainThen())
	}
}
