package eventlog_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

type countingHandler struct {
	n atomic.Int64
}

func (h *countingHandler) Handle(_ context.Context, _ eventlog.Event) {
	h.n.Add(1)
}

func TestQueue_EnqueueAndProcess(t *testing.T) {
	q := eventlog.NewQueue(16)
	h := &countingHandler{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go q.Run(ctx, h, nil)

	for i := 0; i < 10; i++ {
		q.Enqueue(eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{RequestID: "r"}})
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if h.n.Load() == 10 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if h.n.Load() != 10 {
		t.Fatalf("expected 10 events processed, got %d", h.n.Load())
	}
}

func TestQueue_DropsWhenFull(t *testing.T) {
	q := eventlog.NewQueue(2)
	// Fill the queue and try to add more without a running worker.
	for i := 0; i < 10; i++ {
		q.Enqueue(eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{}})
	}
	if q.Backlog() > 2 {
		t.Fatalf("backlog should not exceed buffer size, got %d", q.Backlog())
	}
	if q.DroppedTotal() == 0 {
		t.Error("expected some drops when queue is full")
	}
}

func TestQueue_NonBlockingEnqueue(t *testing.T) {
	q := eventlog.NewQueue(1)
	done := make(chan struct{})
	go func() {
		// This must not block even with a full queue.
		for i := 0; i < 1000; i++ {
			q.Enqueue(eventlog.Event{Type: eventlog.EventTypeDecision})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Enqueue blocked — should be non-blocking")
	}
}
