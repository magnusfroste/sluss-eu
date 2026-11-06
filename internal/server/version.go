package server

import (
	"encoding/json"
	"net/http"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

// VersionJSONHandler serves GET /version: the release, commit and build time
// of the running binary, and nothing else. It needs no key, so a deploy check
// or an operator can ask which build runs without holding an admin secret;
// the same facts are on the public landing page's footer.
func VersionJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version": buildinfo.Release(),
		"commit":  buildinfo.FullCommit(),
		"built":   buildinfo.BuiltAt(),
	})
}
