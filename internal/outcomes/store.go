// Package outcomes stores client-reported routing outcomes (acceptance
// feedback) and aggregates acceptance rates per model and task class.
// Outcomes feed the dashboard (ISSUE-040) and, later, the routing feedback loop.
package outcomes

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrMissingRequestID = errors.New("outcomes: request_id is required")
	ErrInvalidVerdict   = errors.New("outcomes: verdict must be accepted|rejected|partial")
	ErrInvalidRating    = errors.New("outcomes: rating must be between 1 and 5")
)

// Verdict is the client's judgement of a routed response.
type Verdict string

const (
	VerdictAccepted Verdict = "accepted"
	VerdictRejected Verdict = "rejected"
	VerdictPartial  Verdict = "partial"
)

// Outcome is a single client-reported result for a request.
type Outcome struct {
	RequestID  string    `json:"request_id"`
	Verdict    Verdict   `json:"verdict"`
	Model      string    `json:"model,omitempty"`
	TaskType   string    `json:"task_type,omitempty"`
	Rating     int       `json:"rating,omitempty"` // optional 1–5
	Comment    string    `json:"comment,omitempty"`
	ReportedAt time.Time `json:"reported_at"`
}

// Validate checks an outcome submission.
func (o Outcome) Validate() error {
	if o.RequestID == "" {
		return ErrMissingRequestID
	}
	switch o.Verdict {
	case VerdictAccepted, VerdictRejected, VerdictPartial:
	default:
		return ErrInvalidVerdict
	}
	if o.Rating != 0 && (o.Rating < 1 || o.Rating > 5) {
		return ErrInvalidRating
	}
	return nil
}

// AcceptanceRow aggregates acceptance for one (model, task) grouping.
type AcceptanceRow struct {
	Key            string  `json:"key"`
	Model          string  `json:"model"`
	TaskType       string  `json:"task_type"`
	Total          int     `json:"total"`
	Accepted       int     `json:"accepted"`
	Rejected       int     `json:"rejected"`
	Partial        int     `json:"partial"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

// Store is an in-memory, concurrency-safe outcome store.
type Store struct {
	mu       sync.RWMutex
	outcomes []Outcome
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{}
}

// Record validates and stores an outcome. ReportedAt is set if zero.
func (s *Store) Record(o Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if o.ReportedAt.IsZero() {
		o.ReportedAt = time.Now()
	}
	s.mu.Lock()
	s.outcomes = append(s.outcomes, o)
	s.mu.Unlock()
	return nil
}

// Count returns the number of stored outcomes.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.outcomes)
}

// Acceptance returns acceptance rows grouped by model, optionally filtered by
// task type (empty taskFilter = all). Rows are sorted by total descending.
func (s *Store) Acceptance(taskFilter string) []AcceptanceRow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make(map[string]*AcceptanceRow)
	for _, o := range s.outcomes {
		if taskFilter != "" && o.TaskType != taskFilter {
			continue
		}
		key := o.Model + "|" + o.TaskType
		row, ok := groups[key]
		if !ok {
			row = &AcceptanceRow{Key: key, Model: o.Model, TaskType: o.TaskType}
			groups[key] = row
		}
		row.Total++
		switch o.Verdict {
		case VerdictAccepted:
			row.Accepted++
		case VerdictRejected:
			row.Rejected++
		case VerdictPartial:
			row.Partial++
		}
	}

	rows := make([]AcceptanceRow, 0, len(groups))
	for _, row := range groups {
		if row.Total > 0 {
			// Partial counts as half-accepted for the rate.
			row.AcceptanceRate = (float64(row.Accepted) + 0.5*float64(row.Partial)) / float64(row.Total)
		}
		rows = append(rows, *row)
	}
	sortAcceptanceDesc(rows)
	return rows
}

// TaskTypes returns the distinct task types seen, sorted, for dashboard filters.
func (s *Store) TaskTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, o := range s.outcomes {
		if o.TaskType != "" {
			seen[o.TaskType] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sortStrings(out)
	return out
}

func sortAcceptanceDesc(rows []AcceptanceRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Total > rows[j-1].Total; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
