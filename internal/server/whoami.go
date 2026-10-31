package server

// Who is signed in (ISSUE-153). The console menu is the same HTML for every
// visitor, so it asks for the name here. A login with the break-glass
// password has no account behind it: the session records the typed name with
// a "(break-glass)" suffix, and this says so plainly.

import (
	"encoding/json"
	"net/http"
	"strings"
)

type whoami struct {
	User       string `json:"user"`
	Role       string `json:"role"`
	Department string `json:"department,omitempty"`
	BreakGlass bool   `json:"break_glass"`
}

func currentWho(r *http.Request) whoami {
	user := AdminUserFromContext(r.Context())
	w := whoami{Role: RoleFromContext(r.Context()), Department: DepartmentFromContext(r.Context())}
	if name, ok := strings.CutSuffix(user, " ("+breakGlassActor+")"); ok {
		user, w.BreakGlass = name, true
	}
	w.User = user
	return w
}

// WhoAmIHandler serves GET /router/whoami.
func WhoAmIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(currentWho(r))
	}
}
