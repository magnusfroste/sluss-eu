package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const HeaderRequestID = "X-Router-Request-Id"

type requestIDKey struct{}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// RequestID ensures every request carries a stable identifier. If the caller
// supplied X-Router-Request-Id we honour it; otherwise a new "req_<uuid>" is
// minted. The id is set on the response header and on the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = "req_" + uuid.NewString()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
