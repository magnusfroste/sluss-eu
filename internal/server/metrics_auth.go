package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// metricsToken lets a scraper that presents ROUTER_METRICS_TOKEN as a bearer
// through to the metrics; everything else goes to the console gate (a
// signed-in admin, an admin key or the break-glass password). An empty token
// means there is no scraper token, only the gate.
func metricsToken(token string, metrics, gate http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && presentsBearer(r, token) {
			metrics.ServeHTTP(w, r)
			return
		}
		gate.ServeHTTP(w, r)
	})
}

// presentsBearer reports whether the request carries exactly this bearer
// token, compared in constant time.
func presentsBearer(r *http.Request, token string) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
