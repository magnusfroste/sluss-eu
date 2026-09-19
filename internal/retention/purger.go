package retention

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Execer is the subset of *sql.DB / *sql.Tx that the SQL purger needs.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLPurger deletes expired rows via parameterised DELETE statements. Table and
// column names come from the cleaner's static configuration (never user input),
// while cutoffs and tenant ids are always bound as parameters.
type SQLPurger struct {
	DB Execer
}

func (p *SQLPurger) Purge(ctx context.Context, req PurgeRequest) (int64, error) {
	if p == nil || p.DB == nil {
		return 0, fmt.Errorf("retention: SQLPurger has no database handle")
	}
	ts := req.TimestampColumn
	if ts == "" {
		ts = "created_at"
	}

	var sb strings.Builder
	args := make([]any, 0, 1+len(req.ExcludeTenants))
	fmt.Fprintf(&sb, "DELETE FROM %s WHERE %s < $1", req.Table, ts)
	args = append(args, req.Cutoff)

	switch {
	case req.TenantID != "":
		sb.WriteString(" AND tenant_id = $2")
		args = append(args, req.TenantID)
	case len(req.ExcludeTenants) > 0:
		sb.WriteString(" AND tenant_id NOT IN (")
		for i, tid := range req.ExcludeTenants {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "$%d", i+2)
			args = append(args, tid)
		}
		sb.WriteString(")")
	}

	res, err := p.DB.ExecContext(ctx, sb.String(), args...)
	if err != nil {
		return 0, err
	}
	// Not all drivers report affected rows; treat that as a non-fatal zero.
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}
