package eventlog

import "context"

// MultiHandler dispatches each event to all registered handlers in order.
// It implements Handler and is safe for concurrent use.
type multiHandler struct {
	handlers []Handler
}

// MultiHandler returns a Handler that fans out to all provided handlers.
func MultiHandler(handlers ...Handler) Handler {
	filtered := make([]Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	return &multiHandler{handlers: filtered}
}

func (m *multiHandler) Handle(ctx context.Context, e Event) {
	for _, h := range m.handlers {
		h.Handle(ctx, e)
	}
}
