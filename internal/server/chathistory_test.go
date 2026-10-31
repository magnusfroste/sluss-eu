package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every person has their own chat history: two admins never see each other's,
// and a demo-link guest never sees an admin's (ISSUE-154).
func TestChatHistoryIsPerPerson(t *testing.T) {
	store := newUserStore(t)
	get, put := DemoSessionsHandlers(store)
	as := func(user, role string, r *http.Request) *http.Request {
		return r.WithContext(withRole(withAdminUser(r.Context(), user), role))
	}
	save := func(user, role, body string) {
		rec := httptest.NewRecorder()
		put(rec, as(user, role, httptest.NewRequest(http.MethodPut, "/chat/sessions", strings.NewReader(body))))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("save as %s: %d", user, rec.Code)
		}
	}
	load := func(user, role string) string {
		rec := httptest.NewRecorder()
		get(rec, as(user, role, httptest.NewRequest(http.MethodGet, "/chat/sessions", nil)))
		return rec.Body.String()
	}
	save("magnus", roleAdmin, `[{"id":"s1","title":"magnus secret"}]`)
	save("hermes@sluss.eu", roleAdmin, `[{"id":"s2","title":"hermes"}]`)
	if got := load("magnus", roleAdmin); !strings.Contains(got, "magnus secret") {
		t.Fatalf("magnus lost own history: %s", got)
	}
	for _, other := range []struct{ user, role string }{{"hermes@sluss.eu", roleAdmin}, {"demo:acme", roleDemo}, {"demo-guest", roleDemo}, {"x (break-glass)", roleAdmin}} {
		if got := load(other.user, other.role); strings.Contains(got, "magnus secret") {
			t.Fatalf("%s can read magnus's chats: %s", other.user, got)
		}
	}
	// Reset demo data clears every person's history.
	if n, err := store.KVDeletePrefix(demoSessionsKey); err != nil || n != 2 {
		t.Fatalf("reset removed %d (%v), want 2", n, err)
	}
	if got := load("magnus", roleAdmin); strings.Contains(got, "magnus secret") {
		t.Fatal("history survived the reset")
	}
}
