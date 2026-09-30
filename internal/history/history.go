// Package history is the v1.0 durable event sink (ISSUE-070): request history
// and spend aggregates in a single SQLite file on the data volume, written only
// by the async event-queue worker and read by the dashboard. Pure Go
// (modernc.org/sqlite) so the static CGO_ENABLED=0 binary is preserved.
//
// The design borrows the sister project agentanbud's philosophy — one
// container, one SQLite file — and fits its sweet spot: a single writer (the
// event queue) and read-heavy consumers (the dashboard). Postgres/Redis are
// deliberately deferred to Enterprise 2.0 (DECISION_LOG 2026-07-04).
package history

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/spend"
)

const schema = `
CREATE TABLE IF NOT EXISTS requests (
	id INTEGER PRIMARY KEY,
	time TEXT NOT NULL,
	request_id TEXT NOT NULL,
	tenant_id TEXT NOT NULL DEFAULT '',
	task_type TEXT NOT NULL DEFAULT '',
	risk_level TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	provider_model_id TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	blocked INTEGER NOT NULL DEFAULT 0,
	sensitivity TEXT NOT NULL DEFAULT '',
	block_code TEXT NOT NULL DEFAULT '',
	egress TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_requests_time ON requests(time);
CREATE INDEX IF NOT EXISTS idx_requests_request_id ON requests(request_id);
CREATE TABLE IF NOT EXISTS kv (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS roster_providers (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	base_url TEXT NOT NULL DEFAULT '',
	key_env TEXT NOT NULL DEFAULT '',
	compliance_tags TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS roster_models (
	id TEXT PRIMARY KEY,
	provider_id TEXT NOT NULL DEFAULT '',
	provider_model_id TEXT NOT NULL DEFAULT '',
	tier TEXT NOT NULL DEFAULT '',
	input_micros_per_mtok INTEGER NOT NULL DEFAULT 0,
	output_micros_per_mtok INTEGER NOT NULL DEFAULT 0,
	enabled INTEGER NOT NULL DEFAULT 1,
	compliance_tags TEXT NOT NULL DEFAULT '',
	reasoning_capable INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS api_keys (
	key_id TEXT PRIMARY KEY,
	key_hash TEXT NOT NULL,
	tenant_id TEXT NOT NULL DEFAULT '',
	project_id TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL DEFAULT '',
	scopes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT '',
	revoked_at TEXT NOT NULL DEFAULT '',
	last_used_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE TABLE IF NOT EXISTS demo_links (
	token TEXT PRIMARY KEY,
	label TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT '',
	disabled_at TEXT NOT NULL DEFAULT '',
	opens INTEGER NOT NULL DEFAULT 0,
	last_open_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS admin_users (
	username TEXT PRIMARY KEY,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'admin',
	created_at TEXT NOT NULL DEFAULT '',
	disabled_at TEXT NOT NULL DEFAULT '',
	last_login_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL DEFAULT '',
	expires_at TEXT NOT NULL DEFAULT ''
);
`

// migrations are idempotent, additive schema changes for databases created
// before a column existed. "duplicate column name" errors are expected and
// ignored — CREATE TABLE IF NOT EXISTS above already carries the column for
// fresh databases.
var migrations = []string{
	`ALTER TABLE roster_providers ADD COLUMN compliance_tags TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE roster_models ADD COLUMN compliance_tags TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE roster_models ADD COLUMN reasoning_capable INTEGER NOT NULL DEFAULT 0`,
	// Incident evidence (ISSUE-095 follow-up): classification + block code per
	// request row — types/codes only, never content.
	`ALTER TABLE requests ADD COLUMN sensitivity TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE requests ADD COLUMN block_code TEXT NOT NULL DEFAULT ''`,
	// Where the data went (ISSUE-116): "local" | "cloud" per request, recorded
	// at decision time and corrected by the answering attempt.
	`ALTER TABLE requests ADD COLUMN egress TEXT NOT NULL DEFAULT ''`,
}

// Store is a SQLite-backed request history. It implements eventlog.Handler:
// a decision event inserts the row; the successful attempt fills in actual
// tokens and realized cost, joined on request ID. Safe for concurrent use —
// writes are serialized (the event queue is a single worker anyway) and reads
// go through the connection pool with WAL enabled.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// Open opens (creating if needed) the history database at path, enables WAL and
// a busy timeout, and applies the schema.
func Open(path string) (*Store, error) {
	// Ensure the parent directory exists (SQLite won't create it).
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("history: create dir %s: %w", dir, err)
		}
	}
	// _pragma is the modernc.org/sqlite DSN convention.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("history: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("history: apply schema: %w", err)
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// Additive column already present on fresh/upgraded DBs.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			db.Close()
			return nil, fmt.Errorf("history: apply migration: %w", err)
		}
	}
	return &Store{db: db}, nil
}

