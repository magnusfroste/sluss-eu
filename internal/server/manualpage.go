package server

// The manual on the instance (ISSUE-133). Pages are the embedded Markdown
// from manual/, with {{BASE}} replaced by this instance's address, rendered
// with the same safe Markdown dialect as the reports. Public on purpose: it
// holds nothing but product instructions and this instance's own address,
// and an administrator needs it before they can sign in. Not indexed.

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/manual"
)

// ManualOptions configures the manual pages.
type ManualOptions struct {
	PublicURL string
	Logger    *slog.Logger
}

func (o ManualOptions) base(r *http.Request) string {
	return RolloutOptions{PublicURL: o.PublicURL}.base(r)
}

// manualSource returns a page's Markdown with the instance address filled in.
func manualSource(slug, base string) (string, bool) {
	for _, p := range manual.Pages {
		if p.Slug != slug {
			continue
		}
		b, err := fs.ReadFile(manual.Files, slug+".md")
		if err != nil {
			return "", false
		}
		return strings.ReplaceAll(string(b), "{{BASE}}", base), true
	}
	return "", false
}

type manualPageData struct {
	Title  string
	Slug   string
	Pages  []manual.Page
	Body   template.HTML
	RawURL string
}

// ManualHandler serves GET /manual and GET /manual/{page}[.md].
func ManualHandler(o ManualOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		slug := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/manual"), "/")
		raw := strings.HasSuffix(slug, ".md")
		slug = strings.TrimSuffix(slug, ".md")
		if slug == "" {
			slug = "index"
		}
		src, ok := manualSource(slug, o.base(r))
		if !ok {
			http.NotFound(w, r)
			return
		}
		if raw {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = w.Write([]byte(src))
			return
		}
		title := "Manual"
		for _, p := range manual.Pages {
			if p.Slug == slug {
				title = p.Title
			}
		}
		if first, _, found := strings.Cut(src, "\n"); found && strings.HasPrefix(first, "# ") {
			title = strings.TrimPrefix(first, "# ")
		}
		d := manualPageData{Title: title, Slug: slug, Pages: manual.Pages, Body: markdownToHTML(src), RawURL: "/manual/" + slug + ".md"}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := manualTmpl.Execute(w, d); err != nil && o.Logger != nil {
			o.Logger.Error("manual render failed", "err", err)
		}
	}
}

var manualTmpl = template.Must(template.New("manual").Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Sluss manual — {{.Title}}</title>
<style>
  *,*::before,*::after{box-sizing:border-box}
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  .shell{display:flex;min-height:100vh}
  nav{width:250px;flex-shrink:0;background:#060f22;border-right:1px solid #22324f;padding:18px 12px;position:sticky;top:0;height:100vh;overflow-y:auto}
  nav .brand{font-weight:700;padding:4px 10px 14px;display:flex;align-items:center;gap:8px}
  nav .brand .dot{width:10px;height:10px;border-radius:3px;background:#fad100}
  nav a{display:block;padding:8px 10px;border-radius:8px;color:#b7c4dc;text-decoration:none;font-size:.9rem}
  nav a:hover{background:#101c34;color:#fff}
  nav a.on{background:#182a4a;color:#fff;box-shadow:inset 3px 0 0 #fad100}
  nav .back{margin-top:14px;border-top:1px solid #17253f;padding-top:10px;font-size:.82rem}
  main{flex:1;min-width:0;padding:26px 30px 50px}
  .doc{max-width:760px;line-height:1.65;font-size:1rem}
  .doc h1{font-size:1.5rem;margin:0 0 14px}
  .doc h2{font-size:1.1rem;margin:28px 0 8px;border-bottom:1px solid #22324f;padding-bottom:6px}
  .doc h3{font-size:1rem;margin:18px 0 6px}
  .doc p{margin:10px 0;color:#d6deea}
  .doc ul,.doc ol{margin:8px 0 12px 22px;padding:0}.doc li{margin:6px 0}
  .doc .cont{margin:2px 0 6px;color:#d6deea}
  .doc a{color:#7fd3ff}
  .doc blockquote{margin:14px 0;padding:10px 14px;border-left:3px solid #fad100;background:rgba(250,209,0,.06);border-radius:0 8px 8px 0;color:#d6deea}
  .doc code{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.86em;background:#0b1528;border:1px solid #22324f;border-radius:5px;padding:1px 6px}
  .doc pre{background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:12px 14px;overflow-x:auto}
  .doc pre code{border:0;background:none;padding:0}
  .doc table{width:100%;border-collapse:collapse;margin:10px 0 16px;font-size:.92rem}
  .doc th{text-align:left;font-size:.74rem;color:#8fa1bf;text-transform:uppercase;letter-spacing:.05em;padding:8px 10px;border-bottom:1px solid #22324f}
  .doc td{padding:8px 10px;border-bottom:1px solid #17253f;vertical-align:top}
  .raw{margin-top:30px;font-size:.8rem;color:#64748b}.raw a{color:#8fa1bf}
  @media(max-width:760px){.shell{flex-direction:column}nav{width:100%;height:auto;position:static;border-right:0;border-bottom:1px solid #22324f}main{padding:18px 16px 40px}}
</style></head>
<body><div class="shell">
<nav>
  <div class="brand"><span class="dot"></span>Sluss manual</div>
  {{$cur := .Slug}}{{range .Pages}}<a href="/manual/{{if ne .Slug "index"}}{{.Slug}}{{end}}" class="{{if eq .Slug $cur}}on{{end}}">{{.Title}}</a>{{end}}
  <div class="back"><a href="/router/dashboard">← Admin console</a><a href="/chat">Company AI chat</a></div>
</nav>
<main>
  <article class="doc"><h1>{{.Title}}</h1>{{.Body}}</article>
  <div class="raw">For agents and scripts: <a href="{{.RawURL}}">this page as Markdown</a>. Source: <code>manual/</code> in the repository.</div>
</main>
</div></body></html>`))
