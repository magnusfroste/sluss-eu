package middleware

import (
	"net/http"
	"strings"
)

// The browser-facing security headers every response carries. The console
// renders its own pages with inline scripts and styles and loads nothing from
// other origins, so the policy allows inline code on this origin only and
// nothing else: no third-party scripts, no framing, no sniffing.
const (
	contentSecurityPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; " +
		"base-uri 'self'; form-action 'self'; object-src 'none'"
	permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"
	// One year, this host only: a subdomain the operator runs without TLS is
	// not our call to break.
	strictTransportSecurity = "max-age=31536000"
)

// SecurityHeaders sets the standard browser hardening headers on every
// response. Strict-Transport-Security is sent only when the request arrived
// over TLS, directly or through a reverse proxy that says so, because sending
// it over plain HTTP on a LAN install would be wrong.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", permissionsPolicy)
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", strictTransportSecurity)
		}
		next.ServeHTTP(w, r)
	})
}
