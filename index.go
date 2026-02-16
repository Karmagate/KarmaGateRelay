package main

import "net/http"

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>KarmaGate Relay</title>
<meta name="description" content="Lightweight, stateless WebSocket relay server for KarmaGate Bind">
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 36 36'><circle cx='18' cy='18' r='3' fill='%23DEDACF'/><circle cx='18' cy='9' r='3' fill='%23DEDACF'/><circle cx='26' cy='13.5' r='3' fill='%23DEDACF'/><circle cx='26' cy='22.5' r='3' fill='%23DEDACF'/><circle cx='18' cy='27' r='3' fill='%23DEDACF'/><circle cx='10' cy='22.5' r='3' fill='%23DEDACF'/><circle cx='10' cy='13.5' r='3' fill='%23DEDACF'/></svg>">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
--bg:#191919;
--card:#242424;
--border:#333;
--fg:#e5e5e5;
--muted:#737373;
--logo:#DEDACF;
--radius:6px;
}
body{
font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;
background:var(--bg);
color:var(--fg);
min-height:100vh;
display:flex;
align-items:center;
justify-content:center;
padding:24px;
-webkit-font-smoothing:antialiased;
}
.container{
width:100%;
max-width:400px;
display:flex;
flex-direction:column;
align-items:center;
gap:32px;
}
.logo-block{
display:flex;
flex-direction:column;
align-items:center;
gap:16px;
}
.logo svg{color:var(--logo)}
.title{
font-size:16px;
font-weight:600;
letter-spacing:-0.01em;
color:var(--fg);
}
.subtitle{
font-size:11px;
color:var(--muted);
text-align:center;
line-height:1.6;
max-width:320px;
}
.card{
width:100%;
background:var(--card);
border:1px solid var(--border);
border-radius:var(--radius);
overflow:hidden;
}
.card-row{
display:flex;
align-items:center;
justify-content:space-between;
padding:10px 14px;
border-bottom:1px solid var(--border);
}
.card-row:last-child{border-bottom:none}
.card-label{
font-size:11px;
color:var(--muted);
text-transform:uppercase;
letter-spacing:0.04em;
}
.card-value{
font-size:12px;
color:var(--fg);
font-family:'SF Mono',Monaco,Consolas,'Liberation Mono','Courier New',monospace;
}
.badge{
display:inline-flex;
align-items:center;
gap:5px;
font-size:10px;
font-weight:500;
padding:2px 8px;
border-radius:99px;
}
.badge-ok{background:rgba(34,197,94,0.15);color:#4ade80}
.badge-err{background:rgba(239,68,68,0.15);color:#f87171}
.badge-loading{background:rgba(255,255,255,0.06);color:var(--muted)}
.badge-dot{
width:6px;height:6px;border-radius:50%;
animation:pulse 2s ease-in-out infinite;
}
.badge-dot-ok{background:#4ade80}
.badge-dot-err{background:#f87171}
.badge-dot-loading{background:var(--muted)}
@keyframes pulse{
0%,100%{opacity:1}
50%{opacity:0.4}
}
.endpoints{width:100%}
.endpoints-title{
font-size:10px;
color:var(--muted);
text-transform:uppercase;
letter-spacing:0.06em;
margin-bottom:8px;
padding-left:2px;
}
.endpoint{
display:flex;
align-items:center;
gap:10px;
padding:8px 14px;
background:var(--card);
border:1px solid var(--border);
border-radius:var(--radius);
margin-bottom:6px;
}
.endpoint:last-child{margin-bottom:0}
.method{
font-size:9px;
font-weight:700;
letter-spacing:0.05em;
padding:2px 6px;
border-radius:3px;
background:rgba(255,255,255,0.06);
color:var(--muted);
font-family:'SF Mono',Monaco,Consolas,monospace;
flex-shrink:0;
}
.endpoint-path{
font-size:12px;
font-family:'SF Mono',Monaco,Consolas,'Liberation Mono','Courier New',monospace;
color:var(--fg);
}
.endpoint-desc{
font-size:10px;
color:var(--muted);
margin-left:auto;
}
.security{width:100%}
.security-title{
font-size:10px;
color:var(--muted);
text-transform:uppercase;
letter-spacing:0.06em;
margin-bottom:8px;
padding-left:2px;
}
.security-grid{
display:grid;
grid-template-columns:1fr 1fr;
gap:6px;
}
.sec-item{
display:flex;
align-items:center;
gap:6px;
padding:8px 10px;
background:var(--card);
border:1px solid var(--border);
border-radius:var(--radius);
font-size:10px;
color:var(--muted);
}
.links{
display:flex;
gap:16px;
}
.links a{
font-size:10px;
color:var(--muted);
text-decoration:none;
transition:color 0.15s;
}
.links a:hover{color:var(--fg)}
.footer{
font-size:10px;
color:var(--muted);
opacity:0.5;
}
</style>
</head>
<body>
<div class="container">

<div class="logo-block">
<div class="logo">
<svg width="40" height="40" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg">
<circle cx="18" cy="18" r="3" fill="currentColor"/>
<circle cx="18" cy="9" r="3" fill="currentColor"/>
<circle cx="26" cy="13.5" r="3" fill="currentColor"/>
<circle cx="26" cy="22.5" r="3" fill="currentColor"/>
<circle cx="18" cy="27" r="3" fill="currentColor"/>
<circle cx="10" cy="22.5" r="3" fill="currentColor"/>
<circle cx="10" cy="13.5" r="3" fill="currentColor"/>
</svg>
</div>
<div class="title">KarmaGate Relay</div>
<div class="subtitle">Lightweight, stateless WebSocket relay server for KarmaGate Bind.<br>End-to-end encrypted collaboration and voice chat for security teams.</div>
</div>

<div class="card">
<div class="card-row">
<span class="card-label">Status</span>
<span id="status" class="badge badge-loading"><span id="dot" class="badge-dot badge-dot-loading"></span><span id="status-text">Checking</span></span>
</div>
<div class="card-row">
<span class="card-label">Protocol</span>
<span class="card-value">WebSocket (TLS 1.3)</span>
</div>
<div class="card-row">
<span class="card-label">Encryption</span>
<span class="card-value">XChaCha20-Poly1305 E2E</span>
</div>
<div class="card-row">
<span class="card-label">Auth</span>
<span class="card-value">Ed25519 JWT</span>
</div>
<div class="card-row">
<span class="card-label">Voice</span>
<span class="card-value">Opus E2E (XChaCha20)</span>
</div>
</div>

<div class="endpoints">
<div class="endpoints-title">Endpoints</div>
<div class="endpoint">
<span class="method">GET</span>
<span class="endpoint-path">/health</span>
<span class="endpoint-desc">Health check</span>
</div>
<div class="endpoint">
<span class="method">WS</span>
<span class="endpoint-path">/ws</span>
<span class="endpoint-desc">Data &amp; voice relay</span>
</div>
</div>

<div class="security">
<div class="security-title">Security</div>
<div class="security-grid">
<div class="sec-item">E2E Encrypted</div>
<div class="sec-item">Host-signed JWT</div>
<div class="sec-item">Per-IP Rate Limit</div>
<div class="sec-item">Ed25519 Signed</div>
<div class="sec-item">Opus Voice E2E</div>
<div class="sec-item">Binary Voice Frames</div>
</div>
</div>

<div class="links">
<a href="https://karmagate.com">Website</a>
<a href="https://docs.karmagate.com">Documentation</a>
<a href="https://github.com/Karmagate/KarmaGateRelay">GitHub</a>
<a href="https://t.me/karmagate">Telegram</a>
</div>

<div class="footer">&copy; KarmaGate</div>

</div>
<script>
(function(){
var s=document.getElementById('status'),d=document.getElementById('dot'),t=document.getElementById('status-text');
function check(){
fetch('/health').then(function(r){return r.json()}).then(function(j){
if(j.status==='ok'){s.className='badge badge-ok';d.className='badge-dot badge-dot-ok';t.textContent='Online'}
else{fail()}
}).catch(fail);
}
function fail(){s.className='badge badge-err';d.className='badge-dot badge-dot-err';t.textContent='Offline'}
check();setInterval(check,30000);
})();
</script>
</body>
</html>`

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(indexHTML))
}
