package server

import (
	"io"
	"net/http"
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
		v, ok := h.KVGet(demoSessionsKey)
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
		if err := h.KVSet(demoSessionsKey, string(body)); err != nil {
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

// demoTenantInjector adapts the demo chat endpoint to the standard chat handler:
// it injects the demo tenant into the request context (the chat handler reads
// the tenant from context, normally set by auth.Middleware) so no bearer key is
// needed. The endpoint itself is still gated by the dashboard guard.
func demoTenantInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), demoTenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DemoChatPageHandler serves the "seeing is believing" demo: a ChatGPT-style
// page with a session sidebar where each answer shows which model the router
// picked, so you can flip to the dashboard and watch the savings accumulate.
// Sessions live in the browser's localStorage — no backend state needed.
func DemoChatPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// A demo-link guest gets the clean, nav-free page — a focused product demo,
		// not the admin console. Admins keep the left nav to jump back.
		if RoleFromContext(r.Context()) == roleDemo {
			_, _ = w.Write([]byte(demoChatHTMLGuest))
			return
		}
		_, _ = w.Write([]byte(demoChatHTML))
	}
}

const demoChatTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — live routing demo</title>
<!--FAVICON-->
<style>
  /*ADMIN_CSS*/
  :root{
    --bg:#0b1220; --panel:#111a2e; --panel2:#0e1626; --side:#0a1120; --line:#22304d;
    --ink:#e8eef7; --muted:#8fa1bf; --accent:#22c58b; --accent2:#f4b740;
    --user:#1d2b47; --bot:#0e1626;
  }
  *{box-sizing:border-box}
  html,body{height:100%}
  body{margin:0;background:var(--bg);color:var(--ink);
    font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  .chatrow{flex:1;display:flex;min-height:0}
  /* ---- session sidebar (now on the RIGHT) ---- */
  aside.sessions{width:256px;flex-shrink:0;background:var(--side);border-left:1px solid var(--line);
    display:flex;flex-direction:column;order:2}
  .newbtn{margin:14px;padding:11px 14px;border:1px solid var(--line);background:var(--panel2);
    color:var(--ink);border-radius:10px;font-size:14px;font-weight:600;cursor:pointer;
    display:flex;align-items:center;gap:8px;justify-content:center;transition:.15s}
  .newbtn:hover{border-color:var(--accent);color:#fff}
  .slabel{padding:4px 18px;font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted)}
  #sessions{flex:1;overflow-y:auto;padding:2px 10px 12px}
  .srow{display:flex;align-items:center;gap:6px;padding:9px 10px;border-radius:8px;cursor:pointer;
    color:var(--ink);font-size:13.5px;transition:.12s}
  .srow:hover{background:var(--panel2)}
  .srow.active{background:var(--panel)}
  .srow .title{flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .srow .del{opacity:0;border:0;background:transparent;color:var(--muted);cursor:pointer;
    font-size:15px;line-height:1;padding:2px 4px;border-radius:6px;flex-shrink:0}
  .srow:hover .del{opacity:1}
  .srow .del:hover{color:#f07ab0;background:#2a1424}
  .sempty{padding:12px 18px;color:var(--muted);font-size:13px;line-height:1.5}
  .side-foot{border-top:1px solid var(--line);padding:12px 16px;font-size:11.5px;color:var(--muted)}
  /* ---- chat column ---- */
  .col{flex:1;display:flex;flex-direction:column;min-width:0}
  header{display:flex;align-items:center;justify-content:space-between;gap:12px;
    padding:14px 20px;border-bottom:1px solid var(--line);background:var(--panel)}
  .brand{display:flex;align-items:center;gap:9px;font-weight:700}
  .brand .dot{width:10px;height:10px;border-radius:3px;background:var(--accent)}
  .brand small{color:var(--muted);font-weight:400;margin-left:4px}
  .modelpick{display:flex;align-items:center;gap:8px;font-size:12.5px;color:var(--muted)}
  .modelpick select{background:var(--panel2);border:1px solid var(--line);color:var(--ink);
    border-radius:8px;padding:6px 9px;font-size:12.5px;max-width:280px;outline:none}
  .modelpick select:focus{border-color:var(--accent)}
  .pill.pinned{background:#2a2140;color:#c4a7f7}
  main{flex:1;overflow-y:auto;padding:22px 16px}
  .wrap{max-width:760px;margin:0 auto;display:flex;flex-direction:column;gap:16px}
  .intro{color:var(--muted);text-align:center;font-size:15px;line-height:1.5;padding:8px 0 4px}
  .intro b{color:var(--ink)}
  .chips{display:flex;flex-wrap:wrap;gap:8px;justify-content:center;margin-bottom:6px}
  .chip{border:1px solid var(--line);background:var(--panel2);color:var(--ink);
    border-radius:999px;padding:8px 14px;font-size:13.5px;cursor:pointer;transition:.15s}
  .chip:hover{border-color:var(--accent);color:#fff}
  .chip .t{color:var(--muted);font-size:11.5px;margin-left:6px}
  .msg{display:flex;flex-direction:column;gap:6px}
  .msg .who{font-size:12px;color:var(--muted);letter-spacing:.04em;text-transform:uppercase}
  .bubble{padding:13px 16px;border-radius:12px;line-height:1.55;white-space:pre-wrap;word-wrap:break-word}
  .msg.user .bubble{background:var(--user);align-self:flex-end;max-width:85%}
  .msg.bot .bubble{background:var(--bot);border:1px solid var(--line)}
  .bubble.errb{color:#f8b4bc;border-color:#5b2330}
  .bubble.blockb{border-color:#5b2330;background:rgba(239,68,68,.06);line-height:1.55}
  .bubble.blockb b{color:#fca5a5}
  .bubble .bnote{margin-top:8px;font-size:12.5px;color:#8fa1bf}
  .bubble .mono{font-family:ui-monospace,Menlo,monospace;font-size:12px}
  .pill.blocked{background:#3b1414;color:#f87171}
  .think{display:inline-block;color:var(--muted);font-style:italic;animation:pulse 1.6s ease-in-out infinite}
  @keyframes pulse{0%,100%{opacity:.45}50%{opacity:1}}
  @media (prefers-reduced-motion: reduce){.think{animation:none}}
  .badge{display:inline-flex;flex-wrap:wrap;align-items:center;gap:8px;font-size:12.5px;margin-top:2px}
  .pill{display:inline-flex;align-items:center;gap:6px;padding:3px 10px;border-radius:999px;
    font-weight:600;font-family:ui-monospace,Menlo,monospace}
  .pill.task{background:#16233c;color:#9db4dc}
  .pill.cheap{background:#13351f;color:#4fd08a}
  .pill.balanced{background:#33290f;color:#f4b740}
  .pill.premium{background:#361529;color:#f07ab0}
  .switch{display:flex;align-items:flex-start;gap:9px;margin-top:2px;padding:10px 13px;border-radius:10px;
    font-size:13px;line-height:1.45;background:#0f2f22;border:1px solid #1f7a4d;color:#a7f3d0}
  .switch.cloud{background:#33270e;border-color:#7a5a1f;color:#f4d38a}
  .switch b{color:#fff}
  .cursor{display:inline-block;width:7px;height:15px;background:var(--accent);
    animation:blink 1s steps(2) infinite;vertical-align:-2px;margin-left:1px}
  @keyframes blink{50%{opacity:0}}
  @media (prefers-reduced-motion: reduce){.cursor{animation:none}}
  footer{border-top:1px solid var(--line);background:var(--panel);padding:14px 16px}
  .composer{max-width:760px;margin:0 auto;display:flex;gap:10px}
  .composer input{flex:1;background:var(--panel2);border:1px solid var(--line);color:var(--ink);
    border-radius:10px;padding:13px 15px;font-size:15px;outline:none}
  .composer input:focus{border-color:var(--accent)}
  .composer button{background:var(--accent);color:#0b1220;border:0;border-radius:10px;
    padding:0 20px;font-weight:700;font-size:15px;cursor:pointer}
  .composer button:disabled{opacity:.5;cursor:not-allowed}
  .hint{max-width:760px;margin:8px auto 0;color:var(--muted);font-size:12px;text-align:center}
  @media (max-width:760px){aside.sessions{width:64px}.newbtn span,.slabel,.srow .title,.side-foot{display:none}
    .newbtn{justify-content:center}}
</style>
</head>
<body>
<div class="tk-shell">
<!--ADMIN_NAV-->
<div class="tk-main">
<div class="chatrow">
<div class="col">
  <header>
    <div class="brand"><span class="dot"></span>sluss<small>live routing demo</small></div>
    <!--MODEL_PICKER-->
  </header>
  <main><div class="wrap" id="wrap"></div></main>
  <footer>
    <form class="composer" id="form">
      <input id="input" placeholder="Ask anything — simple or hard…" autocomplete="off" autofocus>
      <button id="send" type="submit">Send</button>
    </form>
    <div class="hint" id="hint">Model selection happens locally in &lt;0.05&nbsp;ms — the same engine a real client uses. Auto picks the cheapest capable model per prompt (cost &amp; CO₂ saver).</div>
  </footer>
</div>
<aside class="sessions">
  <button class="newbtn" id="newchat">+ <span>New chat</span></button>
  <div class="slabel">Sessions</div>
  <div id="sessions"></div>
  <div class="side-foot">Saved in your browser + on the server.</div>
</aside>
</div>
</div>
</div>
<script>
// Quick prompts are admin-managed (DB-backed) and loaded from the server; this
// built-in list is only a fallback if the fetch fails.
let QUICK=[
  {label:"Write a git commit message",tier:"cheap",text:"write a concise git commit message for a bugfix"},
  {label:"Debug a Go deadlock",tier:"hard",text:"debug this race condition deadlock in my concurrent Go code, goroutines hang on a mutex"},
  {label:"Security-review a login",tier:"blocked",text:"security review this login form for XSS and secret leakage"},
];
const LS='tok_demo_sessions_v1';
const wrap=document.getElementById('wrap'), sideEl=document.getElementById('sessions');
const form=document.getElementById('form'), input=document.getElementById('input'), send=document.getElementById('send');
let sessions=[], currentId=null;

// Model picker (ISSUE-098, admin control panel only — absent on the guest page).
// "Auto (router)" is the demo mode: the classifier picks the cheapest capable
// model (the cost & CO₂ saver). Pinning a model bypasses classification to
// verify that model end-to-end — policy is still enforced, so a pinned cloud
// model + sensitive data is still blocked (and that block is itself the demo).
const modelSel=document.getElementById('modelsel'), hintEl=document.getElementById('hint');
const HINT_AUTO='Model selection happens locally in <0.05 ms — the same engine a real client uses. Auto picks the cheapest capable model per prompt (cost & CO₂ saver).';
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
  localStorage.setItem(LS,JSON.stringify(sessions));
  // Best-effort server persistence (SQLite) so a curated demo survives redeploys.
  fetch('/chat/sessions',{method:'PUT',credentials:'same-origin',
    headers:{'Content-Type':'application/json'},body:JSON.stringify(sessions)}).catch(()=>{});
}
function cur(){return sessions.find(s=>s.id===currentId)}
function uid(){return 's'+Date.now()+Math.floor(Math.random()*1000)}
function el(cls,html){const d=document.createElement('div');d.className=cls;if(html!==undefined)d.innerHTML=html;return d;}
function escapeHtml(s){return s.replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}
function tierClass(m){if(/premium/.test(m))return'premium';if(/balanced/.test(m))return'balanced';return'cheap'}
function scroll(){const m=document.querySelector('main');m.scrollTop=m.scrollHeight}
function blockWhy(code,msg){
  if(code==='residency_no_compliant_provider')return 'This kind of request may only go to a model with specific compliance tags (for example local or air-gapped), and no such model is configured — so it was blocked instead of falling back to another model.';
  return msg||'The request matched a blocking rule.';
}
function badgeHTML(task,model){return '<span class="pill task">'+escapeHtml(task||'route')+'</span>'+
  (model?'<span class="pill '+tierClass(model)+'">'+escapeHtml(model)+'</span>':'')}
// reasonText turns the ASCII route-reason code from the header into a localized
// sentence (the header itself must stay ASCII — see setRouteReasonHeaders).
function reasonText(reason){
  if(!reason)return'';
  if(reason.indexOf('pii')===0){
    const t=reason.split(':')[1];
    return 'personal data detected'+(t?' ('+escapeHtml(t)+')':'');
  }
  return escapeHtml(reason);
}
// Visible routing switch (ISSUE-083): when a route was driven by content
// sensitivity (PII), show what happened and — if it went to a local-tagged
// provider — that the data stayed in the house. If it still went to the cloud,
// flag it (amber) so a reviewer sees the residual exposure.
function switchHTML(reason,egress,model){
  const rt=reasonText(reason);
  if(!rt)return'';
  const local=egress==='local';
  const cls='switch'+(local?'':' cloud');
  const head=local?'🔀 Routed to a <b>local model</b>':'⚠️ Flagged — routed to cloud';
  const tail=local?' — the data never left the house.':' (no local model configured).';
  return '<div class="'+cls+'">'+head+' ('+escapeHtml(model||'')+') — '+rt+tail+'</div>';
}

function renderSidebar(){
  sideEl.innerHTML='';
  if(!sessions.length){sideEl.appendChild(el('sempty','No sessions yet. Type something to start one.'));return;}
  sessions.forEach(s=>{
    const row=el('srow'+(s.id===currentId?' active':''));
    const t=el('title');t.textContent=s.title;row.appendChild(t);
    const del=document.createElement('button');del.className='del';del.textContent='✕';del.title='Delete';
    del.onclick=e=>{e.stopPropagation();deleteSession(s.id)};
    row.appendChild(del);
    row.onclick=()=>loadSession(s.id);
    sideEl.appendChild(row);
  });
}
function renderMessages(){
  wrap.innerHTML='';
  const s=cur();
  if(!s||!s.messages.length){
    wrap.appendChild(el('intro','Type anything — <b>the router reads the prompt and picks the model</b> before the answer is generated.<br>Every answer shows which model was chosen. Open the dashboard in another tab and watch the savings tick up.'));
    const chips=el('chips');
    QUICK.forEach(q=>{const c=document.createElement('button');c.className='chip';c.type='button';
      c.innerHTML=escapeHtml(q.label)+(q.tier?'<span class="t">'+escapeHtml(q.tier)+'</span>':'');
      if(q.note)c.title=q.note;
      c.onclick=()=>{input.value=q.text;input.focus()};chips.appendChild(c);});
    wrap.appendChild(chips);
    return;
  }
  s.messages.forEach(m=>{
    const row=el('msg '+(m.role==='user'?'user':'bot'));
    row.appendChild(el('who',m.role==='user'?'You':'sluss'));
    if(m.role!=='user'&&(m.task||m.model))row.appendChild(el('badge',badgeHTML(m.task,m.model)+(m.pinned?'<span class="pill pinned">pinned</span>':'')));
    if(m.role!=='user'&&m.switchReason){const c=el('');c.innerHTML=switchHTML(m.switchReason,m.egress,m.model);if(c.firstElementChild)row.appendChild(c.firstElementChild);}
    const b=el('bubble');b.textContent=m.content;row.appendChild(b);
    wrap.appendChild(row);
  });
  scroll();
}
function newChat(){currentId=null;renderMessages();renderSidebar();input.focus()}
function loadSession(id){currentId=id;renderMessages();renderSidebar()}
function deleteSession(id){
  sessions=sessions.filter(s=>s.id!==id);
  if(currentId===id)currentId=sessions.length?sessions[0].id:null;
  persist();renderMessages();renderSidebar();
}

async function ask(text){
  let s=cur();
  if(!s){s={id:uid(),title:text.length>38?text.slice(0,38)+'…':text,messages:[]};sessions.unshift(s);currentId=s.id;wrap.innerHTML='';renderSidebar();}
  s.messages.push({role:'user',content:text});
  const um=el('msg user');um.appendChild(el('who','You'));const ub=el('bubble');ub.textContent=text;um.appendChild(ub);wrap.appendChild(um);
  input.value='';send.disabled=true;scroll();
  const bm=el('msg bot');bm.appendChild(el('who','sluss'));
  const badge=el('badge','<span class="pill task">classifying…</span>');bm.appendChild(badge);
  const bubble=el('bubble');const curEl=el('cursor');bubble.appendChild(curEl);bm.appendChild(bubble);wrap.appendChild(bm);scroll();
  const apiMsgs=s.messages.map(m=>({role:m.role,content:m.content}));
  const pick=pickedModel(), pinned=pick!=='auto';
  let model='',task='',acc='',reason='',egress='';
  try{
    const res=await fetch('/chat/send',{method:'POST',credentials:'same-origin',
      headers:{'Content-Type':'application/json'},body:JSON.stringify({model:pick,stream:true,messages:apiMsgs})});
    if(!res.ok){
      // Clean error card — never dump a raw proxy/HTML error page in the chat.
      let msg='',code='';const raw=await res.text();
      try{const j=JSON.parse(raw);msg=(j.error&&(j.error.message||j.error.code))||'';code=(j.error&&j.error.code)||'';}catch(e){}
      const blocked=res.headers.get('x-router-blocked')||(res.status===403&&code?code:'');
      if(blocked){
        // A policy block is the product working, not an error: say what was
        // stopped, that nothing left, and that it is evidence.
        const btask=res.headers.get('x-router-route-class')||'',bsens=res.headers.get('x-router-sensitivity')||'';
        badge.innerHTML=(btask?'<span class="pill task">'+escapeHtml(btask)+'</span>':'')+
          (bsens&&bsens!=='none'?'<span class="pill task">'+escapeHtml(bsens)+'</span>':'')+
          '<span class="pill blocked">blocked · fail-closed</span>';
        bubble.innerHTML='<b>Stopped by your policy — nothing was sent to any model.</b><br>'+escapeHtml(blockWhy(blocked,msg))+
          '<div class="bnote">Recorded in the tamper-evident audit log as <span class="mono">'+escapeHtml(blocked)+'</span>. An admin can see which rule fired with the dry-run on Policy.</div>';
        bubble.classList.add('blockb');
        curEl.remove();send.disabled=false;return;}
      if(!msg)msg=(raw&&raw.trim().charAt(0)!=='<')?raw.slice(0,300):'upstream error — the model did not answer in time. Try again.';
      bubble.textContent='Error ('+res.status+'): '+msg;bubble.classList.add('errb');
      curEl.remove();send.disabled=false;return;}
    model=res.headers.get('x-router-selected-model')||'';task=res.headers.get('x-router-route-class')||'';
    reason=res.headers.get('x-router-route-reason')||'';egress=res.headers.get('x-router-egress')||'';
    badge.innerHTML=badgeHTML(task,model)+(pinned?'<span class="pill pinned" title="Model pinned by the admin — classification bypassed, policy still enforced.">pinned</span>':'');
    if(reason){const c=el('');c.innerHTML=switchHTML(reason,egress,model);if(c.firstElementChild)bm.insertBefore(c.firstElementChild,bubble);}
    const reader=res.body.getReader(),dec=new TextDecoder();let buf='';let thinking=false;
    while(true){const {done,value}=await reader.read();if(done)break;
      buf+=dec.decode(value,{stream:true});const lines=buf.split('\n');buf=lines.pop();
      for(const line of lines){const t=line.trim();if(!t.startsWith('data:'))continue;
        const p=t.slice(5).trim();if(p==='[DONE]')continue;
        try{const j=JSON.parse(p);const dl=j.choices&&j.choices[0]&&j.choices[0].delta;
          // Reasoning models stream a thinking phase before the answer — show a
          // live indicator instead of a frozen bubble.
          if(dl&&dl.reasoning&&!acc){if(!thinking){thinking=true;bubble.textContent='';const th=el('think','🧠 thinking…');bubble.appendChild(th);bubble.appendChild(curEl);scroll();}}
          const d=dl&&dl.content;
          if(d){acc+=d;thinking=false;bubble.textContent=acc;bubble.appendChild(curEl);scroll();}}catch(e){}}}
    curEl.remove();if(!acc)bubble.textContent='(empty answer)';
  }catch(e){curEl.remove();bubble.textContent='Error: '+e.message;}
  s.messages.push({role:'assistant',content:acc,model:model,task:task,switchReason:reason,egress:egress,pinned:pinned});
  persist();send.disabled=false;input.focus();
}
form.onsubmit=e=>{e.preventDefault();const t=input.value.trim();if(t)ask(t)};
document.getElementById('newchat').onclick=newChat;
// Boot: load sessions from the server (SQLite) first, then localStorage.
async function boot(){
  loadModels();
  // Admin-managed quick prompts (DB-backed); fall back to the built-in list.
  try{const qr=await fetch('/chat/quickprompts',{credentials:'same-origin'});
    if(qr.ok){const q=await qr.json();if(Array.isArray(q)&&q.length)QUICK=q;}}catch(e){}
  try{const res=await fetch('/chat/sessions',{credentials:'same-origin'});
    if(res.ok&&res.status!==204){const j=await res.json();if(Array.isArray(j)&&j.length)sessions=j;}}catch(e){}
  if(!sessions.length){try{sessions=JSON.parse(localStorage.getItem(LS))||[]}catch(e){sessions=[]}}
  if(sessions.length)currentId=sessions[0].id;
  renderMessages();renderSidebar();
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
).Replace(demoChatTemplate)

// demoChatHTMLGuest is the shareable-link view: no admin nav and no model
// picker, so a CISO sees a focused product demo rather than the internal
// console (the guest page always routes with Auto).
var demoChatHTMLGuest = strings.NewReplacer(
	"/*ADMIN_CSS*/", adminShellCSS,
	"<!--ADMIN_NAV-->", "",
	"<!--MODEL_PICKER-->", "",
	"<!--FAVICON-->", faviconLinks,
).Replace(demoChatTemplate)
