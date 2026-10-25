package server

// Rollout kit (ISSUE-128a): what an IT organisation with an MDM (Intune, Jamf,
// Kandji, Fleet) needs to make Sluss the only AI path on managed devices,
// without any agent of ours on the desktop:
//
//  1. Redirect: scripts the MDM runs once that point every tool that honours
//     OPENAI_BASE_URL / ANTHROPIC_BASE_URL at this instance.
//  2. Block + offer: a deny list of direct generative-AI domains for the
//     customer's DNS filter or secure web gateway, a block page on this
//     instance the sinkhole can point at, a web clip and a browser policy
//     that put the company AI chat where ChatGPT used to be.
//  3. Coverage: a read-only check the MDM runs on a schedule, reporting per
//     device whether a provider can still be reached directly and whether
//     Sluss is reachable. Hostname and two answers — never a user, never
//     content. The dashboard turns that into "N devices, M still open".
//
// Nothing here inspects traffic. Blocking is the customer's existing filter;
// Sluss supplies the list, the landing spot and the evidence.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// RolloutOptions wires the page, the generated files and the coverage sink.
type RolloutOptions struct {
	History   *history.Store
	PublicURL string // https://sluss.example.eu — the files embed it
	Auditor   audit.Sink
	Logger    *slog.Logger
}

// genAIDomain is one entry of the deny list.
type genAIDomain struct {
	Domain, Vendor, Kind string // kind: chat | api
}

// genAIDomains is the curated deny list: consumer chat UIs and provider APIs
// that bypass the gateway. Microsoft 365 Copilot is deliberately absent — it
// is contracted separately and has no base URL to redirect.
var genAIDomains = []genAIDomain{
	{"chatgpt.com", "OpenAI", "chat"}, {"chat.openai.com", "OpenAI", "chat"},
	{"api.openai.com", "OpenAI", "api"}, {"platform.openai.com", "OpenAI", "api"},
	{"claude.ai", "Anthropic", "chat"}, {"api.anthropic.com", "Anthropic", "api"}, {"console.anthropic.com", "Anthropic", "api"},
	{"gemini.google.com", "Google", "chat"}, {"aistudio.google.com", "Google", "api"}, {"generativelanguage.googleapis.com", "Google", "api"},
	{"chat.mistral.ai", "Mistral", "chat"}, {"api.mistral.ai", "Mistral", "api"},
	{"chat.deepseek.com", "DeepSeek", "chat"}, {"api.deepseek.com", "DeepSeek", "api"},
	{"grok.com", "xAI", "chat"}, {"api.x.ai", "xAI", "api"},
	{"perplexity.ai", "Perplexity", "chat"}, {"api.perplexity.ai", "Perplexity", "api"},
	{"poe.com", "Quora", "chat"}, {"character.ai", "Character.AI", "chat"},
	{"huggingface.co", "Hugging Face", "chat"}, {"openrouter.ai", "OpenRouter", "api"},
}

// probeAPIs are the provider endpoints the coverage script probes. Each
// answers 401 to an unauthenticated request: that is the signature of the
// real provider, which a DNS sinkhole or a block page never produces.
var probeAPIs = []string{"https://api.openai.com/v1/models", "https://api.anthropic.com/v1/models"}

// rolloutFile is one downloadable artefact.
type rolloutFile struct {
	Name, Title, For, Mime string
	Body                   func(base string) string
}

