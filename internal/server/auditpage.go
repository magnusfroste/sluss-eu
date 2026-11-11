package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/magnusfroste/sluss/internal/audit"
)

// The Audit log page (ISSUE-172): what the log holds, the download, the
// sealed files, and one button that seals the current file. Sealing moves the
// file to the archive and continues the chain in a new one. There is no way
// to delete anything from this page, by design.

type auditPageData struct {
	Notice, Error       string
	Empty               bool
	Records             uint64
	Head                audit.Anchor
	CurFirst, CurLast   uint64
	CurSize             string
	Sealed              []sealedView
	RotateMB            string
	CanSeal             bool
	OnlyContinuedRecord bool
}

type sealedView struct {
	Name          string
	First, Last   uint64
	Size, SHA256  string
	ShortSHA256   string
	RecordsInFile uint64
}

func fmtSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

// AuditPageHandler renders the page.
func AuditPageHandler(chain *audit.ChainSink, rotateMB string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := auditPageData{Notice: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"), RotateMB: rotateMB}
		head, ok := chain.Head()
		d.Empty = !ok
		d.Head = head
		d.Records = head.Seq
		first, last, size := chain.Current()
		d.CurFirst, d.CurLast, d.CurSize = first, last, fmtSize(size)
		segs, err := chain.Segments()
		if err != nil && logger != nil {
			logger.Error("audit page: list sealed files", "err", err)
		}
		for i := len(segs) - 1; i >= 0; i-- { // newest first
			s := segs[i]
			short := s.SHA256
			if len(short) > 16 {
				short = short[:16] + "…"
			}
			d.Sealed = append(d.Sealed, sealedView{Name: s.Name, First: s.FirstSeq, Last: s.LastSeq, Size: fmtSize(s.Bytes),
				SHA256: s.SHA256, ShortSHA256: short, RecordsInFile: s.LastSeq - s.FirstSeq + 1})
		}
		// A file that holds only the record that continues the chain has
		// nothing in it worth sealing yet.
		d.OnlyContinuedRecord = ok && len(segs) > 0 && first == last
		d.CanSeal = ok && !d.OnlyContinuedRecord
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := auditPageTmpl.Execute(w, d); err != nil && logger != nil {
			logger.Error("audit page render failed", "err", err)
		}
	}
}

// AuditSealHandler is the button: it seals the current file and goes back to
// the page. The signed-in person is recorded in the seal.
func AuditSealHandler(chain *audit.ChainSink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := currentWho(r).User
		if actor == "" {
			actor = "admin"
		}
		if first, last, _ := chain.Current(); first == last && first > 1 {
			http.Redirect(w, r, "/router/audit?err="+url.QueryEscape("The current file holds only the record that continues the log. There is nothing to seal yet."), http.StatusSeeOther)
			return
		}
		seg, err := chain.Rotate(actor, "sealed from the console")
		if err != nil {
			http.Redirect(w, r, "/router/audit?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/router/audit?ok="+url.QueryEscape(fmt.Sprintf("Sealed records %d to %d as %s. The log continues in a new file.", seg.FirstSeq, seg.LastSeq, seg.Name)), http.StatusSeeOther)
	}
}

