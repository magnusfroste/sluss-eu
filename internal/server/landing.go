package server

// Public landing page + SEO/AEO surface. This is the only unauthenticated HTML
// page: it carries the pitch, search-engine metadata (title/description/canonical/
// OpenGraph/Twitter) and answer-engine metadata (JSON-LD SoftwareApplication +
// FAQPage, plus /llms.txt) so both search and LLM answer engines can describe the
// product accurately. Everything operational stays behind the dashboard gate.
//
// Visual tone: corporate / restrained (near-monochrome, one muted accent, no
// emoji, tight type) — this is the reference look for the enterprise audience.
// Credibility rule (docs/00-product/08-positioning-ciso.md): control + evidence,
// never legal "compliance" claims.

import (
	"html/template"
	"net/http"
	"strings"
)

const (
	brandName    = "Sluss"
	brandTagline = "LLM egress control & cost-aware routing for enterprises under NIS2"
	brandDesc    = "Sluss is a deterministic, auditable LLM gateway: it keeps sensitive prompts inside the house, routes each request to the right model on cost and quality, and produces a tamper-evident audit trail — control and evidence for security leaders under NIS2, with cost and CO₂ savings on top."
	// brandHeroSub is the hero paragraph: outcome first, mechanism second. The
	// SEO description above stays stable; this is the human-facing sequence.
	brandHeroSub = "Sluss is the control point between your organisation and the AI providers: sensitive prompts stay in the house or are blocked fail-closed, everything else routes to the best cheap model — every decision explainable and recorded in a tamper-evident audit trail. Self-hosted, in your own infrastructure."
	// brandUnlike is the one-line positioning against generic AI gateways.
	brandUnlike = "Generic AI gateways optimise traffic. Sluss decides where your data is allowed to go — and proves it."
)

// LandingOptions carries the public base URL (for canonical/OG/sitemap). Empty
// PublicURL degrades gracefully: relative links, no canonical.
type LandingOptions struct {
	PublicURL string // e.g. https://tokenizer.example.com (no trailing slash)
	// ProfileLabel is the hero label chip, flavoured by the active regime profile
	// (ISSUE-094): "LLM egress control · NIS2" / "· DORA" / "· GDPR" / "· EU".
	// Empty keeps the NIS2 default.
	ProfileLabel string
}

type landingData struct {
	Name, Tagline, Desc, PublicURL, Canonical string
	Label                                     string
	HeroSub, Unlike                           string
	FaviconLinks                              template.HTML
	Logo                                      template.HTML
	JSONLD                                    template.JS
	FAQ                                       []faqItem
}

type faqItem struct{ Q, A string }

func landingFAQ() []faqItem {
	return []faqItem{
		{"What is Sluss?", "An OpenAI-compatible LLM gateway that classifies each prompt with deterministic rules (no LLM in the routing decision) and routes it to the right model based on task, risk, sensitivity, cost and policy — with a full, tamper-evident audit trail."},
		{"How does it help with NIS2?", "It gives the accountable owner control and evidence: policy-enforced data-egress rules, PII detection, provider residency controls, deterministic explainable decisions, and an exportable hash-chained audit log you can verify offline. It is a control and evidence tool — it does not make legal compliance claims."},
		{"Can it keep sensitive data inside the EU or on-prem?", "Yes. Providers and models carry compliance tags (e.g. eu-resident, dpa-signed, private, local). Policy can require or deny tags per task or sensitivity, so a prompt containing personal data is routed to a private/EU provider — or blocked fail-closed — and never silently falls back to a non-compliant provider."},
		{"How does it save money and CO₂?", "Cheap tasks go to cheap models and hard tasks to premium ones, automatically. The dashboard shows spend versus an all-premium baseline and the estimated energy/CO₂e saved — typically a large reduction, labeled as an estimate."},
		{"Is the routing decision explainable and auditable?", "Every decision is deterministic (rules and scoring, never an LLM) and carries human-readable reasons. Control-plane events are recorded in a hash-chained audit log an auditor can verify offline; any edited or removed record breaks the chain."},
		{"How does the CISO set the rules?", "Like firewall rules: condition → action. Embedded compliance packs (NIS2, DORA, GDPR-sovereign) are the starting rulesets; the policy console shows the active rules and lets you dry-run any prompt to see which rule fires, which model is selected and whether data stays local — before a single provider call. Constraints are fail-closed: if no compliant provider exists, the request is blocked, never silently sent to the cloud."},
		{"Can we evaluate it without enforcing anything?", "Yes — monitor mode. Run a compliance pack as the shadow policy: nothing blocks and nothing changes for users, but the gap report counts what the pack would have done with sensitive prompts. Converting to enforcement is a single configuration change."},
		{"Is Sluss itself cloud or self-hosted?", "Self-hosted: one static binary (or container) running in your own infrastructure or EU environment. The gateway that guards your data never lives in someone else's cloud, and no prompt passes through a third party on the way to the model you chose."},
		{"What happens when a security incident must be reported?", "The incident evidence report assembles the LLM-traffic evidence for a chosen window (24h/72h, matching NIS2's reporting deadlines): what was blocked, which data classes were involved and where sensitive prompts actually went — counts and classifications only, never prompt content."},
	}
}

