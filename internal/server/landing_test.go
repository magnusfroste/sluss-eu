package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLandingHasSEOAndAEO(t *testing.T) {
	rec := httptest.NewRecorder()
	LandingHandler(LandingOptions{PublicURL: "https://tok.example.com"})(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<title>Sluss`,
		`<meta name="description"`,
		`<link rel="canonical" href="https://tok.example.com/">`,
		`property="og:title"`,
		`property="og:image" content="https://tok.example.com/favicon.svg"`,
		`name="twitter:card"`,
		`application/ld+json`,
		`"@type":"SoftwareApplication"`,
		`"@type":"FAQPage"`,
		`rel="icon" type="image/svg+xml"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("landing missing %q", want)
		}
	}
	// Corporate tone: no emoji in the landing copy.
	for _, emoji := range []string{"🛡️", "💸", "🌱", "🔀", "🧾", "🎛️"} {
		if strings.Contains(body, emoji) {
			t.Fatalf("landing should be emoji-free (corporate tone), found %q", emoji)
		}
	}
	// FAQ answers are present (AEO content, not just JSON-LD).
	if !strings.Contains(body, "NIS2") || !strings.Contains(body, "hash-chained") {
		t.Fatal("FAQ content missing from the page body")
	}
}

// Buyer positioning (ISSUE-101): the page leads with the risk removed, names
// the USPs as differences, and never links a visitor into a login wall.
func TestLandingBuyerPositioning(t *testing.T) {
	rec := httptest.NewRecorder()
	LandingHandler(LandingOptions{})(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"Decide where the data goes",               // hero: outcome first
		"Say yes to AI — on your rules",            // enablement framing
		"Blocking AI creates shadow AI",            // the cost of saying no
		"Where does our AI data go?",               // the auditor question
		"decides where your data is allowed to go", // the unlike sentence
		"Runs in your infrastructure",              // self-hosted USP
		"Never a silent fallback",                  // fail-closed USP
		"Measure before you enforce",               // monitor-mode-first USP
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("landing missing buyer message %q", want)
		}
	}
	// CTAs must not send an anonymous visitor into the login wall; sign-in
	// lives in the nav only. CRM/lead capture deliberately absent (an external CRM).
	if strings.Contains(body, `href="/chat"`) {
		t.Fatal("landing must not link the gated /chat as a CTA")
	}
	if strings.Count(body, "/router/dashboard") > 1 {
		t.Fatal("dashboard link belongs in the nav only")
	}
	// llms.txt carries the same positioning for answer engines.
	lrec := httptest.NewRecorder()
	LLMSHandler(LandingOptions{})(lrec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	if !strings.Contains(lrec.Body.String(), "allowed to go") || !strings.Contains(lrec.Body.String(), "Self-hosted") {
		t.Fatal("llms.txt missing the unlike/self-hosted positioning")
	}
}

func TestLandingWithoutPublicURLOmitsCanonical(t *testing.T) {
	rec := httptest.NewRecorder()
	LandingHandler(LandingOptions{})(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if strings.Contains(body, "<link rel=\"canonical\"") {
		t.Fatal("no PublicURL → canonical must be omitted")
	}
	if !strings.Contains(body, "application/ld+json") {
		t.Fatal("JSON-LD should still render without a public URL")
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

func TestRobotsAndSitemapAndLLMs(t *testing.T) {
	opts := LandingOptions{PublicURL: "https://tok.example.com"}

	rrec := httptest.NewRecorder()
	RobotsHandler(opts)(rrec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	robots := rrec.Body.String()
	if !strings.Contains(robots, "Disallow: /router/") || !strings.Contains(robots, "Sitemap: https://tok.example.com/sitemap.xml") {
		t.Fatalf("robots.txt wrong:\n%s", robots)
	}

	srec := httptest.NewRecorder()
	SitemapHandler(opts)(srec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if !strings.Contains(srec.Body.String(), "<loc>https://tok.example.com/</loc>") {
		t.Fatalf("sitemap wrong:\n%s", srec.Body.String())
	}

	lrec := httptest.NewRecorder()
	LLMSHandler(opts)(lrec, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	llms := lrec.Body.String()
	for _, want := range []string{"# Sluss", "## What it does", "## FAQ", "control and evidence"} {
		if !strings.Contains(llms, want) {
			t.Fatalf("llms.txt missing %q", want)
		}
	}
}
