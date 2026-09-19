package eventlog

import (
	"context"
	"log/slog"
	"sync/atomic"
)

const defaultQueueSize = 4096

// Handler processes events dequeued by the Worker.
// Implementations must be safe for concurrent calls.
type Handler interface {
	Handle(ctx context.Context, event Event)
}

// Queue is a non-blocking, bounded event queue.
// Enqueue never blocks; if the buffer is full the event is dropped and the
// drop counter is incremented.
type Queue struct {
	ch      chan Event
	dropped atomic.Int64
}

// NewQueue creates a Queue with the given buffer size (0 → defaultQueueSize).
func NewQueue(size int) *Queue {
	if size <= 0 {
		size = defaultQueueSize
	}
	return &Queue{ch: make(chan Event, size)}
}

// Enqueue adds an event to the queue. It never blocks — if the buffer is full
// the event is silently dropped and the internal drop counter is incremented.
func (q *Queue) Enqueue(e Event) {
	select {
	case q.ch <- e:
	default:
		q.dropped.Add(1)
	}
}

// Backlog returns the current number of events waiting to be processed.
func (q *Queue) Backlog() int {
	return len(q.ch)
}

// DroppedTotal returns the cumulative number of events dropped due to a full buffer.
func (q *Queue) DroppedTotal() int64 {
	return q.dropped.Load()
}

// Run starts the queue worker. It reads events from the queue and dispatches
// them to handler until ctx is cancelled. Run blocks until ctx is done.
func (q *Queue) Run(ctx context.Context, handler Handler, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	for {
		select {
		case e := <-q.ch:
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("event handler panic", "recover", r)
					}
				}()
				handler.Handle(ctx, e)
			}()
		case <-ctx.Done():
			// Drain remaining events before exiting.
			for {
				select {
				case e := <-q.ch:
					handler.Handle(ctx, e)
				default:
					return
				}
			}
		}
	}
}