// LandingHandler serves the public landing page.
func LandingHandler(opts LandingOptions) http.HandlerFunc {
	base := strings.TrimRight(opts.PublicURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		label := opts.ProfileLabel
		if label == "" {
			label = "LLM egress control · NIS2"
		}
		d := landingData{
			Name: brandName, Tagline: brandTagline, Desc: brandDesc,
			HeroSub: brandHeroSub, Unlike: brandUnlike,
			Label:        label,
			PublicURL:    base,
			Canonical:    base + "/",
			FaviconLinks: template.HTML(faviconLinks),
			Logo:         template.HTML(brandLogoSVG),
			JSONLD:       template.JS(landingJSONLD(base)),
			FAQ:          landingFAQ(),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := landingTmpl.Execute(w, d); err != nil {
			http.Error(w, "landing render error", http.StatusInternalServerError)
		}
	}
}

// landingJSONLD builds the SoftwareApplication + FAQPage structured data (AEO).
func landingJSONLD(base string) string {
	var faq strings.Builder
	for i, f := range landingFAQ() {
		if i > 0 {
			faq.WriteString(",")
		}
		faq.WriteString(`{"@type":"Question","name":` + jsonString(f.Q) +
			`,"acceptedAnswer":{"@type":"Answer","text":` + jsonString(f.A) + `}}`)
	}
	url := base
	if url == "" {
		url = "/"
	}
	return `{"@context":"https://schema.org","@graph":[` +
		`{"@type":"SoftwareApplication","name":` + jsonString(brandName) +
		`,"applicationCategory":"SecurityApplication","operatingSystem":"Linux, Docker",` +
		`"description":` + jsonString(brandDesc) + `,"url":` + jsonString(url) +
		`,"offers":{"@type":"Offer","price":"0","priceCurrency":"USD"}},` +
		`{"@type":"FAQPage","mainEntity":[` + faq.String() + `]}]}`
}

// jsonString escapes a string for embedding in JSON-LD.
func jsonString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", "")
	return `"` + r.Replace(s) + `"`
}

// RobotsHandler serves robots.txt, pointing crawlers at the sitemap and keeping
// the operational admin surface out of the index.
func RobotsHandler(opts LandingOptions) http.HandlerFunc {
	base := strings.TrimRight(opts.PublicURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		var b strings.Builder
		b.WriteString("User-agent: *\nAllow: /$\nAllow: /connect\nDisallow: /router/\nDisallow: /chat\nDisallow: /v1/\n")
		if base != "" {
			b.WriteString("Sitemap: " + base + "/sitemap.xml\n")
		}
		_, _ = w.Write([]byte(b.String()))
	}
}

