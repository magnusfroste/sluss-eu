// Package spend provides an in-memory spend aggregator updated by the async
// event queue. It accumulates request counts and cost estimates (replaced by
// actual provider usage when available) per model and per tenant.
package spend

import (
	"context"
	"sync"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// ModelRow is one entry in the per-model spend table.
type ModelRow struct {
	ModelID      string
	ProviderID   string
	Requests     int64
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

// TenantRow is one entry in the per-tenant spend table.
type TenantRow struct {
	TenantID string
	Requests int64
	CostUSD  float64
}

// Tracker accumulates spend from decision and attempt events.
type Tracker struct {
	mu       sync.RWMutex
	byModel  map[string]*modelAccum  // model ID → accum
	byTenant map[string]*tenantAccum // tenant ID → accum
}

type modelAccum struct {
	providerID   string
	requests     int64
	inputTokens  int64
	outputTokens int64
	costUSD      float64
}

type tenantAccum struct {
	requests int64
	costUSD  float64
}

// New returns a ready-to-use Tracker.
func New() *Tracker {
	return &Tracker{
		byModel:  make(map[string]*modelAccum),
		byTenant: make(map[string]*tenantAccum),
	}
}

// Handle implements eventlog.Handler. It is safe for concurrent calls.
func (t *Tracker) Handle(_ context.Context, e eventlog.Event) {
	switch e.Type {
	case eventlog.EventTypeDecision:
		if e.Decision != nil {
			t.recordDecision(e.Decision)
		}
	case eventlog.EventTypeAttempt:
		if e.Attempt != nil && e.Attempt.Success {
			t.recordAttempt(e.Attempt)
		}
	}
}

// recordDecision counts requests per model and per tenant. Cost is intentionally
// NOT added here — it is attributed once on the successful attempt (recordAttempt)
// as realized cost, so the running totals reflect actual spend rather than the
// decision-time estimate (and are never double-counted).
func (t *Tracker) recordDecision(d *eventlog.DecisionEvent) {
	if d.Blocked {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	m, ok := t.byModel[d.SelectedModel]
	if !ok {
		m = &modelAccum{providerID: d.SelectedProvider}
		t.byModel[d.SelectedModel] = m
	}
	m.requests++

	if d.TenantID != "" {
		ten, ok := t.byTenant[d.TenantID]
		if !ok {
			ten = &tenantAccum{}
			t.byTenant[d.TenantID] = ten
		}
		ten.requests++
	}
}

// recordAttempt attributes realized cost and token usage once per request, on
// the successful attempt. Cost is the actual cost from provider usage when
// available, otherwise the decision-time estimate carried on the event.
func (t *Tracker) recordAttempt(a *eventlog.AttemptEvent) {
	cost := a.ActualCostUSD
	if cost == 0 {
		cost = a.EstimatedCostUSD
	}
	if cost == 0 && a.InputTokens == 0 && a.OutputTokens == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	m, ok := t.byModel[a.ModelID]
	if !ok {
		m = &modelAccum{providerID: a.ProviderID}
		t.byModel[a.ModelID] = m
	}
	m.inputTokens += int64(a.InputTokens)
	m.outputTokens += int64(a.OutputTokens)
	m.costUSD += cost

	if a.TenantID != "" {
		ten, ok := t.byTenant[a.TenantID]
		if !ok {
			ten = &tenantAccum{}
			t.byTenant[a.TenantID] = ten
		}
		ten.costUSD += cost
	}
}

// ByModel returns a snapshot of per-model spend rows, sorted by requests descending.
func (t *Tracker) ByModel() []ModelRow {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows := make([]ModelRow, 0, len(t.byModel))
	for id, m := range t.byModel {
		rows = append(rows, ModelRow{
			ModelID:      id,
			ProviderID:   m.providerID,
			Requests:     m.requests,
			InputTokens:  m.inputTokens,
			OutputTokens: m.outputTokens,
			CostUSD:      m.costUSD,
		})
	}
	sortModelRowsDesc(rows)
	return rows
}

// ByTenant returns a snapshot of per-tenant spend rows, sorted by requests descending.
func (t *Tracker) ByTenant() []TenantRow {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows := make([]TenantRow, 0, len(t.byTenant))
	for id, ten := range t.byTenant {
		rows = append(rows, TenantRow{
			TenantID: id,
			Requests: ten.requests,
			CostUSD:  ten.costUSD,
		})
	}
	sortTenantRowsDesc(rows)
	return rows
}

// TotalRequests returns the sum of all requests across all models.
func (t *Tracker) TotalRequests() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var total int64
	for _, m := range t.byModel {
		total += m.requests
	}
	return total
}

// TotalCostUSD returns the estimated total spend in USD.
func (t *Tracker) TotalCostUSD() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var total float64
	for _, m := range t.byModel {
		total += m.costUSD
	}
	return total
}

func sortModelRowsDesc(rows []ModelRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Requests > rows[j-1].Requests; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func sortTenantRowsDesc(rows []TenantRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Requests > rows[j-1].Requests; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// EraseTenant removes one tenant's accumulated attribution (GDPR erasure).
// Model-level aggregates are not tenant-attributed and stay.
func (t *Tracker) EraseTenant(tenantID string) {
	if t == nil || tenantID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byTenant, tenantID)
}

// Reset clears all accumulated spend (models + tenants). Used by the audited
// demo-data reset (ISSUE-096); the next periodic flush persists the empty state.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byModel = make(map[string]*modelAccum)
	t.byTenant = make(map[string]*tenantAccum)
}
