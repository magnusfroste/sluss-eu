package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/mcp"
	"github.com/magnusfroste/sluss/internal/registry"
)

func outreachOptions(t *testing.T, write bool) (MCPOptions, *history.Store) {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	snap, _ := registry.DefaultSnapshot()
	regStore, _ := registry.NewStore(snap)
	km := apikeys.NewManager(store, auth.NewInMemoryKeyStore(), nil)
	return MCPOptions{
		Engine:       engine.New(regStore),
		History:      store,
		KeyManager:   km,
		WriteEnabled: write,
		PublicURL:    "https://tokenizer.example.com",
		Version:      snap.RegistryVersion(),
	}, store
}

// Write tools are only registered when WriteEnabled; usage tools are always on.
func TestOutreachWriteGate(t *testing.T) {
	off, _ := outreachOptions(t, false)
	names := toolNames(NewMCPServer(off))
	for _, w := range []string{"mint_key", "revoke_key", "create_demo_link"} {
		if names[w] {
			t.Errorf("write tool %q must not be registered when WriteEnabled=false", w)
		}
	}
	for _, r := range []string{"key_usage", "demo_link_usage"} {
		if !names[r] {
			t.Errorf("read tool %q should always be registered", r)
		}
	}

	on, _ := outreachOptions(t, true)
	names = toolNames(NewMCPServer(on))
	for _, w := range []string{"mint_key", "revoke_key", "create_demo_link"} {
		if !names[w] {
			t.Errorf("write tool %q should be registered when WriteEnabled=true", w)
		}
	}
}

// mint → usage roundtrip: a fresh key is unused; after a logged request for its
// tenant, key_usage reports used=true; revoke flips revoked.
func TestMintUsageRevokeRoundtrip(t *testing.T) {
	o, store := outreachOptions(t, true)

	minted := callTool(t, o, "mint_key", map[string]any{"tenant": "prospect_acme", "project": "trial"})
	keyID, _ := minted["key_id"].(string)
	if keyID == "" || minted["api_key"] == "" {
		t.Fatalf("mint should return key_id + api_key, got %+v", minted)
	}

	usage := callTool(t, o, "key_usage", map[string]any{"key_id": keyID})
	if used, _ := usage["used"].(bool); used {
		t.Fatalf("a fresh key should be unused, got %+v", usage)
	}

	// Simulate a routed request for the prospect's tenant landing in the log.
	store.Handle(context.Background(), decisionForTenant("prospect_acme"))

	usage = callTool(t, o, "key_usage", map[string]any{"key_id": keyID})
	if used, _ := usage["used"].(bool); !used {
		t.Fatalf("after a request the key should read used, got %+v", usage)
	}
	if rc, _ := usage["request_count"].(float64); rc != 1 {
		t.Fatalf("request_count = %v, want 1", usage["request_count"])
	}

	revoked := callTool(t, o, "revoke_key", map[string]any{"key_id": keyID})
	if ok, _ := revoked["revoked"].(bool); !ok {
		t.Fatalf("revoke should succeed, got %+v", revoked)
	}
	usage = callTool(t, o, "key_usage", map[string]any{"key_id": keyID})
	if rev, _ := usage["revoked"].(bool); !rev {
		t.Fatalf("key should read revoked after revoke, got %+v", usage)
	}
}

func TestMintRequiresTenant(t *testing.T) {
	o, _ := outreachOptions(t, true)
	srv := NewMCPServer(o)
	for _, tl := range srv.Tools() {
		if tl.Name != "mint_key" {
			continue
		}
		if _, err := tl.Handler(context.Background(), []byte(`{}`)); err == nil {
			t.Fatal("mint_key without tenant should error")
		}
		return
	}
	t.Fatal("mint_key not registered")
}

func TestDemoLinkCreateAndUsage(t *testing.T) {
	o, store := outreachOptions(t, true)

	created := callTool(t, o, "create_demo_link", map[string]any{})
	url, _ := created["url"].(string)
	if url != "https://tokenizer.example.com/demo?token="+demoTokenOf(t, store) {
		t.Fatalf("unexpected demo url: %q", url)
	}

	// No opens yet (legacy counters are nested under "legacy" since ISSUE-089).
	u := callTool(t, o, "demo_link_usage", map[string]any{})
	legacy, _ := u["legacy"].(map[string]any)
	if opens, _ := legacy["opens"].(float64); opens != 0 {
		t.Fatalf("opens should start at 0, got %+v", u)
	}
	// Simulate two opens.
	incrementDemoOpens(store)
	incrementDemoOpens(store)
	u = callTool(t, o, "demo_link_usage", map[string]any{})
	legacy, _ = u["legacy"].(map[string]any)
	if opens, _ := legacy["opens"].(float64); opens != 2 {
		t.Fatalf("opens = %v, want 2", legacy["opens"])
	}
	if enabled, _ := legacy["enabled"].(bool); !enabled {
		t.Fatalf("link should read enabled, got %+v", u)
	}
}

func demoTokenOf(t *testing.T, store *history.Store) string {
	t.Helper()
	return LoadDemoShareToken(store)
}

func toolNames(srv *mcp.Server) map[string]bool {
	out := map[string]bool{}
	for _, tl := range srv.Tools() {
		out[tl.Name] = true
	}
	return out
}

// decisionForTenant is a minimal non-blocked decision event that inserts one
// countable request row for the tenant (the "prospect used the key" signal).
func decisionForTenant(tenant string) eventlog.Event {
	return eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: "req_" + tenant, TenantID: tenant, TaskType: "summarization",
		SelectedModel: "qwen36-27b", SelectedProvider: "dgx", PromptTokens: 42,
	}}
}
