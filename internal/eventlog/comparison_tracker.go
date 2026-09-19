package eventlog

import (
	"context"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
)

const defaultComparisonRecentLimit = 50

// ComparisonSummary aggregates actual-vs-shadow routing differences across
// decision events that carried a shared DecisionComparison payload.
type ComparisonSummary struct {
	Total                      int64   `json:"total"`
	ChangedCount               int64   `json:"changed_count"`
	RouteChangedCount          int64   `json:"route_changed_count"`
	FallbackChangedCount       int64   `json:"fallback_changed_count"`
	TimeoutChangedCount        int64   `json:"timeout_changed_count"`
	VerifierChangedCount       int64   `json:"verifier_changed_count"`
	PolicyVersionChangedCount  int64   `json:"policy_version_changed_count"`
	CostChangedCount           int64   `json:"cost_changed_count"`
	EstimatedCostDeltaMicroUSD int64   `json:"estimated_cost_delta_microusd"`
	EstimatedCostDeltaUSD      float64 `json:"estimated_cost_delta_usd"`
}

// ComparisonRecord is one persisted shadow-routing comparison plus request
// metadata useful for dashboards and JSON consumers.
type ComparisonRecord struct {
	RequestID   string                    `json:"request_id"`
	TenantID    string                    `json:"tenant_id,omitempty"`
	ProjectID   string                    `json:"project_id,omitempty"`
	TaskType    string                    `json:"task_type,omitempty"`
	RiskLevel   string                    `json:"risk_level,omitempty"`
	Sensitivity string                    `json:"sensitivity,omitempty"`
	DecidedAt   time.Time                 `json:"decided_at"`
	Comparison  engine.DecisionComparison `json:"comparison"`
}

// GapSummary counts what the shadow policy WOULD have done differently for
// SENSITIVE prompts — the "shadow AI" gap report's headline numbers
// (ISSUE-095). Exact since restart, not ring-capped.
type GapSummary struct {
	SensitiveTotal         int64 `json:"sensitive_total"`          // sensitive prompts compared
	SensitivePII           int64 `json:"sensitive_pii"`            // of which pii
	SensitiveSecrets       int64 `json:"sensitive_secrets"`        // of which secrets_possible
	SensitiveOther         int64 `json:"sensitive_other"`          // of which financial/health/legal/security_classified
	SensitiveRouteChanged  int64 `json:"sensitive_route_changed"`  // shadow picked another model
	SensitiveShadowBlocked int64 `json:"sensitive_shadow_blocked"` // shadow would have blocked
}

// sensitiveClasses are the sensitivity values the gap report counts as
// sensitive: the original pii/secrets plus the richer classes (ISSUE-093).
var sensitiveClasses = map[string]bool{
	"pii":                 true,
	"secrets_possible":    true,
	"financial":           true,
	"health":              true,
	"legal":               true,
	"security_classified": true,
}

// ComparisonTracker keeps a bounded in-memory list of recent shadow-routing
// comparisons and cumulative aggregate counts.
type ComparisonTracker struct {
	mu        sync.RWMutex
	maxRecent int
	recent    []ComparisonRecord
	summary   ComparisonSummary
	gap       GapSummary
}

// NewComparisonTracker returns a ready-to-use comparison tracker.
func NewComparisonTracker(maxRecent int) *ComparisonTracker {
	if maxRecent <= 0 {
		maxRecent = defaultComparisonRecentLimit
	}
	return &ComparisonTracker{maxRecent: maxRecent}
}

// Handle implements eventlog.Handler.
func (t *ComparisonTracker) Handle(_ context.Context, e Event) {
	if e.Type != EventTypeDecision || e.Decision == nil || e.Decision.ShadowComparison == nil {
		return
	}
	t.recordDecision(e.Decision)
}

// Summary returns aggregate comparison counts across all recorded shadow decisions.
// Gap returns the sensitive-prompt gap counters (exact since restart).
func (t *ComparisonTracker) Gap() GapSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.gap
}

