package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/spend"
)

func eraseEnv(t *testing.T) (TenantEraseOptions, *history.Store, *audit.MemorySink) {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	sink := audit.NewMemorySink(0)
	tr := spend.New()
	o := TenantEraseOptions{
		History: store, Spend: tr,
		KeyManager: apikeys.NewManager(store, auth.NewInMemoryKeyStore(), nil),
		Auditor:    sink,
	}
	return o, store, sink
}

func decisionFor(tenant, id string) eventlog.Event {
	return eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: id, TenantID: tenant, TaskType: "summarization",
		SelectedModel: "m", SelectedProvider: "p", DecidedAt: time.Now(),
	}}
}

// Erasure deletes ONE tenant's rows + spend attribution + revokes its keys,
// leaves everyone else untouched, and records the act in the audit trail.
func TestTenantErasure(t *testing.T) {
	o, store, sink := eraseEnv(t)
	ctx := context.Background()
	store.Handle(ctx, decisionFor("acme", "r1"))
	store.Handle(ctx, decisionFor("acme", "r2"))
	store.Handle(ctx, decisionFor("other", "r3"))
	o.Spend.Handle(ctx, decisionFor("acme", "r1"))
	o.Spend.Handle(ctx, decisionFor("other", "r3"))
	if _, _, err := o.KeyManager.Mint(apikeys.MintRequest{TenantID: "acme", Actor: "test"}); err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, _, err := o.KeyManager.Mint(apikeys.MintRequest{TenantID: "other", Actor: "test"}); err != nil {
		t.Fatalf("mint: %v", err)
	}

	form := url.Values{"tenant_id": {"acme"}, "revoke_keys": {"on"}}
	req := httptest.NewRequest(http.MethodPost, "/router/data/erase-tenant", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	TenantEraseHandler(o)(rec, req)
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("erase failed: %q", loc)
	}

	// acme's rows are gone; other's remain.
	if _, found := store.ByRequestID("r1"); found {
		t.Fatal("acme row r1 should be erased")
	}
	if _, found := store.ByRequestID("r3"); !found {
		t.Fatal("other tenant's row must remain")
	}
	// Spend attribution: acme gone, other stays.
	for _, row := range o.Spend.ByTenant() {
		if row.TenantID == "acme" {
			t.Fatalf("acme spend attribution should be erased: %+v", row)
		}
	}
	// acme key revoked, other's active.
	keys, _ := o.KeyManager.List()
	for _, k := range keys {
		if k.TenantID == "acme" && k.RevokedAt.IsZero() {
			t.Fatal("acme key should be revoked")
		}
		if k.TenantID == "other" && !k.RevokedAt.IsZero() {
			t.Fatal("other tenant's key must stay active")
		}
	}
	// Audited with counts.
	var found bool
	for _, e := range sink.Entries() {
		if e.Action == actionTenantErasure && e.TenantID == "acme" {
			found = true
			if e.Detail["rows_deleted"] != "2" || e.Detail["keys_revoked"] != "1" {
				t.Fatalf("audit detail wrong: %+v", e.Detail)
			}
		}
	}
	if !found {
		t.Fatal("erasure must be audited")
	}
}

func TestTenantErasureRequiresTenant(t *testing.T) {
	o, _, _ := eraseEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/router/data/erase-tenant", strings.NewReader("tenant_id="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	TenantEraseHandler(o)(rec, req)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("empty tenant should error, got %q", loc)
	}
}

// The policy console renders the erasure form when a data dir exists.
func TestPolicyPageShowsErasureForm(t *testing.T) {
	o, store, _ := eraseEnv(t)
	_ = o
	h := PolicyPageHandler(PolicyPageOptions{History: store, ResetAvailable: true})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/policy", nil))
	body := rec.Body.String()
	for _, want := range []string{"Tenant erasure", "/router/data/erase-tenant", "revoke the tenant"} {
		if !strings.Contains(body, want) {
			t.Errorf("policy page missing %q", want)
		}
	}
}
