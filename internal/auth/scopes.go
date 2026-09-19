package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// Scope names a capability an API key can grant (ISSUE-046). Endpoints require a
// specific scope; a key whose scope set is empty is treated as unrestricted
// (legacy keys), while a key with an explicit set must include the required
// scope or the wildcard tenant.ScopeWildcard.
const (
	ScopeChatCompletions = "chat:completions"
	ScopeRouterDecision  = "router:decision"
	ScopeRouterOutcomes  = "router:outcomes"
)

// AllScopes lists every concrete scope the router defines. It is handy for
// provisioning unrestricted keys explicitly.
func AllScopes() []string {
	return []string{ScopeChatCompletions, ScopeRouterDecision, ScopeRouterOutcomes}
}

// RequireScope returns middleware that enforces scope on the wrapped handler.
// It must run after Middleware so the tenant is on the context. A missing tenant
// or insufficient scope yields 403 with an OpenAI-style error envelope.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t, ok := tenant.FromContext(r.Context())
			if !ok || !t.HasScope(scope) {
				keyID := ""
				if t != nil {
					keyID = t.KeyID
				}
				slog.Default().WarnContext(r.Context(), "insufficient_scope",
					"required_scope", scope, "key_id", keyID)
				writeForbidden(w, "api key missing required scope: "+scope)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeForbidden(w http.ResponseWriter, msg string) {
	writeForbiddenCode(w, msg, "insufficient_scope")
}

func writeForbiddenCode(w http.ResponseWriter, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
		Error: openai.ErrorBody{Message: msg, Type: "forbidden", Code: code},
	})
}