var auditPageTmpl = template.Must(template.New("audit").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"dsHead":   dsHeadFunc,
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Audit log</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.mono{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.88em;overflow-wrap:anywhere}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:.72rem;color:#8fa1bf;text-transform:uppercase;letter-spacing:.05em;font-weight:600;padding:8px 10px 8px 0}
td{padding:10px 10px 10px 0;border-top:1px solid #17253f;vertical-align:top}
.acts{display:flex;gap:10px;flex-wrap:wrap;margin-top:14px;align-items:center}
p.lead{color:#b7c4dc;margin:6px 0 0;max-width:70ch}
</style></head>
<body><div class="tk-shell">
{{adminNav "audit"}}
<div class="tk-main"><div class="ds-content">
{{dsHead "audit" "Audit log" "The tamper-evident record of every decision and every change. It can be downloaded, checked and sealed. It cannot be deleted."}}
{{if .Notice}}<div class="ds-alert lvl-ok" style="margin-bottom:16px"><span class="ds-dot"></span><div>{{.Notice}}</div></div>{{end}}
{{if .Error}}<div class="ds-alert" style="margin-bottom:16px"><span class="ds-dot"></span><div>{{.Error}}</div></div>{{end}}

<div class="ds-grid">
  <div class="ds-card ds-stat ds-c4">
    <div class="ds-label">Records</div>
    <div class="ds-value">{{if .Empty}}0{{else}}{{.Records}}{{end}}</div>
    <div class="ds-hint">{{if .Empty}}Nothing has been recorded yet.{{else}}in one chain, from record 1{{end}}</div>
  </div>
  <div class="ds-card ds-stat ds-c4">
    <div class="ds-label">File in use</div>
    <div class="ds-value" style="font-size:1.6rem">{{.CurSize}}</div>
    <div class="ds-hint">{{if .Empty}}empty{{else}}records {{.CurFirst}} to {{.CurLast}}{{end}}{{if .RotateMB}} · sealed by itself at {{.RotateMB}} MB{{else}} · never sealed by itself (ROUTER_AUDIT_ROTATE_MB is not set){{end}}</div>
  </div>
  <div class="ds-card ds-stat ds-c4">
    <div class="ds-label">Sealed files</div>
    <div class="ds-value">{{len .Sealed}}</div>
    <div class="ds-hint">kept in audit-archive in the data directory, read-only</div>
  </div>

  <div class="ds-card ds-c12">
    <div class="ds-label">Download and check</div>
    <p class="lead">The download is the whole log: every sealed file and the file in use, in order. An auditor checks it on their own computer with <span class="mono">audit-verify</span>; one changed line fails the check.</p>
    {{if not .Empty}}<p class="lead">Last record: number <span class="mono">{{.Head.Seq}}</span>, hash <span class="mono">{{.Head.EntryHash}}</span>. Kept somewhere else, this pair later proves the log was not rewritten.</p>{{end}}
    <div class="acts">
      <a class="ds-btn primary" href="/router/audit/export">Download the whole log</a>
      <a class="ds-btn" href="/manual/prove-it">How to check it</a>
    </div>
  </div>

  <div class="ds-card ds-c12">
    <div class="ds-label">Seal the file in use</div>
    <p class="lead">Sealing closes the file with a last record, moves it to the archive, read-only, and continues the log in a new file as the same chain. Nothing is deleted, and the download stays the whole log. Seal before you back up or hand over the archive, or when the file has grown large.</p>
    <form method="post" action="/router/audit/seal" class="acts" onsubmit="return confirm('Seal the file in use? It moves to the archive and the log continues in a new file. Nothing is deleted.')">
      <button class="ds-btn" type="submit"{{if not .CanSeal}} disabled{{end}}>Seal now</button>
      {{if .Empty}}<span class="ds-hint">Nothing to seal yet.</span>{{else if .OnlyContinuedRecord}}<span class="ds-hint">The file in use holds only the record that continues the log. Nothing to seal yet.</span>{{end}}
    </form>
  </div>
</div>

{{if .Sealed}}
<div class="ds-section"><h2>Sealed files</h2><div class="ds-sub">Newest first. The SHA-256 of each file is also held inside the log, in the record that follows it.</div>
<div class="ds-card"><table>
<thead><tr><th>File</th><th>Records</th><th>Size</th><th>SHA-256</th></tr></thead>
<tbody>
{{range .Sealed}}<tr><td class="mono">{{.Name}}</td><td>{{.First}} to {{.Last}}</td><td>{{.Size}}</td><td class="mono" title="{{.SHA256}}">{{.ShortSHA256}}</td></tr>
{{end}}</tbody></table></div></div>
{{end}}

<div class="ds-foot">Sealed files stay on this server's disk. Back up the audit-archive folder with the rest of the data directory. A copy kept elsewhere can be checked with audit-verify wherever it is.</div>
</div></div></div></body></html>`))