// SitemapHandler serves a minimal sitemap for the public page.
func SitemapHandler(opts LandingOptions) http.HandlerFunc {
	base := strings.TrimRight(opts.PublicURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		loc := base + "/"
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` +
			`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
			`<url><loc>` + loc + `</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>` +
			`<url><loc>` + base + `/connect</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>` +
			`</urlset>`))
	}
}

// LLMSHandler serves /llms.txt — a concise, LLM-readable product summary (the
// emerging AEO convention) so answer engines describe the product accurately.
func LLMSHandler(opts LandingOptions) http.HandlerFunc {
	base := strings.TrimRight(opts.PublicURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		var b strings.Builder
		b.WriteString("# " + brandName + "\n\n")
		b.WriteString("> " + brandTagline + "\n\n")
		b.WriteString(brandDesc + "\n\n")
		b.WriteString("## What it does\n")
		b.WriteString("- Deterministic, rule-based routing of each prompt to the right model (no LLM in the routing decision) on task, risk, sensitivity, cost and policy.\n")
		b.WriteString("- Data-egress control: PII detection plus provider residency/compliance tags (eu-resident, dpa-signed, private, local) enforced by policy; sensitive prompts stay in the house or are blocked fail-closed.\n")
		b.WriteString("- Tamper-evident, hash-chained audit log with offline verification, and a readable control report for a CISO or auditor.\n")
		b.WriteString("- Per-department API keys, cost and CO₂ savings versus an all-premium baseline, and online learning of the best model per task class.\n")
		b.WriteString("- OpenAI- AND Anthropic-compatible: point any client (Cursor, Claude Code, Codex, AnythingLLM, OpenWebUI, the OpenAI/Anthropic SDKs) at the router by setting a base URL + key. See /connect.\n\n")
		b.WriteString("## Positioning\n")
		b.WriteString(brandUnlike + "\n")
		b.WriteString("Sluss sells control and evidence for organisations under NIS2 — it is a tool, not a legal compliance certification, and makes no legal claims. Cost and CO₂ savings are the ROI on top. ")
		b.WriteString("Self-hosted: one static binary in the customer's own infrastructure — the gateway guarding the data never lives in someone else's cloud.\n\n")
		if base != "" {
			b.WriteString("## Links\n- Site: " + base + "/\n\n")
		}
		b.WriteString("## FAQ\n")
		for _, f := range landingFAQ() {
			b.WriteString("### " + f.Q + "\n" + f.A + "\n\n")
		}
		_, _ = w.Write([]byte(b.String()))
	}
}

var landingTmpl = template.Must(template.New("landing").Parse(landingHTML))

const landingHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — {{.Tagline}}</title>
<meta name="description" content="{{.Desc}}">
{{if .PublicURL}}<link rel="canonical" href="{{.Canonical}}">{{end}}
<meta name="robots" content="index,follow">
<meta property="og:type" content="website">
<meta property="og:title" content="{{.Name}} — {{.Tagline}}">
<meta property="og:description" content="{{.Desc}}">
{{if .PublicURL}}<meta property="og:url" content="{{.Canonical}}">
<meta property="og:image" content="{{.PublicURL}}/favicon.svg">{{end}}
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="{{.Name}} — {{.Tagline}}">
<meta name="twitter:description" content="{{.Desc}}">
{{.FaviconLinks}}
<script type="application/ld+json">{{.JSONLD}}</script>
<style>
:root{
  --bg:#09090b; --panel:#101013; --line:#1e1e22; --ink:#f4f4f5; --muted:#8a8a92;
  --faint:#5c5c63; --accent:#3fb27f;
}
*{box-sizing:border-box}
html{-webkit-font-smoothing:antialiased;scroll-behavior:smooth}
body{margin:0;background:var(--bg);color:var(--ink);
  font-family:ui-sans-serif,system-ui,-apple-system,"Inter","Segoe UI",Roboto,sans-serif;
  line-height:1.6;letter-spacing:-0.01em}
