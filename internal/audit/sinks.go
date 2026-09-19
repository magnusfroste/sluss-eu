package audit

import (
	"context"
	"log/slog"
	"sort"
	"sync"
)

// LogSink writes each entry as a structured "audit" log line. It is the default
// sink in every deployment so the audit trail lands wherever logs are shipped.
type LogSink struct {
	Logger *slog.Logger
}

func (s *LogSink) Record(ctx context.Context, e Entry) {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []any{
		"audit_action", string(e.Action),
		"outcome", e.Outcome,
	}
	if e.Actor != "" {
		attrs = append(attrs, "actor", e.Actor)
	}
	if e.TenantID != "" {
		attrs = append(attrs, "tenant_id", e.TenantID)
	}
	if e.ProjectID != "" {
		attrs = append(attrs, "project_id", e.ProjectID)
	}
	if e.Target != "" {
		attrs = append(attrs, "target", e.Target)
	}
	if e.RequestID != "" {
		attrs = append(attrs, "request_id", e.RequestID)
	}
	if e.Reason != "" {
		attrs = append(attrs, "reason", e.Reason)
	}
	// Detail keys are sorted so log lines are deterministic.
	if len(e.Detail) > 0 {
		keys := make([]string, 0, len(e.Detail))
		for k := range e.Detail {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			attrs = append(attrs, "detail_"+k, e.Detail[k])
		}
	}
	logger.InfoContext(ctx, "audit", attrs...)
}

// MemorySink retains the most recent entries in a bounded ring buffer. It is
// safe for concurrent use and backs tests and any in-process audit retrieval.
type MemorySink struct {
	mu      sync.RWMutex
	max     int
	entries []Entry
}

const defaultMemorySinkSize = 1024

// NewMemorySink creates a MemorySink keeping at most max entries (0 → default).
func NewMemorySink(max int) *MemorySink {
	if max <= 0 {
		max = defaultMemorySinkSize
	}
	return &MemorySink{max: max}
}

func (s *MemorySink) Record(_ context.Context, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	if len(s.entries) > s.max {
		// Drop the oldest entries to stay within bounds.
		s.entries = append(s.entries[:0:0], s.entries[len(s.entries)-s.max:]...)
	}
}

// Entries returns a copy of the retained entries, oldest first.
func (s *MemorySink) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

type multiSink struct {
	sinks []Sink
}

// MultiSink returns a Sink that fans each entry out to all non-nil sinks in
// order. It mirrors eventlog.MultiHandler.
func MultiSink(sinks ...Sink) Sink {
	filtered := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			filtered = append(filtered, s)
		}
	}
	return &multiSink{sinks: filtered}
}

func (m *multiSink) Record(ctx context.Context, e Entry) {
	for _, s := range m.sinks {
		s.Record(ctx, e)
	}
}
