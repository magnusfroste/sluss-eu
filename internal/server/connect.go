package server

// Public "Connect / Integrations" catalog (GET /connect). A model router only
// delivers value once clients point at it, so this is the activation surface:
// per-client recipes for pointing IDEs, coding agents, chat UIs and SDKs at the
// router. It doubles as SEO/AEO (people search "point Cursor at an
// OpenAI-compatible base url") and as the in-product onboarding page — when the
// instance has a PublicURL the recipes show that instance's real base URL.
//
// The recipe is uniform because the router is OpenAI- and Anthropic-compatible:
// set a base URL + an API key, keep model on "auto", and the router decides.

import (
	"html/template"
	"net/http"
	"strings"
)

type connectClient struct {
	Name  string
	Kind  string // ide | agent | chat | sdk
	Wire  string // openai | anthropic
	How   string // where to put the base URL + key
	Setup template.HTML
}

// connectClients is the catalog. OpenAI-wire clients use the /v1 base; Anthropic
// -native clients (Claude Code, Codex, Anthropic SDKs) use the /v1/messages shim
// via the Anthropic base URL.
func connectClients(base string) []connectClient {
	oa := base + "/v1"
	return []connectClient{
		{"Cursor", "ide", "openai", "Settings → Models → OpenAI API Key → Override Base URL", template.HTML("Base URL <code>" + oa + "</code>, paste your key, add a model named <code>auto</code>.")},
		{"Cline / Continue", "ide", "openai", "OpenAI Compatible provider", template.HTML("Base URL <code>" + oa + "</code>, API key = your department key, model <code>auto</code>.")},
		{"Claude Code", "agent", "anthropic", "Environment variables", template.HTML("<code>ANTHROPIC_BASE_URL=" + base + "</code> and <code>ANTHROPIC_API_KEY=</code>your key. Uses the <code>/v1/messages</code> shim.")},
		{"Codex CLI", "agent", "openai", "config / env", template.HTML("Point the OpenAI base URL at <code>" + oa + "</code> and set the API key to your department key.")},
		{"Aider", "agent", "openai", "--openai-api-base", template.HTML("<code>aider --openai-api-base " + oa + " --openai-api-key &lt;key&gt; --model auto</code>")},
		{"OpenWebUI", "chat", "openai", "Admin → Settings → Connections → OpenAI API", template.HTML("API Base URL <code>" + oa + "</code>, API Key = your key. Models appear from <code>/v1/models</code>.")},
		{"AnythingLLM", "chat", "openai", "LLM Preference → Generic OpenAI", template.HTML("Base URL <code>" + oa + "</code>, API key = your key, model <code>auto</code>, chat token limit as you like.")},
		{"LibreChat / Jan", "chat", "openai", "Custom OpenAI endpoint", template.HTML("Base URL <code>" + oa + "</code>, API key = your key.")},
		{"OpenAI SDK (Python/JS)", "sdk", "openai", "client base_url", template.HTML("<code>OpenAI(base_url=\"" + oa + "\", api_key=\"&lt;key&gt;\")</code> then call <code>model=\"auto\"</code>.")},
		{"Anthropic SDK", "sdk", "anthropic", "client base_url", template.HTML("<code>Anthropic(base_url=\"" + base + "\", api_key=\"&lt;key&gt;\")</code> — routed via <code>/v1/messages</code>.")},
	}
}

type connectData struct {
	Name, Tagline, PublicURL, Base, OpenAIBase string
	HasBase                                    bool
	FaviconLinks, Logo                         template.HTML
	Clients                                    []connectClient
}

