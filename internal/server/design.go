package server

import (
	"html/template"
	"strings"
)

// The Sluss design system (ISSUE-135). Every admin page gets these tokens and
// components through the shared shell CSS, so pages stop carrying their own
// widths, card styles and table styles. The rules a page follows:
//
//   - One page header: .ds-ph with h1, one sentence, actions on the right.
//   - One content column: .ds-content, max 1200px.
//   - Three widths only: full (.ds-c12) for tables and charts; half, third
//     or quarter (.ds-c6/.ds-c4/.ds-c3) in .ds-grid for cards; .ds-form
//     (640px) for forms.
//   - Components: .ds-card, .ds-stat, .ds-alert, .ds-badge, .ds-table inside
//     .ds-tablewrap, .ds-btn, .ds-links.
//   - What needs the admin comes first (.ds-attention), then numbers, then
//     configuration.
//
// Plain CSS, no build step: the console stays one static binary that works
// air-gapped, and the markup stays readable in a security review.

const designCSS = `
:root{
  --ds-bg:#081328;--ds-surface:#101c34;--ds-surface-2:#0b1528;--ds-line:#22324f;--ds-line-soft:#17253f;
  --ds-ink:#e8eef7;--ds-ink-2:#b7c4dc;--ds-muted:#8fa1bf;--ds-faint:#5f6e87;
  --ds-accent:#fad100;--ds-accent-ink:#081328;
  --ds-ok:#22c55e;--ds-ok-ink:#4ade80;--ds-ok-bg:#0f3a26;--ds-cloud:#3b82f6;--ds-cloud-ink:#7fb2ff;--ds-cloud-bg:#12264a;
  --ds-bad:#ef4444;--ds-bad-ink:#f87171;--ds-bad-bg:#3b1414;--ds-warn:#f4b740;--ds-warn-bg:#33290f;
  --ds-s1:4px;--ds-s2:8px;--ds-s3:12px;--ds-s4:16px;--ds-s5:24px;--ds-s6:32px;--ds-s7:48px;
  --ds-r1:6px;--ds-r2:10px;--ds-r3:14px;
  --ds-content:1200px;--ds-form:640px;
}
.ds-content{width:100%;max-width:var(--ds-content);margin:0 auto;padding:var(--ds-s6) var(--ds-s6) var(--ds-s7);box-sizing:border-box}
.ds-ph{display:flex;align-items:flex-start;gap:var(--ds-s4);margin-bottom:var(--ds-s5);flex-wrap:wrap}
.ds-ph h1{font-size:1.5rem;margin:0;line-height:1.2;color:#f8fafc;font-weight:700}
.ds-ph p{margin:var(--ds-s1) 0 0;color:var(--ds-muted);max-width:640px;font-size:.95rem}
.ds-ph .ds-actions{margin-left:auto;display:flex;gap:var(--ds-s2);flex-wrap:wrap}
.ds-btn{display:inline-flex;align-items:center;gap:var(--ds-s2);border:1px solid var(--ds-line);background:var(--ds-surface);color:var(--ds-ink);border-radius:var(--ds-r1);padding:8px 14px;font:600 .85rem/1.2 inherit;cursor:pointer;text-decoration:none;white-space:nowrap}
.ds-btn:hover{border-color:#3a527c}
.ds-btn.primary{background:var(--ds-accent);border-color:var(--ds-accent);color:var(--ds-accent-ink)}
.ds-btn.sm{padding:5px 10px;font-size:.75rem}
.ds-grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:var(--ds-s4)}
.ds-c12{grid-column:span 12}.ds-c8{grid-column:span 8}.ds-c6{grid-column:span 6}.ds-c4{grid-column:span 4}.ds-c3{grid-column:span 3}
.ds-section{margin-top:var(--ds-s6)}
.ds-section>h2,.ds-card>h2{font-size:1.05rem;margin:0 0 var(--ds-s3);color:#f1f5f9;font-weight:600}
.ds-section>.ds-sub,.ds-card>.ds-sub{color:var(--ds-muted);font-size:.85rem;margin:calc(-1 * var(--ds-s2)) 0 var(--ds-s3)}
.ds-card{background:var(--ds-surface);border:1px solid var(--ds-line);border-radius:var(--ds-r3);padding:var(--ds-s5);min-width:0}
a.ds-card{text-decoration:none;color:inherit;display:block}a.ds-card:hover{border-color:#3a527c}
.ds-stat .ds-label{font-size:.72rem;color:var(--ds-muted);text-transform:uppercase;letter-spacing:.06em}
.ds-stat .ds-value{font-size:2rem;font-weight:700;line-height:1.1;margin-top:var(--ds-s2);color:#f8fafc;font-variant-numeric:tabular-nums}
.ds-stat .ds-hint{color:var(--ds-muted);font-size:.82rem;margin-top:var(--ds-s1)}
.ds-v-ok{color:var(--ds-ok-ink)!important}.ds-v-bad{color:var(--ds-bad-ink)!important}.ds-v-cloud{color:var(--ds-cloud-ink)!important}.ds-v-warn{color:var(--ds-warn)!important}
.ds-attention{display:flex;flex-direction:column;gap:var(--ds-s2)}
.ds-alert{display:flex;align-items:center;gap:var(--ds-s3);padding:var(--ds-s3) var(--ds-s4);border-radius:var(--ds-r2);border:1px solid #7a5a1f;background:var(--ds-warn-bg);font-size:.9rem;color:var(--ds-ink-2)}
.ds-alert .ds-dot{width:8px;height:8px;border-radius:50%;background:var(--ds-warn);flex-shrink:0}
.ds-alert b{color:#fff}
.ds-alert .ds-btn{margin-left:auto}
.ds-alert.lvl-bad{border-color:#5b2330;background:var(--ds-bad-bg)}.ds-alert.lvl-bad .ds-dot{background:var(--ds-bad)}
.ds-alert.lvl-info{border-color:#24406b;background:var(--ds-cloud-bg)}.ds-alert.lvl-info .ds-dot{background:var(--ds-cloud)}
.ds-alert.lvl-ok{border-color:#1f5c38;background:var(--ds-ok-bg)}.ds-alert.lvl-ok .ds-dot{background:var(--ds-ok)}
.ds-badge{display:inline-flex;align-items:center;gap:6px;border-radius:999px;padding:2px 10px;font-size:.75rem;font-weight:600;white-space:nowrap}
.ds-badge.local{background:var(--ds-ok-bg);color:var(--ds-ok-ink)}.ds-badge.cloud{background:var(--ds-cloud-bg);color:var(--ds-cloud-ink)}
.ds-badge.stop{background:var(--ds-bad-bg);color:var(--ds-bad-ink)}.ds-badge.warn{background:var(--ds-warn-bg);color:var(--ds-warn)}.ds-badge.neutral{background:#17253f;color:var(--ds-ink-2)}
.ds-tablewrap{background:var(--ds-surface);border:1px solid var(--ds-line);border-radius:var(--ds-r3);overflow:hidden}
.ds-tablewrap>.ds-scroll{overflow-x:auto;-webkit-overflow-scrolling:touch}
table.ds-table{width:100%;border-collapse:collapse;font-size:.88rem;margin:0}
.ds-table th{text-align:left;font-size:.72rem;color:var(--ds-muted);text-transform:uppercase;letter-spacing:.05em;font-weight:600;padding:10px var(--ds-s4);border-bottom:1px solid var(--ds-line);background:transparent}
.ds-table td{padding:12px var(--ds-s4);border-bottom:1px solid var(--ds-line-soft);vertical-align:middle}
.ds-table tbody tr:last-child td{border-bottom:0}
.ds-table tr:hover td{background:#0d1a33}
.ds-table .ds-num{text-align:right;font-variant-numeric:tabular-nums}
.ds-links{display:flex;flex-direction:column;gap:var(--ds-s2)}
.ds-links a{display:flex;justify-content:space-between;align-items:center;gap:var(--ds-s3);padding:10px var(--ds-s3);border-radius:var(--ds-r2);border:1px solid var(--ds-line-soft);text-decoration:none;color:var(--ds-ink)}
.ds-links a:hover{border-color:var(--ds-line)}
.ds-links a small{color:var(--ds-muted);text-align:right}
.ds-foot{color:var(--ds-faint);font-size:.8rem;margin-top:var(--ds-s3)}
.ds-foot a{color:#7fd3ff}
.ds-form{max-width:var(--ds-form)}
@media(max-width:900px){.ds-alert{flex-wrap:wrap}.ds-alert>div{flex:1 1 220px}.ds-c8,.ds-c6,.ds-c4,.ds-c3{grid-column:span 12}.ds-content{padding:var(--ds-s5) var(--ds-s4) var(--ds-s7)}}
@media(min-width:901px) and (max-width:1180px){.ds-c3{grid-column:span 6}}
`

