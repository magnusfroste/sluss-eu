package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A signed-in admin reads the manual inside the console, with the left menu
// to get back; everyone else gets the public page (ISSUE-146).
func TestManualKeepsTheConsoleMenuForAdmins(t *testing.T) {
	h := ManualHandler(ManualOptions{PublicURL: "https://sluss.example.eu"})
	get := func(role string) string {
		req := httptest.NewRequest(http.MethodGet, "/manual/troubleshooting", nil)
		if role != "" {
			req = req.WithContext(withRole(withAdminUser(req.Context(), "anna"), role))
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", role, rec.Code)
		}
		return rec.Body.String()
	}
	admin := get(roleAdmin)
	for _, want := range []string{`class="tk-shell"`, `href="/router/dashboard"`, `class="toc"`, `href="/manual/troubleshooting" class="on"`} {
		if !strings.Contains(admin, want) {
			t.Errorf("admin manual lacks %q", want)
		}
	}
	public := get("")
	if strings.Contains(public, `class="tk-shell"`) || !strings.Contains(public, "Sluss manual") {
		t.Error("the public manual must stay outside the console")
	}
	if user := get(roleUser); !strings.Contains(user, "Back to the chat") || strings.Contains(user, `class="tk-shell"`) {
		t.Error("an employee gets the public manual with a way back to the chat")
	}
}
