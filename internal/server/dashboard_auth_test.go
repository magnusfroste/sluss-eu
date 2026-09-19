package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardBasicAuth(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := dashboardBasicAuth("s3cret", ok)

	// No credentials → 401 with a Basic challenge so the browser prompts.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/dashboard", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: want 401, got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Errorf("expected WWW-Authenticate challenge, got none")
	}

	// Correct password (any username) → 200.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	req.SetBasicAuth("anyone", "s3cret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct password: want 200, got %d", rec.Code)
	}

	// Wrong password → 401.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	req.SetBasicAuth("anyone", "nope")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: want 401, got %d", rec.Code)
	}
}
