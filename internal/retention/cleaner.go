package retention

import (
	"context"
	"log/slog"
	"time"
)

// Table describes a log table eligible for retention sweeps.
type Table struct {
	Name            string
	TimestampColumn string
	HasTenantID     bool
}

// DefaultTables are the log tables swept by the cleaner. Tables with a tenant_id
// column are swept per tenant (honouring per-tenant retention); tables without
// one are swept by the default retention window only.
func DefaultTables() []Table {
	return []Table{
		{Name: "request_logs", TimestampColumn: "created_at", HasTenantID: true},
		{Name: "audit_log", TimestampColumn: "created_at", HasTenantID: true},
		{Name: "route_attempts", TimestampColumn: "attempted_at", HasTenantID: false},
	}
}

// PurgeRequest is a single delete instruction. An empty TenantID means "all
// tenants except ExcludeTenants" (the default-retention sweep); when TenantID is
// set, ExcludeTenants is ignored.
type PurgeRequest struct {
	Table           string
	TimestampColumn string
	TenantID        string
	ExcludeTenants  []string
	Cutoff          time.Time
}

// Purger executes a PurgeRequest and returns the number of rows removed.
// Implementations must be safe for sequential use by the cleaner.
type Purger interface {
	Purge(ctx context.Context, req PurgeRequest) (int64, error)
}

// Cleaner sweeps expired rows from the configured tables per Settings.
type Cleaner struct {
	Settings *Settings
	Purger   Purger
	Tables   []Table
	Logger   *slog.Logger
}

// NewCleaner builds a cleaner over the default tables.
func NewCleaner(s *Settings, p Purger, logger *slog.Logger) *Cleaner {
	return &Cleaner{Settings: s, Purger: p, Tables: DefaultTables(), Logger: logger}
}

// SweepResult summarizes a single sweep.
type SweepResult struct {
	RowsPurged int64
	Requests   int
	Errors     int
}

// Sweep purges expired rows as of now. It is idempotent and never panics:
// per-request failures are counted and logged so one failing table does not
// abort the whole sweep.
func (c *Cleaner) Sweep(ctx context.Context, now time.Time) SweepResult {
	var res SweepResult
	if c == nil || c.Settings == nil || c.Purger == nil {
		return res
	}
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	overrides := c.Settings.TenantIDs()

	for _, t := range c.Tables {
		for _, req := range c.requestsFor(t, overrides, now) {
			res.Requests++
			n, err := c.Purger.Purge(ctx, req)
			if err != nil {
				res.Errors++
				logger.ErrorContext(ctx, "retention_purge_failed",
					"table", req.Table, "tenant_id", req.TenantID, "err", err.Error())
				continue
			}
			res.RowsPurged += n
			if n > 0 {
				logger.InfoContext(ctx, "retention_purged",
					"table", req.Table, "tenant_id", req.TenantID,
					"rows", n, "cutoff", req.Cutoff.Format(time.RFC3339))
			}
		}
	}
	return res
}

// requestsFor builds the purge plan for one table.
func (c *Cleaner) requestsFor(t Table, overrides []string, now time.Time) []PurgeRequest {
	if !t.HasTenantID {
		// No tenant column: a single default-retention sweep by time.
		return []PurgeRequest{{
			Table:           t.Name,
			TimestampColumn: t.TimestampColumn,
			Cutoff:          c.Settings.Cutoff("", now),
		}}
	}
	reqs := make([]PurgeRequest, 0, len(overrides)+1)
	for _, tid := range overrides {
		reqs = append(reqs, PurgeRequest{
			Table:           t.Name,
			TimestampColumn: t.TimestampColumn,
			TenantID:        tid,
			Cutoff:          c.Settings.Cutoff(tid, now),
		})
	}
	// Default sweep for everyone without an explicit override.
	reqs = append(reqs, PurgeRequest{
		Table:           t.Name,
		TimestampColumn: t.TimestampColumn,
		ExcludeTenants:  overrides,
		Cutoff:          c.Settings.Cutoff("", now),
	})
	return reqs
}

// Run executes Sweep every interval until ctx is cancelled. The first sweep runs
// immediately. now is injectable for tests; pass nil to use time.Now.
func (c *Cleaner) Run(ctx context.Context, interval time.Duration, now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.Sweep(ctx, now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Sweep(ctx, now())
		}
	}
}

// DryRunPurger logs the delete it would run and removes nothing. It lets the
// worker exercise a real retention sweep in environments where no database is
// wired yet.
type DryRunPurger struct {
	Logger *slog.Logger
}

func (p *DryRunPurger) Purge(ctx context.Context, req PurgeRequest) (int64, error) {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "retention_purge_dry_run",
		"table", req.Table,
		"tenant_id", req.TenantID,
		"exclude_tenants", req.ExcludeTenants,
		"cutoff", req.Cutoff.Format(time.RFC3339),
	)
	return 0, nil
}
