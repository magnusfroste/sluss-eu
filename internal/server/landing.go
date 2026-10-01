package server

// Instance home page. Marketing lives on the product site (www.sluss.eu); a
// running Sluss instance is an operator tool, so its root page only says what
// this is and gets the operator in: sign in, connect a client, docs. It is not
// meant to be found by search engines — robots.txt and a noindex meta keep a
// self-hosted gateway out of the index (and out of the product site's way).
// Credibility rule (docs/00-product/08-positioning-ciso.md): control +
// evidence, never legal "compliance" claims.

import (
	"html/template"
	"net/http"
)

const (
	brandName    = "Sluss"
	brandTagline = "the data-sovereign LLM gateway"
	// brandOneLiner is the only product copy on the instance home page.
	brandOneLiner  = "Every prompt is classified with deterministic rules, routed by your policy — sensitive data stays in the house or is blocked fail-closed — and recorded in a tamper-evident audit log."
	productSiteURL = "https://www.sluss.eu"
	repoURL        = "https://github.com/magnusfroste/sluss-eu"
)

// LandingOptions carries the public base URL (used by /connect for copy-paste
// recipes) and the regime label.
type LandingOptions struct {
	PublicURL string // e.g. https://sluss.example.com (no trailing slash)
	// ProfileLabel is the small label chip, flavoured by the active regime
	// profile (ISSUE-094). Empty keeps the NIS2 default.
	ProfileLabel string
}

type landingData struct {
	Name, Tagline, OneLiner, Label string
	ProductSite, Repo              string
	FaviconLinks                   template.HTML
	Logo                           template.HTML
}

// LandingHandler serves the instance home page.
func LandingHandler(opts LandingOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := opts.ProfileLabel
		if label == "" {
			label = "LLM egress control · NIS2"
		}
		d := landingData{
			Name: brandName, Tagline: brandTagline, OneLiner: brandOneLiner, Label: label,
			ProductSite: productSiteURL, Repo: repoURL,
			FaviconLinks: template.HTML(faviconLinks),
			Logo:         template.HTML(brandLogoSVG),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		if err := landingTmpl.Execute(w, d); err != nil {
			http.Error(w, "landing render error", http.StatusInternalServerError)
		}
	}
}

// RobotsHandler keeps the whole instance out of search indexes: a gateway is
// an operator tool, and the product site is where people should land.
func RobotsHandler(_ LandingOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	}
}

var landingTmpl = template.Must(template.New("landing").Parse(landingHTML))

const landingHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{.Name}} — {{.Tagline}}</title>
{{.FaviconLinks}}
<style>
:root{--bg:#09090b;--panel:#101013;--line:#1e1e22;--ink:#f4f4f5;--muted:#8a8a92;--faint:#5c5c63;--accent:#3fb27f}
*{box-sizing:border-box}
html{-webkit-font-smoothing:antialiased}
body{margin:0;background:var(--bg);color:var(--ink);min-height:100vh;display:flex;flex-direction:column;
  font-family:ui-sans-serif,system-ui,-apple-system,"Inter","Segoe UI",Roboto,sans-serif;line-height:1.6;letter-spacing:-0.01em}
.wrap{width:100%;max-width:640px;margin:0 auto;padding:0 24px}
main{flex:1;display:flex;align-items:center}
.brand{display:flex;align-items:center;gap:10px;font-weight:600;font-size:18px}
.lbl{display:inline-block;margin-top:28px;font-size:11px;letter-spacing:.14em;text-transform:uppercase;color:var(--faint)}
h1{font-size:2rem;line-height:1.15;margin:10px 0 14px;font-weight:600;letter-spacing:-0.02em}
.sub{color:var(--muted);font-size:1.02rem;margin:0}
.cta{display:flex;flex-wrap:wrap;gap:12px;margin-top:28px}
.btn{display:inline-block;padding:11px 22px;border-radius:8px;font-weight:500;font-size:15px;text-decoration:none}
.btn.primary{background:var(--accent);color:#05130c}.btn.primary:hover{background:#4cc78e}
.btn.ghost{border:1px solid var(--line);color:var(--ink)}.btn.ghost:hover{border-color:var(--faint)}
.start{margin-top:40px;border:1px solid var(--line);border-radius:12px;background:var(--panel);padding:18px 22px}
.start h2{font-size:.8rem;letter-spacing:.12em;text-transform:uppercase;color:var(--faint);margin:0 0 8px;font-weight:600}
.start ol{margin:0;padding-left:20px;color:var(--muted);font-size:.93rem}
.start li{margin:4px 0}.start b{color:var(--ink);font-weight:500}
.start code{font-family:ui-monospace,Menlo,monospace;font-size:.85rem;color:var(--ink)}
.start a{color:var(--ink)}
footer{border-top:1px solid var(--line);padding:20px 0;color:var(--faint);font-size:13px}
footer .wrap{display:flex;justify-content:space-between;flex-wrap:wrap;gap:10px}
footer a{color:var(--muted);text-decoration:none}footer a:hover{color:var(--ink)}
</style>
</head>
<body>
<main><div class="wrap">
  <div class="brand">{{.Logo}} {{.Name}}</div>
  <span class="lbl">{{.Label}}</span>
  <h1>Decide where your AI data goes — and prove it.</h1>
  <p class="sub">{{.OneLiner}}</p>
  <div class="cta">
    <a class="btn primary" href="/router/dashboard">Sign in</a>
    <a class="btn ghost" href="/connect">Connect a client</a>
  </div>
  <section class="start" aria-label="Getting started">
    <h2>Getting started</h2>
    <ol>
      <li><b>Sign in</b> with the console password (<code>ROUTER_DASHBOARD_PASSWORD</code>) or an admin user.</li>
      <li><b>Add a provider and a model</b> on Providers and Models — tag on-prem providers <code>local</code>. Changes apply immediately.</li>
      <li><b>Pick a policy</b> — start with the NIS2 pack, then <b>dry-run</b> a prompt on Policy to see where it would go.</li>
      <li><b>Point a client</b> at this gateway with an API key — recipes on <a href="/connect">Connect</a>.</li>
    </ol>
  </section>
</div></main>
<footer><div class="wrap">
  <span><a href="{{.ProductSite}}">About Sluss</a> · <a href="{{.Repo}}">Docs &amp; source</a> · <a href="{{.Repo}}/blob/main/SECURITY.md">Security</a></span>
  <span>Open source · AGPL-3.0 · not a legal compliance certification</span>
</div></footer>
</body></html>`
