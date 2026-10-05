const $=s=>document.querySelector(s);let state={media:[],libs:[],system:null,user:null};
async function api(path,opt={}){const r=await fetch(path,{headers:{'Content-Type':'application/json',...(opt.headers||{})},...opt});if(r.status===401){loginView();throw new Error('unauthorized')}const j=await r.json().catch(()=>({}));if(!r.ok)throw new Error(j.error||'Fehler');return j}
function esc(s=''){return String(s).replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]))}
function toast(t){let x=document.createElement('div');x.className='toast';x.textContent=t;document.body.append(x);setTimeout(()=>x.remove(),2500)}
function setupView(info={}){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card setup-card" id="setup"><div class="brand"><span class="mark">▶</span> BuddyFlix</div><div class="setup-step">ERSTEINRICHTUNG</div><h1>Mach es zu deinem Server.</h1><div class="muted">Einmal kurz einrichten, danach gehört die Standard-Anmeldung der Vergangenheit an.</div><label class="field">Servername<input name="server_name" value="${esc(info.server_name||'BuddyFlix')}"></label><label class="field">Admin-Benutzer<input name="admin_user" value="admin" autocomplete="username"></label><label class="field">Admin-Passwort<input name="password" type="password" minlength="8" autocomplete="new-password"></label><label class="field">Passwort wiederholen<input name="repeat" type="password" minlength="8" autocomplete="new-password"></label><label class="field">TMDb API-Key <span class="muted">(optional)</span><input name="tmdb_api_key" placeholder="Kann auch später gesetzt werden"></label><button class="btn primary">BuddyFlix einrichten</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#setup').onsubmit=async e=>{e.preventDefault();if(e.target.password.value!==e.target.repeat.value){$('#err').textContent='Die Passwörter stimmen nicht überein.';return}try{await api('/api/setup',{method:'POST',body:JSON.stringify({server_name:e.target.server_name.value,admin_user:e.target.admin_user.value,password:e.target.password.value,tmdb_api_key:e.target.tmdb_api_key.value})});toast('Einrichtung abgeschlossen');loginView()}catch(x){$('#err').textContent=x.message}}}
function loginView(){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card" id="login"><div class="brand"><span class="mark">▶</span> BuddyFlix</div><h1>Willkommen zurück.</h1><div class="muted">Dein schlanker Media Server.</div><label class="field">Benutzer<input name="u" placeholder="Benutzername" autocomplete="username"></label><label class="field">Passwort<input name="p" type="password" autocomplete="current-password"></label><button class="btn primary">Anmelden</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#login').onsubmit=async e=>{e.preventDefault();try{await api('/api/login',{method:'POST',body:JSON.stringify({Username:e.target.u.value,Password:e.target.p.value})});await boot()}catch(x){$('#err').textContent=x.message}}}
function shell(){document.querySelector('#app').innerHTML=`<div class="shell"><aside class="side"><div class="brand"><span class="mark">▶</span> BuddyFlix</div><nav class="nav"><button data-v="home" class="active">⌂ Startseite</button><button data-v="movies">▣ Filme</button><button data-v="admin">⚙ Verwaltung</button></nav><div class="sidefoot">BuddyFlix v0.1.0<br>ARMHF-first Media Server</div></aside><main class="main"><header class="top"><input id="search" class="search" placeholder="Filme durchsuchen…"><span class="status"><i class="dot"></i> Server online</span><button class="btn ghost" id="logout">Abmelden</button></header><div id="content" class="content"></div></main></div>`;document.querySelectorAll('.nav button').forEach(b=>b.onclick=()=>view(b.dataset.v,b));$('#logout').onclick=async()=>{await api('/api/logout',{method:'POST'});loginView()};let timer;$('#search').oninput=e=>{clearTimeout(timer);timer=setTimeout(async()=>{state.media=await api('/api/media?q='+encodeURIComponent(e.target.value));renderMovies()},180)}}
async function boot(){try{const setup=await fetch('/api/setup/status').then(r=>r.json());if(!setup.setup_done){setupView(setup);return}state.system=await api('/api/system');state.media=await api('/api/media');state.libs=await api('/api/libraries');shell();renderHome()}catch(e){if(e.message!=='unauthorized')loginView()}}
function view(v,b){document.querySelectorAll('.nav button').forEach(x=>x.classList.toggle('active',x===b));if(v==='home')renderHome();if(v==='movies')renderMovies();if(v==='admin')renderAdmin()}
function card(m){return `<article class="card" onclick="detail(${m.id})"><div class="poster">${m.poster?`<img loading="lazy" src="${esc(m.poster)}">`:`<div class="placeholder">▶</div>`}${m.progress>0&&m.progress<95?`<div class="progress"><i style="width:${Math.min(100,m.progress)}%"></i></div>`:''}</div><div class="card-title">${esc(m.title)}</div><div class="card-meta">${m.year||'Film'}${m.progress>=95?' · ✓ gesehen':''}</div></article>`}
function renderHome(){let cont=state.media.filter(m=>m.progress>1&&m.progress<95).slice(0,8),hero=state.media.find(m=>m.backdrop)||state.media[0];$('#content').innerHTML=`${hero?`<section class="hero"><div class="hero-bg" style="background-image:url('${esc(hero.backdrop||hero.poster||'')}')"></div><div class="hero-copy"><div class="muted">BUDDYFLIX EMPFIEHLT</div><h1>${esc(hero.title)}</h1><p>${esc(hero.overview||'Deine Medienbibliothek auf dem QNAP – direkt, schnell und ohne Ballast.')}</p><button class="btn primary" onclick="play(${hero.id})">▶ Abspielen</button></div></section>`:''}${cont.length?`<div class="rowhead"><h2>Weiterschauen</h2></div><div class="grid">${cont.map(card).join('')}</div>`:''}<div class="rowhead"><h2>Neu hinzugefügt</h2><span class="muted">${state.media.length} Titel</span></div><div class="grid">${state.media.slice(0,18).map(card).join('')||'<div class="muted">Noch keine Medien. Lege unter Verwaltung eine Bibliothek an.</div>'}</div>`}
function renderMovies(){if(!$('#content'))return;$('#content').innerHTML=`<div class="rowhead"><h2>Filme</h2><span class="muted">${state.media.length} Titel</span></div><div class="grid">${state.media.map(card).join('')||'<div class="muted">Keine Treffer.</div>'}</div>`}
async function renderAdmin(){
  state.system=await api('/api/system');
  state.libs=await api('/api/libraries');
  state.settings=await api('/api/settings');
  let s=state.system, cfg=state.settings;
  $('#content').innerHTML=`
  <div class="rowhead"><div><h2>Verwaltung</h2><div class="muted">Server, Bibliotheken und Zugriff verwalten</div></div><div class="toolbar"><button class="btn primary" id="scan">Bibliotheken scannen</button></div></div>
  <div class="stats">
    <div class="stat"><b>${s.media}</b><span>Medien</span></div>
    <div class="stat"><b>${s.libraries}</b><span>Bibliotheken</span></div>
    <div class="stat"><b>${s.cpus}</b><span>CPU Threads</span></div>
    <div class="stat"><b>${s.memory_mb} MB</b><span>BuddyFlix RAM</span></div>
  </div>

  <div class="admin-grid">
    <section class="panel">
      <div class="panel-title"><div><h3>Server</h3><p class="muted">Grundlegende Identität deines BuddyFlix-Servers.</p></div><span class="badge">V${esc(s.version)}</span></div>
      <form id="settingsform">
        <label class="field">Servername<input name="server_name" value="${esc(cfg.server_name)}"></label>
        <label class="field">Admin-Benutzer<input name="admin_user" value="${esc(cfg.admin_user)}"></label>
        <div class="kv"><span>Server-ID</span><code>${esc(cfg.server_id)}</code></div>
        <div class="kv"><span>Listen-Adresse</span><code>${esc(cfg.listen)}</code></div>
        <div class="kv"><span>Datenordner</span><code>${esc(cfg.data_dir)}</code></div>
        <button class="btn primary">Einstellungen speichern</button>
      </form>
    </section>

    <section class="panel">
      <div class="panel-title"><div><h3>Sicherheit</h3><p class="muted">Admin-Passwort ändern.</p></div></div>
      <form id="passwordform">
        <label class="field">Aktuelles Passwort<input name="current" type="password" autocomplete="current-password"></label>
        <label class="field">Neues Passwort<input name="next" type="password" minlength="8" autocomplete="new-password"></label>
        <label class="field">Neues Passwort wiederholen<input name="repeat" type="password" minlength="8" autocomplete="new-password"></label>
        <button class="btn primary">Passwort ändern</button>
      </form>
      <div class="notice">Nach dem Passwortwechsel werden alle laufenden Sitzungen beendet.</div>
    </section>

    <section class="panel span-2">
      <div class="panel-title"><div><h3>Bibliotheken</h3><p class="muted">Medienpfade auf dem NAS hinzufügen, bearbeiten und scannen.</p></div><span class="badge">${state.libs.length}</span></div>
      <div class="library-list">
        ${state.libs.map(l=>`<div class="library-item"><div><strong>${esc(l.name)}</strong><div class="muted">${esc(l.type)} · ${esc(l.path)}</div></div><div class="toolbar"><button class="btn ghost" onclick="editLib(${l.id})">Bearbeiten</button><button class="btn danger" onclick="removeLib(${l.id})">Entfernen</button></div></div>`).join('')||'<div class="empty">Noch keine Bibliothek angelegt.</div>'}
      </div>
      <form id="libform" class="inline-form">
        <label class="field">Name<input name="name" placeholder="Filme"></label>
        <label class="field">Typ<select name="type"><option value="movies">Filme</option><option value="shows">Serien</option><option value="other">Andere Videos</option></select></label>
        <label class="field grow">QNAP-Pfad<input name="path" placeholder="/share/Multimedia/Filme"></label>
        <button class="btn primary">Hinzufügen</button>
      </form>
    </section>

    <section class="panel span-2">
      <div class="panel-title"><div><h3>Medienverwaltung</h3><p class="muted">Titel prüfen, Status korrigieren und verwaiste Einträge bereinigen.</p></div><div class="toolbar"><button class="btn ghost" id="filtermeta">Ohne Metadaten</button><button class="btn ghost" id="filtermissing">Fehlende Dateien</button><button class="btn danger" id="cleanupmissing">Fehlende bereinigen</button></div></div>
      <div class="media-admin" id="medialist">
        ${adminMediaRows(state.media)}
      </div>
    </section>

    <section class="panel">
      <div class="panel-title"><div><h3>Metadaten</h3><p class="muted">Poster, Backdrops und Beschreibungen.</p></div><span class="badge ${cfg.tmdb_configured?'ok':'warn'}">${cfg.tmdb_configured?'aktiv':'nicht konfiguriert'}</span></div>
      <form id="tmdbform">
        <label class="field">TMDb API-Key<input name="tmdb_api_key" type="password" placeholder="${cfg.tmdb_configured?'API-Key ist gesetzt':'API-Key eintragen'}"></label>
        <div class="toolbar"><button class="btn primary">Speichern</button>${cfg.tmdb_configured?'<button type="button" class="btn danger" id="cleartmdb">API-Key entfernen</button>':''}</div>
      </form>
      <p class="muted">Wird lokal in deiner BuddyFlix-Konfiguration gespeichert und nicht im Repository.</p>
    </section>

    <section class="panel">
      <div class="panel-title"><div><h3>System</h3><p class="muted">Aktueller Serverzustand.</p></div><span class="badge ${s.scanning?'warn':'ok'}">${s.scanning?'Scan läuft':'bereit'}</span></div>
      <table class="table">
        <tr><td>Plattform</td><td>${s.os}/${s.arch}</td></tr>
        <tr><td>Go</td><td>${s.go}</td></tr>
        <tr><td>Uptime</td><td>${Math.floor(s.uptime_sec/60)} min</td></tr>
        <tr><td>Goroutines</td><td>${s.goroutines}</td></tr>
        <tr><td>Persistenz</td><td>${esc(s.storage)}</td></tr>
      </table>
    </section>
  </div>`;

  $('#scan').onclick=doScan;
  $('#libform').onsubmit=addLib;
  $('#settingsform').onsubmit=saveSettings;
  $('#passwordform').onsubmit=changePassword;
  $('#tmdbform').onsubmit=saveTMDb;
  const clear=$('#cleartmdb'); if(clear) clear.onclick=clearTMDb;
  $('#filtermeta').onclick=()=>renderAdminMedia('metadata');
  $('#filtermissing').onclick=()=>renderAdminMedia('missing');
  $('#cleanupmissing').onclick=cleanupMissing;
}
function adminMediaRows(items){if(!items.length)return '<div class="empty">Keine Medien vorhanden.</div>';return items.map(m=>`<div class="media-admin-row ${m.missing?'is-missing':''}"><div class="media-admin-poster">${m.poster?`<img src="${esc(m.poster)}">`:'▶'}</div><div class="media-admin-main"><strong>${esc(m.title)}</strong><div class="muted">${m.year||'ohne Jahr'} · ${m.overview?'Metadaten vorhanden':'ohne Metadaten'}${m.missing?' · Datei fehlt':''}</div><div class="pathline">${esc(m.path)}</div></div><div class="toolbar"><button class="btn ghost" onclick="identifyMediaById(${m.id})">Identifizieren</button><button class="btn ghost" onclick="editMediaById(${m.id})">Bearbeiten</button><button class="btn ghost" onclick="mediaAction(${m.id},'${m.progress>=95?'mark_unwatched':'mark_watched'}')">${m.progress>=95?'Ungesehen':'Gesehen'}</button><button class="btn ghost" onclick="mediaAction(${m.id},'reset_progress')">Fortschritt 0</button></div></div>`).join('')}
function renderAdminMedia(mode='all'){let items=state.media;if(mode==='metadata')items=items.filter(m=>!m.poster||!m.overview);if(mode==='missing')items=items.filter(m=>m.missing);$('#medialist').innerHTML=adminMediaRows(items)}
async function mediaAction(id,action){try{await api('/api/media/action',{method:'POST',body:JSON.stringify({media_id:id,action})});await refresh();renderAdminMedia();toast('Medienstatus aktualisiert')}catch(x){toast(x.message)}}
async function cleanupMissing(){if(!confirm('Alle Einträge entfernen, deren Mediendatei beim letzten Scan nicht mehr gefunden wurde? Die Dateien selbst werden nicht gelöscht.'))return;try{let r=await api('/api/media/cleanup',{method:'POST'});toast(r.removed+' verwaiste Einträge entfernt');await refresh();renderAdmin()}catch(x){toast(x.message)}}
async function editMediaById(id){let a=await api('/api/media?id='+id);if(a[0])editMedia(a[0])}
async function identifyMediaById(id){let a=await api('/api/media?id='+id);if(a[0])identifyMedia(a[0])}
async function identifyMedia(m){
  let el=document.createElement('div');el.className='modal';
  el.innerHTML=`<div class="modalbox"><div class="panel"><div class="panel-title"><div><h3>Medium identifizieren</h3><p class="muted">TMDb durchsuchen und den richtigen Treffer auswählen.</p></div><button class="close">×</button></div><form id="identifyform" class="identify-form"><label class="field grow">Titel<input name="q" value="${esc(m.title)}"></label><label class="field">Jahr<input name="year" type="number" value="${m.year||''}"></label><button class="btn primary">Suchen</button></form><div id="identifyresults" class="tmdb-results"><div class="empty">Noch keine Suche gestartet.</div></div></div></div>`;
  document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();
  el.querySelector('#identifyform').onsubmit=async e=>{e.preventDefault();let box=el.querySelector('#identifyresults');box.innerHTML='<div class="empty">TMDb wird durchsucht…</div>';try{let results=await api('/api/metadata/search?q='+encodeURIComponent(e.target.q.value)+'&year='+encodeURIComponent(e.target.year.value));box.innerHTML=results.length?results.map((r,i)=>`<div class="tmdb-result"><div class="tmdb-thumb">${r.poster?`<img src="${esc(r.poster)}">`:'▶'}</div><div><strong>${esc(r.title)}</strong><div class="muted">${r.year||'ohne Jahr'}</div><p>${esc(r.overview||'Keine Beschreibung vorhanden.')}</p></div><button class="btn primary" data-pick="${i}">Übernehmen</button></div>`).join(''):'<div class="empty">Keine Treffer.</div>';box.querySelectorAll('[data-pick]').forEach(b=>b.onclick=async()=>{let r=results[Number(b.dataset.pick)];try{await api('/api/metadata/apply',{method:'POST',body:JSON.stringify({media_id:m.id,result:r})});toast('TMDb-Treffer übernommen');el.remove();await refresh();renderAdmin()}catch(x){toast(x.message)}})}catch(x){box.innerHTML='<div class="empty">'+esc(x.message)+'</div>'}}
}
async function addLib(e){e.preventDefault();try{await api('/api/libraries',{method:'POST',body:JSON.stringify({name:e.target.name.value,path:e.target.path.value,type:e.target.type.value})});toast('Bibliothek hinzugefügt');e.target.reset();await refresh();renderAdmin()}catch(x){toast(x.message)}}
async function removeLib(id){if(!confirm('Bibliothek und Index entfernen? Die Mediendateien bleiben unangetastet.'))return;await api('/api/libraries?id='+id,{method:'DELETE'});await refresh();renderAdmin()}
async function editLib(id){let l=state.libs.find(x=>x.id===id);if(!l)return;let el=document.createElement('div');el.className='modal';el.innerHTML=`<div class="modalbox compact"><div class="panel"><div class="panel-title"><h3>Bibliothek bearbeiten</h3><button class="close">×</button></div><form id="editlib"><label class="field">Name<input name="name" value="${esc(l.name)}"></label><label class="field">Typ<select name="type"><option value="movies" ${l.type==='movies'?'selected':''}>Filme</option><option value="shows" ${l.type==='shows'?'selected':''}>Serien</option><option value="other" ${l.type==='other'?'selected':''}>Andere Videos</option></select></label><label class="field">Pfad<input name="path" value="${esc(l.path)}"></label><button class="btn primary">Änderungen speichern</button></form></div></div>`;document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();el.querySelector('#editlib').onsubmit=async e=>{e.preventDefault();try{await api('/api/libraries?id='+id,{method:'PUT',body:JSON.stringify({name:e.target.name.value,path:e.target.path.value,type:e.target.type.value})});toast('Bibliothek aktualisiert');el.remove();await refresh();renderAdmin()}catch(x){toast(x.message)}}}
async function saveSettings(e){e.preventDefault();try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:e.target.server_name.value,admin_user:e.target.admin_user.value})});toast('Servereinstellungen gespeichert');renderAdmin()}catch(x){toast(x.message)}}
async function saveTMDb(e){e.preventDefault();const key=e.target.tmdb_api_key.value.trim();if(!key){toast('Bitte einen API-Key eintragen');return}try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tmdb_api_key:key})});toast('TMDb API-Key gespeichert');renderAdmin()}catch(x){toast(x.message)}}
async function clearTMDb(){if(!confirm('TMDb API-Key wirklich entfernen?'))return;try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tmdb_api_key:''})});toast('TMDb API-Key entfernt');renderAdmin()}catch(x){toast(x.message)}}
async function changePassword(e){e.preventDefault();if(e.target.next.value!==e.target.repeat.value){toast('Die neuen Passwörter stimmen nicht überein');return}try{await api('/api/password',{method:'POST',body:JSON.stringify({current:e.target.current.value,new:e.target.next.value})});toast('Passwort geändert – bitte neu anmelden');setTimeout(loginView,700)}catch(x){toast(x.message)}}
async function doScan(){let b=$('#scan');b.disabled=true;b.textContent='Scan läuft…';try{let r=await api('/api/scan',{method:'POST'});toast(`${r.files_seen} Dateien gefunden`);await refresh();renderAdmin()}catch(x){toast(x.message)}finally{b.disabled=false}}
async function refresh(){state.media=await api('/api/media');state.libs=await api('/api/libraries');state.system=await api('/api/system')}
async function detail(id){let a=await api('/api/media?id='+id),m=a[0];if(!m)return;let el=document.createElement('div');el.className='modal';el.innerHTML=`<div class="modalbox"><div class="moviehero" style="background-image:url('${esc(m.backdrop||m.poster||'')}')"><button class="close">×</button><div class="moviecopy"><div class="muted">${m.year||''}</div><h1>${esc(m.title)}</h1><p>${esc(m.overview||'Für diesen Titel wurden noch keine Metadaten geladen.')}</p><div class="modalactions"><button class="btn primary" id="p">▶ Abspielen</button><button class="btn ghost" id="meta">Identifizieren</button><button class="btn ghost" id="editmedia">Bearbeiten</button></div></div></div></div>`;document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();el.onclick=e=>{if(e.target===el)el.remove()};el.querySelector('#p').onclick=()=>{el.remove();play(id)};el.querySelector('#meta').onclick=()=>{el.remove();identifyMedia(m)};el.querySelector('#editmedia').onclick=()=>{el.remove();editMedia(m)}}}
function editMedia(m){let el=document.createElement('div');el.className='modal';el.innerHTML=`<div class="modalbox compact"><div class="panel"><div class="panel-title"><div><h3>Medium bearbeiten</h3><p class="muted">Metadaten manuell korrigieren.</p></div><button class="close">×</button></div><form id="mediaedit"><label class="field">Titel<input name="title" value="${esc(m.title)}"></label><label class="field">Jahr<input name="year" type="number" min="1888" max="2100" value="${m.year||''}"></label><label class="field">Beschreibung<textarea name="overview" rows="5">${esc(m.overview||'')}</textarea></label><label class="field">Poster-URL<input name="poster" value="${esc(m.poster||'')}"></label><label class="field">Backdrop-URL<input name="backdrop" value="${esc(m.backdrop||'')}"></label><div class="notice"><strong>Datei:</strong><br><code>${esc(m.path)}</code></div><button class="btn primary">Speichern</button></form></div></div>`;document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();el.querySelector('#mediaedit').onsubmit=async e=>{e.preventDefault();try{await api('/api/media?id='+m.id,{method:'PUT',body:JSON.stringify({title:e.target.title.value,year:Number(e.target.year.value)||0,overview:e.target.overview.value,poster:e.target.poster.value,backdrop:e.target.backdrop.value})});toast('Medium gespeichert');el.remove();await refresh();renderMovies()}catch(x){toast(x.message)}}}
async function play(id){let a=await api('/api/media?id='+id),m=a[0];if(!m)return;let el=document.createElement('div');el.className='videoModal';el.innerHTML=`<video controls autoplay src="/stream/${id}"></video><button class="close">×</button>`;document.body.append(el);let v=el.querySelector('video'),last=0;v.onloadedmetadata=()=>{if(m.position>0&&m.duration>0&&m.position<m.duration-20)v.currentTime=m.position};v.ontimeupdate=()=>{if(Date.now()-last>10000){last=Date.now();saveProgress(id,v.currentTime,v.duration)}};v.onended=()=>saveProgress(id,v.duration,v.duration);el.querySelector('.close').onclick=()=>{saveProgress(id,v.currentTime,v.duration);v.pause();el.remove();refresh()}}
async function saveProgress(id,p,d){if(!Number.isFinite(d)||d<=0)return;try{await api('/api/progress',{method:'POST',body:JSON.stringify({media_id:id,Position:p,Duration:d})})}catch{}}
boot();
