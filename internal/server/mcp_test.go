package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/registry"
)

func testMCPOptions(t *testing.T) MCPOptions {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return MCPOptions{
		Engine:     engine.New(store),
		RequestLog: eventlog.NewRequestLogTracker(0),
		Health:     health.New(),
		Version:    snap.RegistryVersion(),
		Dashboard:  DashboardOptions{Version: snap.RegistryVersion()},
	}
}

// callTool runs a registered tool handler and returns its result decoded as the
// JSON wire shape an agent would receive.
func callTool(t *testing.T, o MCPOptions, name string, args any) map[string]any {
	t.Helper()
	srv := NewMCPServer(o)
	raw, _ := json.Marshal(args)
	for _, tl := range srv.Tools() {
		if tl.Name != name {
			continue
		}
		out, err := tl.Handler(context.Background(), raw)
		if err != nil {
			t.Fatalf("%s handler error: %v", name, err)
		}
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("marshal %s result: %v", name, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("unmarshal %s result: %v", name, err)
		}
		return m
	}
	t.Fatalf("tool %q not registered", name)
	return nil
}

func TestMCPToolsRegistered(t *testing.T) {
	srv := NewMCPServer(testMCPOptions(t))
	want := map[string]bool{
		"route_explain": false, "savings_report": false, "recent_requests": false,
		"get_roster": false, "provider_health": false,
	}
	for _, tl := range srv.Tools() {
		if _, ok := want[tl.Name]; ok {
			want[tl.Name] = true
		}
		if tl.InputSchema == nil {
			t.Errorf("tool %q has nil input schema", tl.Name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestMCPRouteExplain(t *testing.T) {
	o := testMCPOptions(t)
	m := callTool(t, o, "route_explain", map[string]any{
		"prompt": "Fix this NullPointerException in my Java service and explain the root cause across these stack frames.",
	})
	if m["selected_model"] == "" || m["selected_model"] == nil {
		t.Fatalf("expected a selected_model, got %v", m["selected_model"])
	}
	if m["task_type"] == "" || m["task_type"] == nil {
		t.Errorf("expected a task_type, got %v", m["task_type"])
	}
	for _, key := range []string{"tier", "fallbacks", "est_cost_usd", "risk", "provider_model_id"} {
		if _, ok := m[key]; !ok {
			t.Errorf("route_explain missing %q", key)
		}
	}
}

func TestMCPRouteExplainMatchesEngine(t *testing.T) {
	o := testMCPOptions(t)
	prompt := "Summarise this paragraph in one sentence."
	m := callTool(t, o, "route_explain", map[string]any{"prompt": prompt})

	// The tool must select exactly what a direct engine dry-run selects — same path.
	req := &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: prompt}}}
	dec, _, err := explainRoute(o.Engine, o.PolicyCache, explainInput{RequestID: "t", Request: req})
	if err != nil {
		t.Fatalf("engine decide: %v", err)
	}
	if got := m["selected_model"]; got != dec.SelectedModel {
		t.Errorf("selected_model = %v, want %q", got, dec.SelectedModel)
	}
	if got := m["provider_model_id"]; got != dec.ProviderModelID {
		t.Errorf("provider_model_id = %v, want %q", got, dec.ProviderModelID)
	}
}

func TestMCPRouteExplainRequiresInput(t *testing.T) {
	o := testMCPOptions(t)
	for _, tl := range NewMCPServer(o).Tools() {
		if tl.Name != "route_explain" {
			continue
		}
		if _, err := tl.Handler(context.Background(), json.RawMessage(`{}`)); err == nil {
			t.Fatal("expected error for empty route_explain input")
		}
	}
}

func TestMCPSavingsReport(t *testing.T) {
	o := testMCPOptions(t)
	m := callTool(t, o, "savings_report", map[string]any{})
	for _, key := range []string{"savings", "green", "total_requests", "total_cost_usd", "route_by_model"} {
		if _, ok := m[key]; !ok {
			t.Errorf("savings_report missing %q (got keys %v)", key, keysOf(m))
		}
	}
	savings, ok := m["savings"].(map[string]any)
	if !ok {
		t.Fatalf("savings not an object: %v", m["savings"])
	}
	if _, ok := savings["saved_usd"]; !ok {
		t.Errorf("savings missing saved_usd")
	}
}

func TestMCPRecentRequests(t *testing.T) {
	o := testMCPOptions(t)
	m := callTool(t, o, "recent_requests", map[string]any{"n": 5})
	if _, ok := m["requests"]; !ok {
		t.Errorf("recent_requests missing requests")
	}
	if _, ok := m["count"]; !ok {
		t.Errorf("recent_requests missing count")
	}
}

func TestMCPGetRoster(t *testing.T) {
	o := testMCPOptions(t)
	m := callTool(t, o, "get_roster", map[string]any{})
	models, ok := m["models"].([]any)
	if !ok {
		t.Fatalf("models not an array: %v", m["models"])
	}
	if len(models) == 0 {
		t.Fatal("expected at least one roster model from the default registry")
	}
	first, ok := models[0].(map[string]any)
	if !ok {
		t.Fatalf("model entry not an object: %v", models[0])
	}
	for _, key := range []string{"id", "tier", "provider_id", "enabled"} {
		if _, ok := first[key]; !ok {
			t.Errorf("roster model missing %q (got %v)", key, keysOf(first))
		}
	}
}

func TestMCPProviderHealth(t *testing.T) {
	o := testMCPOptions(t)
	o.Health.RecordSuccess("openai")
	m := callTool(t, o, "provider_health", map[string]any{})
	provs, ok := m["providers"].([]any)
	if !ok {
		t.Fatalf("providers not an array: %v", m["providers"])
	}
	if len(provs) == 0 {
		t.Fatal("expected at least one provider after a recorded success")
	}
	row := provs[0].(map[string]any)
	for _, key := range []string{"provider", "health_score", "status"} {
		if _, ok := row[key]; !ok {
			t.Errorf("provider_health row missing %q", key)
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
