package server

// Connect inside the console (ISSUE-143): the same client recipes as the
// public /connect page, in the admin frame (menu, Rollout tabs, design
// system), with copy buttons and a key field that fills every snippet in the
// browser. The key never leaves the page: it is not sent, stored or logged.
// Stored department keys are hashes and cannot be shown, so the page links to
// Access to mint one.

import (
	"html/template"
	"log/slog"
	"net/http"
)

type connectGroup struct {
	Title, Sub string
	Clients    []connectClient
}

var connectKinds = []struct{ kind, title, sub string }{
	{"ide", "Editors", "Code completion and chat inside the IDE."},
	{"agent", "Coding agents", "Command-line agents that read and change code."},
	{"chat", "Chat apps", "Self-hosted chat front ends for your teams."},
	{"sdk", "SDKs", "Your own code and scripts."},
}

type consoleConnectData struct {
	Base, OpenAIBase string
	HasPublicURL     bool
	Groups           []connectGroup
	Curl             string
}

// ConsoleConnectHandler renders /router/connect.
func ConsoleConnectHandler(publicURL string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := requestBase(publicURL, r)
		d := consoleConnectData{Base: base, OpenAIBase: base + "/v1", HasPublicURL: publicURL != ""}
		clients := connectClients(base)
		for _, k := range connectKinds {
			g := connectGroup{Title: k.title, Sub: k.sub}
			for _, c := range clients {
				if c.Kind == k.kind {
					g.Clients = append(g.Clients, c)
				}
			}
			if len(g.Clients) > 0 {
				d.Groups = append(d.Groups, g)
			}
		}
		d.Curl = "curl " + d.OpenAIBase + "/chat/completions \\\n" +
			"  -H \"Authorization: Bearer <key>\" -H \"Content-Type: application/json\" \\\n" +
			"  -d '{\"model\":\"auto\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello\"}]}'"
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := consoleConnectTmpl.Execute(w, d); err != nil && logger != nil {
			logger.Error("connect page render failed", "err", err)
		}
	}
}

