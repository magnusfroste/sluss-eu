package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
)

// ISSUE-117: quick filters on the log, and each row explains itself from the
// stored classification against the active policy — no prompt is stored.
func TestLogFiltersAndExplanation(t *testing.T) {
	h, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx := context.Background()
	add := func(id, model, sens, egress string, blocked bool, code string) {
		h.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, TaskType: "summarization", RiskLevel: "medium", SelectedModel: model,
			Sensitivity: sens, Egress: egress, Blocked: blocked, BlockCode: code}})
	}
	add("req-pii", "premium-reasoning", "pii", "local", false, "")
	add("req-plain", "cheap-general", "none", "cloud", false, "")
	add("req-block", "", "secrets_possible", "", true, "residency_no_compliant_provider")

	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewRuntimeCache(snap, "builtin:nis2-baseline")
	if err != nil {
		t.Fatal(err)
	}
	handler := LogPageHandler(LogOptions{History: h, Engine: localTaggedEngine(t), Cache: cache})
	get := func(q string) string {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/router/log"+q, nil))
		return rec.Body.String()
	}

	all := get("")
	if !strings.Contains(all, "the data stayed in the house") || !strings.Contains(all, "data is pii") {
		t.Fatal("pii row should explain itself with the matching rule")
	}
	if !strings.Contains(all, "No provider was called") {
		t.Fatal("blocked row should say nothing was sent")
	}
	if !strings.Contains(all, `show=blocked"`) || !strings.Contains(all, "blocked <span>1</span>") {
		t.Fatal("filter chips with counts missing")
	}

	blocked := get("?show=blocked")
	if !strings.Contains(blocked, "req-block") || strings.Contains(blocked, "req-plain") || !strings.Contains(blocked, "1 of 3") {
		t.Fatal("blocked filter should show only the blocked row")
	}
	sensitive := get("?show=sensitive")
	if !strings.Contains(sensitive, "req-pii") || strings.Contains(sensitive, "req-plain") {
		t.Fatal("sensitive filter should hide rows without sensitive data")
	}
	if strings.Contains(get("?show=bogus"), "of 3") {
		t.Fatal("unknown filter should fall back to all rows")
	}
}
