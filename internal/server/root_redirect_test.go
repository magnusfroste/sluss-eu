package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The bare root now serves the public landing page (SEO/AEO surface), not a
// redirect to the gated dashboard.
func TestRootServesLandingPage(t *testing.T) {
	h := New(Config{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / want 200 (landing), got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>Sluss") || !strings.Contains(body, "application/ld+json") {
		t.Fatalf("root should serve the landing page with SEO/AEO metadata")
	}
	// Public SEO/AEO endpoints are reachable without auth.
	for _, path := range []string{"/favicon.svg", "/robots.txt", "/sitemap.xml", "/llms.txt"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code != http.StatusOK {
			t.Fatalf("GET %s want 200, got %d", path, r.Code)
		}
	}
}
