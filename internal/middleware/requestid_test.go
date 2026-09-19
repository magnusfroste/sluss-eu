package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.HasPrefix(seen, "req_") {
		t.Fatalf("expected generated id to start with req_, got %q", seen)
	}
	if got := rec.Header().Get(HeaderRequestID); got != seen {
		t.Fatalf("response header %q does not match context %q", got, seen)
	}
}

func TestRequestID_HonoursIncoming(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "req_caller_supplied")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "req_caller_supplied" {
		t.Fatalf("expected caller-supplied id, got %q", seen)
	}
	if got := rec.Header().Get(HeaderRequestID); got != "req_caller_supplied" {
		t.Fatalf("response header lost the supplied id, got %q", got)
	}
}
