package auth

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func TestKeyStoreAuditsAddAndDisable(t *testing.T) {
	mem := audit.NewMemorySink(0)
	store := NewInMemoryKeyStore()
	store.SetAuditor(mem)

	tn := &tenant.Tenant{ID: "tn_1", Project: "prj_1", KeyID: "key_1"}
	store.Add("secret-key", tn)

	if !store.Disable("secret-key") {
		t.Fatal("Disable should return true for an existing key")
	}
	if store.Disable("secret-key") {
		t.Error("Disable should return false once the key is gone")
	}

	entries := mem.Entries()
	if len(entries) != 2 {
		t.Fatalf("want 2 audit entries (add+disable), got %d", len(entries))
	}
	if entries[0].Action != audit.ActionAPIKeyAdd || entries[0].Target != "key_1" {
		t.Errorf("add entry wrong: %+v", entries[0])
	}
	if entries[1].Action != audit.ActionAPIKeyDisable || entries[1].TenantID != "tn_1" {
		t.Errorf("disable entry wrong: %+v", entries[1])
	}
}

func TestKeyStoreWithoutAuditorDoesNotPanic(t *testing.T) {
	store := NewInMemoryKeyStore()
	store.Add("k", &tenant.Tenant{ID: "tn", KeyID: "key"})
	if _, ok := store.Lookup(hashKey("k")); !ok {
		t.Error("key should be retrievable after Add")
	}
	store.Disable("k")
}
