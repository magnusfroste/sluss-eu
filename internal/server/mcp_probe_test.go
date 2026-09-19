package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// provider_probe hits a CONFIGURED provider from the router and reports status +
// a plain-language diagnosis. A rewrite transport redirects the probe of a real
// registry provider id onto a local stub, so we exercise the full handler.
func TestProviderProbeReachable(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()

	o := testMCPOptions(t)
	o.ProbeClient = &http.Client{Transport: rewriteHost(srv.URL)}
	provID := firstProviderID(t, o)

	m := callTool(t, o, "provider_probe", map[string]any{"provider_id": provID, "mode": "models"})
	if ok, _ := m["ok"].(bool); !ok {
		t.Fatalf("probe should be ok, got %+v", m)
	}
	if status, _ := m["status"].(float64); status != 200 {
		t.Fatalf("status = %v, want 200 (%+v)", m["status"], m)
	}
	if _, ok := m["diagnosis"].(string); !ok {
		t.Fatalf("diagnosis missing: %+v", m)
	}
	if !strings.HasSuffix(gotPath, "/models") {
		t.Fatalf("probe should hit a /models path, got %q", gotPath)
	}
}

func TestProviderProbeUnknownProvider(t *testing.T) {
	o := testMCPOptions(t)
	srv := NewMCPServer(o)
	for _, tl := range srv.Tools() {
		if tl.Name != "provider_probe" {
			continue
		}
		if _, err := tl.Handler(t.Context(), []byte(`{"provider_id":"nope"}`)); err == nil {
			t.Fatal("unknown provider_id should error")
		}
		return
	}
	t.Fatal("provider_probe not registered")
}

func TestRecentErrorsTool(t *testing.T) {
	o := testMCPOptions(t)
	ring := eventlog.NewErrorRing(10)
	ring.Handle(t.Context(), eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		RequestID: "r1", ProviderID: "dgx", ModelID: "qwen36-27b", ErrorCode: "provider_5xx",
	}})
	o.Errors = ring
	m := callTool(t, o, "recent_errors", map[string]any{"n": 5})
	if c, _ := m["count"].(float64); c != 1 {
		t.Fatalf("want 1 error, got %v (%+v)", m["count"], m)
	}
}

func firstProviderID(t *testing.T, o MCPOptions) string {
	t.Helper()
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ps := snap.Providers()
	if len(ps) == 0 {
		t.Fatal("no providers in default registry")
	}
	return ps[0].ID
}

// rewriteHost sends every request to the test server regardless of the URL host,
// preserving the path, so a probe of a real provider id lands on our stub.
type rewriteHost string

func (h rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := http.NewRequest(req.Method, string(h)+req.URL.Path, req.Body)
	if err != nil {
		return nil, err
	}
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}
