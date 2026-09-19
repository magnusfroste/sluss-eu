package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/spend"
)

// The reset clears operational data (request rows, spend, demo sessions,
// in-memory counters) and records ITSELF in the audit trail. It never touches
// keys/users/roster (not exercised here — the handler simply has no access).
func TestDataResetClearsOperationalDataAndAudits(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	// Seed: one request row, some spend, a demo session blob, a gap comparison.
	store.Handle(context.Background(), decisionForTenant("acme"))
	sp := spend.New()
	sp.Handle(context.Background(), decisionForTenant("acme"))
	_ = store.KVSet(demoSessionsKey, `[{"id":"s1"}]`)
	tr := eventlog.NewComparisonTracker(10)
	tr.Handle(context.Background(), shadowEvent("pii", "cloud", "local", false, true))
	mem := audit.NewMemorySink(10)

	if len(store.Recent(10)) != 1 {
		t.Fatal("seed row missing")
	}

	h := DataResetHandler(DataResetOptions{
		History: store, Spend: sp, Comparisons: tr, Auditor: mem,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/data/reset", nil)
	h(rec, req.WithContext(withAdminUser(req.Context(), "anna")))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want redirect, got %d", rec.Code)
	}
	if got := store.Recent(10); len(got) != 0 {
		t.Fatalf("request rows should be gone, got %d", len(got))
	}
	if v, _ := store.KVGet(demoSessionsKey); v != "" {
		t.Fatalf("demo sessions should be cleared, got %q", v)
	}
	if sp.TotalRequests() != 0 {
		t.Fatal("spend should be reset")
	}
	if g := tr.Gap(); g.SensitiveTotal != 0 {
		t.Fatalf("gap counters should be reset: %+v", g)
	}
	// The reset is evidence: audited with the acting user.
	entries := mem.Entries()
	if len(entries) == 0 || entries[len(entries)-1].Action != actionDataReset || entries[len(entries)-1].Actor != "anna" {
		t.Fatalf("reset must be audited with actor, got %+v", entries)
	}
}
