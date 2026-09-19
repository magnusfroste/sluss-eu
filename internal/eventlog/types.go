// Package eventlog defines the async event types emitted by the routing engine
// and provider executor. The queue is non-blocking; failed enqueues are counted
// but never propagated to the request path.
package eventlog

import (
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
)

// EventType identifies the kind of event.
type EventType string

const (
	EventTypeDecision EventType = "decision"
	EventTypeAttempt  EventType = "attempt"
)

// Event is the union type enqueued by the router.
type Event struct {
	Type     EventType
	Decision *DecisionEvent
	Attempt  *AttemptEvent
}

// DecisionEvent is emitted once per request after the routing engine has
// selected a model. It captures the full routing context.
type DecisionEvent struct {
	RequestID         string
	TenantID          string
	ProjectID         string
	TaskType          string
	RiskLevel         string
	Sensitivity       string
	SelectedModel     string
	SelectedProvider  string
	ProviderModelID   string // concrete model sent to the provider (e.g. openai/gpt-4o-mini)
	PolicyVersion     string
	PromptTokens      int
	EstimatedCostUSD  float64
	RoutingDurationMs int64 // whole milliseconds, for DB/logs
	// RoutingDurationMicros preserves sub-millisecond precision for the
	// latency histogram — routing is typically well under 1 ms, so storing
	// only whole milliseconds would round every observation to zero.
	RoutingDurationMicros int64
	Blocked               bool
	BlockCode             string
	ShadowComparison      *engine.DecisionComparison
	// ExperimentArm is the live A/B arm ("control"/"treatment") this request was
	// bucketed into, or "" when no experiment is running. Lets outcomes be sliced
	// by arm.
	ExperimentArm string
	DecidedAt     time.Time
}

// AttemptEvent is emitted once per provider call (primary or fallback).
type AttemptEvent struct {
	RequestID    string
	TenantID     string
	ProjectID    string
	ProviderID   string
	ModelID      string
	AttemptIndex int
	Success      bool
	ErrorCode    string
	DurationMs   int64
	InputTokens  int
	OutputTokens int
	// ActualCostUSD is the realized cost computed from provider token usage; 0
	// when usage is unavailable (e.g. streaming), in which case spend falls back
	// to EstimatedCostUSD.
	ActualCostUSD float64
	// EstimatedCostUSD carries the decision-time estimate so spend accounting has
	// a fallback and can attribute realized cost once, on the successful attempt.
	EstimatedCostUSD float64
	FirstTokenMs     int64
	AttemptedAt      time.Time
}