// Close flushes and closes the database.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	return s.db.Close()
}

// Handle implements eventlog.Handler. Errors are logged by the caller's queue
// worker via panic recovery only; here they are swallowed after best effort —
// history must never affect the request path.
func (s *Store) Handle(_ context.Context, e eventlog.Event) {
	switch e.Type {
	case eventlog.EventTypeDecision:
		if e.Decision != nil {
			s.insertDecision(e.Decision)
		}
	case eventlog.EventTypeAttempt:
		// Failed attempts are handled too (ISSUE-113): they zero the pre-call
		// estimate so a dead provider never shows up as spend.
		if e.Attempt != nil {
			s.fillAttempt(e.Attempt)
		}
	}
}

func (s *Store) insertDecision(d *eventlog.DecisionEvent) {
	at := d.DecidedAt
	if at.IsZero() {
		at = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`INSERT INTO requests
		(time, request_id, tenant_id, task_type, risk_level, model, provider_model_id, provider, input_tokens, cost_usd, blocked, sensitivity, block_code, egress)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		at.UTC().Format(time.RFC3339Nano), d.RequestID, d.TenantID, d.TaskType, d.RiskLevel,
		d.SelectedModel, d.ProviderModelID, d.SelectedProvider, d.PromptTokens, d.EstimatedCostUSD, boolToInt(d.Blocked),
		d.Sensitivity, d.BlockCode, d.Egress)
}

func (s *Store) fillAttempt(a *eventlog.AttemptEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !a.Success {
		// A failed attempt spent nothing we can account for. The decision row
		// carried the pre-call estimate; leaving it there made failed requests
		// count as spend (ISSUE-113: a dead provider showed as -8000% savings).
		// A later successful fallback attempt overwrites this with real cost.
		_, _ = s.db.Exec(`UPDATE requests SET cost_usd = 0
			WHERE id = (SELECT MAX(id) FROM requests WHERE request_id = ?)`, a.RequestID)
		return
	}
	cost := a.ActualCostUSD
	if cost <= 0 {
		cost = a.EstimatedCostUSD // usage unavailable (e.g. streaming): keep the estimate
	}
	_, _ = s.db.Exec(`UPDATE requests SET
			input_tokens = ?,
			output_tokens = ?,
			cost_usd = CASE WHEN ? > 0 THEN ? ELSE cost_usd END,
			model = CASE WHEN ? != '' THEN ? ELSE model END,
			provider = CASE WHEN ? != '' THEN ? ELSE provider END,
			egress = CASE WHEN ? != '' THEN ? ELSE egress END
		WHERE id = (SELECT MAX(id) FROM requests WHERE request_id = ?)`,
		a.InputTokens, a.OutputTokens, cost, cost, a.ModelID, a.ModelID, a.ProviderID, a.ProviderID, a.Egress, a.Egress, a.RequestID)
}

// Recent returns up to n most recent requests, newest first.
func (s *Store) Recent(n int) []eventlog.RequestLogRecord {
	if s == nil {
		return nil
	}
	if n <= 0 {
		n = 100
	}
	rows, err := s.db.Query(`SELECT time, request_id, task_type, risk_level, model, provider_model_id, provider,
			input_tokens, output_tokens, cost_usd, blocked, sensitivity, block_code, egress
		FROM requests ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanRequestRows(rows)
}

func scanRequestRows(rows *sql.Rows) []eventlog.RequestLogRecord {
	var out []eventlog.RequestLogRecord
	for rows.Next() {
		var r eventlog.RequestLogRecord
		var ts string
		var blocked int
		if err := rows.Scan(&ts, &r.RequestID, &r.TaskType, &r.RiskLevel, &r.Model, &r.ProviderModelID, &r.Provider,
			&r.InputTokens, &r.OutputTokens, &r.CostUSD, &blocked, &r.Sensitivity, &r.BlockCode, &r.Egress); err != nil {
			continue
		}
		r.Time, _ = time.Parse(time.RFC3339Nano, ts)
		r.Blocked = blocked != 0
		out = append(out, r)
	}
	return out
}

// EraseTenant deletes one tenant's request-history rows (GDPR art. 17 erasure).
// Returns the number of rows removed. The audit chain is NEVER touched here —
// the erasure itself must be recorded by the caller.
func (s *Store) EraseTenant(tenantID string) (int64, error) {
	if s == nil || tenantID == "" {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM requests WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Since returns up to limit requests decided at or after t, newest first — the
// incident-evidence window query (24h early warning / 72h notification).
func (s *Store) Since(t time.Time, limit int) []eventlog.RequestLogRecord {
	if s == nil {
		return nil
	}
	if limit <= 0 {
		limit = 10_000
	}
	rows, err := s.db.Query(`SELECT time, request_id, task_type, risk_level, model, provider_model_id, provider,
			input_tokens, output_tokens, cost_usd, blocked, sensitivity, block_code, egress
		FROM requests WHERE time >= ? ORDER BY id DESC LIMIT ?`,
		t.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanRequestRows(rows)
}

// ByRequestID returns the most recent logged row for a request ID (as seen in
// the x-router-request-id header/logs), and whether one was found. No raw prompt
// is stored (masking) — this is the decision row only.
func (s *Store) ByRequestID(id string) (eventlog.RequestLogRecord, bool) {
	if s == nil || id == "" {
		return eventlog.RequestLogRecord{}, false
	}
	var r eventlog.RequestLogRecord
	var ts string
	var blocked int
	err := s.db.QueryRow(`SELECT time, request_id, task_type, risk_level, model, provider_model_id, provider,
			input_tokens, output_tokens, cost_usd, blocked, sensitivity, block_code, egress
		FROM requests WHERE request_id = ? ORDER BY id DESC LIMIT 1`, id).Scan(
		&ts, &r.RequestID, &r.TaskType, &r.RiskLevel, &r.Model, &r.ProviderModelID, &r.Provider,
		&r.InputTokens, &r.OutputTokens, &r.CostUSD, &blocked, &r.Sensitivity, &r.BlockCode, &r.Egress)
	if err != nil {
		return eventlog.RequestLogRecord{}, false
	}
	r.Time, _ = time.Parse(time.RFC3339Nano, ts)
	r.Blocked = blocked != 0
	return r, true
}

// ByModelSince is ByModel restricted to a time window, so a reporting agent
// can ask "what did yesterday cost?" instead of only all-time totals
// (ISSUE-110). A zero time means no lower bound (same set as ByModel).
func (s *Store) ByModelSince(t time.Time) []spend.ModelRow {
	if s == nil {
		return nil
	}
	if t.IsZero() {
		return s.ByModel()
	}
	rows, err := s.db.Query(`SELECT model, provider, COUNT(*), COALESCE(SUM(input_tokens),0),
			COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM requests WHERE blocked = 0 AND model != '' AND time >= ?
		GROUP BY model, provider ORDER BY COUNT(*) DESC`,
		t.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []spend.ModelRow
	for rows.Next() {
		var r spend.ModelRow
		if err := rows.Scan(&r.ModelID, &r.ProviderID, &r.Requests, &r.InputTokens, &r.OutputTokens, &r.CostUSD); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ByModel aggregates non-blocked requests per model for the dashboard's route
// distribution and the savings/green counterfactuals.
func (s *Store) ByModel() []spend.ModelRow {
	if s == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT model, provider, COUNT(*), COALESCE(SUM(input_tokens),0),
			COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM requests WHERE blocked = 0 AND model != '' GROUP BY model, provider ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []spend.ModelRow
	for rows.Next() {
		var r spend.ModelRow
		if err := rows.Scan(&r.ModelID, &r.ProviderID, &r.Requests, &r.InputTokens, &r.OutputTokens, &r.CostUSD); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ByTenant aggregates non-blocked requests per tenant.
func (s *Store) ByTenant() []spend.TenantRow {
	if s == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT tenant_id, COUNT(*), COALESCE(SUM(cost_usd),0)
		FROM requests WHERE blocked = 0 GROUP BY tenant_id ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []spend.TenantRow
	for rows.Next() {
		var r spend.TenantRow
		if err := rows.Scan(&r.TenantID, &r.Requests, &r.CostUSD); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// TenantUsage summarizes a tenant's realized activity from the durable request
// log — the "has this prospect tested?" signal for agent-driven follow-up.
type TenantUsage struct {
	Requests     int       `json:"requests"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	LastActivity time.Time `json:"last_activity"` // zero = never
}

// UsageForTenant aggregates non-blocked requests for one tenant id. Mint a
// per-prospect key with a unique tenant to make this attributable to one prospect.
func (s *Store) UsageForTenant(tenantID string) TenantUsage {
	var u TenantUsage
	if s == nil || strings.TrimSpace(tenantID) == "" {
		return u
	}
	var last sql.NullString
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), MAX(time)
		FROM requests WHERE blocked = 0 AND tenant_id = ?`, tenantID).
		Scan(&u.Requests, &u.InputTokens, &u.OutputTokens, &last)
	if err != nil {
		return TenantUsage{}
	}
	if last.Valid {
		u.LastActivity = parseTime(last.String)
	}
	return u
}

// KVGet returns the value for key and whether it was present. Used to persist
// small opaque blobs like the demo chat sessions across restarts/redeploys.
func (s *Store) KVGet(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	var v string
	if err := s.db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&v); err != nil {
		return "", false
	}
	return v, true
}

// KVSet upserts value for key.
func (s *Store) KVSet(key, value string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// TotalRequests returns the number of non-blocked requests recorded.
func (s *Store) TotalRequests() int64 {
	var n int64
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE blocked = 0`).Scan(&n)
	return n
}

// TotalCostUSD returns the total realized/estimated cost recorded.
func (s *Store) TotalCostUSD() float64 {
	var v float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(cost_usd),0) FROM requests WHERE blocked = 0`).Scan(&v)
	return v
}

// BlockedRequests returns the number of requests policy blocked before any
// provider call (ISSUE-076: the compliance report's "what the gate stopped").
func (s *Store) BlockedRequests() int64 {
	if s == nil {
		return 0
	}
	var n int64
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE blocked = 1`).Scan(&n)
	return n
}

// Empty reports whether no requests have been recorded yet.
func (s *Store) Empty() bool {
	var n int64
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n)
	return n == 0
}

// ImportSpendSnapshot seeds the history with one synthetic row per model from a
// legacy spend.json snapshot, preserving aggregate continuity when a deployment
// upgrades from the file-based tracker. No-op unless the store is empty.
func (s *Store) ImportSpendSnapshot(snap spend.Snapshot) error {
	if !s.Empty() {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range snap.Models {
		// One synthetic row per model carrying the aggregate; request count is
		// preserved separately by repeating rows only for counting purposes —
		// instead we store a single row and rely on aggregate columns. To keep
		// TotalRequests meaningful we insert Requests rows only when small,
		// otherwise a single carrier row (counts are approximate post-import).
		if _, err := s.db.Exec(`INSERT INTO requests
			(time, request_id, tenant_id, task_type, model, provider, input_tokens, output_tokens, cost_usd)
			VALUES (?, ?, '', 'imported', ?, ?, ?, ?, ?)`,
			now, "imported-"+m.ModelID, m.ModelID, m.ProviderID, m.InputTokens, m.OutputTokens, m.CostUSD); err != nil {
			return fmt.Errorf("history: import snapshot: %w", err)
		}
	}
	return nil
}

// StartRetention launches a background sweep that deletes rows older than
// days once per interval, until ctx is cancelled. days <= 0 disables it.
func (s *Store) StartRetention(ctx context.Context, days int, interval time.Duration, logger *slog.Logger) {
	if s == nil || days <= 0 {
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)
				s.mu.Lock()
				res, err := s.db.Exec(`DELETE FROM requests WHERE time < ?`, cutoff)
				s.mu.Unlock()
				if err != nil {
					logger.Warn("history retention sweep failed", "err", err)
					continue
				}
				if n, _ := res.RowsAffected(); n > 0 {
					logger.Info("history retention sweep", "deleted", n, "older_than_days", days)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ResetRequests deletes ALL request-log rows — the durable half of the audited
// demo-data reset (ISSUE-096). It never touches api_keys, admin_users, the
// roster or the audit chain (the chain records the reset itself).
func (s *Store) ResetRequests() (int64, error) {
	if s == nil {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM requests`)
	if err != nil {
		return 0, fmt.Errorf("history: reset requests: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// EgressRow is one bucket of "where the data went" (ISSUE-116): request
// counts per (blocked, egress, sensitivity, model). Model is kept so rows
// recorded before the egress column existed can be classified by the caller.
type EgressRow struct {
	Blocked     bool
	Egress      string
	Sensitivity string
	Model       string
	Provider    string
	Count       int
}

// EgressRows aggregates all retained requests into EgressRow buckets.
// Classifications only — never content.
func (s *Store) EgressRows() []EgressRow {
	if s == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT blocked, egress, sensitivity, model, provider, COUNT(*)
		FROM requests GROUP BY blocked, egress, sensitivity, model, provider`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []EgressRow
	for rows.Next() {
		var r EgressRow
		var blocked int
		if err := rows.Scan(&blocked, &r.Egress, &r.Sensitivity, &r.Model, &r.Provider, &r.Count); err != nil {
			continue
		}
		r.Blocked = blocked != 0
		out = append(out, r)
	}
	return out
}
