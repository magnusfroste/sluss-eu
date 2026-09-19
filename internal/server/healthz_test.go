package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubChecker struct {
	name string
	err  error
}

func (s stubChecker) Name() string { return s.name }
func (s stubChecker) Ready() error { return s.err }

func TestHealthz_AlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	HealthzHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status=%q", body["status"])
	}
}

func TestReadyz_AllOK(t *testing.T) {
	rec := httptest.NewRecorder()
	ReadyzHandler(stubChecker{name: "db", err: nil}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyz_FailureReports503(t *testing.T) {
	rec := httptest.NewRecorder()
	ReadyzHandler(
		stubChecker{name: "db", err: errors.New("conn refused")},
		stubChecker{name: "policy", err: nil},
	).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body struct {
		Status  string            `json:"status"`
		Reasons map[string]string `json:"reasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("expected status=not_ready, got %q", body.Status)
	}
	if body.Reasons["db"] == "" {
		t.Fatalf("expected db failure reason, got %#v", body.Reasons)
	}
}
