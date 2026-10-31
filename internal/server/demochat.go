package server

import (
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// demoSessionsKey is the KV key under which the demo chat sessions blob is
// persisted in the history store.
const demoSessionsKey = "demo_sessions"

// DemoSessionsHandlers returns GET and PUT handlers that persist the demo chat
// sessions (an opaque JSON blob managed by the browser) in SQLite, so a curated
// demo survives restarts and redeploys. When no history store is configured the
// handlers are gentle no-ops and the browser keeps using localStorage.
func DemoSessionsHandlers(h *history.Store) (get, put http.HandlerFunc) {
	get = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if h == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		v, ok := h.KVGet(chatSessionsKey(r))
		if !ok {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write([]byte(v))
	}
	put = func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		if err := h.KVSet(chatSessionsKey(r), string(body)); err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	return get, put
}

// demoTenant is the synthetic tenant used by the demo chat page so a visitor can
// try the router from the browser without holding an API key. It carries all
// scopes; budget caps (keyed per tenant) don't apply to it.
var demoTenant = &tenant.Tenant{
	ID:      "tn_demo",
	Project: "prj_demo",
	KeyID:   "key_demo",
	Scopes:  auth.AllScopes(),
}

// demoTenantInjector adapts the chat page's send endpoint to the standard chat
// handler: it injects a tenant into the request context (the chat handler
// reads the tenant from context, normally set by auth.Middleware) so no bearer
// key is needed. The endpoint itself is still gated by the chat guard.
//
// A session signed in through the company IdP (ISSUE-130) carries a
// department: its traffic is attributed to that department as the tenant,
// with the person on the key id, so the log, the dashboard and the audit
// chain say who asked. Every other session (password, demo link) stays on
// the synthetic demo tenant.
func demoTenantInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := demoTenant
		if dept := DepartmentFromContext(r.Context()); dept != "" {
			t = &tenant.Tenant{
				ID: dept, Project: "chat", KeyID: "user:" + AdminUserFromContext(r.Context()),
				Scopes: auth.AllScopes(),
			}
		}
		ctx := tenant.WithTenant(r.Context(), t)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// chatSessionsKey is where a session's chat history lives: the shared demo
// blob for operators and demo guests, a private key per identified user.
func chatSessionsKey(r *http.Request) string {
	if DepartmentFromContext(r.Context()) != "" {
		return demoSessionsKey + ":" + AdminUserFromContext(r.Context())
	}
	return demoSessionsKey
}

// DemoChatPageHandler serves the chat (ISSUE-144): a modern assistant layout
// — history on the left grouped by day, a centered composer on an empty chat,
// answers rendered as Markdown with copyable code, stop and regenerate —
// where each answer still shows which model the router picked and why.
// Sessions persist server-side (SQLite) with localStorage as the fallback.
func DemoChatPageHandler(searchOn bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		role, user, dept := RoleFromContext(r.Context()), AdminUserFromContext(r.Context()), DepartmentFromContext(r.Context())
		// A demo-link guest gets the clean, nav-free page — a focused product
		// demo, not the admin console. An identified employee (ISSUE-130) gets
		// the same clean page as the company AI chat, with their name and a
		// sign-out. Admins keep the left nav to jump back.
		page, tagline, employee := demoChatHTML, "live routing demo", false
		switch role {
		case roleDemo:
			page = demoChatHTMLGuest
		case roleUser:
			page, tagline, employee = demoChatHTMLGuest, "company AI", true
		}
		ident := ""
		if dept != "" || role == roleUser {
			ident = `<div class="ident"><span class="mono">` + template.HTMLEscapeString(user) + `</span>` +
				func() string {
					if dept != "" {
						return ` · ` + template.HTMLEscapeString(dept)
					}
					return ""
				}() + ` · <a href="/router/logout">Sign out</a></div>`
		}
		flags := "const EMPLOYEE=" + strconv.FormatBool(employee) + ",SEARCH_ON=" + strconv.FormatBool(searchOn) + ";"
		_, _ = w.Write([]byte(strings.NewReplacer(
			"<!--IDENTITY-->", ident, "<!--TAGLINE-->", tagline, "/*FLAGS*/", flags,
		).Replace(page)))
	}
}

const demoChatTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Sluss — chat</title>
<!--FAVICON-->
<style>
  /*ADMIN_CSS*/
  :root{
    --bg:#081328; --surface:#0d1a31; --raised:#13213c; --side:#060f22; --line:#1f2f4d; --line2:#2a3b5c;
    --ink:#e8eef7; --ink2:#cfdaea; --muted:#8fa1bf; --faint:#64748b; --accent:#fad100;
    --ubub:#1a2a48; --code:#050c1b;
  }
  *{box-sizing:border-box}
  html,body{height:100%}
  body{margin:0;background:var(--bg);color:var(--ink);overflow:hidden;
    font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;-webkit-font-smoothing:antialiased}
  .tk-shell{height:100vh;height:100dvh;min-height:0}
  .tk-main{min-height:0}
  button{font-family:inherit}
  .sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
  svg.i{width:18px;height:18px;flex:none;stroke:currentColor;fill:none;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
  .chat{flex:1;display:flex;min-height:0;height:100%;position:relative}

  /* ---- history sidebar ---- */
  /* The panel collapses by width inside the chat area — it never slides over
     the admin menu — and is hidden from focus once closed (ISSUE-144 fix). */
  .chat{overflow:hidden}
  .side{width:260px;flex:none;background:var(--side);border-right:1px solid var(--line);min-height:0;overflow:hidden;
    transition:width .2s ease,border-color .2s ease,visibility 0s linear 0s}
  .side-in{width:260px;height:100%;display:flex;flex-direction:column}
  .chat.side-off .side{width:0;border-right-width:0;visibility:hidden;transition:width .2s ease,border-color .2s ease,visibility 0s linear .2s}
  @media (prefers-reduced-motion: reduce){.side{transition:none!important}}
  .side-top{display:flex;gap:6px;padding:12px 10px 8px}
  .newbtn{flex:1;display:flex;align-items:center;gap:9px;padding:9px 12px;border-radius:10px;border:1px solid var(--line2);
    background:transparent;color:var(--ink);font-size:14px;font-weight:500;cursor:pointer}
  .newbtn:hover{background:var(--raised)}
  .iconbtn{display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:9px;border:0;
    background:transparent;color:var(--muted);cursor:pointer;flex:none}
  .iconbtn:hover{background:var(--raised);color:var(--ink)}
  .ssearch{padding:0 10px 8px}
  .ssearch input{width:100%;background:var(--surface);border:1px solid var(--line);border-radius:9px;color:var(--ink);padding:8px 11px;font-size:13px;outline:none;font-family:inherit}
  .ssearch input:focus{border-color:var(--line2)}
  #sessions{flex:1;overflow-y:auto;padding:0 8px 12px}
  .sgroup{font-size:11.5px;color:var(--faint);font-weight:600;padding:14px 10px 6px}
  .srow{display:flex;align-items:center;gap:4px;border-radius:9px;color:var(--ink2);font-size:13.5px;position:relative}
  .srow a{flex:1;min-width:0;padding:8px 10px;color:inherit;text-decoration:none;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;border-radius:9px}
  .srow:hover{background:var(--surface)}
  .srow.active{background:var(--raised);color:var(--ink)}
  .srow .del{opacity:0;border:0;background:transparent;color:var(--muted);cursor:pointer;border-radius:7px;width:28px;height:28px;flex:none;display:inline-flex;align-items:center;justify-content:center;margin-right:4px}
  .srow:hover .del,.srow.active .del,.srow .del:focus{opacity:1}
  .srow .del:hover{color:#f07ab0;background:#2a1424}
  .sempty{padding:14px 12px;color:var(--faint);font-size:13px;line-height:1.5}
  .side-foot{border-top:1px solid var(--line);padding:11px 14px;font-size:11.5px;color:var(--faint);line-height:1.45}
  .scrim{display:none}

  /* ---- main column ---- */
  .col{flex:1;display:flex;flex-direction:column;min-width:0;min-height:0;position:relative}
  .top{display:flex;align-items:center;gap:8px;padding:10px 14px;min-height:56px}
  .brand{display:flex;align-items:center;gap:8px;font-weight:600;font-size:15px;white-space:nowrap}
  .brand small{color:var(--faint);font-weight:400;font-size:12.5px}
  #sideopen{display:none}
  .chat.side-off #sideopen{display:inline-flex}
  .modelpick{display:flex;align-items:center;margin-left:6px}
  .modelpick label{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}
  .modelpick select{appearance:none;-webkit-appearance:none;background:transparent url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%238fa1bf' stroke-width='2.4' stroke-linecap='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E") no-repeat right 10px center;
    border:1px solid transparent;color:var(--ink);border-radius:10px;padding:7px 30px 7px 11px;font-size:14px;font-weight:500;max-width:230px;outline:none;cursor:pointer;font-family:inherit}
  .modelpick select:hover,.modelpick select:focus{background-color:var(--raised);border-color:var(--line)}
  .modelpick select option{background:#0d1a31;color:var(--ink)}
  .ident{margin-left:auto;font-size:12.5px;color:var(--muted);display:flex;gap:4px;align-items:center;white-space:nowrap}
  .ident a{color:#7fd3ff;text-decoration:none}.ident .mono{font-family:ui-monospace,Menlo,monospace;color:var(--ink2)}
  main{flex:1;overflow-y:auto;min-height:0;scroll-behavior:auto}
  .wrap{max-width:780px;margin:0 auto;padding:12px 20px 36px;display:flex;flex-direction:column;gap:30px}

  /* empty state: greeting + centered composer */
  .hello{display:none;text-align:center;padding:0 20px;margin-bottom:22px}
  .hello h1{font-size:1.9rem;font-weight:600;margin:0;letter-spacing:-.02em}
  .hello p{color:var(--muted);font-size:14.5px;line-height:1.55;max-width:560px;margin:10px auto 0}
  .chips{display:none;flex-wrap:wrap;gap:8px;justify-content:center;max-width:780px;margin:14px auto 0;padding:0 20px}
  .chip{border:1px solid var(--line);background:transparent;color:var(--ink2);border-radius:999px;padding:8px 14px;font-size:13.5px;cursor:pointer}
  .chip:hover{background:var(--raised);color:var(--ink)}
  .chip .t{color:var(--faint);font-size:11.5px;margin-left:6px}
  .chat.is-empty main{flex:0 0 auto}
  .chat.is-empty .col{justify-content:center}
  .chat.is-empty .hello{display:block}
  .chat.is-empty .chips{display:flex}
  .chat.is-empty .top{position:absolute;top:0;left:0;right:0}
  .chat.is-empty .dock{padding-bottom:0}
  .chat.is-empty .hint{display:none}

  /* ---- messages ---- */
  .msg.user{display:flex;flex-direction:column;align-items:flex-end;gap:6px}
  .ubub{background:var(--ubub);padding:11px 16px;border-radius:20px;border-bottom-right-radius:6px;max-width:80%;line-height:1.55;white-space:pre-wrap;word-wrap:break-word;font-size:15px}
  .fmeta{font-size:12px;color:var(--muted)}
  .msg.bot{display:flex;gap:14px;align-items:flex-start}
  .av{width:30px;height:30px;border-radius:50%;border:1px solid var(--line2);display:flex;align-items:center;justify-content:center;flex:none;margin-top:1px;background:var(--surface)}
  .av svg{width:16px;height:16px}
  .mbody{flex:1;min-width:0;display:flex;flex-direction:column;gap:8px}
  .route{display:flex;flex-wrap:wrap;align-items:center;gap:6px;font-size:12px;min-height:22px}
  .pill{display:inline-flex;align-items:center;gap:6px;padding:2px 9px;border-radius:999px;font-weight:600;font-size:11.5px;font-family:ui-monospace,Menlo,monospace;line-height:1.6}
  .pill.task{background:#16233c;color:#9db4dc}
  .pill.cheap{background:#13351f;color:#4fd08a}.pill.balanced{background:#33290f;color:#f4b740}.pill.premium{background:#361529;color:#f07ab0}
  .pill.pinned{background:#2a2140;color:#c4a7f7}.pill.fallback{background:#33290f;color:#f4b740;cursor:help}
  .pill.blocked{background:#3b1414;color:#f87171}
  .pill.search{background:#12264a;color:#7fb2ff}.pill.search.skip{background:#33290f;color:#f4b740}
  .switch{display:flex;align-items:flex-start;gap:9px;padding:10px 13px;border-radius:12px;font-size:13px;line-height:1.45;background:#0f2f22;border:1px solid #1f7a4d;color:#a7f3d0}
  .switch.cloud{background:#33270e;border-color:#7a5a1f;color:#f4d38a}
  .switch b{color:#fff}
  .blockcard{border:1px solid #5b2330;background:rgba(239,68,68,.06);border-radius:14px;padding:13px 16px;line-height:1.55}
  .blockcard b{color:#fca5a5}
  .blockcard .bnote{margin-top:8px;font-size:12.5px;color:var(--muted)}
  .errcard{border:1px solid #5b2330;border-radius:14px;padding:12px 15px;color:#f8b4bc;line-height:1.5}
  .mono{font-family:ui-monospace,Menlo,monospace;font-size:.88em}
  .think{color:var(--muted);font-size:14.5px;background:linear-gradient(90deg,var(--muted) 0%,#e8eef7 50%,var(--muted) 100%);background-size:200% 100%;
    -webkit-background-clip:text;background-clip:text;color:transparent;animation:shine 1.6s linear infinite}
  @keyframes shine{from{background-position:100% 0}to{background-position:-100% 0}}
  .dots{display:inline-flex;gap:4px;padding:6px 0}
  .dots i{width:7px;height:7px;border-radius:50%;background:var(--muted);animation:bop 1.2s infinite ease-in-out}
  .dots i:nth-child(2){animation-delay:.15s}.dots i:nth-child(3){animation-delay:.3s}
  @keyframes bop{0%,80%,100%{opacity:.25;transform:translateY(0)}40%{opacity:1;transform:translateY(-3px)}}
  @media (prefers-reduced-motion: reduce){.think,.dots i{animation:none}.think{color:var(--muted);background:none}}
  .sources{font-size:12.5px;color:var(--muted);line-height:1.7;display:flex;flex-wrap:wrap;gap:6px}
  .sources a{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--line);border-radius:999px;padding:3px 10px;color:var(--ink2);text-decoration:none;max-width:260px}
  .sources a span{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .sources a b{color:var(--faint);font-weight:600}
  .sources a:hover{border-color:var(--line2);color:var(--ink)}
  .acts{display:flex;gap:2px;margin-left:-8px;opacity:.85}
  .acts button{display:inline-flex;align-items:center;gap:6px;border:0;background:transparent;color:var(--muted);border-radius:8px;padding:6px 8px;font-size:12.5px;cursor:pointer}
  .acts button:hover{background:var(--raised);color:var(--ink)}
  .acts svg.i{width:16px;height:16px}

  /* ---- answer text (markdown) ---- */
  .md{font-size:15.5px;line-height:1.7;color:var(--ink);word-wrap:break-word;overflow-wrap:anywhere}
  .md>*:first-child{margin-top:0}.md>*:last-child{margin-bottom:0}
  .md p{margin:0 0 .9em}
  .md h3,.md h4,.md h5,.md h6{margin:1.3em 0 .5em;line-height:1.35;font-weight:600}
  .md h3{font-size:1.25em}.md h4{font-size:1.1em}.md h5,.md h6{font-size:1em}
  .md ul,.md ol{margin:0 0 .9em;padding-left:1.5em}
  .md li{margin:.25em 0}.md li>ul,.md li>ol{margin:.25em 0 0}
  .md a{color:#7fd3ff}
  .md strong{color:#fff;font-weight:600}
  .md blockquote{margin:0 0 .9em;padding:2px 0 2px 14px;border-left:3px solid var(--line2);color:var(--ink2)}
  .md hr{border:0;border-top:1px solid var(--line);margin:1.3em 0}
  .md code{font-family:ui-monospace,"IBM Plex Mono",Menlo,monospace;font-size:.86em;background:var(--raised);border-radius:5px;padding:.12em .38em}
  .md .code{margin:0 0 1em;border:1px solid var(--line);border-radius:12px;overflow:hidden;background:var(--code)}
  .md .code-h{display:flex;align-items:center;justify-content:space-between;padding:6px 8px 6px 14px;background:#0b1528;color:var(--muted);font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .md .code-h button{display:inline-flex;align-items:center;gap:5px;border:0;background:transparent;color:var(--muted);font-size:12px;cursor:pointer;border-radius:6px;padding:4px 7px}
  .md .code-h button:hover{color:var(--ink);background:var(--raised)}
  .md pre{margin:0;padding:14px 16px;overflow-x:auto;font-size:13px;line-height:1.55}
  .md pre code{background:none;padding:0;font-size:inherit;white-space:pre}
  .md .tbl{overflow-x:auto;margin:0 0 1em;border:1px solid var(--line);border-radius:10px}
  .md table{border-collapse:collapse;width:100%;font-size:14px}
  .md th,.md td{padding:8px 12px;border-bottom:1px solid var(--line);text-align:left;vertical-align:top}
  .md th{color:var(--muted);font-weight:600;font-size:12.5px;background:#0b1528}
  .md tr:last-child td{border-bottom:0}
  .caret{display:inline-block;width:8px;height:8px;border-radius:50%;background:var(--ink);vertical-align:middle;margin-left:4px;animation:bop 1s infinite}

  /* ---- composer ---- */
  .dock{padding:0 20px 14px}
  .tobottom{position:absolute;left:50%;transform:translateX(-50%);bottom:150px;width:36px;height:36px;border-radius:50%;border:1px solid var(--line2);background:var(--surface);color:var(--ink);cursor:pointer;display:flex;align-items:center;justify-content:center;box-shadow:0 6px 18px rgba(0,0,0,.35)}
  .tobottom[hidden]{display:none}
  .composer{max-width:780px;margin:0 auto;background:var(--surface);border:1px solid var(--line2);border-radius:26px;padding:10px 10px 8px 16px;box-shadow:0 8px 30px rgba(0,0,0,.25);transition:border-color .15s}
  .composer:focus-within{border-color:#3d5380}
  .composer textarea{display:block;width:100%;resize:none;border:0;background:transparent;color:var(--ink);font-size:15.5px;line-height:1.5;outline:none;font-family:inherit;max-height:220px;padding:6px 4px 4px 0;min-height:32px}
  .composer textarea::placeholder{color:var(--faint)}
  .crow{display:flex;align-items:center;gap:4px;margin-left:-8px}
  .tool{display:inline-flex;align-items:center;gap:6px;height:36px;padding:0 10px;border-radius:999px;border:1px solid transparent;background:transparent;color:var(--muted);cursor:pointer;font-size:13.5px}
  .tool:hover{background:var(--raised);color:var(--ink)}
  .tool[aria-pressed="true"]{color:#7fb2ff;background:#12264a;border-color:#244a84}
  .tool:disabled{opacity:.4;cursor:not-allowed}
  .crow .spacer{flex:1}
  .sendbtn{width:36px;height:36px;border-radius:50%;border:0;background:var(--accent);color:#081328;cursor:pointer;display:inline-flex;align-items:center;justify-content:center;flex:none}
  .sendbtn svg.i{stroke-width:2.4}
  .sendbtn:disabled{background:#2a3b5c;color:#64748b;cursor:not-allowed}
  .sendbtn.stop{background:var(--ink);color:#081328}
  .attached{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 6px -4px}
  .attached:empty{display:none}
  .fchip{display:inline-flex;align-items:center;gap:7px;background:var(--raised);border:1px solid var(--line);border-radius:10px;padding:6px 8px 6px 10px;font-size:12.5px;max-width:240px}
  .fchip span{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .fchip button{border:0;background:transparent;color:var(--muted);cursor:pointer;font-size:15px;line-height:1;padding:0 2px}
  .fchip button:hover{color:#f07ab0}
  .hint{max-width:780px;margin:8px auto 0;color:var(--faint);font-size:11.5px;text-align:center;line-height:1.45}
  .chat.dragging .composer{border-color:var(--accent);border-style:dashed}

  @media (max-width:900px){
    /* Narrow: a drawer over the chat, whatever the desktop choice was. */
    .side,.chat.side-off .side{position:absolute;top:0;bottom:0;left:0;z-index:30;width:280px;border-right:1px solid var(--line);
      transform:translateX(-100%);visibility:hidden;transition:transform .2s ease,visibility 0s linear .2s}
    .side-in{width:280px}
    .chat.side-open .side{transform:none;visibility:visible;box-shadow:12px 0 40px rgba(0,0,0,.45);transition:transform .2s ease,visibility 0s}
    .chat.side-open .scrim{display:block;position:absolute;inset:0;background:rgba(3,8,18,.55);z-index:25}
    #sideopen{display:inline-flex}
  }
  @media (max-width:760px){
    .tk-nav{display:none}
    .top{padding:8px 10px;gap:4px;flex-wrap:wrap}
    .brand small{display:none}
    .modelpick select{max-width:190px;font-size:13.5px}
    .ident{margin-left:0;width:100%;order:9;font-size:12px}
    .wrap{padding:8px 14px 28px;gap:24px}
    .ubub{max-width:88%}
    .msg.bot{gap:10px}.av{display:none}
    .dock{padding:0 10px 10px;padding-bottom:max(10px,env(safe-area-inset-bottom))}
    .hello h1{font-size:1.5rem}
    .hint{display:none}
    .tool .lbl{display:none}
  }
</style>
</head>
<body>
<div class="tk-shell">
<!--ADMIN_NAV-->
<div class="tk-main">
<div class="chat is-empty" id="chat">
  <aside class="side" id="side" aria-label="Chat history">
   <div class="side-in">
    <div class="side-top">
      <button class="newbtn" id="newchat" type="button"><svg class="i" viewBox="0 0 24 24"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>New chat</button>
      <button class="iconbtn" id="sideclose" type="button" aria-label="Hide chat history" title="Hide chat history"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/></svg></button>
    </div>
    <div class="ssearch"><input id="ssearch" type="search" placeholder="Search chats" aria-label="Search chats" autocomplete="off"></div>
    <nav id="sessions"></nav>
    <div class="side-foot">Chats are kept for you on this server. Delete one with ×.</div>
   </div>
  </aside>
  <div class="scrim" id="scrim"></div>
  <div class="col">
    <header class="top">
      <button class="iconbtn" id="sideopen" type="button" aria-label="Show chat history" title="Chat history"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/></svg></button>
      <div class="brand"><!--LOGO-->Sluss<small><!--TAGLINE--></small></div>
      <!--MODEL_PICKER-->
      <!--IDENTITY-->
    </header>
    <div class="hello" id="hello"><h1>What can I help with?</h1><p id="hellosub"></p></div>
    <main id="main"><div class="wrap" id="wrap" aria-live="polite"></div></main>
    <button class="tobottom" id="tobottom" type="button" aria-label="Scroll to the latest message" hidden><svg class="i" viewBox="0 0 24 24"><path d="M12 5v14"/><path d="m19 12-7 7-7-7"/></svg></button>
    <footer class="dock">
      <form class="composer" id="form">
        <div class="attached" id="attached"></div>
        <label for="input" class="sr">Message</label>
        <textarea id="input" rows="1" placeholder="Ask anything" autocomplete="off" autofocus></textarea>
        <div class="crow">
          <button type="button" class="tool" id="attachbtn" title="Attach a text file (txt, md, csv, json, code) or an image. Read in your browser and classified by the gateway before any model sees it."><svg class="i" viewBox="0 0 24 24"><path d="m21.4 11.6-8.5 8.5a5.5 5.5 0 0 1-7.8-7.8l8.5-8.5a3.7 3.7 0 0 1 5.2 5.2l-8.5 8.5a1.8 1.8 0 0 1-2.6-2.6l7.8-7.8"/></svg><span class="lbl">Attach</span></button>
          <input type="file" id="filein" multiple hidden>
          <button type="button" class="tool" id="searchbtn" aria-pressed="false" title="Search the web through the company search engine before answering. Skipped automatically when the prompt contains personal or confidential data."><svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a14 14 0 0 1 0 18a14 14 0 0 1 0-18"/></svg><span class="lbl">Search</span></button>
          <span class="spacer"></span>
          <button id="send" class="sendbtn" type="submit" aria-label="Send" title="Send (Enter)" disabled><svg class="i" viewBox="0 0 24 24"><path d="M12 19V5"/><path d="m5 12 7-7 7 7"/></svg></button>
        </div>
      </form>
      <div class="chips" id="chips"></div>
      <div class="hint" id="hint">Model selection happens locally in &lt;0.05&nbsp;ms — the same engine a real client uses. Auto picks the cheapest capable model per prompt (cost &amp; CO₂ saver).</div>
    </footer>
  </div>
</div>
</div>
</div>
<template id="logo"><!--LOGO--></template>
<script>
// Quick prompts are admin-managed (DB-backed) and loaded from the server; this
// built-in list is only a fallback if the fetch fails.
let QUICK=[
  {label:"Write a git commit message",tier:"cheap",text:"write a concise git commit message for a bugfix"},
  {label:"Debug a Go deadlock",tier:"hard",text:"debug this race condition deadlock in my concurrent Go code, goroutines hang on a mutex"},
  {label:"Security-review a login",tier:"blocked",text:"security review this login form for XSS and secret leakage"},
];
/*FLAGS*/
const LS='tok_demo_sessions_v1', LS_SIDE='sluss_chat_side';
const IC={
  copy:'<svg class="i" viewBox="0 0 24 24"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg>',
  check:'<svg class="i" viewBox="0 0 24 24"><path d="m5 12 5 5L20 7"/></svg>',
  redo:'<svg class="i" viewBox="0 0 24 24"><path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/></svg>',
  x:'<svg class="i" viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>',
  send:'<svg class="i" viewBox="0 0 24 24"><path d="M12 19V5"/><path d="m5 12 7-7 7 7"/></svg>',
  stop:'<svg class="i" viewBox="0 0 24 24" style="fill:currentColor;stroke:none"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>',
  file:'<svg class="i" viewBox="0 0 24 24"><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z"/><path d="M14 3v5h5"/></svg>',
  img:'<svg class="i" viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-5-5L5 21"/></svg>',
};
const LOGO=document.getElementById('logo').innerHTML;
const chatEl=document.getElementById('chat'), mainEl=document.getElementById('main'), wrap=document.getElementById('wrap'), sideEl=document.getElementById('sessions');
const form=document.getElementById('form'), input=document.getElementById('input'), send=document.getElementById('send');
const toBottom=document.getElementById('tobottom'), chipsEl=document.getElementById('chips');
let sessions=[], currentId=null, streaming=null; // streaming: AbortController while an answer streams

// Attachments (ISSUE-129): read in the browser, sent as array-form content so
// the gateway classifies the document text (or flags an image as an
// attachment it cannot read) before any model sees it. Nothing is uploaded
// anywhere else.
const TEXT_EXT=/\.(txt|md|markdown|csv|tsv|json|ya?ml|xml|log|go|py|js|ts|tsx|jsx|java|rs|c|h|cpp|cs|rb|php|sql|html?|css|sh|ps1|ini|toml|env|cfg)$/i;
const TEXT_MAX=300*1024, IMG_MAX=2*1024*1024;
let attached=[]; // {name, kind:'text'|'image', text?, data?}
const attachedEl=document.getElementById('attached'), fileIn=document.getElementById('filein');
const searchBtn=document.getElementById('searchbtn');
let searchOn=false;
if(!SEARCH_ON){searchBtn.disabled=true;searchBtn.title='Web search is not configured on this instance (ROUTER_SEARCH_URL).';}
searchBtn.onclick=()=>{searchOn=!searchOn;searchBtn.setAttribute('aria-pressed',searchOn?'true':'false');input.focus();};
document.getElementById('attachbtn').onclick=()=>fileIn.click();
async function addFiles(files){
  for(const f of files){
    if(f.type.startsWith('image/')){
      if(f.size>IMG_MAX){alert(f.name+': images up to 2 MB.');continue;}
      attached.push({name:f.name,kind:'image',data:await readAs(f,'dataurl')});
    }else if(TEXT_EXT.test(f.name)||f.type.startsWith('text/')){
      if(f.size>TEXT_MAX){alert(f.name+': text files up to 300 KB.');continue;}
      attached.push({name:f.name,kind:'text',text:await readAs(f,'text')});
    }else{alert(f.name+': PDF and Office files are not read yet — paste the text for now.');}
  }
  renderAttached();
}
fileIn.onchange=async()=>{await addFiles(fileIn.files);fileIn.value='';};
// Drop files anywhere on the chat.
chatEl.addEventListener('dragover',e=>{if(e.dataTransfer&&[].indexOf.call(e.dataTransfer.types,'Files')>=0){e.preventDefault();chatEl.classList.add('dragging');}});
chatEl.addEventListener('dragleave',e=>{if(e.target===chatEl||!chatEl.contains(e.relatedTarget))chatEl.classList.remove('dragging');});
chatEl.addEventListener('drop',e=>{if(!e.dataTransfer||!e.dataTransfer.files.length)return;e.preventDefault();chatEl.classList.remove('dragging');addFiles(e.dataTransfer.files);});
function readAs(f,how){return new Promise((ok,no)=>{const r=new FileReader();r.onload=()=>ok(r.result);r.onerror=no;how==='text'?r.readAsText(f):r.readAsDataURL(f);});}
function renderAttached(){
  attachedEl.innerHTML='';
  attached.forEach((a,i)=>{const c=el('fchip',(a.kind==='image'?IC.img:IC.file)+'<span>'+esc(a.name)+'</span>');
    const x=document.createElement('button');x.type='button';x.textContent='×';x.setAttribute('aria-label','Remove '+a.name);
    x.onclick=()=>{attached.splice(i,1);renderAttached();};c.appendChild(x);attachedEl.appendChild(c);});
  syncSend();
}
function partsFor(text){
  const parts=[{type:'text',text:text}];
  attached.forEach(a=>{if(a.kind==='text')parts.push({type:'text',text:'--- Attached file: '+a.name+' ---\n'+a.text});else parts.push({type:'image_url',image_url:{url:a.data}});});
  return parts;
}

// Model picker (ISSUE-098, admin control panel only — absent on the guest page).
// "Auto (router)" is the demo mode: the classifier picks the cheapest capable
// model (the cost & CO₂ saver). Pinning a model bypasses classification to
// verify that model end-to-end — policy is still enforced, so a pinned cloud
// model + sensitive data is still blocked (and that block is itself the demo).
const modelSel=document.getElementById('modelsel'), hintEl=document.getElementById('hint');
const HINT_AUTO='Model selection happens locally in <0.05 ms — the same engine a real client uses. Auto picks the cheapest capable model per prompt (cost & CO₂ saver).';
const HINT_PIN='Pinned: classification is bypassed — policy is still enforced (a sensitive prompt to a cloud model is still blocked).';
function pickedModel(){return modelSel&&modelSel.value?modelSel.value:'auto'}
async function loadModels(){
  if(!modelSel)return;
  try{
    const r=await fetch('/chat/models',{credentials:'same-origin'});
    if(!r.ok)return;
    const ms=await r.json();
    ms.forEach(m=>{
      const o=document.createElement('option');o.value=m.id;
      let label=m.id+' — '+m.tier;
      if(m.egress==='local')label+=' · local';
      if(m.out_usd_per_mtok>0)label+=' · $'+m.out_usd_per_mtok.toFixed(2)+'/Mtok out';
      o.textContent=label;modelSel.appendChild(o);
    });
    modelSel.onchange=()=>{if(hintEl)hintEl.textContent=pickedModel()==='auto'?HINT_AUTO:HINT_PIN};
  }catch(e){}
}

function persist(){
  try{localStorage.setItem(LS,JSON.stringify(sessions));}catch(e){}
  // Best-effort server persistence (SQLite) so chats survive redeploys.
  fetch('/chat/sessions',{method:'PUT',credentials:'same-origin',
    headers:{'Content-Type':'application/json'},body:JSON.stringify(sessions)}).catch(()=>{});
}
function cur(){return sessions.find(s=>s.id===currentId)}
function uid(){return 's'+Date.now()+Math.floor(Math.random()*1000)}
function el(cls,html){const d=document.createElement('div');d.className=cls;if(html!==undefined)d.innerHTML=html;return d;}
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
const escapeHtml=esc;
function tierClass(m){if(/premium/.test(m))return'premium';if(/balanced/.test(m))return'balanced';return'cheap'}

// ---- Markdown (answers only): a small, escaping renderer. Every piece of
// model text is HTML-escaped before any tag is added, links must be http(s),
// and nothing is evaluated — model output can never inject markup or script.
function inl(raw){
  return raw.split(/(\x60[^\x60\n]+\x60)/).map((seg,i)=>{
    if(i%2)return '<code>'+esc(seg.slice(1,-1))+'</code>';
    let s=esc(seg);
    s=s.replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)]+)\)/g,'<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
    s=s.replace(/\*\*([^*\n]+)\*\*/g,'<strong>$1</strong>').replace(/__([^_\n]+)__/g,'<strong>$1</strong>');
    s=s.replace(/(^|[^*\w])\*([^*\n]+)\*(?!\*)/g,'$1<em>$2</em>');
    s=s.replace(/~~([^~\n]+)~~/g,'<del>$1</del>');
    return s;
  }).join('');
}
const RE_FENCE=/^\s*(\x60{3}|~{3})\s*([\w+#.-]*)\s*$/, RE_H=/^(#{1,6})\s+(.*)$/, RE_HR=/^\s*([-*_])(\s*\1){2,}\s*$/,
  RE_Q=/^\s*>\s?/, RE_LI=/^(\s*)([-*+]|\d+[.)])\s+(.*)$/, RE_SEP=/^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/;
function isBlockStart(l,next){return RE_FENCE.test(l)||RE_H.test(l)||RE_HR.test(l)||RE_Q.test(l)||RE_LI.test(l)||(l.indexOf('|')>=0&&next!==undefined&&RE_SEP.test(next));}
function cells(l){l=l.trim();if(l.startsWith('|'))l=l.slice(1);if(l.endsWith('|'))l=l.slice(0,-1);return l.split('|').map(c=>c.trim());}
function codeBlock(lang,code){
  return '<div class="code"><div class="code-h"><span>'+esc(lang||'code')+'</span><button type="button" class="copyc" aria-label="Copy code">'+IC.copy+'<span>Copy</span></button></div><pre><code>'+esc(code)+'</code></pre></div>';
}
function md(src){
  const L=String(src||'').replace(/\r/g,'').split('\n'), out=[];let i=0;
  while(i<L.length){
    const l=L[i];
    let m=l.match(RE_FENCE);
    if(m){const fence=m[1],buf=[];i++;while(i<L.length&&L[i].trim()!==fence){buf.push(L[i]);i++;}i++;out.push(codeBlock(m[2],buf.join('\n')));continue;}
    if(!l.trim()){i++;continue;}
    if((m=l.match(RE_H))){const n=Math.min(6,m[1].length+2);out.push('<h'+n+'>'+inl(m[2].replace(/\s+#+\s*$/,''))+'</h'+n+'>');i++;continue;}
    if(RE_HR.test(l)){out.push('<hr>');i++;continue;}
    if(RE_Q.test(l)){const buf=[];while(i<L.length&&RE_Q.test(L[i])){buf.push(L[i].replace(RE_Q,''));i++;}out.push('<blockquote>'+md(buf.join('\n'))+'</blockquote>');continue;}
    if(l.indexOf('|')>=0&&i+1<L.length&&RE_SEP.test(L[i+1])){
      const head=cells(l);i+=2;const rows=[];
      while(i<L.length&&L[i].indexOf('|')>=0&&L[i].trim()){rows.push(cells(L[i]));i++;}
      out.push('<div class="tbl"><table><thead><tr>'+head.map(c=>'<th>'+inl(c)+'</th>').join('')+'</tr></thead><tbody>'+
        rows.map(r=>'<tr>'+head.map((_,k)=>'<td>'+inl(r[k]||'')+'</td>').join('')+'</tr>').join('')+'</tbody></table></div>');continue;}
    if((m=l.match(RE_LI))){
      const base=m[1].length, ordered=/\d/.test(m[2]), start=ordered?parseInt(m[2],10):1, items=[];
      while(i<L.length){
        const lm=L[i].match(RE_LI);
        if(lm&&lm[1].length<=base+1&&(/\d/.test(lm[2])===ordered)){items.push({first:lm[3],rest:[]});i++;continue;}
        if(items.length&&(L[i].trim()===''?(i+1<L.length&&/^\s{2,}\S/.test(L[i+1])):/^\s{2,}\S/.test(L[i])||(!isBlockStart(L[i],L[i+1])&&L[i].trim()!==''&&!lm))){
          if(L[i].trim()==='')items[items.length-1].rest.push('');else items[items.length-1].rest.push(L[i].replace(new RegExp('^\\s{0,'+(base+4)+'}'),''));i++;continue;}
        break;
      }
      const tag=ordered?'ol':'ul';
      out.push('<'+tag+(ordered&&start!==1?' start="'+start+'"':'')+'>'+items.map(it=>{
        const rest=it.rest.join('\n');
        return '<li>'+inl(it.first)+(rest.trim()?(/^\s*([-*+]|\d+[.)])\s/m.test(rest)||RE_FENCE.test(rest.split('\n')[0])?md(rest):'<br>'+inl(rest.trim()).replace(/\n/g,'<br>')):'')+'</li>';}).join('')+'</'+tag+'>');
      continue;
    }
    const buf=[l];i++;
    while(i<L.length&&L[i].trim()&&!isBlockStart(L[i],L[i+1])){buf.push(L[i]);i++;}
    out.push('<p>'+buf.map(inl).join('<br>')+'</p>');
  }
  return out.join('');
}

// ---- scrolling: follow the answer only while the reader is at the bottom.
let stick=true;
function nearBottom(){return mainEl.scrollHeight-mainEl.scrollTop-mainEl.clientHeight<80}
mainEl.addEventListener('scroll',()=>{stick=nearBottom();toBottom.hidden=stick||chatEl.classList.contains('is-empty');});
function scroll(force){if(force||stick){mainEl.scrollTop=mainEl.scrollHeight;stick=true;toBottom.hidden=true;}}
toBottom.onclick=()=>{mainEl.scrollTo({top:mainEl.scrollHeight,behavior:'smooth'});};

function blockWhy(code,msg){
  if(code==='residency_no_compliant_provider')return 'This kind of request may only go to a model with specific compliance tags (for example local or air-gapped), and no such model is configured — so it was blocked instead of falling back to another model.';
  return msg||'The request matched a blocking rule.';
}
function badgeHTML(task,model){return (task?'<span class="pill task">'+esc(task)+'</span>':'')+
  (model?'<span class="pill '+tierClass(model)+'">'+esc(model)+'</span>':'')}
// reasonText turns the ASCII route-reason code from the header into a localized
// sentence (the header itself must stay ASCII — see setRouteReasonHeaders).
function reasonText(reason){
  if(!reason)return'';
  if(reason.indexOf('pii')===0){
    const t=reason.split(':')[1];
    return 'personal data detected'+(t?' ('+esc(t)+')':'');
  }
  return esc(reason);
}
// Visible routing switch (ISSUE-083): when a route was driven by content
// sensitivity (PII), show what happened and — if it went to a local-tagged
// provider — that the data stayed in the house. If it still went to the cloud,
// flag it (amber) so a reviewer sees the residual exposure.
function switchHTML(reason,egress,model,src){
  let rt=reasonText(reason);
  if(!rt)return'';
  const local=egress==='local';
  const cls='switch'+(local?'':' cloud');
  const head=local?'🔀 Routed to a <b>local model</b>':'⚠️ Flagged — routed to cloud';
  // The whole conversation is classified: a harmless follow-up after a
  // message with personal data still stays local — say why.
  if(src==='conversation')rt+=' earlier in this conversation';
  const tail=local?(src==='conversation'?' — the whole conversation stays in the house.':' — the data never left the house.'):' (no local model configured).';
  return '<div class="'+cls+'">'+head+' ('+esc(model||'')+') — '+rt+tail+'</div>';
}
function sourcesEl(list){return el('sources',list.map((x,i)=>'<a href="'+esc(x.u)+'" target="_blank" rel="noopener noreferrer" title="'+esc(x.u)+'"><b>'+(i+1)+'</b><span>'+esc(x.t||x.u)+'</span></a>').join(''));}

// ---- history sidebar, grouped by age, filterable.
function stamp(s){return s.at||parseInt(String(s.id).slice(1,14),10)||0}
function ageGroup(t){
  const d=new Date(),today=new Date(d.getFullYear(),d.getMonth(),d.getDate()).getTime(),day=864e5;
  if(t>=today)return'Today';if(t>=today-day)return'Yesterday';if(t>=today-7*day)return'Previous 7 days';if(t>=today-30*day)return'Previous 30 days';return'Older';
}
const ssearch=document.getElementById('ssearch');ssearch.oninput=()=>renderSidebar();
function renderSidebar(){
  sideEl.innerHTML='';
  const q=ssearch.value.trim().toLowerCase();
  const list=sessions.slice().sort((a,b)=>stamp(b)-stamp(a)).filter(s=>!q||(s.title||'').toLowerCase().indexOf(q)>=0||s.messages.some(m=>(m.content||'').toLowerCase().indexOf(q)>=0));
  if(!list.length){sideEl.appendChild(el('sempty',q?'No chat matches.':'No chats yet. Ask something to start one.'));return;}
  let last='';
  list.forEach(s=>{
    const g=ageGroup(stamp(s));if(g!==last){sideEl.appendChild(el('sgroup',g));last=g;}
    const row=el('srow'+(s.id===currentId?' active':''));
    const a=document.createElement('a');a.href='#';a.textContent=s.title||'New chat';a.title=s.title||'';
    a.onclick=e=>{e.preventDefault();loadSession(s.id);};
    const del=document.createElement('button');del.className='del';del.type='button';del.innerHTML=IC.x;del.setAttribute('aria-label','Delete chat: '+(s.title||''));
    del.onclick=e=>{e.stopPropagation();if(confirm('Delete this chat?'))deleteSession(s.id);};
    row.appendChild(a);row.appendChild(del);sideEl.appendChild(row);
  });
}
function setSide(open){
  const narrow=matchMedia('(max-width:900px)').matches;
  if(narrow){chatEl.classList.toggle('side-open',open);}
  else{chatEl.classList.toggle('side-off',!open);try{localStorage.setItem(LS_SIDE,open?'1':'0');}catch(e){}}
  // Move focus with the panel so keyboard users never land in a hidden one.
  (open?document.getElementById('newchat'):document.getElementById('sideopen')).focus({preventScroll:true});
}
// Leaving the narrow layout closes the drawer; the desktop choice stays.
matchMedia('(max-width:900px)').addEventListener('change',()=>chatEl.classList.remove('side-open'));
document.getElementById('sideopen').onclick=()=>setSide(true);
document.getElementById('sideclose').onclick=()=>setSide(false);
document.getElementById('scrim').onclick=()=>setSide(false);
try{if(localStorage.getItem(LS_SIDE)==='0')chatEl.classList.add('side-off');}catch(e){}

// ---- messages
function userEl(m){
  const row=el('msg user');const b=el('ubub');b.textContent=m.content;row.appendChild(b);
  if(m.files&&m.files.length)row.appendChild(el('fmeta','📎 '+m.files.map(esc).join(', ')));
  return row;
}
function botShell(){
  const row=el('msg bot');row.appendChild(el('av',LOGO));
  const body=el('mbody'),route=el('route'),notes=el('notes'),mdEl=el('md'),extra=el(''),acts=el('acts');
  body.append(route,notes,mdEl,extra,acts);row.appendChild(body);
  return {row,route,notes,mdEl,extra,acts};
}
function addActs(b,m,isLast){
  b.acts.innerHTML='';
  const cp=document.createElement('button');cp.type='button';cp.innerHTML=IC.copy+'<span>Copy</span>';cp.title='Copy the answer';
  cp.onclick=()=>copyText(m.content||'',cp);b.acts.appendChild(cp);
  if(isLast){const rg=document.createElement('button');rg.type='button';rg.innerHTML=IC.redo+'<span>Regenerate</span>';rg.title='Ask again — the router decides afresh';
    rg.onclick=regenerate;b.acts.appendChild(rg);}
}
function botFromHistory(m,isLast){
  const b=botShell();
  b.route.innerHTML=badgeHTML(m.task,m.model)+(m.pinned?'<span class="pill pinned">pinned</span>':'');
  if(m.switchReason)b.notes.innerHTML=switchHTML(m.switchReason,m.egress,m.model);
  b.mdEl.innerHTML=m.content?md(m.content):'<p style="color:var(--muted)">(empty answer)</p>';
  if(m.sources&&m.sources.length)b.extra.appendChild(sourcesEl(m.sources));
  addActs(b,m,isLast);
  return b.row;
}
function renderMessages(){
  wrap.innerHTML='';
  const s=cur();
  const empty=!s||!s.messages.length;
  chatEl.classList.toggle('is-empty',empty);
  toBottom.hidden=true;
  if(empty){renderChips();return;}
  const lastBot=s.messages.map(m=>m.role).lastIndexOf('assistant');
  s.messages.forEach((m,i)=>wrap.appendChild(m.role==='user'?userEl(m):botFromHistory(m,i===lastBot&&i===s.messages.length-1)));
  scroll(true);
}
function renderChips(){
  chipsEl.innerHTML='';
  QUICK.forEach(q=>{const c=document.createElement('button');c.className='chip';c.type='button';
    c.innerHTML=esc(q.label)+(q.tier?'<span class="t">'+esc(q.tier)+'</span>':'');
    if(q.note)c.title=q.note;
    c.onclick=()=>{input.value=q.text;grow();syncSend();input.focus()};chipsEl.appendChild(c);});
}
function newChat(){if(streaming)streaming.abort();currentId=null;renderMessages();renderSidebar();setSideIfNarrow();input.focus()}
function loadSession(id){if(streaming)streaming.abort();currentId=id;renderMessages();renderSidebar();setSideIfNarrow();}
function setSideIfNarrow(){if(matchMedia('(max-width:900px)').matches)chatEl.classList.remove('side-open');}
function deleteSession(id){
  sessions=sessions.filter(s=>s.id!==id);
  if(currentId===id)currentId=null;
  persist();renderMessages();renderSidebar();
}
async function copyText(t,btn){
  try{if(navigator.clipboard&&window.isSecureContext)await navigator.clipboard.writeText(t);
    else{const ta=document.createElement('textarea');ta.value=t;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove();}
    const old=btn.innerHTML;btn.innerHTML=IC.check+'<span>Copied</span>';setTimeout(()=>{btn.innerHTML=old;},1300);}catch(e){}
}
wrap.addEventListener('click',e=>{const b=e.target.closest('.copyc');if(b)copyText(b.closest('.code').querySelector('code').textContent,b);});

// ---- composer
function grow(){input.style.height='auto';input.style.height=Math.min(input.scrollHeight,220)+'px';}
function syncSend(){
  if(streaming){send.disabled=false;send.classList.add('stop');send.innerHTML=IC.stop;send.setAttribute('aria-label','Stop');send.title='Stop';return;}
  send.classList.remove('stop');send.innerHTML=IC.send;send.setAttribute('aria-label','Send');send.title='Send (Enter)';
  send.disabled=!input.value.trim()&&!attached.length;
}
input.addEventListener('input',()=>{grow();syncSend();});
input.addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing){e.preventDefault();if(!streaming)form.requestSubmit?form.requestSubmit():form.onsubmit(e);}});
form.onsubmit=e=>{
  e.preventDefault();
  if(streaming){streaming.abort();return;}
  const t=input.value.trim();if(t||attached.length)ask(t||'See the attached file.');
};

function regenerate(){
  const s=cur();if(!s||streaming)return;
  if(s.messages.length&&s.messages[s.messages.length-1].role==='assistant')s.messages.pop();
  const last=wrap.lastElementChild;if(last&&last.classList.contains('bot'))last.remove();
  ask(null,{regen:true});
}

async function ask(text,opts){
  opts=opts||{};
  let s=cur();
  if(!opts.regen){
    if(!s){s={id:uid(),title:text.length>48?text.slice(0,48)+'…':text,messages:[]};sessions.unshift(s);currentId=s.id;}
    const files=attached.map(a=>a.name), parts=attached.length?partsFor(text):null;
    const um=parts?{role:'user',content:text,parts:parts,files:files}:{role:'user',content:text};
    s.messages.push(um);
    if(chatEl.classList.contains('is-empty')){chatEl.classList.remove('is-empty');wrap.innerHTML='';}
    wrap.querySelectorAll('.acts button:nth-child(2)').forEach(b=>b.remove()); // only the newest answer regenerates
    wrap.appendChild(userEl(um));
    attached=[];renderAttached();input.value='';grow();
  }
  s.at=Date.now();renderSidebar();
  const b=botShell();
  b.route.innerHTML='<span class="think">Reading your prompt and choosing a model…</span>';
  b.mdEl.innerHTML='<span class="dots"><i></i><i></i><i></i></span>';
  wrap.appendChild(b.row);scroll(true);
  const apiMsgs=s.messages.map(m=>({role:m.role,content:m.parts?m.parts:m.content}));
  const pick=pickedModel(), pinned=pick!=='auto';
  let model='',task='',acc='',reason='',egress='',sources=[],stopped=false;
  streaming=new AbortController();syncSend();
  let frame=0;const paint=()=>{frame=0;b.mdEl.innerHTML=md(acc)+'<span class="caret"></span>';scroll();};
  try{
    const res=await fetch('/chat/send',{method:'POST',credentials:'same-origin',signal:streaming.signal,
      headers:{'Content-Type':'application/json'},body:JSON.stringify({model:pick,stream:true,messages:apiMsgs,search:searchOn})});
    if(!res.ok){
      // Clean error card — never dump a raw proxy/HTML error page in the chat.
      let msg='',code='';const raw=await res.text();
      try{const j=JSON.parse(raw);msg=(j.error&&(j.error.message||j.error.code))||'';code=(j.error&&j.error.code)||'';}catch(e){}
      const blocked=res.headers.get('x-router-blocked')||(res.status===403&&code?code:'');
      if(blocked){
        // A policy block is the product working, not an error: say what was
        // stopped, that nothing left, and that it is evidence.
        const btask=res.headers.get('x-router-route-class')||'',bsens=res.headers.get('x-router-sensitivity')||'';
        b.route.innerHTML=(btask?'<span class="pill task">'+esc(btask)+'</span>':'')+
          (bsens&&bsens!=='none'?'<span class="pill task">'+esc(bsens)+'</span>':'')+
          '<span class="pill blocked">blocked · fail-closed</span>';
        b.mdEl.innerHTML='<div class="blockcard"><b>Stopped by your policy — nothing was sent to any model.</b><br>'+esc(blockWhy(blocked,msg))+
          '<div class="bnote">Recorded in the tamper-evident audit log as <span class="mono">'+esc(blocked)+'</span>. An admin can see which rule fired with the dry-run on Policy.</div></div>';
        return;}
      if(!msg)msg=(raw&&raw.trim().charAt(0)!=='<')?raw.slice(0,300):'upstream error — the model did not answer in time. Try again.';
      b.route.innerHTML='';b.mdEl.innerHTML='<div class="errcard">Error ('+res.status+'): '+esc(msg)+'</div>';
      return;}
    model=res.headers.get('x-router-selected-model')||'';task=res.headers.get('x-router-route-class')||'';
    reason=res.headers.get('x-router-route-reason')||'';egress=res.headers.get('x-router-egress')||'';
    const primary=res.headers.get('x-router-primary-model')||'',fbIdx=res.headers.get('x-router-fallback-index')||'';
    const sensSrc=res.headers.get('x-router-sensitivity-source')||'';
    b.route.innerHTML=badgeHTML(task,model)+(pinned?'<span class="pill pinned" title="Model pinned by the admin — classification bypassed, policy still enforced.">pinned</span>':'')+
      (fbIdx?'<span class="pill fallback" title="The primary model ('+esc(primary)+') did not answer — the policy-approved fallback chain, built before the first call, took over. The fallback never widens egress.">fallback · from '+esc(primary||'primary')+'</span>':'');
    if(reason)b.notes.innerHTML=switchHTML(reason,egress,model,sensSrc);
    const sh=res.headers.get('x-router-search')||'';
    if(sh){
      if(sh.startsWith('done')){try{sources=JSON.parse(atob((res.headers.get('x-router-sources')||'').replace(/-/g,'+').replace(/_/g,'/')));}catch(e){sources=[];}
        b.route.insertAdjacentHTML('beforeend','<span class="pill search" title="Fetched through the company search engine before the model answered.">web search · '+sources.length+' sources</span>');}
      else if(sh.startsWith('skipped')){b.route.insertAdjacentHTML('beforeend','<span class="pill search skip" title="The prompt was classified as '+esc(sh.split(';')[1]||'sensitive')+' — nothing was sent to a search engine.">search skipped · '+esc(sh.split(';')[1]||'sensitive')+' in prompt</span>');}
      else if(sh==='failed'){b.route.insertAdjacentHTML('beforeend','<span class="pill search skip">search unavailable</span>');}
      else if(sh==='empty'){b.route.insertAdjacentHTML('beforeend','<span class="pill search skip">no web results</span>');}
    }
    const reader=res.body.getReader(),dec=new TextDecoder();let buf='';let thinking=false;
    while(true){const {done,value}=await reader.read();if(done)break;
      buf+=dec.decode(value,{stream:true});const lines=buf.split('\n');buf=lines.pop();
      for(const line of lines){const t=line.trim();if(!t.startsWith('data:'))continue;
        const p=t.slice(5).trim();if(p==='[DONE]')continue;
        try{const j=JSON.parse(p);const dl=j.choices&&j.choices[0]&&j.choices[0].delta;
          // Reasoning models stream a thinking phase before the answer — show a
          // live indicator instead of a frozen bubble.
          if(dl&&dl.reasoning&&!acc&&!thinking){thinking=true;b.mdEl.innerHTML='<span class="think">Thinking…</span>';scroll();}
          const d=dl&&dl.content;
          if(d){acc+=d;thinking=false;if(!frame)frame=requestAnimationFrame(paint);}}catch(e){}}}
  }catch(e){
    if(e.name==='AbortError'){stopped=true;}
    else{b.mdEl.innerHTML='<div class="errcard">Error: '+esc(e.message)+'</div>';streaming=null;syncSend();return;}
  }finally{if(frame){cancelAnimationFrame(frame);frame=0;}}
  if(b.mdEl.querySelector('.blockcard,.errcard')){streaming=null;syncSend();return;}
  b.mdEl.innerHTML=acc?md(acc):(stopped?'':'<p style="color:var(--muted)">(empty answer)</p>');
  if(stopped)b.mdEl.insertAdjacentHTML('beforeend','<p style="color:var(--faint);font-size:13px">Stopped.</p>');
  if(sources.length)b.extra.appendChild(sourcesEl(sources));
  const msg={role:'assistant',content:acc,model:model,task:task,switchReason:reason,egress:egress,pinned:pinned,sources:sources};
  s.messages.push(msg);addActs(b,msg,true);
  streaming=null;syncSend();persist();scroll();
  if(!matchMedia('(pointer:coarse)').matches)input.focus();
}
// A blocked or failed request ends the stream state too.
const _ask=ask;ask=async function(t,o){try{await _ask(t,o);}finally{if(streaming){streaming=null;}syncSend();}};

document.getElementById('newchat').onclick=newChat;
document.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.shiftKey&&(e.key==='o'||e.key==='O')){e.preventDefault();newChat();}
  if(e.key==='Escape'&&chatEl.classList.contains('side-open')){setSide(false);return;}
  if(e.key==='Escape'&&streaming)streaming.abort();
});
// Boot: load sessions from the server (SQLite) first, then localStorage.
async function boot(){
  document.getElementById('hellosub').innerHTML=EMPLOYEE
    ?'Your questions are routed by company policy. Personal and confidential data stays on company models.'
    :'Sluss reads each prompt and picks the model <b>before</b> anything is sent. Every answer shows which model was chosen.';
  if(EMPLOYEE)hintEl.textContent='Routed by company policy. Personal and confidential data stays on company models; web search only runs when the prompt contains none.';
  loadModels();
  renderChips();
  // Admin-managed quick prompts (DB-backed); fall back to the built-in list.
  try{const qr=await fetch('/chat/quickprompts',{credentials:'same-origin'});
    if(qr.ok){const q=await qr.json();if(Array.isArray(q)&&q.length)QUICK=q;}}catch(e){}
  try{const res=await fetch('/chat/sessions',{credentials:'same-origin'});
    if(res.ok&&res.status!==204){const j=await res.json();if(Array.isArray(j)&&j.length)sessions=j;}}catch(e){}
  if(!sessions.length){try{sessions=JSON.parse(localStorage.getItem(LS))||[]}catch(e){sessions=[]}}
  // Open on a fresh chat, like other assistants; history is one click away.
  currentId=null;
  renderMessages();renderSidebar();syncSend();
}
boot();
</script>
</body>
</html>`

// modelPickerHTML is the admin-only model picker (ISSUE-098): the chat page is
// the admin's control panel, and the picker is its verification switch — Auto
// demos the router (cheapest capable model = cost & CO₂ saver), pinning a model
// verifies it end-to-end with policy still enforced.
const modelPickerHTML = `<div class="modelpick" title="Auto lets the router pick the cheapest capable model per prompt. Pin a model to verify it end-to-end — classification is bypassed, policy still applies.">
      <label for="modelsel">Model</label>
      <select id="modelsel"><option value="auto">Auto (router)</option></select>
    </div>`

// demoChatHTML is the chat page with the shared admin frame injected once at
// startup: the left admin nav (chat active), the shell CSS and the model picker.
var demoChatHTML = strings.NewReplacer(
	"/*ADMIN_CSS*/", adminShellCSS,
	"<!--ADMIN_NAV-->", adminNavHTML("chat"),
	"<!--MODEL_PICKER-->", modelPickerHTML,
	"<!--FAVICON-->", faviconLinks,
	"<!--LOGO-->", brandLogoSVG,
).Replace(demoChatTemplate)

// demoChatHTMLGuest is the shareable-link view: no admin nav and no model
// picker, so a CISO sees a focused product demo rather than the internal
// console (the guest page always routes with Auto).
var demoChatHTMLGuest = strings.NewReplacer(
	"/*ADMIN_CSS*/", adminShellCSS,
	"<!--ADMIN_NAV-->", "",
	"<!--MODEL_PICKER-->", "",
	"<!--FAVICON-->", faviconLinks,
	"<!--LOGO-->", brandLogoSVG,
).Replace(demoChatTemplate)
