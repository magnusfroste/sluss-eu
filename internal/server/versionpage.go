package server

// The Version page (ISSUE-138): what this instance runs, whether a newer
// version exists and what it brings, and the exact upgrade step for the way
// Sluss is deployed. Linked from the version in the menu footer, which gets a
// dot when an update is waiting.

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

// activeUpdates is the process-wide checker the menu footer reads. Set once
// by New; nil when the server runs without one (tests).
var activeUpdates *UpdateChecker

type upgradePath struct {
	Key, Title string
	Steps      []string
	Code       string
}

var upgradePaths = []upgradePath{
	{"easypanel", "EasyPanel", []string{
		"Open the Sluss service in EasyPanel and press Deploy. If the deploy does not land (the footer still shows the old version), press Stop, then Deploy.",
		"With the GitHub deploy webhook configured, every merge to main deploys by itself and the workflow checks that it landed.",
	}, ""},
	{"docker", "Docker", []string{
		"Pull the new image and recreate the container with the same flags and the same /data volume. Pin a tag (sha-… or v…) in production instead of latest.",
	}, "docker pull ghcr.io/magnusfroste/sluss-eu:latest\ndocker rm -f sluss\ndocker run -d --name sluss --restart unless-stopped -v sluss-data:/data … ghcr.io/magnusfroste/sluss-eu:latest"},
	{"compose", "Docker Compose", []string{
		"From the directory with your docker-compose.yml:",
	}, "docker compose pull\ndocker compose up -d"},
	{"agenthotel", "AgentHotel", []string{
		"Open the Sluss agent in AgentHotel and press Redeploy. The hotel fetches main, rebuilds, and the footer here shows the commit it built.",
	}, ""},
}

type versionPageData struct {
	Commit, Short, Built string
	S                    UpdateStatus
	Paths                []upgradePath
	Notice               string
}

// VersionPageHandler renders /router/system.
func VersionPageHandler(u *UpdateChecker, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := u.Status()
		paths := make([]upgradePath, 0, len(upgradePaths))
		// The configured deployment first, the rest after it.
		for _, p := range upgradePaths {
			if p.Key == s.Deploy {
				paths = append(paths, p)
			}
		}
		for _, p := range upgradePaths {
			if p.Key != s.Deploy {
				paths = append(paths, p)
			}
		}
		d := versionPageData{
			Commit: buildinfo.FullCommit(), Short: buildinfo.Short(), Built: buildinfo.BuiltAt(),
			S: s, Paths: paths, Notice: r.URL.Query().Get("ok"),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := versionTmpl.Execute(w, d); err != nil && logger != nil {
			logger.Error("version page render failed", "err", err)
		}
	}
}

// VersionCheckHandler runs a check now (the "Check now" button).
func VersionCheckHandler(u *UpdateChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u != nil && !u.Cfg.Disabled {
			u.Check(r.Context())
		}
		http.Redirect(w, r, "/router/system?ok=checked", http.StatusSeeOther)
	}
}

var versionTmpl = template.Must(template.New("version").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"dsHead":   dsHeadFunc,
	"short":    shortCommit,
	"day": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02")
	},
	"clock": func(t time.Time) string {
		if t.IsZero() {
			return "not yet"
		}
		return t.UTC().Format("2006-01-02 15:04 UTC")
	},
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Version</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.mono{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.88em}
pre{background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:12px 14px;overflow-x:auto;margin:10px 0 0}
.commits{display:flex;flex-direction:column}
.commits a{display:flex;gap:12px;padding:10px 0;border-bottom:1px solid #17253f;text-decoration:none;color:#e8eef7}
.commits a:last-child{border-bottom:0}
.commits .d{color:#8fa1bf;font-size:.82rem;min-width:84px}
details.path{border-top:1px solid #17253f;padding:12px 0}
details.path summary{cursor:pointer;font-weight:600}
.path p{color:#b7c4dc;margin:8px 0 0}
</style></head>
<body><div class="tk-shell">
{{adminNav "system"}}
<div class="tk-main"><div class="ds-content">
{{dsHead "system" "Version" "What this instance runs, what is new, and how to upgrade the way you deployed it."}}
{{if .Notice}}<div class="ds-alert lvl-ok" style="margin-bottom:16px"><span class="ds-dot"></span><div><b>Checked.</b> The status below is fresh.</div></div>{{end}}

<div class="ds-grid">
  <div class="ds-card ds-stat ds-c6">
    <div class="ds-label">This instance</div>
    <div class="ds-value mono" style="font-size:1.6rem">{{.Short}}</div>
    <div class="ds-hint">built {{.Built}} · <a style="color:#7fd3ff" href="{{.S.RepoURL}}/commit/{{.Commit}}">see the commit</a></div>
  </div>
  <div class="ds-card ds-stat ds-c6">
    <div class="ds-label">Latest on main</div>
    {{if .S.Disabled}}<div class="ds-value" style="font-size:1.3rem">Check is off</div><div class="ds-hint">ROUTER_UPDATE_CHECK=off — for air-gapped installs. Compare the footer with the repository by hand.</div>
    {{else if .S.Error}}<div class="ds-value" style="font-size:1.3rem">Unknown</div><div class="ds-hint">{{.S.Error}}</div>
    {{else if .S.Available}}<div class="ds-value ds-v-warn">{{.S.Behind}} {{if eq .S.Behind 1}}change{{else}}changes{{end}} behind</div><div class="ds-hint">latest <span class="mono">{{short .S.Latest}}</span> · checked {{clock .S.Checked}}</div>
    {{else if not .S.Checked.IsZero}}<div class="ds-value ds-v-ok" style="font-size:1.6rem">Up to date</div><div class="ds-hint">checked {{clock .S.Checked}}</div>
    {{else}}<div class="ds-value" style="font-size:1.3rem">Not checked yet</div><div class="ds-hint">The first check runs shortly after start.</div>{{end}}
    {{if not .S.Disabled}}<form method="post" action="/router/system/check" style="margin-top:12px"><button class="ds-btn sm" type="submit">Check now</button></form>{{end}}
  </div>
</div>

{{if .S.Commits}}
<div class="ds-section"><h2>What is new</h2><div class="ds-sub">Newest first{{if gt .S.Behind (len .S.Commits)}} — the {{len .S.Commits}} latest of {{.S.Behind}}{{end}}.</div>
<div class="ds-card"><div class="commits">
{{range .S.Commits}}<a href="{{$.S.RepoURL}}/commit/{{.SHA}}"><span class="d">{{day .Date}}</span><span>{{.Subject}}</span></a>{{end}}
</div></div></div>
{{end}}

<div class="ds-section"><h2>How to upgrade</h2><div class="ds-sub">{{if .S.Deploy}}Your deployment ({{.S.Deploy}}) first.{{else}}Set ROUTER_DEPLOYMENT (docker, compose, easypanel or agenthotel) to show yours first.{{end}} Sluss never upgrades itself: a gateway that guards your data does not get control over its own host.</div>
<div class="ds-card">
{{range $i, $p := .Paths}}<details class="path"{{if eq $i 0}} open{{end}}><summary>{{$p.Title}}</summary>
{{range $p.Steps}}<p>{{.}}</p>{{end}}{{if $p.Code}}<pre class="mono">{{$p.Code}}</pre>{{end}}
</details>{{end}}
</div></div>

<div class="ds-foot">The check is one request to the public GitHub API every 6 hours and sends nothing about this instance. Data, keys and the audit chain live in the /data volume and survive every upgrade.</div>
</div></div></div></body></html>`))
