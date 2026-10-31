package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWhoAmIMarksBreakGlass(t *testing.T) {
	get := func(user string) whoami {
		req := httptest.NewRequest(http.MethodGet, "/router/whoami", nil)
		req = req.WithContext(withRole(withAdminUser(req.Context(), user), roleAdmin))
		rec := httptest.NewRecorder()
		WhoAmIHandler()(rec, req)
		var w whoami
		_ = json.Unmarshal(rec.Body.Bytes(), &w)
		return w
	}
	if w := get("demo@sluss.eu (break-glass)"); w.User != "demo@sluss.eu" || !w.BreakGlass || w.Role != roleAdmin {
		t.Fatalf("break-glass = %+v", w)
	}
	if w := get("magnus"); w.User != "magnus" || w.BreakGlass {
		t.Fatalf("named = %+v", w)
	}
}

func TestNavFooterAsksWhoIsSignedIn(t *testing.T) {
	if h := adminNavHTML("dashboard"); !strings.Contains(h, `id="tk-who"`) || !strings.Contains(h, "/router/whoami") || strings.Contains(h, `<div class="tk-foot">admin ·`) {
		t.Fatal("the menu foot should show the signed-in person, not a fixed 'admin'")
	}
}