func (o RolloutOptions) base(r *http.Request) string {
	if b := strings.TrimRight(strings.TrimSpace(o.PublicURL), "/"); b != "" {
		return b
	}
	scheme := "http"
	if requestIsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func hostOf(base string) string {
	h := strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
	if i := strings.IndexAny(h, "/:"); i >= 0 {
		h = h[:i]
	}
	return h
}

// uuidFor makes a stable UUID-shaped id from a seed, so a re-downloaded
// profile is the same profile to the MDM (an update, not a second one).
func uuidFor(seed string) string {
	s := sha256.Sum256([]byte(seed))
	h := fmt.Sprintf("%x", s[:16])
	return strings.ToUpper(h[0:8] + "-" + h[8:12] + "-4" + h[13:16] + "-A" + h[17:20] + "-" + h[20:32])
}

var rolloutFiles = []rolloutFile{
	{"sluss-env.sh", "Point tools at Sluss — macOS & Linux", "Jamf policy / Fleet script, runs as root once", "text/x-shellscript", envSh},
	{"sluss-env.ps1", "Point tools at Sluss — Windows", "Intune platform script (system context)", "text/plain", envPs1},
	{"genai-domains.csv", "Deny list — generative AI domains", "DNS filter / secure web gateway (Umbrella, Zscaler, Cloudflare Gateway)", "text/csv", domainsCSV},
	{"genai-domains.json", "Deny list — JSON", "Same list for APIs and scripts; includes the allow entry", "application/json", domainsJSON},
	{"genai-defender-indicators.csv", "Deny list — Defender indicators", "Defender for Endpoint: Settings → Endpoints → Indicators → Import", "text/csv", defenderIndicators},
	{"sluss-webclip.mobileconfig", "Web clip — company AI chat", "macOS & iOS configuration profile (Jamf, Intune, Mosyle)", "application/x-apple-aspen-config", webClip},
	{"sluss-browser-policy.json", "Browser policy — bookmark + DoH off", "Chrome/Edge managed policy (Intune settings catalog, Jamf plist)", "application/json", browserPolicy},
	{"coverage.sh", "Coverage check — macOS & Linux", "Jamf extension attribute / Fleet query, daily", "text/x-shellscript", coverageSh},
	{"coverage.ps1", "Coverage check — Windows", "Intune remediation (detection script), daily", "text/plain", coveragePs1},
}

func envSh(base string) string {
	return `#!/bin/sh
# Sluss rollout kit: point every tool that honours OPENAI_BASE_URL /
# ANTHROPIC_BASE_URL at the company gateway. Run as root once (Jamf policy,
# Fleet script, Ansible). Idempotent. The API key is NOT set here: hand it out
# per department (` + base + `/router/keys) through your MDM's secret handling
# or let the user paste it once.
set -eu
SLUSS="` + base + `"
MARK="# sluss-rollout"
write_block() {
  f="$1"; mkdir -p "$(dirname "$f")"
  [ -f "$f" ] && grep -q "$MARK" "$f" && sed -i.bak "/$MARK/,/$MARK end/d" "$f" && rm -f "$f.bak"
  {
    echo "$MARK"
    echo "export OPENAI_BASE_URL=\"$SLUSS/v1\""
    echo "export OPENAI_API_BASE=\"$SLUSS/v1\""
    echo "export ANTHROPIC_BASE_URL=\"$SLUSS\""
    echo "$MARK end"
  } >> "$f"
}
case "$(uname -s)" in
  Darwin)
    write_block /etc/zshenv
    # GUI apps do not read shell files: a LaunchAgent sets the same variables
    # in each user's launchd session at login.
    cat > /Library/LaunchAgents/eu.sluss.env.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>eu.sluss.env</string>
  <key>ProgramArguments</key><array>
    <string>/bin/sh</string><string>-c</string>
    <string>launchctl setenv OPENAI_BASE_URL "$SLUSS/v1"; launchctl setenv OPENAI_API_BASE "$SLUSS/v1"; launchctl setenv ANTHROPIC_BASE_URL "$SLUSS"</string>
  </array>
  <key>RunAtLoad</key><true/>
</dict></plist>
EOF
    chmod 644 /Library/LaunchAgents/eu.sluss.env.plist
    ;;
  *)
    write_block /etc/profile.d/sluss.sh
    chmod 644 /etc/profile.d/sluss.sh
    ;;
esac
echo "sluss: tools now use $SLUSS (takes effect at next login)"
`
}

func envPs1(base string) string {
	return `# Sluss rollout kit: point every tool that honours OPENAI_BASE_URL /
# ANTHROPIC_BASE_URL at the company gateway. Deploy as an Intune platform
# script in SYSTEM context (or run once as administrator). Idempotent. The API
# key is NOT set here: hand it out per department (` + base + `/router/keys).
$Sluss = "` + base + `"
[Environment]::SetEnvironmentVariable("OPENAI_BASE_URL", "$Sluss/v1", "Machine")
[Environment]::SetEnvironmentVariable("OPENAI_API_BASE", "$Sluss/v1", "Machine")
[Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", "$Sluss", "Machine")
Write-Output "sluss: tools now use $Sluss (new processes pick it up)"
`
}

func domainsCSV(base string) string {
	var b strings.Builder
	b.WriteString("domain,action,vendor,kind,note\n")
	for _, d := range genAIDomains {
		fmt.Fprintf(&b, "%s,deny,%s,%s,direct generative AI\n", d.Domain, d.Vendor, d.Kind)
	}
	fmt.Fprintf(&b, "%s,allow,Sluss,gateway,the company AI path\n", hostOf(base))
	return b.String()
}

func domainsJSON(base string) string {
	type entry struct {
		Domain string `json:"domain"`
		Action string `json:"action"`
		Vendor string `json:"vendor"`
		Kind   string `json:"kind"`
	}
	out := struct {
		Generated string  `json:"generated"`
		BlockPage string  `json:"block_page"`
		Entries   []entry `json:"entries"`
	}{time.Now().UTC().Format("2006-01-02"), base + "/blocked", nil}
	for _, d := range genAIDomains {
		out.Entries = append(out.Entries, entry{d.Domain, "deny", d.Vendor, d.Kind})
	}
	out.Entries = append(out.Entries, entry{hostOf(base), "allow", "Sluss", "gateway"})
	j, _ := json.MarshalIndent(out, "", "  ")
	return string(j) + "\n"
}

func webClip(base string) string {
	esc := template.HTMLEscapeString
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>PayloadContent</key>
  <array>
    <dict>
      <key>PayloadType</key><string>com.apple.webClip.managed</string>
      <key>PayloadVersion</key><integer>1</integer>
      <key>PayloadIdentifier</key><string>eu.sluss.webclip.chat</string>
      <key>PayloadUUID</key><string>` + uuidFor("webclip:"+base) + `</string>
      <key>PayloadDisplayName</key><string>Company AI</string>
      <key>Label</key><string>Company AI</string>
      <key>URL</key><string>` + esc(base) + `/chat</string>
      <key>IsRemovable</key><false/>
      <key>FullScreen</key><true/>
      <key>Precomposed</key><true/>
    </dict>
  </array>
  <key>PayloadDisplayName</key><string>Sluss — company AI chat</string>
  <key>PayloadDescription</key><string>Puts the company AI chat on the home screen and in Launchpad.</string>
  <key>PayloadIdentifier</key><string>eu.sluss.rollout.webclip</string>
  <key>PayloadOrganization</key><string>Sluss</string>
  <key>PayloadRemovalDisallowed</key><true/>
  <key>PayloadScope</key><string>System</string>
  <key>PayloadType</key><string>Configuration</string>
  <key>PayloadUUID</key><string>` + uuidFor("profile:"+base) + `</string>
  <key>PayloadVersion</key><integer>1</integer>
</dict>
</plist>
`
}

func browserPolicy(base string) string {
	out := map[string]any{
		"_comment": "Chrome and Edge managed policy. Intune: settings catalog / ADMX; Jamf: the same keys in com.google.Chrome / com.microsoft.Edge. DnsOverHttpsMode=off keeps the browser on the company DNS so the deny list applies.",
		"ManagedBookmarks": []any{
			map[string]string{"toplevel_name": "Company"},
			map[string]string{"name": "Company AI chat", "url": base + "/chat"},
		},
		"DnsOverHttpsMode":        "off",
		"BuiltInDnsClientEnabled": false,
	}
	j, _ := json.MarshalIndent(out, "", "  ")
	return string(j) + "\n"
}

func coverageSh(base string) string {
	probes := strings.Join(probeAPIs, " ")
	return `#!/bin/sh
# Sluss coverage check: can this device still reach a generative-AI provider
# directly, and does it reach Sluss? Reports hostname, OS and two answers —
# never a user, never content. Run daily from your MDM (Jamf extension
# attribute, Fleet, cron). Set SLUSS_KEY to a department key from
# ` + base + `/router/keys through your MDM's secret handling.
SLUSS="` + base + `"
KEY="${SLUSS_KEY:-REPLACE_WITH_DEPARTMENT_KEY}"
code() { curl -sS -m 6 -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || echo 000; }
direct=closed
# A real provider answers 401 to an unauthenticated call. A sinkhole or a
# block page never does — so 401 means the direct path is open.
for u in ` + probes + `; do
  [ "$(code "$u")" = "401" ] && direct=open && break
done
sluss=unreachable
[ "$(code "$SLUSS/healthz")" = "200" ] && sluss=ok
curl -sS -m 10 -o /dev/null -X POST "$SLUSS/router/coverage" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d "{\"device\":\"$(hostname)\",\"os\":\"$(uname -s)\",\"direct_ai\":\"$direct\",\"sluss\":\"$sluss\"}"
echo "direct_ai=$direct sluss=$sluss"
# Jamf extension attribute: <result>direct_ai=$direct</result>
`
}

func coveragePs1(base string) string {
	var probes []string
	for _, p := range probeAPIs {
		probes = append(probes, `"`+p+`"`)
	}
	return `# Sluss coverage check: can this device still reach a generative-AI provider
# directly, and does it reach Sluss? Reports hostname, OS and two answers —
# never a user, never content. Deploy as an Intune remediation detection
# script, daily. Set the key through your MDM's secret handling.
$Sluss = "` + base + `"
$Key = if ($env:SLUSS_KEY) { $env:SLUSS_KEY } else { "REPLACE_WITH_DEPARTMENT_KEY" }
function Code($u) {
  try { return (Invoke-WebRequest -Uri $u -Method Get -TimeoutSec 6 -UseBasicParsing).StatusCode }
  catch { if ($_.Exception.Response) { return [int]$_.Exception.Response.StatusCode } else { return 0 } }
}
$direct = "closed"
# A real provider answers 401 to an unauthenticated call; a sinkhole or a
# block page never does — so 401 means the direct path is open.
foreach ($u in @(` + strings.Join(probes, ", ") + `)) { if ((Code $u) -eq 401) { $direct = "open"; break } }
$sluss = if ((Code "$Sluss/healthz") -eq 200) { "ok" } else { "unreachable" }
$body = @{ device = $env:COMPUTERNAME; os = "Windows"; direct_ai = $direct; sluss = $sluss } | ConvertTo-Json -Compress
try { Invoke-RestMethod -Uri "$Sluss/router/coverage" -Method Post -Headers @{ Authorization = "Bearer $Key" } -ContentType "application/json" -Body $body -TimeoutSec 10 | Out-Null } catch {}
Write-Output "direct_ai=$direct sluss=$sluss"
# Intune remediation: exit 1 when the direct path is open, so the device shows as non-compliant.
if ($direct -eq "open") { exit 1 } else { exit 0 }
`
}

// RolloutFileHandler serves one generated file by name.
func RolloutFileHandler(o RolloutOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		for _, f := range rolloutFiles {
			if f.Name != name {
				continue
			}
			w.Header().Set("Content-Type", f.Mime+"; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="`+f.Name+`"`)
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, f.Body(o.base(r)))
			return
		}
		http.NotFound(w, r)
	}
}