// ConnectHandler serves the public/instance Connect catalog.
func ConnectHandler(opts LandingOptions) http.HandlerFunc {
	base := strings.TrimRight(opts.PublicURL, "/")
	display := base
	if display == "" {
		display = "https://tokenizer.your-company" // placeholder for the public/dev view
	}
	return func(w http.ResponseWriter, r *http.Request) {
		d := connectData{
			Name: brandName, Tagline: "Point your clients at the router",
			PublicURL: base, Base: display, OpenAIBase: display + "/v1", HasBase: base != "",
			FaviconLinks: template.HTML(faviconLinks), Logo: template.HTML(brandLogoSVG),
			Clients: connectClients(display),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := connectTmpl.Execute(w, d); err != nil {
			http.Error(w, "connect render error", http.StatusInternalServerError)
		}
	}
}

var connectTmpl = template.Must(template.New("connect").Parse(connectHTML))

const connectHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — Connect your client to the router</title>
<meta name="description" content="Point Cursor, Claude Code, Codex, AnythingLLM, OpenWebUI and any OpenAI- or Anthropic-compatible client at Sluss: set the base URL and your key, keep model on auto, and the router decides.">
{{if .PublicURL}}<link rel="canonical" href="{{.PublicURL}}/connect">{{end}}
<meta name="robots" content="index,follow">
{{.FaviconLinks}}
<style>
:root{--bg:#081328;--panel:#101c34;--line:rgba(255,255,255,.09);--ink:#eaeff5;--muted:#9aa5b8;--faint:#6b7891;--accent:#fad100}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);background-image:radial-gradient(60% 50% at 70% 0%,rgba(250,209,0,.08),transparent 70%);font-family:"IBM Plex Sans",ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;line-height:1.6;letter-spacing:-0.01em}
.wrap{max-width:960px;margin:0 auto;padding:0 24px}
nav{display:flex;align-items:center;justify-content:space-between;padding:22px 0;border-bottom:1px solid var(--line)}
.brand{display:flex;align-items:center;gap:9px;font-weight:600;font-size:15px}
nav a{color:var(--muted);text-decoration:none;font-size:14px;border:1px solid var(--line);padding:8px 15px;border-radius:8px}
.hero{padding:56px 0 8px;max-width:720px}
.lbl{font-size:11px;letter-spacing:.14em;text-transform:uppercase;color:var(--accent);font-family:"IBM Plex Mono",ui-monospace,monospace}
h1{font-size:2.2rem;line-height:1.1;margin:12px 0 14px;font-weight:600;letter-spacing:-0.02em}
.sub{color:var(--muted)}
.recipe{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:18px 20px;margin:22px 0}
.recipe h2{font-size:1rem;margin:0 0 10px}
.kv{display:grid;grid-template-columns:110px 1fr auto;gap:8px 12px;align-items:center;font-size:.92rem}
.kv .k{color:var(--faint);font-size:.8rem}
code{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;background:#0b1528;border:1px solid var(--line);border-radius:6px;padding:2px 7px;font-size:.86rem;color:#eaeff5}
button.cp{background:transparent;border:1px solid var(--line);color:var(--muted);border-radius:6px;padding:4px 10px;font-size:12px;cursor:pointer}
button.cp:hover{color:#081328;background:var(--accent);border-color:var(--accent)}
table{width:100%;border-collapse:collapse;margin:8px 0 40px}
th{text-align:left;font-size:.72rem;color:var(--faint);text-transform:uppercase;letter-spacing:.06em;padding:10px 10px;border-bottom:1px solid var(--line)}
td{padding:11px 10px;border-bottom:1px solid var(--line);font-size:.9rem;vertical-align:top}
.tag{display:inline-block;font-size:11px;padding:2px 8px;border-radius:20px;border:1px solid var(--line);color:var(--muted)}
.tag.anthropic{color:#d6b4f0;border-color:#3a2a4a}.tag.openai{color:#9fdcc4;border-color:#26382f}
h2.sec{font-size:1.35rem;margin:40px 0 6px}
.note{color:var(--faint);font-size:13px}
footer{border-top:1px solid var(--line);margin-top:40px;padding:26px 0;color:var(--faint);font-size:13px}
@media(max-width:640px){.kv{grid-template-columns:1fr}}
</style></head>
<body>
<div class="wrap">
<nav><span class="brand">{{.Logo}} {{.Name}}</span><a href="/">Overview</a></nav>

<section class="hero">
  <span class="lbl">Connect</span>
  <h1>Point your client at the router.</h1>
  <p class="sub">Sluss is OpenAI- and Anthropic-compatible. Set the base URL and your key, keep the model on <code>auto</code>, and the router classifies each prompt and picks the right model — with egress control and an audit trail.{{if not .HasBase}} <span class="note">(Examples use a placeholder host — set <code>ROUTER_PUBLIC_URL</code> so this page shows your instance's real base URL.)</span>{{end}}</p>
</section>

<div class="recipe">
  <h2>The three values (every client)</h2>
  <div class="kv">
    <span class="k">Base URL</span><code id="b1">{{.OpenAIBase}}</code><button class="cp" onclick="cp('b1')">Copy</button>
    <span class="k">Anthropic</span><code id="b2">{{.Base}}</code><button class="cp" onclick="cp('b2')">Copy</button>
    <span class="k">API key</span><code>your department key (from the Keys page)</code><span></span>
    <span class="k">Model</span><code id="b3">auto</code><button class="cp" onclick="cp('b3')">Copy</button>
  </div>
</div>

<h2 class="sec">Per client</h2>
<table>
<thead><tr><th>Client</th><th>Wire</th><th>Where</th><th>Setup</th></tr></thead>
<tbody>
{{range .Clients}}
<tr>
  <td><b>{{.Name}}</b></td>
  <td><span class="tag {{.Wire}}">{{.Wire}}</span></td>
  <td class="note">{{.How}}</td>
  <td>{{.Setup}}</td>
</tr>
{{end}}
</tbody>
</table>

<p class="note">Anthropic-wire clients use the <code>/v1/messages</code> endpoint; OpenAI-wire clients use <code>/v1/chat/completions</code>. Both run through the same routing, policy and audit path. Mint per-department keys on the Keys page for attribution and accountability.</p>

<footer>{{.Name}} — the router only delivers value once your clients point at it.</footer>
</div>
<script>
function cp(id){var t=document.getElementById(id).textContent;navigator.clipboard&&navigator.clipboard.writeText(t)}
</script>
</body></html>`
