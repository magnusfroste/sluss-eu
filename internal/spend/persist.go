package spend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Snapshot is a serializable copy of the tracker's aggregates. It carries only
// counts, tokens and cost — no prompt text or secrets — so it is safe to persist
// to a volume and survive restarts/redeploys.
type Snapshot struct {
	Models  []ModelRow  `json:"models"`
	Tenants []TenantRow `json:"tenants"`
}

// Snapshot returns the current aggregates as a serializable value.
func (t *Tracker) Snapshot() Snapshot {
	return Snapshot{Models: t.ByModel(), Tenants: t.ByTenant()}
}

// Restore merges a snapshot into the tracker, replacing any existing entries for
// the same model/tenant. Intended to be called once at startup before traffic.
func (t *Tracker) Restore(s Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, m := range s.Models {
		t.byModel[m.ModelID] = &modelAccum{
			providerID:   m.ProviderID,
			requests:     m.Requests,
			inputTokens:  m.InputTokens,
			outputTokens: m.OutputTokens,
			costUSD:      m.CostUSD,
		}
	}
	for _, tn := range s.Tenants {
		t.byTenant[tn.TenantID] = &tenantAccum{requests: tn.Requests, costUSD: tn.CostUSD}
	}
}

// SaveJSON writes the current aggregates to path atomically (write-temp+rename),
// creating the parent directory if needed.
func (t *Tracker) SaveJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadJSON reads a snapshot from path. A missing file returns an empty snapshot
// and no error, so first-run startup is clean.
func LoadJSON(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}
