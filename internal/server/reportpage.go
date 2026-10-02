package server

import (
	"html"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

// Reports (incident evidence, gap report, control report) are generated as
// Markdown so they can be pasted into an incident channel, attached to a
// ticket, or read by an agent over MCP. In a browser that Markdown used to
// render as raw monospace text — the pages a CISO is sent to as "evidence"
// looked the least finished (ISSUE-122). A browser now gets the same report
// as a themed console page with the key figures as cards and a download
// button; curl, agents and `?format=md` still get the Markdown unchanged.

// reportWantsHTML is the content negotiation: browsers say text/html, tools
// do not. `?format=md|json` always wins.
func reportWantsHTML(r *http.Request) bool {
	switch r.URL.Query().Get("format") {
	case "md", "markdown", "json":
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// reportStat is one key figure shown above the report body.
type reportStat struct {
	Label string
	Value string
	Sub   string
	Tone  string // "", "ok", "bad", "warn"
}

type reportPageData struct {
	Title       string
	Subtitle    string
	Stats       []reportStat
	Body        template.HTML
	DownloadURL string
	JSONURL     string
}

func renderReportPage(w http.ResponseWriter, d reportPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = reportTmpl.Execute(w, d)
}

var (
	mdBold = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdCode = regexp.MustCompile("`([^`]+)`")
)

// mdInline renders the inline subset the report writers use: bold and code.
// Input is HTML-escaped first, so report content can never inject markup.
func mdInline(s string) string {
	s = html.EscapeString(s)
	s = mdCode.ReplaceAllString(s, "<code>$1</code>")
	s = mdBold.ReplaceAllString(s, "<strong>$1</strong>")
	return s
}

// markdownToHTML converts the small, predictable Markdown dialect the report
// renderers emit (headings, bullets, blockquotes, fenced code, pipe tables,
// paragraphs) into HTML. It is deliberately not a general Markdown parser.
func markdownToHTML(md string) template.HTML {
	var b strings.Builder
	lines := strings.Split(md, "\n")
	inList, inQuote, inCode, inTable := false, false, false, false
	var para []string
	closeAll := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + mdInline(strings.Join(para, " ")) + "</p>\n")
			para = nil
		}
		if inList {
			b.WriteString("</ul>\n")
			inList = false
		}
		if inQuote {
			b.WriteString("</blockquote>\n")
			inQuote = false
		}
		if inTable {
			b.WriteString("</tbody></table>\n")
			inTable = false
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, " ")
		if strings.HasPrefix(line, "```") {
			if inCode {
				b.WriteString("</code></pre>\n")
				inCode = false
			} else {
				closeAll()
				b.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			b.WriteString(html.EscapeString(raw) + "\n")
			continue
		}
		trim := strings.TrimSpace(line)
		switch {
		case trim == "":
			closeAll()
		case strings.HasPrefix(trim, "### "):
			closeAll()
			b.WriteString("<h3>" + mdInline(trim[4:]) + "</h3>\n")
		case strings.HasPrefix(trim, "## "):
			closeAll()
			b.WriteString("<h2>" + mdInline(trim[3:]) + "</h2>\n")
		case strings.HasPrefix(trim, "# "):
			closeAll() // the page header already shows the title
		case strings.HasPrefix(trim, "|"):
			cells := strings.Split(strings.Trim(trim, "|"), "|")
			if !inTable {
				closeAll()
				b.WriteString("<table><thead><tr>")
				for _, c := range cells {
					b.WriteString("<th>" + mdInline(strings.TrimSpace(c)) + "</th>")
				}
				b.WriteString("</tr></thead><tbody>\n")
				inTable = true
				continue
			}
			if strings.Trim(strings.ReplaceAll(strings.ReplaceAll(trim, "|", ""), "-", ""), " :") == "" {
				continue // the |---|---| separator row
			}
			b.WriteString("<tr>")
			for _, c := range cells {
				b.WriteString("<td>" + mdInline(strings.TrimSpace(c)) + "</td>")
			}
			b.WriteString("</tr>\n")
		case strings.HasPrefix(trim, "- "):
			if len(para) > 0 || inQuote || inTable {
				closeAll()
			}
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			b.WriteString("<li>" + mdInline(trim[2:]) + "</li>\n")
		case strings.HasPrefix(trim, "> "):
			if len(para) > 0 || inList || inTable {
				closeAll()
			}
			if !inQuote {
				b.WriteString("<blockquote>")
				inQuote = true
			} else {
				b.WriteString(" ")
			}
			b.WriteString(mdInline(trim[2:]))
		default:
			if inList && strings.HasPrefix(line, "  ") {
				// continuation of a nested bullet ("  - sub") → flatten
				b.WriteString("<li class=\"sub\">" + mdInline(strings.TrimPrefix(trim, "- ")) + "</li>\n")
				continue
			}
			if inList || inQuote || inTable {
				closeAll()
			}
			para = append(para, trim)
		}
	}
	closeAll()
	if inCode {
		b.WriteString("</code></pre>\n")
	}
	return template.HTML(b.String())
}

var reportTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — {{.Title}}</title>
<style>
  {{adminCSS}}
  *,*::before,*::after{box-sizing:border-box}
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{display:flex;align-items:flex-start;justify-content:space-between;gap:14px;flex-wrap:wrap;padding:20px 26px;border-bottom:1px solid #22324f}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px;max-width:720px}
  .acts{display:flex;gap:8px;flex-wrap:wrap}
  .btn{display:inline-block;padding:8px 14px;border-radius:8px;font-weight:600;font-size:13px;text-decoration:none;border:1px solid #22324f;color:#e8eef7;background:#101c34}
  .btn.primary{background:#fad100;color:#081328;border-color:#fad100}
  .wrap{padding:22px 26px 40px;max-width:980px}
  .stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px;margin-bottom:22px}
  .stat{background:#101c34;border:1px solid #22324f;border-radius:10px;padding:14px 16px}
  .stat .l{font-size:.72rem;color:#8fa1bf;text-transform:uppercase;letter-spacing:.05em}
  .stat .v{font-size:1.6rem;font-weight:700;margin-top:2px}
  .stat .s{font-size:.78rem;color:#8fa1bf;margin-top:2px}
  .stat.ok .v{color:#4ade80}.stat.bad .v{color:#f87171}.stat.warn .v{color:#f4b740}
  .doc{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:6px 24px 18px;line-height:1.6;font-size:.95rem}
  .doc h2{font-size:1.05rem;margin:22px 0 8px;color:#f8fafc;border-bottom:1px solid #22324f;padding-bottom:6px}
  .doc h3{font-size:.95rem;margin:16px 0 6px;color:#cbd5e1}
  .doc p{margin:8px 0;color:#d6deea}
  .doc ul{margin:6px 0 10px 20px;padding:0}.doc li{margin:3px 0}.doc li.sub{margin-left:18px;list-style:circle}
  .doc blockquote{margin:10px 0;padding:10px 14px;border-left:3px solid #fad100;background:rgba(250,209,0,.06);color:#d6deea;border-radius:0 8px 8px 0}
  .doc code{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.86em;background:#0b1528;border:1px solid #22324f;border-radius:5px;padding:1px 6px}
  .doc pre{background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:12px 14px;overflow-x:auto}
  .doc pre code{border:0;background:none;padding:0}
  .doc table{width:100%;border-collapse:collapse;margin:10px 0 14px;font-size:.9rem}
  .doc th{text-align:left;font-size:.72rem;color:#8fa1bf;text-transform:uppercase;letter-spacing:.05em;padding:7px 10px;border-bottom:1px solid #22324f}
  .doc td{padding:7px 10px;border-bottom:1px solid #17253f;vertical-align:top}
  .foot{color:#64748b;font-size:.78rem;margin-top:14px}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "reports"}}
<div class="tk-main">
<header>
  <div><h1>{{.Title}}</h1><div class="sub">{{.Subtitle}}</div></div>
  <div class="acts"><a class="btn primary" href="{{.DownloadURL}}" download>Download as Markdown</a><a class="btn" href="{{.JSONURL}}">JSON</a></div>
</header>
<div class="wrap">
  {{if .Stats}}<div class="stats">{{range .Stats}}<div class="stat {{.Tone}}"><div class="l">{{.Label}}</div><div class="v">{{.Value}}</div>{{if .Sub}}<div class="s">{{.Sub}}</div>{{end}}</div>{{end}}</div>{{end}}
  <div class="doc">{{.Body}}</div>
  <div class="foot">Counts and classifications only — never prompt content. A control report, not a legal attestation. The Markdown download is the paste-ready version of this page.</div>
</div></div></div>
</body></html>`))
