package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

func TestPerTokenConversion(t *testing.T) {
	// $0.00000015 / token = $0.15 / Mtok = 150000 micros / Mtok.
	if got := perTokenToMicrosPerMTok("0.00000015"); got != 150000 {
		t.Errorf("prompt conversion = %d, want 150000", got)
	}
	if got := perTokenToMicrosPerMTok("0.000003"); got != 3000000 {
		t.Errorf("completion conversion = %d, want 3000000", got)
	}
	if got := perTokenToMicrosPerMTok("0"); got != 0 {
		t.Errorf("zero price should be 0, got %d", got)
	}
	if got := perTokenToMicrosPerMTok("bad"); got != 0 {
		t.Errorf("unparseable should be 0, got %d", got)
	}
}

func TestFetchAndApply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"openai/gpt-4o-mini","pricing":{"prompt":"0.00000015","completion":"0.0000006"}},
			{"id":"anthropic/claude-sonnet-4.5","pricing":{"prompt":"0.000003","completion":"0.000015"}},
			{"id":"some/free-model","pricing":{"prompt":"0","completion":"0"}}
		]}`))
	}))
	defer srv.Close()

	prices, err := FetchOpenRouter(context.Background(), srv.URL, "", srv.Client())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(prices) != 2 { // free model skipped
		t.Fatalf("want 2 priced models, got %d: %+v", len(prices), prices)
	}
	if p := prices["openai/gpt-4o-mini"]; p.InputMicrosPerMTok != 150000 || p.OutputMicrosPerMTok != 600000 {
		t.Errorf("gpt-4o-mini price wrong: %+v", p)
	}

	def := registry.Definition{Models: []registry.Model{
		{ID: "cheap-general", ProviderID: "openrouter", ProviderModelID: "openai/gpt-4o-mini", Cost: registry.CostMetadata{InputMicrosPerMillionToken: 999}},
		{ID: "premium", ProviderID: "openrouter", ProviderModelID: "anthropic/claude-sonnet-4.5"},
		{ID: "custom", ProviderID: "minimax", ProviderModelID: "glm-4.6"}, // not in catalog → untouched
	}}
	n := ApplyToDefinition(&def, prices, "openrouter")
	if n != 2 {
		t.Fatalf("want 2 models updated, got %d", n)
	}
	if def.Models[0].Cost.InputMicrosPerMillionToken != 150000 {
		t.Errorf("cheap not updated: %d", def.Models[0].Cost.InputMicrosPerMillionToken)
	}
	if def.Models[2].Cost.InputMicrosPerMillionToken != 0 {
		t.Errorf("custom model should be untouched, got %d", def.Models[2].Cost.InputMicrosPerMillionToken)
	}
}

// A custom provider whose model slug COLLIDES with a catalog slug must keep its
// admin-set (negotiated) price — the sync only touches the catalog provider.
func TestApplyRespectsCustomProviderPrice(t *testing.T) {
	prices := map[string]Price{"anthropic/claude-sonnet-4.5": {InputMicrosPerMTok: 3000000, OutputMicrosPerMTok: 15000000}}
	def := registry.Definition{Models: []registry.Model{
		{ID: "or-premium", ProviderID: "openrouter", ProviderModelID: "anthropic/claude-sonnet-4.5"},
		// A specific provider selling the same slug at a negotiated rate.
		{ID: "myvendor-1", ProviderID: "myvendor", ProviderModelID: "anthropic/claude-sonnet-4.5",
			Cost: registry.CostMetadata{InputMicrosPerMillionToken: 900000, OutputMicrosPerMillionToken: 4500000}},
	}}
	ApplyToDefinition(&def, prices, "openrouter")
	if def.Models[0].Cost.InputMicrosPerMillionToken != 3000000 {
		t.Errorf("openrouter model should be synced: %d", def.Models[0].Cost.InputMicrosPerMillionToken)
	}
	if def.Models[1].Cost.InputMicrosPerMillionToken != 900000 {
		t.Errorf("custom vendor's negotiated price must stand, got %d", def.Models[1].Cost.InputMicrosPerMillionToken)
	}
}
