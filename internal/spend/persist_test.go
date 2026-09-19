package spend

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	src := New()
	src.Restore(Snapshot{
		Models: []ModelRow{
			{ModelID: "cheap-general", ProviderID: "openrouter", Requests: 8, InputTokens: 1000, OutputTokens: 200, CostUSD: 0.000064},
			{ModelID: "premium-reasoning", ProviderID: "openrouter", Requests: 2, InputTokens: 500, OutputTokens: 400, CostUSD: 0.0134},
		},
		Tenants: []TenantRow{{TenantID: "tn_local", Requests: 10, CostUSD: 0.013464}},
	})

	path := filepath.Join(t.TempDir(), "sub", "spend.json")
	if err := src.SaveJSON(path); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	snap, err := LoadJSON(path)
	if err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	dst := New()
	dst.Restore(snap)

	if got, want := dst.TotalRequests(), src.TotalRequests(); got != want {
		t.Errorf("requests: got %d want %d", got, want)
	}
	if got, want := dst.TotalCostUSD(), src.TotalCostUSD(); got != want {
		t.Errorf("cost: got %.6f want %.6f", got, want)
	}
	if len(dst.ByModel()) != 2 {
		t.Errorf("expected 2 model rows, got %d", len(dst.ByModel()))
	}
}

func TestLoadJSONMissingFileIsEmpty(t *testing.T) {
	snap, err := LoadJSON(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if len(snap.Models) != 0 || len(snap.Tenants) != 0 {
		t.Errorf("missing file should yield empty snapshot, got %+v", snap)
	}
}
