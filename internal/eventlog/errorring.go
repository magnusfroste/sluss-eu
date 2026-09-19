package eventlog

import (
	"context"
	"sync"
	"time"
)

// ErrorRing keeps the last N failed attempts in memory so a live instance can be
// debugged without shelling into logs: it turns "why did that request 502?" into
// an MCP query (recent_errors). It implements Handler and taps the same event
// stream as spend/history — a failed AttemptEvent (Success=false with an error
// code) is recorded. Ephemeral by design: cleared on restart, which is exactly
// when you look (right after a failure).
type ErrorRing struct {
	mu   sync.Mutex
	buf  []AttemptError
	next int
	size int
	full bool
}

// AttemptError is one recorded upstream failure — no prompt text, ever.
type AttemptError struct {
	At           time.Time `json:"at"`
	RequestID    string    `json:"request_id"`
	ProviderID   string    `json:"provider_id"`
	ModelID      string    `json:"model_id"`
	ErrorCode    string    `json:"error_code"`
	AttemptIndex int       `json:"attempt_index"`
	DurationMs   int64     `json:"duration_ms"`
	TenantID     string    `json:"tenant_id,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
}

// NewErrorRing returns a ring holding the most recent size failures (0 → 100).
func NewErrorRing(size int) *ErrorRing {
	if size <= 0 {
		size = 100
	}
	return &ErrorRing{buf: make([]AttemptError, size), size: size}
}

// Handle records failed attempts and ignores everything else.
func (r *ErrorRing) Handle(_ context.Context, e Event) {
	if e.Type != EventTypeAttempt || e.Attempt == nil {
		return
	}
	a := e.Attempt
	if a.Success || a.ErrorCode == "" {
		return
	}
	r.mu.Lock()
	r.buf[r.next] = AttemptError{
		At:           a.AttemptedAt,
		RequestID:    a.RequestID,
		ProviderID:   a.ProviderID,
		ModelID:      a.ModelID,
		ErrorCode:    a.ErrorCode,
		AttemptIndex: a.AttemptIndex,
		DurationMs:   a.DurationMs,
		TenantID:     a.TenantID,
		ProjectID:    a.ProjectID,
	}
	r.next = (r.next + 1) % r.size
	if r.next == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

// Recent returns up to n most recent failures, newest first.
func (r *ErrorRing) Recent(n int) []AttemptError {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	count := r.next
	if r.full {
		count = r.size
	}
	if n <= 0 || n > count {
		n = count
	}
	out := make([]AttemptError, 0, n)
	// Walk backwards from the most recently written slot.
	idx := (r.next - 1 + r.size) % r.size
	for i := 0; i < n; i++ {
		out = append(out, r.buf[idx])
		idx = (idx - 1 + r.size) % r.size
	}
	return out
}

// Reset clears the ring (audited demo-data reset, ISSUE-096).
func (r *ErrorRing) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.buf = make([]AttemptError, r.size)
	r.next, r.full = 0, false
	r.mu.Unlock()
}
