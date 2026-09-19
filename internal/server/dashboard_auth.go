package server

import (
	"crypto/subtle"
	"net/http"
)

// dashboardBasicAuth gates a handler behind HTTP Basic Auth so the dashboard is
// viewable directly in a browser (a browser nav can't send a Bearer token). On a
// missing/invalid credential it returns 401 with a Basic challenge, which makes
// the browser show its native login prompt. Any username is accepted; only the
// password must match (compared in constant time). This is enabled only when a
// dashboard password is configured.
func dashboardBasicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="tokenizer dashboard"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
