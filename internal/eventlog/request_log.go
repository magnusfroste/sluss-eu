package eventlog

import (
	"context"
	"sync"
	"time"
)

const defaultRequestLogLimit = 100

// RequestLogRecord is one request's routing outcome for the dashboard request
// log — timestamp, classification, selected model, tokens and cost, similar to a
// provider's "generations" log.
type RequestLogRecord struct {
	Time            time.Time `json:"time"`
	RequestID       string    `json:"request_id"`
	TaskType        string    `json:"task_type,omitempty"`
	RiskLevel       string    `json:"risk_level,omitempty"`
	Sensitivity     string    `json:"sensitivity,omitempty"`       // classification only, never content
	Model           string    `json:"model,omitempty"`             // internal role, e.g. cheap-general
	ProviderModelID string    `json:"provider_model_id,omitempty"` // concrete model, e.g. openai/gpt-4o-mini
	Provider        string    `json:"provider,omitempty"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	CostUSD         float64   `json:"cost_usd"`
	Blocked         bool      `json:"blocked,omitempty"`
	BlockCode       string    `json:"block_code,omitempty"`
}

// RequestLogTracker keeps a bounded in-memory ring of recent request records. A
// decision event creates the record (task, model, estimated cost); the
// successful attempt then fills in actual tokens and cost, joined by request ID.
type RequestLogTracker struct {
	mu      sync.Mutex
	limit   int
	records []RequestLogRecord // oldest first
}

// NewRequestLogTracker returns a tracker retaining the last `limit` requests.
func NewRequestLogTracker(limit int) *RequestLogTracker {
	if limit <= 0 {
		limit = defaultRequestLogLimit
	}
	return &RequestLogTracker{limit: limit}
}

// Handle implements eventlog.Handler.
func (t *RequestLogTracker) Handle(_ context.Context, e Event) {
	switch e.Type {
	case EventTypeDecision:
		if e.Decision != nil {
			t.addDecision(e.Decision)
		}
	case EventTypeAttempt:
		if e.Attempt != nil && e.Attempt.Success {
			t.fillAttempt(e.Attempt)
		}
	}
}

func (t *RequestLogTracker) addDecision(d *DecisionEvent) {
	at := d.DecidedAt
	if at.IsZero() {
		at = time.Now()
	}
	rec := RequestLogRecord{
		Time:            at,
		RequestID:       d.RequestID,
		TaskType:        d.TaskType,
		RiskLevel:       d.RiskLevel,
		Sensitivity:     d.Sensitivity,
		Model:           d.SelectedModel,
		ProviderModelID: d.ProviderModelID,
		Provider:        d.SelectedProvider,
		InputTokens:     d.PromptTokens,
		CostUSD:         d.EstimatedCostUSD,
		Blocked:         d.Blocked,
		BlockCode:       d.BlockCode,
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, rec)
	if len(t.records) > t.limit {
		t.records = t.records[len(t.records)-t.limit:]
	}
}

func (t *RequestLogTracker) fillAttempt(a *AttemptEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := len(t.records) - 1; i >= 0; i-- {
		if t.records[i].RequestID == a.RequestID {
			t.records[i].InputTokens = a.InputTokens
			t.records[i].OutputTokens = a.OutputTokens
			if a.ActualCostUSD > 0 {
				t.records[i].CostUSD = a.ActualCostUSD
			}
			if a.ModelID != "" {
				t.records[i].Model = a.ModelID
			}
			if a.ProviderID != "" {
				t.records[i].Provider = a.ProviderID
			}
			return
		}
	}
}

// Recent returns up to n most recent records, newest first.
func (t *RequestLogTracker) Recent(n int) []RequestLogRecord {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]RequestLogRecord, 0)
	for i := len(t.records) - 1; i >= 0 && (n <= 0 || len(out) < n); i-- {
		out = append(out, t.records[i])
	}
	return out
}

// Reset clears the in-memory request log (audited demo-data reset, ISSUE-096).
func (t *RequestLogTracker) Reset() {
	t.mu.Lock()
	t.records = nil
	t.mu.Unlock()
}
