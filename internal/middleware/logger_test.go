package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggerUsesDefaultLoggerWhenNil(t *testing.T) {
	h := Logger(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
