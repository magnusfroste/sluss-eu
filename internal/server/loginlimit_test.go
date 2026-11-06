package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
)

func TestLoginLimiterBrakesAfterFreeAttempts(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < loginFreeAttempts; i++ {
		if w := l.failed("magnus", "10.0.0.1"); w != 0 {
			t.Fatalf("attempt %d should still be free, got wait %v", i+1, w)
		}
	}
	if w := l.failed("magnus", "10.0.0.1"); w != loginFirstWait {
		t.Fatalf("sixth failure should impose %v, got %v", loginFirstWait, w)
	}
	if w := l.wait("magnus", "10.0.0.1"); w != loginFirstWait {
		t.Fatalf("wait should be %v, got %v", loginFirstWait, w)
	}
	// Another account from the same address is not blocked yet: the address
	// has more room than one account.
	if w := l.wait("hermes", "10.0.0.1"); w != 0 {
		t.Fatalf("other account from the same address should be free, got %v", w)
	}
	// The wait doubles and is capped.
	now = now.Add(loginFirstWait)
	for i := 0; i < 8; i++ {
		l.failed("magnus", "10.0.0.1")
	}
	if w := l.wait("magnus", "10.0.0.1"); w != loginMaxWait {
		t.Fatalf("wait should be capped at %v, got %v", loginMaxWait, w)
	}
	// Success clears the account.
	l.succeeded("magnus")
	if w := l.wait("magnus", "10.0.0.1"); w != 0 {
		t.Fatalf("after success the account should be free, got %v", w)
	}
	// A window that expired starts over.
	now = now.Add(loginWindow + time.Second)
	if w := l.failed("magnus", "10.0.0.1"); w != 0 {
		t.Fatalf("a new window should start free, got %v", w)
	}
}

func TestLoginHandlerRefusesWhileThrottled(t *testing.T) {
	mem := audit.NewMemorySink(0)
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: newUserStore(t), BreakGlass: "bg-secret", Auditor: mem}
	h := a.LoginHandler()
	post := func(pass string) *httptest.ResponseRecorder {
		form := url.Values{"username": {"admin"}, "password": {pass}}
		req := httptest.NewRequest(http.MethodPost, "/router/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "192.0.2.7:51000"
		rr := httptest.NewRecorder()
		h(rr, req)
		return rr
	}
	for i := 0; i < loginFreeAttempts; i++ {
		if rr := post("wrong"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Wrong username or password") {
			t.Fatalf("attempt %d: want the plain error, got %d %q", i+1, rr.Code, rr.Body.String()[:80])
		}
	}
	rr := post("wrong")
	if rr.Code != http.StatusTooManyRequests || !strings.Contains(rr.Body.String(), "Too many attempts") {
		t.Fatalf("sixth failure should be throttled: %d", rr.Code)
	}
	// Even the right password is refused while the wait runs.
	if rr := post("bg-secret"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("right password during the wait must be refused, got %d", rr.Code)
	}
	var failed, throttled int
	for _, e := range mem.Entries() {
		switch e.Action {
		case ActionConsoleLoginFailed:
			failed++
		case ActionConsoleLoginThrottled:
			throttled++
		}
	}
	if failed < loginFreeAttempts || throttled < 1 {
		t.Fatalf("failed logins and the throttle must be audited: failed=%d throttled=%d", failed, throttled)
	}
}
