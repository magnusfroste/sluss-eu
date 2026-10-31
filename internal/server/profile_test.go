package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/history"
)

func asUser(r *http.Request, user, role, dept string) *http.Request {
	return r.WithContext(withDepartment(withRole(withAdminUser(r.Context(), user), role), dept))
}

func postProfile(h http.HandlerFunc, r *http.Request, v url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, r.URL.Path, strings.NewReader(v.Encode())).WithContext(r.Context())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h(rec, req)
	return rec
}

func TestProfileChangesOwnPasswordOnly(t *testing.T) {
	store := newUserStore(t)
	hash, _ := HashPassword("old-password")
	_ = store.UpsertAdminUser(history.AdminUser{Username: "magnus", PasswordHash: hash, Role: roleAdmin})
	h := ProfilePasswordHandler(store, nil)
	base := asUser(httptest.NewRequest(http.MethodPost, "/me/password", nil), "magnus", roleAdmin, "")

	if rec := postProfile(h, base, url.Values{"current": {"wrong"}, "new": {"new-password"}, "again": {"new-password"}}); !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatal("a wrong current password must be refused")
	}
	if rec := postProfile(h, base, url.Values{"current": {"old-password"}, "new": {"new-password"}, "again": {"other-password"}}); !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatal("mismatched new passwords must be refused")
	}
	if rec := postProfile(h, base, url.Values{"current": {"old-password"}, "new": {"new-password"}, "again": {"new-password"}}); !strings.Contains(rec.Header().Get("Location"), "ok=") {
		t.Fatalf("change refused: %s", rec.Header().Get("Location"))
	}
	u, _, _ := store.GetAdminUser("magnus")
	if !VerifyPassword(u.PasswordHash, "new-password") || u.Role != roleAdmin {
		t.Fatal("the new password was not saved, or the role changed")
	}
	// A break-glass session has no account to change.
	bg := asUser(httptest.NewRequest(http.MethodPost, "/me/password", nil), "magnus (break-glass)", roleAdmin, "")
	if rec := postProfile(h, bg, url.Values{"current": {"new-password"}, "new": {"x-password-2"}, "again": {"x-password-2"}}); !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatal("break-glass must not change the named account's password")
	}
}

func TestProfilePageAndHistory(t *testing.T) {
	store := newUserStore(t)
	_ = store.KVSet(demoSessionsKey+":anna@example.eu", `[{"id":"s1","messages":[]}]`)
	_ = store.KVSet(demoSessionsKey+":bob@example.eu", `[{"id":"s2","messages":[]}]`)
	anna := asUser(httptest.NewRequest(http.MethodGet, "/me", nil), "anna@example.eu", roleUser, "finance")

	rec := httptest.NewRecorder()
	ProfilePageHandler(store, nil)(rec, anna)
	body := rec.Body.String()
	for _, want := range []string{"anna@example.eu", "finance", "single sign-on", "Back to the chat", "Delete all my chats"} {
		if !strings.Contains(body, want) {
			t.Errorf("employee profile lacks %q", want)
		}
	}
	if strings.Contains(body, `class="tk-shell"`) || strings.Contains(body, "Current password") {
		t.Error("an SSO employee gets no console frame and no password form")
	}

	dl, del := ProfileHistoryHandler(store, nil)
	rec = httptest.NewRecorder()
	dl(rec, anna)
	if !strings.Contains(rec.Body.String(), `"s1"`) || strings.Contains(rec.Body.String(), `"s2"`) {
		t.Fatalf("download must be anna's own chats only: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	del(rec, asUser(httptest.NewRequest(http.MethodPost, "/me/history/delete", nil), "anna@example.eu", roleUser, "finance"))
	if v, _ := store.KVGet(demoSessionsKey + ":anna@example.eu"); v != "[]" {
		t.Fatalf("anna's history not deleted: %q", v)
	}
	if v, _ := store.KVGet(demoSessionsKey + ":bob@example.eu"); !strings.Contains(v, "s2") {
		t.Fatal("deleting anna's history touched bob's")
	}
}
