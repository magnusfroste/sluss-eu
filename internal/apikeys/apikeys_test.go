package apikeys

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
)

func newManager(t *testing.T) (*Manager, *auth.InMemoryKeyStore) {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	ks := auth.NewInMemoryKeyStore()
	return NewManager(store, ks, nil), ks
}

func TestGenerateIsRandomAndHashed(t *testing.T) {
	p1, h1, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	p2, _, _ := Generate()
	if p1 == p2 {
		t.Fatal("two generated keys collided")
	}
	if !strings.HasPrefix(p1, "tk_") {
		t.Fatalf("key %q missing tk_ prefix", p1)
	}
	if h1 != auth.HashKey(p1) {
		t.Fatal("returned hash does not match HashKey(plaintext)")
	}
	if strings.Contains(h1, p1) || len(h1) != 64 {
		t.Fatalf("hash looks wrong: %q", h1)
	}
}

func TestMintActivatesAndAuthenticates(t *testing.T) {
	mgr, ks := newManager(t)
	plaintext, rec, err := mgr.Mint(MintRequest{TenantID: "tn_eko", ProjectID: "prj", Role: auth.RoleUser})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if !strings.HasPrefix(plaintext, "tk_") || rec.KeyID == "" {
		t.Fatalf("bad mint result: %q %+v", plaintext, rec)
	}
	// The freshly minted key authenticates through the in-memory keystore.
	ten, ok := ks.Lookup(auth.HashKey(plaintext))
	if !ok || ten.ID != "tn_eko" || ten.KeyID != rec.KeyID {
		t.Fatalf("minted key does not authenticate: ok=%v tenant=%+v", ok, ten)
	}
}

func TestMintRequiresTenant(t *testing.T) {
	mgr, _ := newManager(t)
	if _, _, err := mgr.Mint(MintRequest{}); err == nil {
		t.Fatal("mint without tenant_id should fail")
	}
}

func TestRevokeEvictsAndPersists(t *testing.T) {
	mgr, ks := newManager(t)
	plaintext, rec, _ := mgr.Mint(MintRequest{TenantID: "tn_eko"})

	ok, err := mgr.Revoke(rec.KeyID, "test")
	if err != nil || !ok {
		t.Fatalf("revoke: ok=%v err=%v", ok, err)
	}
	// Revoked key no longer authenticates.
	if _, ok := ks.Lookup(auth.HashKey(plaintext)); ok {
		t.Fatal("revoked key still authenticates")
	}
	// Revoking again → not found.
	if ok, _ := mgr.Revoke(rec.KeyID, "test"); ok {
		t.Fatal("second revoke should report not-found")
	}
	// List shows it as revoked (kept for audit).
	all, _ := mgr.List()
	if len(all) != 1 || !all[0].Revoked() {
		t.Fatalf("list should show one revoked key: %+v", all)
	}
}

func TestLoadIntoKeystoreSkipsRevoked(t *testing.T) {
	mgr, _ := newManager(t)
	pActive, _, _ := mgr.Mint(MintRequest{TenantID: "tn_a"})
	_, revRec, _ := mgr.Mint(MintRequest{TenantID: "tn_b"})
	mgr.Revoke(revRec.KeyID, "test")

	// Fresh keystore, reload from DB (simulating a restart).
	ks2 := auth.NewInMemoryKeyStore()
	mgr2 := NewManager(mgrStore(mgr), ks2, nil)
	n, err := mgr2.LoadIntoKeystore()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("loaded %d keys, want 1 (revoked excluded)", n)
	}
	if _, ok := ks2.Lookup(auth.HashKey(pActive)); !ok {
		t.Fatal("active key should load into a fresh keystore")
	}
}

func TestNoSecretPersisted(t *testing.T) {
	mgr, _ := newManager(t)
	plaintext, _, _ := mgr.Mint(MintRequest{TenantID: "tn_eko"})
	all, _ := mgr.List()
	for _, k := range all {
		if strings.Contains(k.KeyHash, plaintext) || k.KeyHash == plaintext {
			t.Fatal("plaintext leaked into stored hash")
		}
	}
}

// mgrStore exposes the manager's store for the reload test.
func mgrStore(m *Manager) *history.Store { return m.store }
