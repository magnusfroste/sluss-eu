package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/registry"
)

func localTaggedEngine(t *testing.T) *engine.Engine {
	t.Helper()
	def := registry.DefaultDefinition()
	for i := range def.Models {
		if def.Models[i].ID == "premium-reasoning" {
			def.Models[i].ComplianceTags = []string{"local", "on-prem"}
		}
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatal(err)
	}
	return engine.New(store)
}

// ISSUE-116: the dashboard answers "where did the data go" from history —
// PII kept local counts as detected (the old card only counted blocks), and
// rows written before egress was stored are classified from current tags.
func TestDataFlowFromHistory(t *testing.T) {
	h, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx := context.Background()
	add := func(id, model, sens, egress string, blocked bool) {
		h.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, SelectedModel: model, Sensitivity: sens, Egress: egress, Blocked: blocked}})
	}
	add("a", "premium-reasoning", "pii", "local", false)
	add("b", "premium-reasoning", "pii", "", false) // legacy row → local via tags
	add("c", "cheap-general", "none", "cloud", false)
	add("d", "", "source_code", "", true)
	add("e", "cheap-general", "legal", "", false) // legacy, cloud via tags

	eng := localTaggedEngine(t)
	v := buildDataFlow(h, nil, eng)
	if v.Local != 2 || v.Cloud != 2 || v.Blocked != 1 {
		t.Fatalf("totals local=%d cloud=%d blocked=%d", v.Local, v.Cloud, v.Blocked)
	}
	if v.PII.Total() != 2 || v.PII.Local != 2 || v.PII.Cloud != 0 {
		t.Fatalf("pii row = %+v", v.PII)
	}
	if v.Rows[0].Sensitivity != "pii" || v.Rows[len(v.Rows)-1].Sensitivity != "none" {
		t.Fatalf("rows not in sensitivity order: %+v", v.Rows)
	}

	htmlH, _ := DashboardHandler(DashboardOptions{History: h, Engine: eng})
	rec := httptest.NewRecorder()
	htmlH(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	body := rec.Body.String()
	for _, want := range []string{"Where your data went", "2 kept in the house", "Stayed in the house", "left the house — allowed by your policy"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}

	logH := LogPageHandler(LogOptions{History: h, Engine: eng})
	rec = httptest.NewRecorder()
	logH(rec, httptest.NewRequest(http.MethodGet, "/router/log", nil))
	lb := rec.Body.String()
	for _, want := range []string{`eg-local">local`, `eg-cloud">cloud`, `eg-blocked">blocked`} {
		if !strings.Contains(lb, want) {
			t.Fatalf("log missing %q", want)
		}
	}
}
