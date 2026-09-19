package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/auth"
)

// mcpAuth gates the MCP endpoint behind the same credentials as the rest of the
// service: a valid Bearer API key, or — when configured — the dashboard password
// (presented as a Bearer token or HTTP Basic password). The password path lets
// an analysis agent connect with the one shared secret an operator already has,
// without minting an API key. The API-key path reuses auth.Middleware so hashing
// and lookup stay in one place.
func mcpAuth(store auth.KeyStore, dashboardPassword string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		keyGuard := auth.Middleware(store)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if dashboardPassword != "" && presentsPassword(r, dashboardPassword) {
				next.ServeHTTP(w, r)
				return
			}
			keyGuard.ServeHTTP(w, r)
		})
	}
}

// presentsPassword reports whether the request carries the dashboard password,
// either as "Authorization: Bearer <password>" or HTTP Basic auth. Comparison is
// constant-time.
func presentsPassword(r *http.Request, password string) bool {
	if _, pass, ok := r.BasicAuth(); ok {
		if subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1 {
			return true
		}
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(password)) == 1 {
			return true
		}
	}
	return false
}
