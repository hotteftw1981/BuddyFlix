const $=s=>document.querySelector(s);let state={media:[],libs:[],system:null,user:null};let metadataPollTimer=null;
async function api(path,opt={}){const r=await fetch(path,{headers:{'Content-Type':'application/json',...(opt.headers||{})},...opt});if(r.status===401){loginView();throw new Error('unauthorized')}const j=await r.json().catch(()=>({}));if(!r.ok)throw new Error(j.error||'Fehler');return j}
function esc(s=''){return String(s).replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]))}
function toast(t){let x=document.createElement('div');x.className='toast';x.textContent=t;document.body.append(x);setTimeout(()=>x.remove(),2500)}
function setupView(info={}){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card setup-card" id="setup"><div class="brand brand-image"><img class="brand-logo" src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><div class="setup-step">ERSTEINRICHTUNG</div><h1>Mach es zu deinem Server.</h1><div class="muted">Einmal kurz einrichten, danach gehört die Standard-Anmeldung der Vergangenheit an.</div><label class="field">Servername<input name="server_name" value="${esc(info.server_name||'BuddyFlix')}"></label><label class="field">Admin-Benutzer<input name="admin_user" value="admin" autocomplete="username"></label><label class="field">Admin-Passwort<input name="password" type="password" minlength="8" autocomplete="new-password"></label><label class="field">Passwort wiederholen<input name="repeat" type="password" minlength="8" autocomplete="new-password"></label><div class="notice">Metadatenanbieter werden von BuddyFlix verwaltet. Für die normale Einrichtung ist kein eigener API-Key vorgesehen.</div><button class="btn primary">BuddyFlix einrichten</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#setup').onsubmit=async e=>{e.preventDefault();if(e.target.password.value!==e.target.repeat.value){$('#err').textContent='Die Passwörter stimmen nicht überein.';return}try{await api('/api/setup',{method:'POST',body:JSON.stringify({server_name:e.target.server_name.value,admin_user:e.target.admin_user.value,password:e.target.password.value})});toast('Einrichtung abgeschlossen');loginView()}catch(x){$('#err').textContent=x.message}}}
function loginView(){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card" id="login"><div class="brand brand-image"><img class="brand-logo" src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><h1>Willkommen zurück.</h1><div class="muted">Dein schlanker Media Server.</div><label class="field">Benutzer<input name="u" placeholder="Benutzername" autocomplete="username"></label><label class="field">Passwort<input name="p" type="password" autocomplete="current-password"></label><button class="btn primary">Anmelden</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#login').onsubmit=async e=>{e.preventDefault();try{await api('/api/login',{method:'POST',body:JSON.stringify({Username:e.target.u.value,Password:e.target.p.value})});await boot()}catch(x){$('#err').textContent=x.message}}}
function shell(){document.querySelector('#app').innerHTML=`<div class="shell"><aside class="side"><div class="brand brand-image"><img class="brand-logo" src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><nav class="nav"><button data-v="home" class="active">⌂ Startseite</button><button data-v="movies">▣ Filme</button><button data-v="admin">⚙ Verwaltung</button></nav><div class="sidefoot">${esc(state.system?.server_name||'BuddyFlix')}<br>BuddyFlix v${esc(state.system?.version||'dev')} · ARMHF-first</div></aside><main class="main"><header class="top"><input id="search" class="search" placeholder="Filme durchsuchen…"><span class="status"><i class="dot"></i> Server online</span><button class="btn ghost" id="logout">Abmelden</button></header><div id="content" class="content"></div></main></div>`;document.querySelectorAll('.nav button').forEach(b=>b.onclick=()=>view(b.dataset.v,b));$('#logout').onclick=async()=>{await api('/api/logout',{method:'POST'});loginView()};let timer;$('#search').oninput=e=>{clearTimeout(timer);timer=setTimeout(async()=>{state.media=await api('/api/media?q='+encodeURIComponent(e.target.value));renderMovies()},180)}}
async function boot(){try{const setup=await fetch('/api/setup/status').then(r=>r.json());if(!setup.setup_done){setupView(setup);return}state.system=await api('/api/system');state.media=await api('/api/media');state.libs=await api('/api/libraries');shell();renderHome()}catch(e){if(e.message!=='unauthorized')loginView()}}
async function view(v,b){document.querySelectorAll('.nav button').forEach(x=>x.classList.toggle('active',x===b));try{if(v==='home')renderHome();if(v==='movies')renderMovies();if(v==='admin')await renderAdmin()}catch(e){const box=$('#content');if(box)box.innerHTML='<section class="panel"><h2>Verwaltung konnte nicht geladen werden</h2><p class="muted">'+esc(e.message||'Unbekannter Fehler')+'</p><button class="btn primary" onclick="location.reload()">Neu laden</button></section>';toast('Fehler beim Laden der Verwaltung')}}
function card(m){let pct=mediaProgress(m),resumable=m.position>20&&m.duration>0&&pct<95;return `<article class="card" onclick="detail(${m.id})"><div class="poster">${m.poster?`<img loading="lazy" src="${esc(m.poster)}" alt="">`:`<div class="placeholder"><span>▶</span><small>Kein Poster</small></div>`}<div class="poster-shade"></div><button class="poster-play" aria-label="${resumable?'Fortsetzen':'Abspielen'}" onclick="event.stopPropagation();play(${m.id},'${resumable?'resume':'start'}')">▶</button>${m.progress>0&&m.progress<95?`<div class="progress"><i style="width:${Math.min(100,m.progress)}%"></i></div>`:''}${m.progress>=95?'<span class="watched-mark">✓</span>':''}</div><div class="card-title">${esc(m.title)}</div><div class="card-meta"><span>${m.year||'Film'}</span>${resumable?`<span>${Math.round(pct)}%</span>`:m.progress>=95?'<span>Gesehen</span>':''}</div></article>`}

function homeSection(title,items,subtitle=''){if(!items.length)return '';return `<section class="home-section"><div class="rowhead"><div><h2>${esc(title)}</h2>${subtitle?`<div class="row-subtitle">${esc(subtitle)}</div>`:''}</div><span class="rail-count">${items.length}</span></div><div class="media-rail">${items.map(card).join('')}</div></section>`}

function pickSomething(){let pool=state.media.filter(m=>!m.missing);if(!pool.length){toast('Noch keine Filme vorhanden');return}let unseen=pool.filter(m=>mediaProgress(m)<1),choices=unseen.length?unseen:pool,m=choices[Math.floor(Math.random()*choices.length)];detail(m.id)}

function renderHome(){
  let continueItems=state.media.filter(m=>m.progress>1&&m.progress<95).slice(0,12);
  let newest=state.media.slice(0,18);
  let unseen=state.media.filter(m=>mediaProgress(m)<1).slice(0,14);
  let hero=continueItems.find(m=>m.backdrop)||state.media.find(m=>m.backdrop)||state.media.find(m=>m.poster)||state.media[0];
  let heroPct=hero?mediaProgress(hero):0,heroResume=hero&&hero.position>20&&hero.duration>0&&heroPct<95;
  $('#content').innerHTML=`
    ${hero?`<section class="hero hero-cinematic">
      <div class="hero-bg" style="background-image:url('${esc(hero.backdrop||hero.poster||'')}')"></div>
      <div class="hero-vignette"></div>
      <div class="hero-copy">
        <div class="hero-kicker">HEUTE AUF BUDDYFLIX</div>
        <h1>${esc(hero.title)}</h1>
        <div class="hero-meta"><span>${hero.year||'Film'}</span>${heroResume?`<span>${Math.round(heroPct)}% angesehen</span>`:hero.progress>=95?'<span>✓ Gesehen</span>':''}</div>
        <p>${esc(hero.overview||'Deine eigene Filmbibliothek. Direkt vom NAS, ohne Umwege und ohne Ballast.')}</p>
        <div class="hero-actions">
          <button class="btn primary hero-play" onclick="play(${hero.id},'${heroResume?'resume':'start'}')">▶ ${heroResume?'Fortsetzen':'Abspielen'}</button>
          <button class="btn hero-info" onclick="detail(${hero.id})">ⓘ Details</button>
          <button class="btn hero-random" onclick="pickSomething()">⤨ Was guck ich heute?</button>
        </div>
        ${heroResume?`<div class="hero-progress"><i style="width:${heroPct}%"></i></div>`:''}
      </div>
    </section>`:''}
    ${homeSection('Weiterschauen',continueItems,'Genau da weitermachen, wo du aufgehört hast')}
    ${homeSection('Neu hinzugefügt',newest,`${state.media.length} Titel in deiner Bibliothek`)}
    ${homeSection('Noch nicht angesehen',unseen,'Vielleicht ist heute Abend ja was dabei')}
    ${!state.media.length?'<div class="empty home-empty">Noch keine Medien. Lege unter Verwaltung eine Bibliothek an.</div>':''}
  `
}
function renderMovies(){if(!$('#content'))return;$('#content').innerHTML=`<div class="rowhead"><h2>Filme</h2><span class="muted">${state.media.length} Titel</span></div><div class="grid">${state.media.map(card).join('')||'<div class="muted">Keine Treffer.</div>'}</div>`}
async function renderAdmin(){
  const content=$('#content');
  if(content) content.innerHTML='<div class="muted">Verwaltung wird geladen…</div>';
  state.system=await api('/api/system');
  state.libs=await api('/api/libraries');
  state.media=await api('/api/media?include_missing=1');
  state.settings=await api('/api/settings');
  state.metadataJob=await api('/api/metadata/bulk');
  state.metadataProviders=await api('/api/metadata/providers');
  let s=state.system, cfg=state.settings;
  let reviewCount=state.media.filter(m=>m.metadata_state==='review'&&m.pending_metadata).length;
  let providerBadges=(state.metadataProviders||[]).map(p=>{let src=p.credential_source==='builtin'?'BuddyFlix':p.credential_source==='override'?'eigener Override':p.credential_source==='environment'?'Server-Umgebung':'';let name=p.name==='thetvdb'?'TheTVDB':p.name.toUpperCase();return `<span class="badge ${p.configured?'ok':'warn'}">${esc(name)} · ${p.configured?(src||'aktiv'):'nicht konfiguriert'}</span>`}).join(' ');
  let tvdbSource=cfg.tvdb_credential_source||'missing';
  let tvdbStatus=tvdbSource==='builtin'?'BuddyFlix-TheTVDB-Zugang aktiv':tvdbSource==='override'?'Eigener TheTVDB-Override aktiv':tvdbSource==='environment'?'TheTVDB-Zugang über Server-Umgebung aktiv':'TheTVDB-Projektzugang fehlt in diesem Build';
  $('#content').innerHTML=`
  <div class="rowhead"><div><h2>Verwaltung</h2><div class="muted">Server, Bibliotheken und Zugriff verwalten</div></div><div class="toolbar"><button class="btn primary" id="scan">Bibliotheken scannen</button></div></div>
  <div class="stats">
    <div class="stat"><b>${s.media}</b><span>Vorhandene Medien</span></div>
    <div class="stat"><b>${s.missing_media||0}</b><span>Fehlende Einträge</span></div>
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
      <div class="panel-title"><div><h3>Metadaten</h3><p class="muted">Poster, Backdrops und Beschreibungen.</p></div><span class="badge ${cfg.tvdb_configured?'ok':'warn'}">${cfg.tvdb_configured?'bereit':'nicht bereit'}</span></div>
      <div class="notice"><strong>${esc(tvdbStatus)}</strong><br>${tvdbSource==='missing'?'Dieser Build enthält noch kein TheTVDB-Projektcredential.':'Normale Benutzer müssen keinen eigenen API-Key eintragen.'}</div>
      <div class="provider-attribution">Metadata provided by <a href="https://thetvdb.com" target="_blank" rel="noreferrer">TheTVDB</a>. Please consider adding missing information or subscribing.</div>
      <details class="provider-advanced">
        <summary>Expertenoption: eigenen TheTVDB-Key verwenden</summary>
        <form id="tvdbform">
          <label class="field">TheTVDB API-Key<input name="tvdb_api_key" type="password" placeholder="${cfg.tvdb_override?'Eigener Override ist gesetzt':'Optionaler eigener Override'}"></label>
          <div class="toolbar"><button class="btn ghost">Override speichern</button>${cfg.tvdb_override?'<button type="button" class="btn danger" id="cleartvdb">Override entfernen</button>':''}</div>
        </form>
        <p class="muted">Ein eigener Key überschreibt nur auf diesem Server den BuddyFlix-Projektzugang.</p>
      </details>
      <div class="metadata-auto">
        <div class="metadata-auto-head"><div><strong>BuddyFlix Metadata Engine</strong><div class="muted">TheTVDB ist der primäre Provider. Eindeutige Treffer werden übernommen, unsichere landen bei „Bitte prüfen“.</div><div class="provider-badges">${providerBadges}</div></div><span class="badge ${reviewCount?'warn':'ok'}">${reviewCount} zu prüfen</span></div>
        <div class="toolbar"><button type="button" class="btn primary" id="bulkmeta" ${cfg.tvdb_configured?'':'disabled'}>Metadaten automatisch laden</button><button type="button" class="btn ghost" id="filterreview">Bitte prüfen (${reviewCount})</button></div>
        <div class="metadata-job" id="metajob">${metadataJobMarkup(state.metadataJob)}</div>
      </div>
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
  const tvdbForm=$('#tvdbform'); if(tvdbForm) tvdbForm.onsubmit=saveTVDB;
  const clearTVDBButton=$('#cleartvdb'); if(clearTVDBButton) clearTVDBButton.onclick=clearTVDB;
  $('#filtermeta').onclick=()=>renderAdminMedia('metadata');
  $('#filtermissing').onclick=()=>renderAdminMedia('missing');
  $('#filterreview').onclick=()=>renderAdminMedia('review');
  $('#bulkmeta').onclick=startBulkMetadata;
  $('#cleanupmissing').onclick=cleanupMissing;
  if(state.metadataJob?.running) scheduleMetadataPoll();
}
function adminMediaRows(items){
  if(!items.length)return '<div class="empty">Keine Medien vorhanden.</div>';
  return items.map(m=>{
    let suggestion=m.pending_metadata;
    let suggested=suggestion?`<div class="metadata-suggestion"><span class="badge warn">Bitte prüfen · ${m.metadata_confidence||0}%</span><strong>${esc(suggestion.title)}</strong><span class="muted">${suggestion.year||'ohne Jahr'} · ${esc((suggestion.provider||'Quelle').toUpperCase())}</span></div>`:'';
    let ids=m.external_ids||{},idParts=[];if(ids.tvdb)idParts.push('TheTVDB '+esc(ids.tvdb));if(ids.tmdb)idParts.push('TMDb '+esc(ids.tmdb));if(ids.imdb)idParts.push('IMDb '+esc(ids.imdb));if(ids.wikidata)idParts.push('Wikidata '+esc(ids.wikidata));
    let source=m.metadata_provider?`<div class="metadata-source">Quelle: <strong>${esc(m.metadata_provider.toUpperCase())}</strong>${idParts.length?' · '+idParts.join(' · '):''}</div>`:'';
    return `<div class="media-admin-row ${m.missing?'is-missing':''} ${suggestion?'has-suggestion':''}"><div class="media-admin-poster">${m.poster?`<img src="${esc(m.poster)}">`:'▶'}</div><div class="media-admin-main"><strong>${esc(m.title)}</strong><div class="muted">${m.year||'ohne Jahr'} · ${m.overview?'Metadaten vorhanden':'ohne Metadaten'}${m.missing?' · Datei fehlt':''}</div>${source}${suggested}<div class="pathline">${esc(m.path)}</div></div><div class="toolbar">${suggestion?`<button class="btn primary" onclick="applyPendingMetadata(${m.id})">Vorschlag übernehmen</button>`:''}<button class="btn ghost" onclick="identifyMediaById(${m.id})">Identifizieren</button><button class="btn ghost" onclick="editMediaById(${m.id})">Bearbeiten</button><button class="btn ghost" onclick="mediaAction(${m.id},'${m.progress>=95?'mark_unwatched':'mark_watched'}')">${m.progress>=95?'Ungesehen':'Gesehen'}</button><button class="btn ghost" onclick="mediaAction(${m.id},'reset_progress')">Fortschritt 0</button></div></div>`;
  }).join('')
}
function renderAdminMedia(mode='all'){
  let items=state.media;
  if(mode==='metadata')items=items.filter(m=>!m.poster||!m.overview);
  if(mode==='missing')items=items.filter(m=>m.missing);
  if(mode==='review')items=items.filter(m=>m.metadata_state==='review'&&m.pending_metadata);
  $('#medialist').innerHTML=adminMediaRows(items)
}
async function applyPendingMetadata(id){
  let m=state.media.find(x=>x.id===id);
  if(!m?.pending_metadata){toast('Kein Vorschlag vorhanden');return}
  try{
    await api('/api/metadata/apply',{method:'POST',body:JSON.stringify({media_id:id,result:m.pending_metadata})});
    toast('Metadaten übernommen');
    await renderAdmin()
  }catch(x){toast(x.message)}
}
function metadataJobMarkup(job={}){
  if(!job.started)return '<div class="muted">Noch kein automatischer Metadatenlauf gestartet.</div>';
  let total=Number(job.total)||0,done=Number(job.done)||0,pct=total?Math.min(100,Math.round(done/total*100)):100;
  let summary=`${job.auto_matched||0} automatisch · ${job.review||0} prüfen · ${job.no_match||0} ohne Treffer${job.errors?` · ${job.errors} Fehler`:''}`;
  if(job.running)return `<div class="metadata-job-head"><strong>${done} / ${total}</strong><span>${esc(job.current_title||'Metadaten werden durchsucht…')}</span></div><div class="metadata-job-bar"><i style="width:${pct}%"></i></div><div class="muted">${summary}</div>`;
  return `<div class="metadata-job-head"><strong>Letzter Lauf beendet</strong><span>${summary}</span></div>${job.error?`<div class="metadata-job-error">${esc(job.error)}</div>`:''}`
}
async function startBulkMetadata(){
  let b=$('#bulkmeta');if(!b)return;
  b.disabled=true;b.textContent='Metadatenlauf startet…';
  try{
    state.metadataJob=await api('/api/metadata/bulk',{method:'POST'});
    let box=$('#metajob');if(box)box.innerHTML=metadataJobMarkup(state.metadataJob);
    if(state.metadataJob.running){toast(`${state.metadataJob.total} Titel werden geprüft`);scheduleMetadataPoll()}
    else{toast('Keine Medien ohne Metadaten gefunden');b.disabled=false;b.textContent='Metadaten automatisch laden'}
  }catch(x){toast(x.message);b.disabled=false;b.textContent='Metadaten automatisch laden'}
}
function scheduleMetadataPoll(){
  clearTimeout(metadataPollTimer);
  metadataPollTimer=setTimeout(pollMetadataJob,1000)
}
async function pollMetadataJob(){
  try{
    let job=await api('/api/metadata/bulk');state.metadataJob=job;
    let box=$('#metajob');if(box)box.innerHTML=metadataJobMarkup(job);
    if(job.running){scheduleMetadataPoll();return}
    await refresh();
    if(box){toast(`Metadaten fertig: ${job.auto_matched||0} automatisch, ${job.review||0} zu prüfen`);await renderAdmin()}
  }catch(x){let box=$('#metajob');if(box)box.innerHTML='<div class="metadata-job-error">'+esc(x.message)+'</div>'}
}
async function mediaAction(id,action){try{await api('/api/media/action',{method:'POST',body:JSON.stringify({media_id:id,action})});await refresh();renderAdminMedia();toast('Medienstatus aktualisiert')}catch(x){toast(x.message)}}
async function cleanupMissing(){if(!confirm('Alle Einträge entfernen, deren Mediendatei beim letzten Scan nicht mehr gefunden wurde? Die Dateien selbst werden nicht gelöscht.'))return;try{let r=await api('/api/media/cleanup',{method:'POST'});toast(r.removed+' verwaiste Einträge entfernt');await refresh();renderAdmin()}catch(x){toast(x.message)}}
async function editMediaById(id){let a=await api('/api/media?id='+id+'&include_missing=1');if(a[0])editMedia(a[0])}
async function identifyMediaById(id){let a=await api('/api/media?id='+id+'&include_missing=1');if(a[0])identifyMedia(a[0])}
async function identifyMedia(m){
  let el=document.createElement('div');el.className='modal';
  el.innerHTML=`<div class="modalbox"><div class="panel"><div class="panel-title"><div><h3>Medium identifizieren</h3><p class="muted">Metadatenanbieter durchsuchen und den richtigen Treffer auswählen.</p></div><button class="close">×</button></div><form id="identifyform" class="identify-form"><label class="field grow">Titel<input name="q" value="${esc(m.title)}"></label><label class="field">Jahr<input name="year" type="number" value="${m.year||''}"></label><button class="btn primary">Suchen</button></form><div id="identifyresults" class="tmdb-results"><div class="empty">Noch keine Suche gestartet.</div></div></div></div>`;
  document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();
  el.querySelector('#identifyform').onsubmit=async e=>{e.preventDefault();let box=el.querySelector('#identifyresults');box.innerHTML='<div class="empty">Metadaten werden durchsucht…</div>';try{let results=await api('/api/metadata/search?q='+encodeURIComponent(e.target.q.value)+'&year='+encodeURIComponent(e.target.year.value));box.innerHTML=results.length?results.map((r,i)=>`<div class="tmdb-result"><div class="tmdb-thumb">${r.poster?`<img src="${esc(r.poster)}">`:'▶'}</div><div><strong>${esc(r.title)}</strong><div class="muted">${r.year||'ohne Jahr'}</div><p>${esc(r.overview||'Keine Beschreibung vorhanden.')}</p></div><button class="btn primary" data-pick="${i}">Übernehmen</button></div>`).join(''):'<div class="empty">Keine Treffer.</div>';box.querySelectorAll('[data-pick]').forEach(b=>b.onclick=async()=>{let r=results[Number(b.dataset.pick)];try{await api('/api/metadata/apply',{method:'POST',body:JSON.stringify({media_id:m.id,result:r})});toast('Metadaten-Treffer übernommen');el.remove();await refresh();renderAdmin()}catch(x){toast(x.message)}})}catch(x){box.innerHTML='<div class="empty">'+esc(x.message)+'</div>'}}
}
async function addLib(e){e.preventDefault();try{await api('/api/libraries',{method:'POST',body:JSON.stringify({name:e.target.name.value,path:e.target.path.value,type:e.target.type.value})});toast('Bibliothek hinzugefügt');e.target.reset();await refresh();renderAdmin()}catch(x){toast(x.message)}}
async function removeLib(id){if(!confirm('Bibliothek und Index entfernen? Die Mediendateien bleiben unangetastet.'))return;await api('/api/libraries?id='+id,{method:'DELETE'});await refresh();renderAdmin()}
async function editLib(id){let l=state.libs.find(x=>x.id===id);if(!l)return;let el=document.createElement('div');el.className='modal';el.innerHTML=`<div class="modalbox compact"><div class="panel"><div class="panel-title"><h3>Bibliothek bearbeiten</h3><button class="close">×</button></div><form id="editlib"><label class="field">Name<input name="name" value="${esc(l.name)}"></label><label class="field">Typ<select name="type"><option value="movies" ${l.type==='movies'?'selected':''}>Filme</option><option value="shows" ${l.type==='shows'?'selected':''}>Serien</option><option value="other" ${l.type==='other'?'selected':''}>Andere Videos</option></select></label><label class="field">Pfad<input name="path" value="${esc(l.path)}"></label><button class="btn primary">Änderungen speichern</button></form></div></div>`;document.body.append(el);el.querySelector('.close').onclick=()=>el.remove();el.querySelector('#editlib').onsubmit=async e=>{e.preventDefault();try{await api('/api/libraries?id='+id,{method:'PUT',body:JSON.stringify({name:e.target.name.value,path:e.target.path.value,type:e.target.type.value})});toast('Bibliothek aktualisiert');el.remove();await refresh();renderAdmin()}catch(x){toast(x.message)}}}
async function saveSettings(e){e.preventDefault();try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:e.target.server_name.value,admin_user:e.target.admin_user.value})});toast('Servereinstellungen gespeichert');renderAdmin()}catch(x){toast(x.message)}}
async function saveTVDB(e){e.preventDefault();const key=e.target.tvdb_api_key.value.trim();if(!key){toast('Bitte einen API-Key eintragen');return}try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tvdb_api_key:key})});toast('Eigener TheTVDB-Override gespeichert');renderAdmin()}catch(x){toast(x.message)}}
async function clearTVDB(){if(!confirm('Eigenen TheTVDB-Override wirklich entfernen und wieder den BuddyFlix-Projektzugang verwenden?'))return;try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tvdb_api_key:''})});toast('TheTVDB-Override entfernt');renderAdmin()}catch(x){toast(x.message)}}
async function saveTMDb(e){e.preventDefault();const key=e.target.tmdb_api_key.value.trim();if(!key){toast('Bitte einen API-Key eintragen');return}try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tmdb_api_key:key})});toast('Eigener TMDb-Override gespeichert');renderAdmin()}catch(x){toast(x.message)}}
async function clearTMDb(){if(!confirm('Eigenen TMDb-Override wirklich entfernen und wieder den BuddyFlix-Standardzugang verwenden?'))return;try{await api('/api/settings',{method:'PUT',body:JSON.stringify({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tmdb_api_key:''})});toast('TMDb-Override entfernt');renderAdmin()}catch(x){toast(x.message)}}
async function changePassword(e){e.preventDefault();if(e.target.next.value!==e.target.repeat.value){toast('Die neuen Passwörter stimmen nicht überein');return}try{await api('/api/password',{method:'POST',body:JSON.stringify({current:e.target.current.value,new:e.target.next.value})});toast('Passwort geändert – bitte neu anmelden');setTimeout(loginView,700)}catch(x){toast(x.message)}}
async function doScan(){let b=$('#scan');b.disabled=true;b.textContent='Scan läuft…';try{let r=await api('/api/scan',{method:'POST'});let msg=`${r.files_seen} Mediendateien gefunden`;if(r.skipped_system_dirs)msg+=` · ${r.skipped_system_dirs} Systemordner übersprungen`;if(r.skipped_libraries?.length)msg+=` · Bibliotheken übersprungen: ${r.skipped_libraries.join(', ')}`;toast(msg);await refresh();renderAdmin()}catch(x){toast(x.message)}finally{b.disabled=false}}
async function refresh(){state.media=await api('/api/media');state.libs=await api('/api/libraries');state.system=await api('/api/system')}
function fmtTime(sec){sec=Math.max(0,Math.floor(Number(sec)||0));let h=Math.floor(sec/3600),m=Math.floor((sec%3600)/60),s=sec%60;return h?`${h}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`:`${m}:${String(s).padStart(2,'0')}`}
function humanSize(bytes){bytes=Number(bytes)||0;if(!bytes)return '';let units=['B','KB','MB','GB','TB'],i=0;while(bytes>=1024&&i<units.length-1){bytes/=1024;i++}return (i<3?Math.round(bytes):bytes.toFixed(bytes<10?1:0))+' '+units[i]}
function mediaProgress(m){return Math.max(0,Math.min(100,Number(m.progress)||0))}
function closeWithEscape(el,onClose){let done=false;const finish=()=>{if(done)return;done=true;document.removeEventListener('keydown',key);if(onClose)onClose();el.remove()};const key=e=>{if(e.key==='Escape')finish()};document.addEventListener('keydown',key);return finish}
async function detail(id){
  let a=await api('/api/media?id='+id),m=a[0];if(!m)return;
  let pct=mediaProgress(m),resumable=m.position>20&&m.duration>0&&pct<95,watched=pct>=95;
  let el=document.createElement('div');el.className='modal detail-modal';
  let poster=m.poster?`<div class="movieposter"><img src="${esc(m.poster)}" alt=""></div>`:'<div class="movieposter movieposter-empty"><span>▶</span><small>Kein Poster</small></div>';
  let progress=resumable?`<div class="detail-progress"><div class="detail-progress-head"><strong>Weiterschauen</strong><span>${fmtTime(m.position)} von ${fmtTime(m.duration)} · ${Math.round(pct)}%</span></div><div class="detail-progress-bar"><i style="width:${pct}%"></i></div></div>`:'';
  let status=watched?'<span class="detail-pill watched">✓ Gesehen</span>':resumable?'<span class="detail-pill started">▶ Angefangen</span>':'<span class="detail-pill">Noch nicht angesehen</span>';
  let ext=(m.path||'').split('.').pop()?.toUpperCase(),size=m.size?humanSize(m.size):'',source=m.metadata_provider?m.metadata_provider==='thetvdb'?'TheTVDB':m.metadata_provider.toUpperCase():'';
  let tech=[ext,size].filter(Boolean).join(' · ');
  el.innerHTML=`<div class="modalbox movie-detail-box"><div class="moviehero moviehero-detail" style="background-image:url('${esc(m.backdrop||m.poster||'')}')"><div class="detail-backdrop-fade"></div><button class="close" aria-label="Schließen">×</button><div class="movie-detail-layout">${poster}<div class="moviecopy moviecopy-detail"><div class="detail-kicker">BUDDYFLIX FILM</div><h1>${esc(m.title)}</h1><div class="detail-meta"><span class="detail-pill">${m.year||'Jahr unbekannt'}</span>${m.duration>0?`<span class="detail-pill">${fmtTime(m.duration)}</span>`:''}${status}</div><p class="detail-overview">${esc(m.overview||'Für diesen Titel wurden noch keine Metadaten geladen. Du kannst ihn trotzdem direkt abspielen oder später identifizieren.')}</p>${progress}<div class="modalactions detail-actions"><button class="btn primary detail-play" id="p">▶ ${resumable?'Fortsetzen':'Abspielen'}</button>${resumable?'<button class="btn ghost" id="restart">↺ Von Anfang</button>':''}<button class="btn ghost" id="meta">◎ Identifizieren</button><button class="btn ghost" id="editmedia">✎ Bearbeiten</button></div><div class="detail-foot">${source?`<span>Metadaten: <strong>${esc(source)}</strong></span>`:''}${tech?`<span>${esc(tech)}</span>`:''}</div></div></div></div></div>`;
  document.body.append(el);
  const close=closeWithEscape(el);
  el.querySelector('.close').onclick=close;
  el.onclick=e=>{if(e.target===el)close()};
  el.querySelector('#p').onclick=()=>{close();play(id,resumable?'resume':'start')};
  let restart=el.querySelector('#restart');if(restart)restart.onclick=()=>{close();play(id,'start')};
  el.querySelector('#meta').onclick=()=>{close();identifyMedia(m)};
  el.querySelector('#editmedia').onclick=()=>{close();editMedia(m)}
}
function editMedia(m){let el=document.createElement('div');el.className='modal';el.innerHTML=`<div class="modalbox compact"><div class="panel"><div class="panel-title"><div><h3>Medium bearbeiten</h3><p class="muted">Metadaten manuell korrigieren.</p></div><button class="close">×</button></div><form id="mediaedit"><label class="field">Titel<input name="title" value="${esc(m.title)}"></label><label class="field">Jahr<input name="year" type="number" min="1888" max="2100" value="${m.year||''}"></label><label class="field">Beschreibung<textarea name="overview" rows="5">${esc(m.overview||'')}</textarea></label><label class="field">Poster-URL<input name="poster" value="${esc(m.poster||'')}"></label><label class="field">Backdrop-URL<input name="backdrop" value="${esc(m.backdrop||'')}"></label><div class="notice"><strong>Datei:</strong><br><code>${esc(m.path)}</code></div><button class="btn primary">Speichern</button></form></div></div>`;document.body.append(el);let close=closeWithEscape(el);el.querySelector('.close').onclick=close;el.querySelector('#mediaedit').onsubmit=async e=>{e.preventDefault();try{await api('/api/media?id='+m.id,{method:'PUT',body:JSON.stringify({title:e.target.title.value,year:Number(e.target.year.value)||0,overview:e.target.overview.value,poster:e.target.poster.value,backdrop:e.target.backdrop.value})});toast('Medium gespeichert');close();await refresh();renderMovies()}catch(x){toast(x.message)}}}
async function resumePrompt(m){
  let el=document.createElement('div');el.className='modal resume-modal';
  el.innerHTML=`<div class="resume-card"><button class="close" aria-label="Schließen">×</button><div class="resume-kicker">WEITERSCHAUEN</div><h2>${esc(m.title)}</h2><p>Du warst bei <strong>${fmtTime(m.position)}</strong> von ${fmtTime(m.duration)}.</p><div class="resume-actions"><button class="btn primary" id="resume">▶ Fortsetzen</button><button class="btn ghost" id="restart">Von Anfang</button></div></div>`;
  document.body.append(el);let close=closeWithEscape(el);el.querySelector('.close').onclick=close;el.onclick=e=>{if(e.target===el)close()};
  el.querySelector('#resume').onclick=()=>{close();openPlayer(m,m.position)};
  el.querySelector('#restart').onclick=()=>{close();openPlayer(m,0)}
}
async function play(id,mode='auto'){
  let a=await api('/api/media?id='+id),m=a[0];if(!m)return;
  let pct=mediaProgress(m),resumable=m.position>20&&m.duration>0&&pct<95;
  if(mode==='auto'&&resumable){resumePrompt(m);return}
  openPlayer(m,mode==='resume'&&resumable?m.position:0)
}
function openPlayer(m,startAt=0){
  let el=document.createElement('div');el.className='videoModal';
  el.innerHTML=`<div class="video-titlebar"><strong>${esc(m.title)}</strong><span>${m.year||''}</span></div><video controls autoplay playsinline src="/stream/${m.id}"></video><button class="close player-close" aria-label="Player schließen">×</button>`;
  document.body.append(el);let v=el.querySelector('video'),last=0,closed=false;
  const finish=()=>{if(closed)return;closed=true;saveProgress(m.id,v.currentTime,v.duration);v.pause();document.removeEventListener('keydown',key);el.remove();refresh()};
  const key=e=>{if(e.key==='Escape')finish()};
  document.addEventListener('keydown',key);
  v.onloadedmetadata=()=>{if(startAt>0&&startAt<v.duration-20)v.currentTime=startAt};
  v.ontimeupdate=()=>{if(Date.now()-last>10000){last=Date.now();saveProgress(m.id,v.currentTime,v.duration)}};
  v.onpause=()=>{if(!closed)saveProgress(m.id,v.currentTime,v.duration)};
  v.onended=async()=>{await saveProgress(m.id,v.duration,v.duration);await refresh()};
  el.querySelector('.close').onclick=finish
}
async function saveProgress(id,p,d){if(!Number.isFinite(d)||d<=0)return;try{await api('/api/progress',{method:'POST',body:JSON.stringify({media_id:id,Position:p,Duration:d})})}catch{}}
boot();