// CoverageReportHandler takes the script's report. Gated by auth.Middleware
// (a department key), so the row is attributed to that department.
func CoverageReportHandler(o RolloutOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o.History == nil {
			http.Error(w, "coverage needs ROUTER_DATA_DIR", http.StatusServiceUnavailable)
			return
		}
		var in struct {
			Device, OS, DirectAI, Sluss string
		}
		var raw struct {
			Device   string `json:"device"`
			OS       string `json:"os"`
			DirectAI string `json:"direct_ai"`
			Sluss    string `json:"sluss"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&raw); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		in.Device, in.OS, in.DirectAI, in.Sluss = clip(raw.Device, 128), clip(raw.OS, 32), raw.DirectAI, raw.Sluss
		if in.Device == "" || (in.DirectAI != "open" && in.DirectAI != "closed") {
			http.Error(w, "device and direct_ai (open|closed) are required", http.StatusBadRequest)
			return
		}
		if in.Sluss != "ok" {
			in.Sluss = "unreachable"
		}
		t, _ := tenant.FromContext(r.Context())
		rep := history.CoverageReport{Device: in.Device, OS: in.OS, DirectAI: in.DirectAI, Sluss: in.Sluss, ReportedAt: time.Now()}
		if t != nil {
			rep.TenantID = t.ID
		}
		if err := o.History.UpsertCoverage(rep); err != nil {
			http.Error(w, "could not store the report", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// fromLine names the blocked host when the sinkhole passed it along.
func fromLine(from string) string {
	if from == "" {
		return ""
	}
	return `<span class="from">` + template.HTMLEscapeString(from) + `</span> is blocked on this network. `
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// BlockedPageHandler is the public landing spot a DNS sinkhole or web
// gateway points at: what was blocked, and where the sanctioned path is.
func BlockedPageHandler(o RolloutOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.Header().Set("Cache-Control", "no-store")
		from := clip(r.URL.Query().Get("from"), 80)
		_, _ = w.Write([]byte(strings.NewReplacer(
			"{{CHAT}}", template.HTMLEscapeString(o.base(r)+"/chat"),
			"{{FROMLINE}}", fromLine(from),
			"{{FAVICON}}", faviconLinks,
			"{{LOGO}}", brandLogoSVG,
		).Replace(blockedHTML)))
	}
}

const blockedHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Use the company AI</title>
{{FAVICON}}
<style>
:root{--bg:#081328;--panel:#101c34;--line:rgba(255,255,255,.09);--ink:#eaeff5;--muted:#9aa5b8;--accent:#fad100}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font-family:"IBM Plex Sans",ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;
  background-image:radial-gradient(60% 50% at 70% 0%,rgba(250,209,0,.10),transparent 70%);display:flex;min-height:100vh;align-items:center;justify-content:center;padding:16px}
.box{width:100%;max-width:440px;background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:28px 26px}
.brand{display:flex;align-items:center;gap:9px;font-weight:600;margin-bottom:18px}
h1{font-size:1.25rem;margin:0 0 10px}
p{color:var(--muted);font-size:.93rem;line-height:1.55;margin:0 0 12px}
.from{font-family:ui-monospace,Menlo,monospace;color:var(--ink)}
a.go{display:block;text-align:center;background:var(--accent);color:#081328;text-decoration:none;border-radius:8px;padding:12px;font-weight:700;margin-top:18px}
.sub{color:#6b7891;font-size:12px;margin-top:14px}
</style></head>
<body>
<div class="box">
  <div class="brand">{{LOGO}} Sluss</div>
  <h1>Generative AI outside the company is blocked here</h1>
  <p>{{FROMLINE}}The company runs its own AI gateway: the same models, with personal and confidential data kept where policy says it must stay, and every request logged for the organisation's own evidence.</p>
  <a class="go" href="{{CHAT}}">Open the company AI chat →</a>
  <div class="sub">Sign in with your company account. Questions: your IT service desk.</div>
</div>
</body></html>`

// --- the console page ---------------------------------------------------------

type rolloutPageData struct {
	Base    string
	Host    string
	Files   []rolloutFile
	Domains []genAIDomain
	Devices []history.CoverageReport
	Summary history.CoverageSummary
	Durable bool
	Notice  string
	Guides  []filterGuide
}

// RolloutPageHandler renders the kit: downloads, the deny list, the block
// page link and the live coverage table.
func RolloutPageHandler(o RolloutOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := o.base(r)
		d := rolloutPageData{Base: base, Host: hostOf(base), Files: rolloutFiles, Domains: genAIDomains, Durable: o.History != nil, Guides: guidesFor(base)}
		if o.History != nil {
			d.Devices, _ = o.History.ListCoverage()
			d.Summary = o.History.SummarizeCoverage()
		}
		if strings.TrimSpace(o.PublicURL) == "" {
			d.Notice = "ROUTER_PUBLIC_URL is not set: the files use this request's host (" + base + "). Set it so every download carries the public address."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := rolloutTmpl.Execute(w, d); err != nil && o.Logger != nil {
			o.Logger.Error("rollout page render failed", "err", err)
		}
	}
}

var rolloutTmpl = template.Must(template.New("rollout").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"ago": func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return fmtTime(t)
	},
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Rollout</title>
<style>
  {{adminCSS}}
  *,*::before,*::after{box-sizing:border-box}
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{padding:20px 26px;border-bottom:1px solid #22324f}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.88rem;margin-top:4px;max-width:760px;line-height:1.5}
  .wrap{padding:22px 26px 40px;max-width:980px;display:flex;flex-direction:column;gap:14px}
  .step{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:16px 20px}
  .sh{display:flex;align-items:center;gap:12px}
  .num{width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:.85rem;background:#17253f;color:#fad100;flex-shrink:0}
  .st{font-weight:600}
  .body{margin:10px 0 0 40px;color:#b7c4dc;font-size:.9rem;line-height:1.55}
  .body b{color:#e8eef7}
  code,.mono{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.85em}
  code{background:#0b1528;border:1px solid #22324f;border-radius:5px;padding:1px 6px}
  .files{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:10px;margin-top:12px}
  .file{display:flex;flex-direction:column;gap:3px;background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:10px 12px;text-decoration:none;color:#e8eef7}
  .file:hover{border-color:#fad100}
  .file .n{font-weight:600;font-size:.88rem}.file .f{color:#8fa1bf;font-size:.78rem}.file .m{color:#fad100;font-size:.78rem}
  table{width:100%;border-collapse:collapse;font-size:.85rem;margin-top:10px}
  th{text-align:left;color:#8fa1bf;font-weight:600;font-size:.75rem;text-transform:uppercase;letter-spacing:.04em;padding:6px 8px;border-bottom:1px solid #22324f}
  td{padding:7px 8px;border-bottom:1px solid #17253f;vertical-align:top}
  .pill{font-size:.76rem;font-weight:700;padding:2px 9px;border-radius:999px;background:#17253f;color:#8fa1bf}
  .pill.open{background:#3b1414;color:#f87171}.pill.closed{background:#0f3a26;color:#4ade80}.pill.ok{background:#12264a;color:#7fb2ff}
  .kpi{display:flex;gap:18px;flex-wrap:wrap;margin-top:8px}
  .kpi div{background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:10px 14px;min-width:150px}
  .kpi .v{font-size:1.5rem;font-weight:700}.kpi .v.bad{color:#f87171}.kpi .l{color:#8fa1bf;font-size:.76rem;text-transform:uppercase;letter-spacing:.04em}
  .banner{padding:11px 16px;border-radius:10px;font-size:.88rem;background:#33290f;color:#f4b740;border:1px solid #7a5a1f}
  .tbl{overflow-x:auto}
  .guides{margin-top:16px;border-top:1px solid #22324f;padding-top:12px}
  .gh{color:#8fa1bf;font-size:.78rem;text-transform:uppercase;letter-spacing:.05em;margin-bottom:6px}
  .guide{background:#0b1528;border:1px solid #22324f;border-radius:8px;margin-top:8px}
  .guide summary{cursor:pointer;padding:10px 12px;display:flex;flex-wrap:wrap;gap:8px;align-items:center}
  .guide[open] summary{border-bottom:1px solid #22324f}
  .gfit{color:#8fa1bf;font-size:.8rem;flex:1;min-width:180px}
  .guide ol{margin:10px 14px 4px 30px;padding:0}.guide li{margin-bottom:8px}
  .guide a{color:#7fd3ff}
  .gnote{margin:8px 14px 12px;color:#b7c4dc;font-size:.86rem}
  .dom{columns:3;column-gap:18px;font-size:.84rem;margin-top:8px}.dom div{break-inside:avoid;padding:2px 0}
  @media(max-width:700px){.body{margin-left:0}.dom{columns:1}}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "rollout"}}
<div class="tk-main">
<header>
  <h1>Rollout</h1>
  <div class="sub">Make Sluss the only AI path on managed devices with the tools you already have: your MDM points the apps here, your DNS filter or web gateway blocks the direct way out, and a daily check proves it per device. No agent of ours on the desktop, nothing inspects traffic.</div>
</header>
<div class="wrap">
  {{if .Notice}}<div class="banner">{{.Notice}}</div>{{end}}

  <div class="step">
    <div class="sh"><span class="num">1</span><span class="st">Point the tools here</span></div>
    <div class="body">Everything that honours <code>OPENAI_BASE_URL</code> / <code>ANTHROPIC_BASE_URL</code> — SDKs, Claude Code, Continue, scripts — follows the environment, so a developer never has to remember. Run one script per platform from Intune, Jamf or Fleet. The department key is handed out separately (<a href="/router/keys" style="color:#7fd3ff">Keys</a>), through the MDM's secret handling. Tools without a base URL (GitHub Copilot, M365 Copilot, the ChatGPT app) cannot be redirected: they are blocked in step 2 or contracted separately.
      <div class="files">
        {{range .Files}}{{if or (eq .Name "sluss-env.sh") (eq .Name "sluss-env.ps1")}}<a class="file" href="/router/rollout/file/{{.Name}}" download><span class="n">{{.Title}}</span><span class="f">{{.For}}</span><span class="m">{{.Name}} ↓</span></a>{{end}}{{end}}
      </div>
    </div>
  </div>

  <div class="step">
    <div class="sh"><span class="num">2</span><span class="st">Block the direct way out, offer the company way in</span></div>
    <div class="body">Load the deny list into the DNS filter or secure web gateway you already run (Umbrella, Zscaler, Cloudflare Gateway, NextDNS) and point its block page or sinkhole at <code>{{.Base}}/blocked</code> — a page on this instance that says what happened and links to the company chat. Keep <b>{{.Host}}</b> on the allow side. The web clip puts the chat on every home screen and in Launchpad, also on managed phones; the browser policy adds a managed bookmark and turns DNS-over-HTTPS off so the deny list applies.
      <div class="files">
        {{range .Files}}{{if or (eq .Name "genai-domains.csv") (eq .Name "genai-domains.json") (eq .Name "sluss-webclip.mobileconfig") (eq .Name "sluss-browser-policy.json")}}<a class="file" href="/router/rollout/file/{{.Name}}" download><span class="n">{{.Title}}</span><span class="f">{{.For}}</span><span class="m">{{.Name}} ↓</span></a>{{end}}{{end}}
      </div>
      <details style="margin-top:12px"><summary style="cursor:pointer;color:#8fa1bf">The {{len .Domains}} domains on the list</summary>
        <div class="dom">{{range .Domains}}<div><span class="mono">{{.Domain}}</span> <span style="color:#6b7891">· {{.Vendor}} {{.Kind}}</span></div>{{end}}</div>
      </details>
      <div style="margin-top:12px"><a href="/blocked?from=chatgpt.com" target="_blank" style="color:#7fd3ff">Preview the block page ↗</a></div>
      <div class="guides">
        <div class="gh">Step by step, for the filter you run</div>
        {{range .Guides}}<details class="guide" id="g-{{.ID}}"><summary><b>{{.Name}}</b><span class="gfit">{{.Fits}}</span>{{if .Redirect}}<span class="pill closed" title="The block sends the user to this instance's /blocked page.">redirects to Sluss</span>{{else}}<span class="pill" title="The product shows its own block notice; the web clip and bookmark carry the offer.">block only</span>{{end}}</summary>
          <ol>{{range .Steps}}<li>{{.}}</li>{{end}}</ol>
          {{if .Note}}<p class="gnote">{{.Note}}</p>{{end}}
          <p class="gnote"><b>Check it:</b> on a managed device open <code>chatgpt.com</code> — you should see the block{{if .Redirect}} and land on this instance's page{{end}}. Then run the coverage script from step 3: the device should report <span class="pill closed">closed</span>.</p>
        </details>{{end}}
      </div>
    </div>
  </div>

  <div class="step">
    <div class="sh"><span class="num">3</span><span class="st">Prove it, per device</span></div>
    <div class="body">A read-only check the MDM runs daily: does a provider API still answer directly from this device (a real provider answers 401; a sinkhole never does), and is Sluss reachable? It reports hostname, OS and those two answers — never a user, never content — to <code>POST {{.Base}}/router/coverage</code> with a department key, so each department's fleet shows separately. Intune can treat an open path as non-compliant; Jamf can show it as an extension attribute.
      <div class="files">
        {{range .Files}}{{if or (eq .Name "coverage.sh") (eq .Name "coverage.ps1")}}<a class="file" href="/router/rollout/file/{{.Name}}" download><span class="n">{{.Title}}</span><span class="f">{{.For}}</span><span class="m">{{.Name}} ↓</span></a>{{end}}{{end}}
      </div>
      {{if not .Durable}}<p style="color:#f4b740;margin-top:12px">Coverage reports need <code>ROUTER_DATA_DIR</code> (a data volume) to be stored.</p>{{else}}
      <div class="kpi">
        <div><div class="v">{{.Summary.Devices}}</div><div class="l">devices reported</div></div>
        <div><div class="v{{if gt .Summary.DirectOpen 0}} bad{{end}}">{{.Summary.DirectOpen}}</div><div class="l">direct AI path open</div></div>
        <div><div class="v{{if gt .Summary.SlussDown 0}} bad{{end}}">{{.Summary.SlussDown}}</div><div class="l">cannot reach Sluss</div></div>
        <div><div class="v" style="font-size:1rem;padding-top:6px">{{ago .Summary.Latest}}</div><div class="l">latest report</div></div>
      </div>
      {{if .Devices}}<div class="tbl"><table>
        <thead><tr><th>Device</th><th>OS</th><th>Department</th><th>Direct AI</th><th>Sluss</th><th>Reported</th></tr></thead>
        <tbody>{{range .Devices}}<tr><td class="mono">{{.Device}}</td><td>{{.OS}}</td><td>{{.TenantID}}</td><td><span class="pill {{.DirectAI}}">{{.DirectAI}}</span></td><td><span class="pill {{if eq .Sluss "ok"}}ok{{else}}open{{end}}">{{.Sluss}}</span></td><td>{{ago .ReportedAt}}</td></tr>{{end}}</tbody>
      </table></div>{{else}}<p style="color:#8fa1bf;margin-top:12px">No device has reported yet. Run the check on one machine to see the first row.</p>{{end}}
      {{end}}
    </div>
  </div>
</div></div></div>
</body></html>`))
