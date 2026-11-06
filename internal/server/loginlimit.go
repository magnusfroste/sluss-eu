package server

// Login throttling for the console's password form. A password form with no
// brake is a password-guessing service; this one slows a guesser down without
// locking a real person out for good. Failures are counted per account and per
// client address. After loginFreeAttempts failures in loginWindow, every
// further attempt is refused for a wait that doubles each time, up to
// loginMaxWait. A successful sign-in clears the account's count.
//
// All of it is in memory: a restart forgets, which is fine for a brake.

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	loginFreeAttempts = 5
	loginWindow       = 15 * time.Minute
	loginFirstWait    = 30 * time.Second
	loginMaxWait      = 15 * time.Minute
	// A client address gets more room than one account, so a shared office
	// address is not locked out by one person's typos.
	loginFreeAttemptsPerAddr = 20
)

type loginAttempts struct {
	failures int
	first    time.Time // start of the current window
	until    time.Time // refused before this time
}

type loginLimiter struct {
	mu    sync.Mutex
	now   func() time.Time
	byKey map[string]*loginAttempts
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{now: time.Now, byKey: map[string]*loginAttempts{}}
}

// wait reports how long the account or the address still has to wait before
// another attempt is accepted; zero means "go ahead".
func (l *loginLimiter) wait(user, addr string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	var longest time.Duration
	for _, k := range []string{"u:" + user, "a:" + addr} {
		if a, ok := l.byKey[k]; ok && a.until.After(now) {
			if d := a.until.Sub(now); d > longest {
				longest = d
			}
		}
	}
	return longest
}

// failed records one wrong password and returns the wait it now imposes, if
// any, so the caller can say so and audit it.
func (l *loginLimiter) failed(user, addr string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	var longest time.Duration
	for _, e := range []struct {
		key  string
		free int
	}{{"u:" + user, loginFreeAttempts}, {"a:" + addr, loginFreeAttemptsPerAddr}} {
		a := l.byKey[e.key]
		if a == nil || now.Sub(a.first) > loginWindow {
			a = &loginAttempts{first: now}
			l.byKey[e.key] = a
		}
		a.failures++
		if a.failures > e.free {
			wait := loginFirstWait << uint(a.failures-e.free-1)
			if wait > loginMaxWait || wait <= 0 {
				wait = loginMaxWait
			}
			a.until = now.Add(wait)
			if wait > longest {
				longest = wait
			}
		}
	}
	return longest
}

// succeeded clears the account's count. The address keeps its count: a
// guesser who finally gets one account right is still a guesser.
func (l *loginLimiter) succeeded(user string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byKey, "u:"+user)
}

// sweep forgets windows that have expired, so the map does not grow with every
// address that ever failed once. Called under the lock.
func (l *loginLimiter) sweep(now time.Time) {
	if len(l.byKey) < 1000 {
		return
	}
	for k, a := range l.byKey {
		if now.Sub(a.first) > loginWindow && !a.until.After(now) {
			delete(l.byKey, k)
		}
	}
}

// clientAddr is the caller's address as seen through a reverse proxy: the
// first X-Forwarded-For entry when one is present, else the TCP peer.
func clientAddr(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// humanWait says a wait in whole seconds or minutes, for the login form.
func humanWait(d time.Duration) string {
	if d < time.Minute {
		s := int(d.Round(time.Second) / time.Second)
		if s < 1 {
			s = 1
		}
		return strconv.Itoa(s) + " seconds"
	}
	m := int(d.Round(time.Minute) / time.Minute)
	if m < 1 {
		m = 1
	}
	if m == 1 {
		return "1 minute"
	}
	return strconv.Itoa(m) + " minutes"
}
