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
		if it.Level == "bad" && strings.Contains(it.Title, "glm-local on dgx is failing") && strings.Contains(it.Detail, "3 failed requests") && it.Href == "/router/providers" {
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

func TestNavShowsSixGroupsAndSubPagesOfActiveGroup(t *testing.T) {
	html := adminNavHTML("providers")
	for _, want := range []string{">Overview<", ">Activity<", ">Policy<", ">Models<", ">Access<", ">Rollout<", ">Open chat<", ">Manual<", ">Sign out<"} {
		if !strings.Contains(html, want) {
			t.Fatalf("nav lacks %q", want)
		}
	}
	if !strings.Contains(html, `tk-sublink on" href="/router/providers"`) || !strings.Contains(html, `href="/router/risk"`) {
		t.Fatalf("active group's pages should be listed with providers marked:\n%s", html)
	}
	if strings.Contains(adminNavHTML("dashboard"), `href="/router/risk"`) {
		t.Fatal("sub-pages of an inactive group should not be listed")
	}
	if strings.Count(html, `class="tk-link`) != 9 {
		t.Fatalf("want 6 groups + 2 bottom links + sign out, got %d", strings.Count(html, `class="tk-link`))
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
