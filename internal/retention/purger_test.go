package retention

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// fakeExecer captures the last executed statement and returns a fixed result.
type fakeExecer struct {
	query string
	args  []any
	rows  int64
}

type fakeResult struct{ rows int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.rows, nil }

func (e *fakeExecer) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	return fakeResult{rows: e.rows}, nil
}

func TestSQLPurgerTenantScoped(t *testing.T) {
	fe := &fakeExecer{rows: 5}
	p := &SQLPurger{DB: fe}
	cutoff := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	n, err := p.Purge(context.Background(), PurgeRequest{
		Table:           "request_logs",
		TimestampColumn: "created_at",
		TenantID:        "tn_1",
		Cutoff:          cutoff,
	})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 5 {
		t.Errorf("rows = %d, want 5", n)
	}
	if !strings.Contains(fe.query, "DELETE FROM request_logs WHERE created_at < $1") {
		t.Errorf("unexpected query: %s", fe.query)
	}
	if !strings.Contains(fe.query, "tenant_id = $2") {
		t.Errorf("query missing tenant filter: %s", fe.query)
	}
	if len(fe.args) != 2 || fe.args[1] != "tn_1" {
		t.Errorf("args = %v, want [cutoff tn_1]", fe.args)
	}
}

func TestSQLPurgerDefaultSweepExcludesTenants(t *testing.T) {
	fe := &fakeExecer{rows: 2}
	p := &SQLPurger{DB: fe}

	_, err := p.Purge(context.Background(), PurgeRequest{
		Table:           "audit_log",
		TimestampColumn: "created_at",
		ExcludeTenants:  []string{"tn_a", "tn_b"},
		Cutoff:          time.Now(),
	})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if !strings.Contains(fe.query, "tenant_id NOT IN ($2, $3)") {
		t.Errorf("query missing exclusion: %s", fe.query)
	}
	if len(fe.args) != 3 || fe.args[1] != "tn_a" || fe.args[2] != "tn_b" {
		t.Errorf("args = %v, want [cutoff tn_a tn_b]", fe.args)
	}
}

func TestSQLPurgerTimeOnly(t *testing.T) {
	fe := &fakeExecer{rows: 1}
	p := &SQLPurger{DB: fe}

	_, err := p.Purge(context.Background(), PurgeRequest{
		Table:           "route_attempts",
		TimestampColumn: "attempted_at",
		Cutoff:          time.Now(),
	})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if strings.Contains(fe.query, "tenant_id") {
		t.Errorf("time-only sweep should not filter by tenant: %s", fe.query)
	}
	if !strings.Contains(fe.query, "attempted_at < $1") {
		t.Errorf("query should use attempted_at: %s", fe.query)
	}
	if len(fe.args) != 1 {
		t.Errorf("args = %v, want [cutoff]", fe.args)
	}
}

func TestSQLPurgerNoHandle(t *testing.T) {
	p := &SQLPurger{}
	if _, err := p.Purge(context.Background(), PurgeRequest{Table: "x"}); err == nil {
		t.Error("expected error when DB handle is nil")
	}
}
