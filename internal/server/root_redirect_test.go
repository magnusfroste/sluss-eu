package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The bare root serves the instance home page (sign in, connect, getting
// started), not a redirect to the gated dashboard. Marketing lives on the
// product site, so the SEO/AEO endpoints are gone.
func TestRootServesLandingPage(t *testing.T) {
	h := New(Config{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / want 200 (landing), got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>Sluss") || !strings.Contains(body, "Sign in") {
		t.Fatalf("root should serve the instance home page")
	}
	for _, path := range []string{"/favicon.svg", "/robots.txt"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code != http.StatusOK {
			t.Fatalf("GET %s want 200, got %d", path, r.Code)
		}
	}
	for _, path := range []string{"/sitemap.xml", "/llms.txt"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code == http.StatusOK && strings.Contains(r.Body.String(), "Sluss") {
			t.Fatalf("GET %s should no longer serve marketing content", path)
		}
	}
}
