// Package budget implements per-tenant/project spend caps (ISSUE-051). An
// Evaluator compares accrued spend (from an in-memory Ledger) against a Cap and
// returns a Verdict: OK, a warning past a threshold, or over-budget with an
// action (block or downgrade). All reads are in-memory and lock-light so the
// check stays on the fast path without adding a provider/DB round-trip.
package budget

import (
	"strings"
	"sync"
)

// Action is what to do when a scope is over budget.
type Action string

const (
	// ActionBlock rejects the request when over budget.
	ActionBlock Action = "block"
	// ActionDowngrade forces cheaper routing instead of rejecting.
	ActionDowngrade Action = "downgrade"
)

// Status is the budget state of a scope.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusOver Status = "over"
)

// DefaultWarnThreshold is the fraction of the limit at which a warning fires.
const DefaultWarnThreshold = 0.8

// Cap is a spend limit for a scope.
type Cap struct {
	LimitMicroUSD int64
	// WarnThreshold is the fraction of the limit that triggers StatusWarn.
	// Values <= 0 fall back to DefaultWarnThreshold; values are clamped to <= 1.
	WarnThreshold float64
	// Action when over budget. Empty defaults to ActionBlock.
	Action Action
}

func (c Cap) warn() float64 {
	w := c.WarnThreshold
	if w <= 0 {
		w = DefaultWarnThreshold
	}
	if w > 1 {
		w = 1
	}
	return w
}

func (c Cap) action() Action {
	if c.Action == ActionDowngrade {
		return ActionDowngrade
	}
	return ActionBlock
}

// Verdict is the outcome of a budget check.
type Verdict struct {
	Status        Status
	Action        Action // meaningful only when Status == StatusOver
	SpentMicroUSD int64
	LimitMicroUSD int64
	Fraction      float64
	Scope         string // human label for logging, e.g. "tenant=tn_1 project=prj_1"
}

// Blocked reports whether the request should be rejected.
func (v Verdict) Blocked() bool { return v.Status == StatusOver && v.Action == ActionBlock }

// Downgrade reports whether the request should be routed more cheaply.
func (v Verdict) Downgrade() bool { return v.Status == StatusOver && v.Action == ActionDowngrade }

type scopeKey struct {
	tenant  string
	project string
}

// Caps holds per-tenant and per-project budget limits. Project caps take
// precedence over tenant caps. Configure during setup; reads are concurrent-safe.
type Caps struct {
	mu      sync.RWMutex
	tenant  map[string]Cap
	project map[scopeKey]Cap
}

// NewCaps returns an empty cap set.
func NewCaps() *Caps {
	return &Caps{tenant: map[string]Cap{}, project: map[scopeKey]Cap{}}
}

// SetTenant sets a tenant-wide cap.
func (c *Caps) SetTenant(tenant string, cap Cap) {
	if tenant == "" {
		return
	}
	c.mu.Lock()
	c.tenant[tenant] = cap
	c.mu.Unlock()
}

// SetProject sets a project-scoped cap (takes precedence over the tenant cap).
func (c *Caps) SetProject(tenant, project string, cap Cap) {
	if tenant == "" || project == "" {
		return
	}
	c.mu.Lock()
	c.project[scopeKey{tenant, project}] = cap
	c.mu.Unlock()
}

// lookup returns the applicable cap and whether it is project-scoped.
func (c *Caps) lookup(tenant, project string) (cap Cap, projectScoped, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if project != "" {
		if cp, found := c.project[scopeKey{tenant, project}]; found {
			return cp, true, true
		}
	}
	if ct, found := c.tenant[tenant]; found {
		return ct, false, true
	}
	return Cap{}, false, false
}

// Ledger accrues spend per tenant and per tenant/project in micro-USD.
type Ledger struct {
	mu       sync.RWMutex
	byTenant map[string]int64
	byScope  map[scopeKey]int64
}

// NewLedger returns an empty ledger.
func NewLedger() *Ledger {
	return &Ledger{byTenant: map[string]int64{}, byScope: map[scopeKey]int64{}}
}

// Add records micros of spend against a tenant (and project, when non-empty).
// Negative amounts are ignored.
func (l *Ledger) Add(tenant, project string, micros int64) {
	if tenant == "" || micros <= 0 {
		return
	}
	l.mu.Lock()
	l.byTenant[tenant] += micros
	if project != "" {
		l.byScope[scopeKey{tenant, project}] += micros
	}
	l.mu.Unlock()
}

// SpentTenant returns total spend for a tenant.
func (l *Ledger) SpentTenant(tenant string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.byTenant[tenant]
}

// SpentProject returns total spend for a tenant/project.
func (l *Ledger) SpentProject(tenant, project string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.byScope[scopeKey{tenant, project}]
}

// Evaluator checks accrued spend against configured caps.
type Evaluator struct {
	Caps   *Caps
	Ledger *Ledger
}

// NewEvaluator builds an Evaluator over the given caps and ledger.
func NewEvaluator(caps *Caps, ledger *Ledger) *Evaluator {
	return &Evaluator{Caps: caps, Ledger: ledger}
}

// Check returns the budget verdict for a tenant/project. When no cap applies (or
// the evaluator is not fully configured) it returns StatusOK so the request
// proceeds unchanged — budgets are opt-in.
func (e *Evaluator) Check(tenant, project string) Verdict {
	if e == nil || e.Caps == nil || e.Ledger == nil {
		return Verdict{Status: StatusOK}
	}
	cap, projectScoped, ok := e.Caps.lookup(tenant, project)
	if !ok || cap.LimitMicroUSD <= 0 {
		return Verdict{Status: StatusOK}
	}

	var spent int64
	if projectScoped {
		spent = e.Ledger.SpentProject(tenant, project)
	} else {
		spent = e.Ledger.SpentTenant(tenant)
	}

	v := Verdict{
		SpentMicroUSD: spent,
		LimitMicroUSD: cap.LimitMicroUSD,
		Fraction:      float64(spent) / float64(cap.LimitMicroUSD),
		Scope:         scopeLabel(tenant, project, projectScoped),
	}
	switch {
	case spent >= cap.LimitMicroUSD:
		v.Status = StatusOver
		v.Action = cap.action()
	case v.Fraction >= cap.warn():
		v.Status = StatusWarn
	default:
		v.Status = StatusOK
	}
	return v
}

func scopeLabel(tenant, project string, projectScoped bool) string {
	var b strings.Builder
	b.WriteString("tenant=")
	b.WriteString(tenant)
	if projectScoped {
		b.WriteString(" project=")
		b.WriteString(project)
	}
	return b.String()
}
