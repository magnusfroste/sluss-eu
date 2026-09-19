package budget

import (
	"context"
	"math"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// usdToMicros converts a USD float amount to micro-USD, rounding to the nearest
// micro. Non-positive amounts become zero.
func usdToMicros(usd float64) int64 {
	if usd <= 0 {
		return 0
	}
	return int64(math.Round(usd * 1_000_000))
}

// Handle implements eventlog.Handler so the Ledger accrues spend from the async
// event queue. Spend is attributed from the decision event, which carries the
// tenant/project and the estimated cost; attempt events carry no tenant/project
// so they are not used here. It never blocks the request path.
func (l *Ledger) Handle(_ context.Context, e eventlog.Event) {
	if e.Type != eventlog.EventTypeDecision {
		return
	}
	d := e.Decision
	if d == nil || d.Blocked {
		return
	}
	l.Add(d.TenantID, d.ProjectID, usdToMicros(d.EstimatedCostUSD))
}