func (t *ComparisonTracker) Summary() ComparisonSummary {
	if t == nil {
		return ComparisonSummary{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.summary
}

// Recent returns the most recent comparison records, optionally filtered by task type.
func (t *ComparisonTracker) Recent(taskFilter string) []ComparisonRecord {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]ComparisonRecord, 0, len(t.recent))
	for _, record := range t.recent {
		if taskFilter != "" && record.TaskType != taskFilter {
			continue
		}
		out = append(out, cloneComparisonRecord(record))
	}
	return out
}

func (t *ComparisonTracker) recordDecision(d *DecisionEvent) {
	record := ComparisonRecord{
		RequestID:   d.RequestID,
		TenantID:    d.TenantID,
		ProjectID:   d.ProjectID,
		TaskType:    d.TaskType,
		RiskLevel:   d.RiskLevel,
		Sensitivity: d.Sensitivity,
		DecidedAt:   d.DecidedAt,
		Comparison:  cloneDecisionComparison(*d.ShadowComparison),
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.summary.Total++
	if record.Comparison.Changed {
		t.summary.ChangedCount++
	}
	if record.Comparison.RouteChanged {
		t.summary.RouteChangedCount++
	}
	if record.Comparison.FallbackChanged {
		t.summary.FallbackChangedCount++
	}
	if record.Comparison.TimeoutChanged {
		t.summary.TimeoutChangedCount++
	}
	if record.Comparison.RequiresVerifierChanged {
		t.summary.VerifierChangedCount++
	}
	if record.Comparison.PolicyVersionChanged {
		t.summary.PolicyVersionChangedCount++
	}
	if record.Comparison.CostChanged {
		t.summary.CostChangedCount++
	}
	t.summary.EstimatedCostDeltaMicroUSD += record.Comparison.EstimatedCostDeltaMicroUSD
	t.summary.EstimatedCostDeltaUSD = float64(t.summary.EstimatedCostDeltaMicroUSD) / 1_000_000

	// Gap counters (ISSUE-095): what the shadow pack would have changed for
	// sensitive prompts. Exact since restart, independent of the recent ring.
	if sensitiveClasses[record.Sensitivity] {
		t.gap.SensitiveTotal++
		switch record.Sensitivity {
		case "pii":
			t.gap.SensitivePII++
		case "secrets_possible":
			t.gap.SensitiveSecrets++
		default:
			t.gap.SensitiveOther++
		}
		if record.Comparison.Secondary.Blocked {
			t.gap.SensitiveShadowBlocked++
		} else if record.Comparison.RouteChanged {
			t.gap.SensitiveRouteChanged++
		}
	}

	t.recent = append([]ComparisonRecord{record}, t.recent...)
	if len(t.recent) > t.maxRecent {
		t.recent = t.recent[:t.maxRecent]
	}
}

func cloneComparisonRecord(in ComparisonRecord) ComparisonRecord {
	in.Comparison = cloneDecisionComparison(in.Comparison)
	return in
}

func cloneDecisionComparison(in engine.DecisionComparison) engine.DecisionComparison {
	in.Primary = cloneRouteDecision(in.Primary)
	in.Secondary = cloneRouteDecision(in.Secondary)
	return in
}

func cloneRouteDecision(in engine.RouteDecision) engine.RouteDecision {
	in.Fallbacks = append([]engine.FallbackEntry(nil), in.Fallbacks...)
	in.DecisionReasons = append([]string(nil), in.DecisionReasons...)
	return in
}

// Reset clears the recent ring, summary and gap counters (audited demo-data
// reset, ISSUE-096) — e.g. to start a fresh monitor-mode measurement window.
func (t *ComparisonTracker) Reset() {
	t.mu.Lock()
	t.recent = nil
	t.summary = ComparisonSummary{}
	t.gap = GapSummary{}
	t.mu.Unlock()
}
