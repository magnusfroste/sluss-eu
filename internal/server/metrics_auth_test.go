package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsTokenOpensMetricsOnlyForTheToken(t *testing.T) {
	gateHits := 0
	gate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { gateHits++; w.WriteHeader(http.StatusUnauthorized) })
	metrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := metricsToken("scrape-token", metrics, gate)
	get := func(authz string) int {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if c := get("Bearer scrape-token"); c != http.StatusOK {
		t.Errorf("scraper token: want 200 from the metrics, got %d", c)
	}
	if c := get(""); c != http.StatusUnauthorized || gateHits != 1 {
		t.Errorf("no credentials must go to the gate: code %d, gate hits %d", c, gateHits)
	}
	if c := get("Bearer wrong"); c != http.StatusUnauthorized || gateHits != 2 {
		t.Errorf("wrong token must go to the gate: code %d, gate hits %d", c, gateHits)
	}
	// No token configured: everything goes to the gate.
	h = metricsToken("", metrics, gate)
	if c := get("Bearer scrape-token"); c != http.StatusUnauthorized || gateHits != 3 {
		t.Errorf("without a configured token the bearer must not open metrics: %d", c)
	}
}
