package server

import (
	"html/template"
	"strings"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

// adminShellCSS styles the shared admin frame: a fixed left navigation panel
// present on every admin page (dashboard, log, chat, and more to come). Classes
// are tk- prefixed to avoid colliding with per-page styles.
const adminShellCSS = `
.tk-shell{display:flex;min-height:100vh}
.tk-nav{width:212px;flex-shrink:0;background:#080e1a;border-right:1px solid #22304d;
  display:flex;flex-direction:column;padding:14px 10px;gap:3px;position:sticky;top:0;height:100vh}
.tk-brand{display:flex;align-items:center;gap:9px;font-weight:700;color:#e8eef7;
  padding:6px 12px 16px;font-size:15px}
.tk-brand .tk-dot{width:11px;height:11px;border-radius:3px;background:#22c58b;flex-shrink:0}
.tk-link{display:flex;align-items:center;gap:11px;padding:10px 12px;border-radius:9px;
  color:#b7c4dc;text-decoration:none;font-size:14px;font-weight:500;transition:.12s}
.tk-link:hover{background:#0e1626;color:#fff}
.tk-link.active{background:#12203a;color:#fff}
.tk-link .tk-ic{width:18px;height:18px;flex-shrink:0;display:flex;align-items:center;justify-content:center;color:#7f90ad}
.tk-link:hover .tk-ic,.tk-link.active .tk-ic{color:#cfe0ff}
.tk-link .tk-ic svg{width:17px;height:17px;stroke:currentColor;fill:none;stroke-width:1.6;stroke-linecap:round;stroke-linejoin:round}
.tk-sec{color:#5f6e87;font-size:10.5px;letter-spacing:.09em;text-transform:uppercase;padding:14px 12px 6px}
.tk-foot{margin-top:auto;color:#5f6e87;font-size:11px;padding:12px 12px 4px;border-top:1px solid #16223b}
.tk-build{color:inherit;text-decoration:none;font-family:ui-monospace,Menlo,monospace}.tk-build:hover{text-decoration:underline}
.tk-main{flex:1;min-width:0;display:flex;flex-direction:column}
@media(max-width:760px){.tk-nav{width:58px;padding:12px 8px}
  .tk-lbl,.tk-brand .tk-bt,.tk-sec,.tk-foot{display:none}
  .tk-link{justify-content:center;padding:11px 0}.tk-link .tk-ic{width:auto}}
`

type navItem struct {
	key, href, icon, label string // icon is a monochrome inline SVG (currentColor)
	group                  string // sidebar section: Evidence · Control · Show
}

// Minimal single-stroke line icons — a corporate, non-emoji nav (matches the
// landing page's restrained tone).
const (
	icDashboard = `<svg viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="1"/><rect x="13" y="4" width="7" height="7" rx="1"/><rect x="4" y="13" width="7" height="7" rx="1"/><rect x="13" y="13" width="7" height="7" rx="1"/></svg>`
	icLog       = `<svg viewBox="0 0 24 24"><line x1="8" y1="6" x2="20" y2="6"/><line x1="8" y1="12" x2="20" y2="12"/><line x1="8" y1="18" x2="20" y2="18"/><circle cx="4.5" cy="6" r="1"/><circle cx="4.5" cy="12" r="1"/><circle cx="4.5" cy="18" r="1"/></svg>`
	icModels    = `<svg viewBox="0 0 24 24"><path d="M12 3 3 8l9 5 9-5z"/><path d="M3 12l9 5 9-5"/><path d="M3 16l9 5 9-5"/></svg>`
	icRisk      = `<svg viewBox="0 0 24 24"><path d="M12 3.5 L19 6.2 V11.5 C19 15.6 16 18.8 12 20.5 C8 18.8 5 15.6 5 11.5 V6.2 Z"/><path d="M9 12 L11.2 14.2 L15.2 10"/></svg>`
	icProviders = `<svg viewBox="0 0 24 24"><rect x="3.5" y="5" width="17" height="6" rx="1.5"/><rect x="3.5" y="13" width="17" height="6" rx="1.5"/><line x1="7" y1="8" x2="7" y2="8"/><line x1="7" y1="16" x2="7" y2="16"/></svg>`
	icKeys      = `<svg viewBox="0 0 24 24"><circle cx="8" cy="8" r="3.4"/><path d="M10.4 10.4 20 20"/><path d="M17 17l2.2-2.2"/><path d="M14.6 14.6l2.2-2.2"/></svg>`
	icUsers     = `<svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="3.4"/><path d="M5.5 20c0-3.6 2.9-6 6.5-6s6.5 2.4 6.5 6"/></svg>`
	icPrompts   = `<svg viewBox="0 0 24 24"><path d="M13 3 5 13h5l-1 8 8-11h-5z"/></svg>`
	icChat      = `<svg viewBox="0 0 24 24"><path d="M20 4H4v12h5v4l4-4h7z"/></svg>`
	icConnect   = `<svg viewBox="0 0 24 24"><path d="M9 15l6-6"/><path d="M11 6l1-1a3.5 3.5 0 0 1 5 5l-1 1"/><path d="M13 18l-1 1a3.5 3.5 0 0 1-5-5l1-1"/></svg>`
	icPolicy    = `<svg viewBox="0 0 24 24"><path d="M12 3l7 3v5c0 4.4-3 8-7 10-4-2-7-5.6-7-10V6z"/><path d="M9.5 12l2 2 3.5-4"/></svg>`
)

// Grouped the way the product is pitched: Evidence (what happened, provably),
// Control (what is allowed to happen), Show (connect clients, run the demo).
var adminNavItems = []navItem{
	{"dashboard", "/router/dashboard", icDashboard, "Dashboard", "Evidence"},
	{"log", "/router/log", icLog, "Request log", "Evidence"},
	{"policy", "/router/policy", icPolicy, "Policy", "Control"},
	{"models", "/router/models", icModels, "Models", "Control"},
	{"providers", "/router/providers", icProviders, "Providers", "Control"},
	{"risk", "/router/risk", icRisk, "Risk register", "Control"},
	{"keys", "/router/keys", icKeys, "Keys", "Control"},
	{"users", "/router/users", icUsers, "Users", "Control"},
	{"prompts", "/router/prompts", icPrompts, "Demo prompts", "Show"},
	{"chat", "/chat", icChat, "Live chat", "Show"},
	{"connect", "/connect", icConnect, "Connect", "Show"},
}

// adminNavHTML renders the left navigation with the given page marked active.
func adminNavHTML(active string) string {
	var b strings.Builder
	b.WriteString(`<nav class="tk-nav"><div class="tk-brand">` + brandLogoSVG + `<span class="tk-bt">Sluss</span></div>`)
	group := ""
	for _, it := range adminNavItems {
		if it.group != group {
			group = it.group
			b.WriteString(`<div class="tk-sec">` + group + `</div>`)
		}
		cls := "tk-link"
		if it.key == active {
			cls += " active"
		}
		b.WriteString(`<a class="` + cls + `" href="` + it.href + `"><span class="tk-ic">` + it.icon + `</span><span class="tk-lbl">` + it.label + `</span></a>`)
	}
	b.WriteString(`<div class="tk-foot"><a class="tk-link" href="/router/logout" style="padding:6px 12px"><span class="tk-lbl">Sign out</span></a>admin · <a class="tk-build" href="https://github.com/magnusfroste/sluss-eu/commit/` + template.HTMLEscapeString(buildinfo.FullCommit()) + `" title="Built ` + template.HTMLEscapeString(buildinfo.BuiltAt()) + `">` + template.HTMLEscapeString(buildinfo.Short()) + `</a></div></nav>`)
	return b.String()
}

// Template helpers for the html/template pages (dashboard, log, providers).
// adminCSSFunc returns template.CSS — inside a <style> element html/template
// uses a CSS context, so a template.HTML value would be escaped to ZgotmplZ.
func adminCSSFunc() template.CSS               { return template.CSS(adminShellCSS) }
func adminNavFunc(active string) template.HTML { return template.HTML(adminNavHTML(active)) }
