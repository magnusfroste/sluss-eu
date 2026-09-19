// Package audit provides a security audit trail for sensitive control-plane
// changes: policy reloads, API-key mutations, and blocked requests (ISSUE-044).
//
// It mirrors the eventlog package's sink/fan-out shape but is intentionally
// synchronous and small — audit volume is low, entries are security-relevant,
// and ordering matters more than throughput. Audit recording must never affect
// the request path: all helpers are nil-safe and side-effect free on the caller.
package audit

import (
	"context"
	"time"
)

// Action identifies the kind of audited control-plane change.
type Action string

const (
	// ActionPolicyReload is recorded when a compiled policy snapshot is
	// (re)loaded into the cache, per scope.
	ActionPolicyReload Action = "policy.reload"
	// ActionAPIKeyAdd is recorded when an API key is added to the key store.
	ActionAPIKeyAdd Action = "api_key.add"
	// ActionAPIKeyDisable is recorded when an API key is disabled/removed.
	ActionAPIKeyDisable Action = "api_key.disable"
	// ActionRequestBlocked is recorded when policy blocks a request before any
	// provider call.
	ActionRequestBlocked Action = "request.blocked"
	// ActionPolicyConsole is recorded when the policy console (no-YAML rule
	// editing, ISSUE-093) changes the active ruleset: rule add/delete while
	// active, activate, or rollback to the baseline.
	ActionPolicyConsole Action = "policy.console"
	// ActionProviderTags is recorded when a provider's compliance/risk-register
	// tags change (ISSUE-093): the tags are supply-chain facts policy routes on,
	// so who changed them and when is evidence.
	ActionProviderTags Action = "provider.tags"
)

// Outcome describes the result of an audited action.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
	OutcomeBlocked = "blocked"
)

// Entry is a single immutable audit record. It deliberately carries no prompt
// text or secret material — only identifiers and reasons safe to retain.
type Entry struct {
	Time      time.Time
	Action    Action
	Actor     string // who/what performed the action (e.g. "system", key id)
	TenantID  string
	ProjectID string
	Target    string // the resource affected (key id, policy version, model)
	Outcome   string // OutcomeSuccess | OutcomeFailure | OutcomeBlocked
	RequestID string // set when the entry is tied to a request
	Reason    string
	Detail    map[string]string
}

// Sink consumes audit entries. Implementations must be safe for concurrent use.
type Sink interface {
	Record(ctx context.Context, e Entry)
}

// Record sends e to sink, defaulting Time and Outcome. It is nil-safe: a nil
// sink is a no-op, so callers can hold an optional Sink without guarding every
// call site.
func Record(ctx context.Context, sink Sink, e Entry) {
	if sink == nil {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.Outcome == "" {
		e.Outcome = OutcomeSuccess
	}
	sink.Record(ctx, e)
}