var consoleConnectTmpl = template.Must(template.New("cconnect").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"dsHead":   dsHeadFunc,
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Connect clients</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.mono,code,pre{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace}
code{background:#0b1528;border:1px solid #22324f;border-radius:6px;padding:1px 6px;font-size:.84rem}
.vals{display:grid;grid-template-columns:auto 1fr auto;gap:10px 14px;align-items:center}
.vals .k{color:#8fa1bf;font-size:.82rem}
.vals .v{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.92rem;background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:8px 11px;word-break:break-all}
.vals input{width:100%;background:#0b1528;border:1px solid #22324f;color:#e8eef7;border-radius:8px;padding:8px 11px;font:inherit;font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.9rem}
.vals input:focus{outline:none;border-color:#fad100}
.cp{background:transparent;border:1px solid #2a3b5c;color:#cfe0f7;border-radius:7px;padding:6px 12px;font-size:12px;cursor:pointer;white-space:nowrap}
.cp:hover{background:#fad100;color:#081328;border-color:#fad100}
.cp.done{background:#13351f;color:#8ff0bf;border-color:#1f5c38}
.keyhint{grid-column:2 / 4;color:#8fa1bf;font-size:12.5px;margin-top:-4px}
.keyhint a,.ds-sub a{color:#7fd3ff}
.cl{display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:14px}
.cc{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:16px 18px;display:flex;flex-direction:column;gap:8px;min-width:0}
.cc h3{margin:0;font-size:1rem;display:flex;gap:8px;align-items:center}
.wire{font-size:10.5px;font-weight:600;padding:2px 8px;border-radius:999px;font-family:"IBM Plex Mono",ui-monospace,monospace}
.wire.openai{background:#0f2e24;color:#9fdcc4}.wire.anthropic{background:#2a1d3a;color:#d6b4f0}
.cc .where{color:#8fa1bf;font-size:12.5px}
.cc .how{font-size:13px;color:#cfe0f7;line-height:1.55}
.snip{position:relative;margin:0}
.snip pre{margin:0;background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:10px 12px;padding-right:70px;font-size:12px;white-space:pre-wrap;word-break:break-all;color:#e8eef7}
.snip .cp{position:absolute;top:6px;right:6px}
@media(max-width:640px){.vals{grid-template-columns:1fr}.keyhint{grid-column:auto}.cl{grid-template-columns:1fr}}
</style></head>
<body><div class="tk-shell">
{{adminNav "connect"}}
<div class="tk-main"><div class="ds-content">
{{dsHead "connect" "Connect clients" "Point editors, agents, chat apps and your own code at Sluss. Every client needs the same three values."}}

<div class="ds-card">
  <h2>The three values</h2>
  <div class="vals">
    <span class="k">Base URL</span><span class="v" id="v-base">{{.OpenAIBase}}</span><button class="cp" type="button" data-copy="v-base">Copy</button>
    <span class="k">Anthropic clients</span><span class="v" id="v-anth">{{.Base}}</span><button class="cp" type="button" data-copy="v-anth">Copy</button>
    <span class="k">Model</span><span class="v" id="v-model">auto</span><button class="cp" type="button" data-copy="v-model">Copy</button>
    <label class="k" for="key">API key</label><input id="key" type="password" autocomplete="off" spellcheck="false" placeholder="Paste a department key to fill it into every snippet"><button class="cp" type="button" id="showkey">Show</button>
    <span class="keyhint">Sluss stores keys only as hashes, so it cannot show an existing one. <a href="/router/keys">Create a key for the department</a> and paste it here. It stays in this browser tab, is never sent or saved, and is gone when you leave.</span>
  </div>
  {{if not .HasPublicURL}}<div class="ds-foot">These use the address you opened the console with. Set ROUTER_PUBLIC_URL if clients reach Sluss under another address.</div>{{end}}
</div>

{{range .Groups}}
<div class="ds-section"><h2>{{.Title}}</h2><div class="ds-sub">{{.Sub}}</div>
<div class="cl">
{{range .Clients}}
  <div class="cc">
    <h3>{{.Name}} <span class="wire {{.Wire}}">{{.Wire}}</span></h3>
    <div class="where">{{.How}}</div>
    <div class="how">{{.Setup}}</div>
    <div class="snip"><pre data-tpl="{{.Copy}}">{{.Copy}}</pre><button class="cp" type="button">Copy</button></div>
  </div>
{{end}}
</div></div>
{{end}}

<div class="ds-section"><h2>Does it work?</h2><div class="ds-sub">Run this from the client's machine. The answer comes from the model Sluss picked, and the request shows up in the <a href="/router/log">request log</a> within a second.</div>
<div class="ds-card"><div class="snip"><pre data-tpl="{{.Curl}}">{{.Curl}}</pre><button class="cp" type="button">Copy</button></div></div>
</div>

<div class="ds-foot">OpenAI-wire clients call <code>/v1/chat/completions</code>; Anthropic-wire clients call <code>/v1/messages</code>. Both go through the same routing, policy and audit trail. One key per department makes the request log and reports show who used what. The public version of this page, without the console, is at <a href="/connect">/connect</a>.</div>
</div></div></div>
<script>
(function(){
  var key=document.getElementById('key'), pres=[].slice.call(document.querySelectorAll('pre[data-tpl]'));
  function fill(){var k=key.value.trim();pres.forEach(function(p){var t=p.getAttribute('data-tpl');p.textContent=k?t.split('<key>').join(k):t;});}
  key.addEventListener('input',fill);
  document.getElementById('showkey').addEventListener('click',function(){var s=key.type==='password';key.type=s?'text':'password';this.textContent=s?'Hide':'Show';});
  function copy(btn,text){
    var done=function(){btn.classList.add('done');var o=btn.textContent;btn.textContent='Copied';setTimeout(function(){btn.classList.remove('done');btn.textContent=o;},1400);};
    if(navigator.clipboard&&window.isSecureContext){navigator.clipboard.writeText(text).then(done);return;}
    var ta=document.createElement('textarea');ta.value=text;document.body.appendChild(ta);ta.select();try{document.execCommand('copy');done();}catch(e){}ta.remove();
  }
  document.querySelectorAll('button.cp').forEach(function(b){
    if(b.id==='showkey')return;
    b.addEventListener('click',function(){
      var id=b.getAttribute('data-copy'), src=id?document.getElementById(id):b.parentNode.querySelector('pre');
      copy(b,src.textContent);
    });
  });
})();
</script>
</body></html>`))