// dsHeadHTML renders the shared page header (ISSUE-136): title, one sentence,
// and — when the page belongs to a menu group with more than one page — the
// group's pages as tabs. Every console page uses it, so headers and tabs look
// and sit the same everywhere.
func dsHeadHTML(active, title, sub string) string {
	var b strings.Builder
	b.WriteString(`<div class="ds-top"><div class="ds-ph"><div><h1>` + template.HTMLEscapeString(title) + `</h1>`)
	if sub != "" {
		b.WriteString(`<p>` + template.HTMLEscapeString(sub) + `</p>`)
	}
	b.WriteString(`</div></div>`)
	for _, g := range adminNavGroups {
		in := false
		for _, p := range g.pages {
			in = in || p.key == active
		}
		if !in || len(g.pages) < 2 {
			continue
		}
		b.WriteString(`<nav class="ds-tabs">`)
		for _, p := range g.pages {
			cls := ""
			if p.key == active {
				cls = ` class="on"`
			}
			b.WriteString(`<a` + cls + ` href="` + p.href + `">` + template.HTMLEscapeString(p.label) + `</a>`)
		}
		b.WriteString(`</nav>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func dsHeadFunc(active, title, sub string) template.HTML {
	return template.HTML(dsHeadHTML(active, title, sub))
}

// designFrameCSS makes the existing pages sit in the design system's frame
// without rewriting each template: the content column, card radius and table
// rhythm are the same on every page. It wins over a page's own rules by
// specificity (.tk-main >), not by !important, except for padding and width,
// which pages set in many shapes.
const designFrameCSS = `
.ds-top{width:100%;max-width:var(--ds-content);margin:0 auto;padding:var(--ds-s6) var(--ds-s6) 0;box-sizing:border-box}
.ds-top .ds-ph{margin-bottom:var(--ds-s4)}
.ds-tabs{display:flex;gap:var(--ds-s1);border-bottom:1px solid var(--ds-line);margin-bottom:var(--ds-s5);overflow-x:auto}
.ds-tabs a{padding:10px var(--ds-s4);color:var(--ds-muted);text-decoration:none;font-weight:600;font-size:.92rem;border-bottom:2px solid transparent;margin-bottom:-1px;white-space:nowrap}
.ds-tabs a:hover{color:#fff}.ds-tabs a.on{color:#fff;border-color:var(--ds-accent)}
.tk-main>.wrap{width:100%!important;max-width:var(--ds-content)!important;margin:0 auto!important;padding:0 var(--ds-s6) var(--ds-s7)!important;box-sizing:border-box;display:flex;flex-direction:column;gap:var(--ds-s4)}
.tk-main>.wrap .card,.tk-main>.wrap form.addf{border-radius:var(--ds-r3);border-color:var(--ds-line);background:var(--ds-surface)}
.tk-main>.wrap form.addf{max-width:none}
.tk-main>.wrap th{font-size:.72rem;color:var(--ds-muted);text-transform:uppercase;letter-spacing:.05em;font-weight:600}
.tk-main>.wrap td{border-bottom-color:var(--ds-line-soft)}
.ds-toolbar{display:flex;align-items:center;gap:var(--ds-s3);flex-wrap:wrap}
.ds-toolbar nav,.ds-toolbar .acts{display:flex;gap:var(--ds-s2);flex-wrap:wrap}
.ds-toolbar-note{margin-left:auto;color:var(--ds-muted);font-size:.82rem}
@media(max-width:900px){.ds-top{padding:var(--ds-s5) var(--ds-s4) 0}.tk-main>.wrap{padding:0 var(--ds-s4) var(--ds-s7)!important}}
`
