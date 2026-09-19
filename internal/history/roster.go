package history

// Roster persistence (ISSUE-073): the admin-editable provider connections and
// routable models live in the same SQLite file as request history, making the DB
// the single source of truth for which model fills which tier. Keys are never
// stored here — a model's provider references the env var name holding the key
// (KeyEnv), resolved from the environment at startup (the CISO posture).
//
// These methods trade in providercfg types so callers (main.go wiring, the admin
// pages, the seed/migration helper) share one roster shape. Prices are stored as
// integer micro-USD per million tokens (matching registry.CostMetadata); the
// providercfg USD-per-Mtok float converts on the way in and out.

import (
	"fmt"
	"strings"

	"github.com/magnusfroste/sluss/internal/providercfg"
)

// usdPerMTokToMicros converts USD/Mtok (the providercfg float) into micro-USD
// per million tokens (the DB integer). Mirrors registry's cost scaling.
func usdPerMTokToMicros(usd float64) int64 { return int64(usd*1e6 + 0.5) }

// microsToUSDPerMTok is the inverse of usdPerMTokToMicros.
func microsToUSDPerMTok(micros int64) float64 { return float64(micros) / 1e6 }

// joinTags/splitTags map a tag slice to the DB's comma-separated column and
// back. Empty slices round-trip to the empty string.
func joinTags(tags []string) string { return strings.Join(tags, ",") }

func splitTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LoadRosterProviders returns all provider connections, ordered by id.
func (s *Store) LoadRosterProviders() ([]providercfg.Provider, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT id, name, base_url, key_env, compliance_tags FROM roster_providers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("history: load roster providers: %w", err)
	}
	defer rows.Close()
	var out []providercfg.Provider
	for rows.Next() {
		var p providercfg.Provider
		var tags string
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.KeyEnv, &tags); err != nil {
			return nil, fmt.Errorf("history: scan roster provider: %w", err)
		}
		p.ComplianceTags = splitTags(tags)
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertRosterProvider inserts or replaces a provider connection by id.
func (s *Store) UpsertRosterProvider(p providercfg.Provider) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO roster_providers (id, name, base_url, key_env, compliance_tags)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, base_url=excluded.base_url, key_env=excluded.key_env, compliance_tags=excluded.compliance_tags`,
		p.ID, p.Name, p.BaseURL, p.KeyEnv, joinTags(p.ComplianceTags))
	if err != nil {
		return fmt.Errorf("history: upsert roster provider %q: %w", p.ID, err)
	}
	return nil
}

// DeleteRosterProvider removes a provider connection by id (its models, which
// reference it by provider_id, are left in place and surface as "key missing").
func (s *Store) DeleteRosterProvider(id string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM roster_providers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("history: delete roster provider %q: %w", id, err)
	}
	return nil
}

// LoadRosterModels returns all routable models, ordered by provider then id.
func (s *Store) LoadRosterModels() ([]providercfg.Model, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT id, provider_id, provider_model_id, tier,
			input_micros_per_mtok, output_micros_per_mtok, enabled, compliance_tags, reasoning_capable
		FROM roster_models ORDER BY provider_id, id`)
	if err != nil {
		return nil, fmt.Errorf("history: load roster models: %w", err)
	}
	defer rows.Close()
	var out []providercfg.Model
	for rows.Next() {
		var m providercfg.Model
		var inMicros, outMicros int64
		var enabled, reasoning int
		var tags string
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.ProviderModelID, &m.Tier, &inMicros, &outMicros, &enabled, &tags, &reasoning); err != nil {
			return nil, fmt.Errorf("history: scan roster model: %w", err)
		}
		m.InputUSDPerMTok = microsToUSDPerMTok(inMicros)
		m.OutputUSDPerMTok = microsToUSDPerMTok(outMicros)
		m.Enabled = enabled != 0
		m.ComplianceTags = splitTags(tags)
		m.ReasoningCapable = reasoning != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertRosterModel inserts or replaces a model by id.
func (s *Store) UpsertRosterModel(m providercfg.Model) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO roster_models
			(id, provider_id, provider_model_id, tier, input_micros_per_mtok, output_micros_per_mtok, enabled, compliance_tags, reasoning_capable)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider_id=excluded.provider_id,
			provider_model_id=excluded.provider_model_id,
			tier=excluded.tier,
			input_micros_per_mtok=excluded.input_micros_per_mtok,
			output_micros_per_mtok=excluded.output_micros_per_mtok,
			enabled=excluded.enabled,
			compliance_tags=excluded.compliance_tags,
			reasoning_capable=excluded.reasoning_capable`,
		m.ID, m.ProviderID, m.ProviderModelID, m.Tier,
		usdPerMTokToMicros(m.InputUSDPerMTok), usdPerMTokToMicros(m.OutputUSDPerMTok), boolToInt(m.Enabled),
		joinTags(m.ComplianceTags), boolToInt(m.ReasoningCapable))
	if err != nil {
		return fmt.Errorf("history: upsert roster model %q: %w", m.ID, err)
	}
	return nil
}

// DeleteRosterModel removes a model by id.
func (s *Store) DeleteRosterModel(id string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM roster_models WHERE id = ?`, id); err != nil {
		return fmt.Errorf("history: delete roster model %q: %w", id, err)
	}
	return nil
}
