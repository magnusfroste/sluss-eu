package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

func TestAttentionNamesFailingModelAndSetup(t *testing.T) {
	ring := eventlog.NewErrorRing(10)
	now := time.Now()
	for i := 0; i < 3; i++ {
		ring.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
			ProviderID: "dgx", ModelID: "glm-local", Success: false, ErrorCode: "model_unavailable", AttemptedAt: now.Add(-time.Minute),
		}})
	}
	d := &DashboardData{}
	items := buildAttention(DashboardOptions{ErrorRing: ring}, d, now)
	var failing, setup bool
	for _, it := range items {
		if it.Level == "bad" && strings.Contains(it.Title, "glm-local on dgx is failing") && strings.Contains(it.Detail, "3 failed requests") && it.Href == "/router/models?m=glm-local" {
			failing = true
		}
		if strings.Contains(it.Title, "Setup is not finished") {
			setup = true
		}
	}
	if !failing || !setup {
		t.Fatalf("attention = %+v", items)
	}
	// Old failures do not count.
	if got := buildAttention(DashboardOptions{ErrorRing: ring}, d, now.Add(2*time.Hour)); len(got) != 1 {
		t.Fatalf("failures older than an hour should drop out, got %+v", got)
	}
}

func TestNavShowsSixGroupsAndPageTabsShowTheGroup(t *testing.T) {
	html := adminNavHTML("providers")
	for _, want := range []string{">Overview<", ">Activity<", ">Policy<", ">Models<", ">Access<", ">Rollout<", ">Open chat<", ">Manual<", ">Sign out<"} {
		if !strings.Contains(html, want) {
			t.Fatalf("nav lacks %q", want)
		}
	}
	if strings.Count(html, `class="tk-link`) != 9 {
		t.Fatalf("want 6 groups + 2 bottom links + sign out, got %d", strings.Count(html, `class="tk-link`))
	}
	if !strings.Contains(html, `tk-link active" href="/router/models"`) {
		t.Fatal("the Models group should be active on the Providers page")
	}
	// The group's pages are tabs in the page header, not a second menu.
	head := dsHeadHTML("providers", "Providers", "x")
	for _, want := range []string{`href="/router/models">Models<`, `class="on" href="/router/providers">Providers<`, `href="/router/risk">Risk register<`} {
		if !strings.Contains(head, want) {
			t.Fatalf("tabs lack %q:\n%s", want, head)
		}
	}
	if strings.Contains(dsHeadHTML("policy", "Policy", ""), "ds-tabs") {
		t.Fatal("a single-page group has no tabs")
	}
}

func TestLabelsAreHumanAndNeverRaw(t *testing.T) {
	if dataClassLabel("pii") != "Personal data" || dataClassLabel("secrets_possible") != "Secrets and keys" || dataClassLabel("brand_new_class") != "Brand new class" {
		t.Fatal("data class labels")
	}
	if taskLabel("trivial_git") != "Git commit" || blockLabel("residency_no_compliant_provider") != "No allowed model for this data" {
		t.Fatal("task/block labels")
	}
}

func TestPlainRuleReadsLikeASentence(t *testing.T) {
	if got := plainRule("data is pii or secrets_possible"); got != "data is personal data or secrets and keys" {
		t.Fatalf("got %q", got)
	}
	if got := plainRule("task is security_review"); got != "task is security review" {
		t.Fatalf("got %q", got)
	}
	if got := plainRule("every request"); got != "every request" {
		t.Fatalf("got %q", got)
	}
}