.wrap{max-width:960px;margin:0 auto;padding:0 24px}
.lbl{font-size:11px;letter-spacing:.14em;text-transform:uppercase;color:var(--faint)}
nav{display:flex;align-items:center;justify-content:space-between;padding:22px 0;border-bottom:1px solid var(--line)}
.brand{display:flex;align-items:center;gap:9px;font-weight:600;font-size:15px}
nav a.enter{color:var(--muted);text-decoration:none;font-size:14px;border:1px solid var(--line);padding:8px 15px;border-radius:8px}
nav a.enter:hover{color:var(--ink);border-color:var(--faint)}
.hero{padding:76px 0 32px;max-width:720px}
h1{font-size:2.7rem;line-height:1.08;margin:14px 0 20px;font-weight:600;letter-spacing:-0.02em}
.sub{font-size:1.12rem;color:var(--muted);max-width:620px}
.cta{display:flex;flex-wrap:wrap;gap:12px;margin-top:30px}
.btn{display:inline-block;padding:11px 20px;border-radius:8px;font-weight:500;font-size:15px;text-decoration:none}
.btn.primary{background:var(--accent);color:#05130c}
.btn.primary:hover{background:#4cc78e}
.btn.ghost{border:1px solid var(--line);color:var(--ink)}
.btn.ghost:hover{border-color:var(--faint)}
.meta{color:var(--faint);font-size:13px;margin-top:16px}
.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:1px;background:var(--line);border:1px solid var(--line);border-radius:12px;overflow:hidden;margin:56px 0}
.cell{background:var(--panel);padding:24px 22px}
.cell h3{margin:10px 0 8px;font-size:1rem;font-weight:600}
.cell p{margin:0;color:var(--muted);font-size:.92rem}
h2{font-size:1.5rem;margin:64px 0 8px;font-weight:600;letter-spacing:-0.01em}
.steps{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:22px}
.step{border-left:2px solid var(--line);padding:2px 0 2px 16px}
.step .n{color:var(--accent);font-weight:600;font-size:13px}
.step h3{margin:4px 0 6px;font-size:.98rem;font-weight:600}
.step p{margin:0;color:var(--muted);font-size:.9rem}
.faq{margin-top:22px}
.faq details{border-bottom:1px solid var(--line);padding:4px 0}
.faq summary{cursor:pointer;font-weight:500;padding:16px 0;list-style:none;display:flex;justify-content:space-between}
.faq summary::-webkit-details-marker{display:none}
.faq summary::after{content:"+";color:var(--faint)}
.faq details[open] summary::after{content:"–"}
.faq p{color:var(--muted);margin:0 0 18px;max-width:760px}
footer{border-top:1px solid var(--line);margin-top:72px;padding:26px 0;color:var(--faint);font-size:13px;
  display:flex;justify-content:space-between;flex-wrap:wrap;gap:10px}
@media(max-width:720px){.grid,.steps{grid-template-columns:1fr}h1{font-size:2.1rem}}
</style>
</head>
<body>
<div class="wrap">
<nav>
  <span class="brand">{{.Logo}} {{.Name}}</span>
  <a class="enter" href="/router/dashboard">Sign in</a>
</nav>

<section class="hero">
  <span class="lbl">{{.Label}}</span>
  <h1>Your teams already use AI. Decide where the data goes — and prove it.</h1>
  <p class="sub">{{.HeroSub}}</p>
  <div class="cta">
    <a class="btn primary" href="#risk">See what it removes</a>
    <a class="btn ghost" href="#how">How it works</a>
    <a class="btn ghost" href="/connect">Connect a client</a>
  </div>
  <p class="meta">Deterministic routing — no LLM in the decision, so every choice is explainable and auditable.</p>
</section>

<span class="lbl" id="risk">The risk you remove</span>
<h2>Say yes to AI — on your rules.</h2>
<div class="grid" style="margin-top:22px">
  <div class="cell"><span class="lbl">Shadow AI</span><h3>Uncontrolled AI use is already happening</h3><p>Employees paste customer data into public chatbots you cannot see. Route their tools through Sluss and usage becomes visible and governed — run monitor mode for two weeks and the gap report tells you exactly how many prompts with personal data went to a cloud model.</p></div>
  <div class="cell"><span class="lbl">Evidence gap</span><h3>&ldquo;Where does our AI data go?&rdquo;</h3><p>When the board or an auditor asks, an access log is not an answer. Every routing decision here is explainable and recorded in a hash-chained audit log; the incident evidence for a 24/72-hour reporting window is one URL.</p></div>
  <div class="cell"><span class="lbl">The cost of no</span><h3>Blocking AI creates shadow AI</h3><p>A ban pushes usage underground and forfeits the productivity. Sluss is how you say yes: sensitive data stays in the house or is blocked fail-closed — everything else gets the best model at the lowest cost.</p></div>
</div>

<span class="lbl">Why Sluss</span>
<h2>{{.Unlike}}</h2>
<div class="grid" style="margin-top:22px">
  <div class="cell"><span class="lbl">Deterministic</span><h3>Explainable by design</h3><p>No LLM in the routing decision — rules and scoring only, with human-readable reasons on every choice. An auditor can follow it.</p></div>
  <div class="cell"><span class="lbl">Fail-closed</span><h3>Never a silent fallback</h3><p>If no provider satisfies the policy's residency and compliance requirements, the request is blocked and audited — it is never quietly sent to the cloud.</p></div>
  <div class="cell"><span class="lbl">Self-hosted</span><h3>Runs in your infrastructure</h3><p>One static binary or container, in your environment. The gateway that guards your data never lives in someone else's cloud.</p></div>
  <div class="cell"><span class="lbl">Evidence</span><h3>Proof you can hand over</h3><p>A tamper-evident, hash-chained audit log you verify offline, a readable control report, and per-window incident evidence.</p></div>
  <div class="cell"><span class="lbl">Monitor first</span><h3>Measure before you enforce</h3><p>Run a compliance pack in shadow mode: nothing changes for users, and the gap report quantifies the exposure before you flip enforcement on.</p></div>
  <div class="cell"><span class="lbl">ROI</span><h3>Cost &amp; CO₂ down</h3><p>Cheap tasks to cheap models, hard tasks to premium — automatically. Savings shown versus an all-premium baseline, labeled as estimates.</p></div>
</div>

<span class="lbl" id="how">How it works</span>
<h2>Classify. Route by policy. Prove it.</h2>
<div class="steps">
  <div class="step"><div class="n">01</div><h3>Classify</h3><p>Rule-based signals — task, risk, PII, sensitivity — in under a millisecond. No model call.</p></div>
  <div class="step"><div class="n">02</div><h3>Route by policy</h3><p>Score candidates on cost, quality, health and compliance tags; enforce residency; pick the right model.</p></div>
  <div class="step"><div class="n">03</div><h3>Prove it</h3><p>Every decision is logged, explainable, and exportable as tamper-evident evidence.</p></div>
</div>

<span class="lbl">Policy</span>
<h2>Run it like a firewall.</h2>
<div class="steps">
  <div class="step"><div class="n">01</div><h3>Pick a compliance pack</h3><p>Embedded rulesets per regime — NIS2, DORA, GDPR-sovereign — enabled with one setting. Adapt the rules like a firewall's defaults: condition &rarr; action, evaluated block-first, fail-closed.</p></div>
  <div class="step"><div class="n">02</div><h3>Start in monitor mode</h3><p>Run the pack as a shadow policy: nothing blocks, nothing changes. After two weeks the gap report says how many prompts with personal data went to a cloud model — and what the pack would have done instead.</p></div>
  <div class="step"><div class="n">03</div><h3>Enforce and prove</h3><p>Flip one setting to enforce. Dry-run any prompt in the policy console to see which rule fires. Every decision and every block lands in the tamper-evident audit chain.</p></div>
</div>

<span class="lbl">Questions</span>
<h2>FAQ</h2>
<div class="faq">
{{range .FAQ}}<details><summary>{{.Q}}</summary><p>{{.A}}</p></details>
{{end}}</div>

<footer>
  <span>{{.Name}} — control and evidence for LLM usage.</span>
  <span>Not a legal compliance certification.</span>
</footer>
</div>
</body></html>`
