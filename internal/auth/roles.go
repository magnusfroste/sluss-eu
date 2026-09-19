package auth

import (
	"log/slog"
	"net/http"

	"github.com/magnusfroste/sluss/internal/tenant"
)

const (
	RoleUser  = tenant.RoleUser
	RoleAdmin = tenant.RoleAdmin
)

// RequireRole returns middleware that enforces role on the wrapped handler.
// It must run after Middleware so the tenant is on the context. A missing
// tenant or insufficient role yields 403 with an OpenAI-style error envelope.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t, ok := tenant.FromContext(r.Context())
			if !ok || !t.HasRole(role) {
				keyID := ""
				currentRole := ""
				if t != nil {
					keyID = t.KeyID
					currentRole = t.Role
				}
				slog.Default().WarnContext(r.Context(), "insufficient_role",
					"required_role", role, "role", currentRole, "key_id", keyID)
				writeForbiddenCode(w, "api key missing required role: "+role, "insufficient_role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
