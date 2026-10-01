package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The instance home page (marketing moved to the product site): what this is,
// sign in, connect a client, getting started — and nothing for search engines.
func TestLandingIsInstanceHome(t *testing.T) {
	rec := httptest.NewRecorder()
	LandingHandler(LandingOptions{PublicURL: "https://sluss.example.com"})(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Fatalf("X-Robots-Tag = %q, want noindex", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<title>Sluss`,
		`<meta name="robots" content="noindex, nofollow">`,
		`rel="icon" type="image/svg+xml"`,
		`class="btn primary" href="/router/dashboard">Sign in`, // sign-in is THE action now
		`href="/connect"`,
		"Getting started",
		"fail-closed",
		"https://www.sluss.eu", // product info lives on the product site
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("instance home missing %q", want)
		}
	}
	// Marketing/SEO surface is gone from the instance.
	for _, gone := range []string{"application/ld+json", "og:title", "twitter:card", "FAQ", `rel="canonical"`} {
		if strings.Contains(body, gone) {
			t.Fatalf("instance home should not carry marketing/SEO markup %q", gone)
		}
	}
	// Corporate tone: no emoji.
	for _, emoji := range []string{"🛡️", "💸", "🌱", "🔀", "🧾", "🎛️"} {
		if strings.Contains(body, emoji) {
			t.Fatalf("home should be emoji-free, found %q", emoji)
		}
	}
	// The gated chat is never linked from the public page.
	if strings.Contains(body, `href="/chat"`) {
		t.Fatal("home must not link the gated /chat")
	}
}

func TestFaviconServesSVGShield(t *testing.T) {
	rec := httptest.NewRecorder()
	FaviconHandler()(rec, httptest.NewRequest(http.MethodGet, "/favicon.svg", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "<svg") || !strings.Contains(rec.Body.String(), "shield") && !strings.Contains(rec.Body.String(), "path") {
		t.Fatal("favicon should be an SVG shield")
	}
}

func TestFaviconICORedirects(t *testing.T) {
	rec := httptest.NewRecorder()
	FaviconICOHandler()(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/favicon.svg" {
		t.Fatalf("favicon.ico should 301 → /favicon.svg, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRobotsKeepsInstanceOutOfIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	RobotsHandler(LandingOptions{PublicURL: "https://sluss.example.com"})(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if got := rec.Body.String(); got != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("robots.txt = %q", got)
	}
}
