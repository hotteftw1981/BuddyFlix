const $=s=>document.querySelector(s);let state={media:[],libs:[],system:null,user:null,profiles:[],activeProfile:null,profileSelected:false};let metadataPollTimer=null,heroTimer=null;let movieFilter='all',movieSort='recent',movieDecade='all',movieView='posters',heroIndex=0,currentView='home';
async function api(path,opt={}){const r=await fetch(path,{headers:{'Content-Type':'application/json',...(opt.headers||{})},...opt});if(r.status===401){loginView();throw new Error('unauthorized')}const j=await r.json().catch(()=>({}));if(!r.ok)throw new Error(j.error||'Fehler');return j}
function esc(s=''){return String(s).replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]))}
function toast(t){let x=document.createElement('div');x.className='toast';x.textContent=t;document.body.append(x);setTimeout(()=>x.remove(),2500)}
function setupView(info={}){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card setup-card" id="setup"><div class="brand brand-image"><img class="brand-logo" src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><div class="setup-step">ERSTEINRICHTUNG</div><h1>Mach es zu deinem Server.</h1><div class="muted">Einmal kurz einrichten, danach gehört die Standard-Anmeldung der Vergangenheit an.</div><label class="field">Servername<input name="server_name" value="${esc(info.server_name||'BuddyFlix')}"></label><label class="field">Admin-Benutzer<input name="admin_user" value="admin" autocomplete="username"></label><label class="field">Admin-Passwort<input name="password" type="password" minlength="8" autocomplete="new-password"></label><label class="field">Passwort wiederholen<input name="repeat" type="password" minlength="8" autocomplete="new-password"></label><div class="notice">Metadatenanbieter werden von BuddyFlix verwaltet. Für die normale Einrichtung ist kein eigener API-Key vorgesehen.</div><button class="btn primary">BuddyFlix einrichten</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#setup').onsubmit=async e=>{e.preventDefault();if(e.target.password.value!==e.target.repeat.value){$('#err').textContent='Die Passwörter stimmen nicht überein.';return}try{await api('/api/setup',{method:'POST',body:JSON.stringify({server_name:e.target.server_name.value,admin_user:e.target.admin_user.value,password:e.target.password.value})});toast('Einrichtung abgeschlossen');loginView()}catch(x){$('#err').textContent=x.message}}}
function loginView(){document.querySelector('#app').innerHTML=`<div class="login"><form class="login-card" id="login"><div class="brand brand-image"><img class="brand-logo" src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><h1>Willkommen zurück.</h1><div class="muted">Dein schlanker Media Server.</div><label class="field">Benutzer<input name="u" placeholder="Benutzername" autocomplete="username"></label><label class="field">Passwort<input name="p" type="password" autocomplete="current-password"></label><button class="btn primary">Anmelden</button><div id="err" class="muted" style="margin-top:14px"></div></form></div>`;$('#login').onsubmit=async e=>{e.preventDefault();try{await api('/api/login',{method:'POST',body:JSON.stringify({Username:e.target.u.value,Password:e.target.p.value})});await boot()}catch(x){$('#err').textContent=x.message}}}
function shell(){let p=state.activeProfile||state.profiles[0]||{name:'Profil',avatar:'🍿'};document.querySelector('#app').innerHTML=`<div class="shell theater-shell"><aside class="side side-rail"><div class="rail-logo"><img src="/assets/buddyflix-logo.png" alt="BuddyFlix"></div><nav class="nav rail-nav"><button data-v="home" class="active" title="Startseite"><span class="nav-icon">⌂</span><span class="rail-label">Start</span></button><button data-v="movies" title="Filme"><span class="nav-icon">▣</span><span class="rail-label">Filme</span></button><button data-v="mylist" title="Meine Liste"><span class="nav-icon">♥</span><span class="rail-label">Liste</span></button><button data-v="admin" title="Verwaltung"><span class="nav-icon">⚙</span><span class="rail-label">Admin</span></button></nav><div class="rail-bottom"><span class="side-server-dot"></span><span class="rail-version">v${esc(state.system?.version||'dev')}</span></div></aside><main class="main theater-main"><header class="top theater-top"><div class="top-search-wrap"><span class="top-search-icon">⌕</span><input id="search" class="search" placeholder="Was willst du sehen?"></div><div class="top-actions"><button class="profile-pill" id="profileMenu"><span class="profile-pill-avatar">${esc(p.avatar||'🍿')}</span><span class="profile-pill-copy"><b>${esc(p.name||'Profil')}</b><small>Profil wechseln</small></span><span class="profile-chevron">⌄</span></button><button class="btn ghost cinema-toggle" id="cinema" title="Kino-Fokus">◩ Kino</button><span class="status"><i class="dot"></i> Online</span><button class="btn ghost top-logout" id="logout">Abmelden</button></div></header><div id="content" class="content theater-content"></div></main></div>`;document.querySelectorAll('.nav button').forEach(b=>b.onclick=()=>view(b.dataset.v,b));$('#logout').onclick=async()=>{await api('/api/logout',{method:'POST'});loginView()};$('#profileMenu').onclick=()=>showProfileSwitcher();$('#cinema').onclick=toggleCinemaMode;let timer;$('#search').oninput=e=>{clearTimeout(timer);timer=setTimeout(async()=>{stopHeroRotation();state.media=await api('/api/media?q='+encodeURIComponent(e.target.value));movieFilter='all';movieDecade='all';renderMovies()},180)}}
async function boot(){try{const setup=await fetch('/api/setup/status').then(r=>r.json());if(!setup.setup_done){setupView(setup);return}state.system=await api('/api/system');let profileInfo=await api('/api/profiles');state.profiles=profileInfo.profiles||[];state.profileSelected=!!profileInfo.selected;state.activeProfile=state.profiles.find(p=>p.id===profileInfo.active_id)||state.profiles[0]||null;state.media=await api('/api/media');state.libs=await api('/api/libraries');shell();renderHome();if(state.profiles.length>1&&!state.profileSelected)setTimeout(()=>showProfileSwitcher(),80)}catch(e){if(e.message!=='unauthorized')loginView()}}
async function view(v,b){currentView=v;document.querySelectorAll('.nav button').forEach(x=>x.classList.toggle('active',x===b));if(v!=='home')stopHeroRotation();try{if(v==='home')renderHome();if(v==='movies')renderMovies();if(v==='mylist')renderMyList();if(v==='admin')await renderAdmin()}catch(e){const box=$('#content');if(box)box.innerHTML='<section class="panel"><h2>Verwaltung konnte nicht geladen werden</h2><p class="muted">'+esc(e.message||'Unbekannter Fehler')+'</p><button class="btn primary" onclick="location.reload()">Neu laden</button></section>';toast('Fehler beim Laden der Verwaltung')}}
function profileByID(id){return state.profiles.find(p=>p.id===id)}
function avatarChoices(){return ['🍿','🎬','😎','🚀','🦊','🐼','🤠','👾','🛸','🔥','⭐','🎧']}
async function reloadProfiles(){
  let info=await api('/api/profiles');
  state.profiles=info.profiles||[];
  state.profileSelected=!!info.selected;
  state.activeProfile=state.profiles.find(p=>p.id===info.active_id)||state.profiles[0]||null;
}
async function selectProfile(id){
  await api('/api/profiles/select',{method:'POST',body:JSON.stringify({profile_id:id})});
  await reloadProfiles();
  state.media=await api('/api/media');
  document.querySelectorAll('.profile-modal').forEach(x=>x.remove());
  shell();renderHome();toast('Profil gewechselt');
}
function showProfileSwitcher(){
  document.querySelectorAll('.profile-modal').forEach(x=>x.remove());
  let el=document.createElement('div');el.className='profile-modal';
  el.innerHTML=`<div class="profile-stage"><button class="profile-close" aria-label="Schließen">×</button><div class="profile-stage-brand"><img src="/assets/buddyflix-logo.png" alt="BuddyFlix"><span>WER SCHAUT?</span></div><h1>Wer schaut gerade?</h1><p>Jedes Profil hat seinen eigenen Fortschritt und Gesehen-Status.</p><div class="profile-grid">${state.profiles.map((p,i)=>`<article class="profile-card ${state.activeProfile?.id===p.id?'active':''}" onclick="selectProfile(${p.id})"><div class="profile-avatar">${esc(p.avatar||'🎬')}</div><strong>${esc(p.name)}</strong><span>${state.activeProfile?.id===p.id?'Aktiv':'Auswählen'}</span><button class="profile-edit" onclick="event.stopPropagation();openProfileEditor(${p.id})" title="Profil bearbeiten">✎</button></article>`).join('')}${state.profiles.length<8?`<article class="profile-card profile-add" onclick="openProfileEditor()"><div class="profile-avatar">＋</div><strong>Profil hinzufügen</strong><span>Eigener Fortschritt</span></article>`:''}</div></div>`;
  document.body.append(el);
  const close=()=>el.remove();el.querySelector('.profile-close').onclick=close;el.onclick=e=>{if(e.target===el)close()};
}
function openProfileEditor(id=0){
  document.querySelectorAll('.profile-editor-modal').forEach(x=>x.remove());
  let p=id?profileByID(id):null,avatar=p?.avatar||'🍿';
  let el=document.createElement('div');el.className='profile-modal profile-editor-modal';
  el.innerHTML=`<div class="profile-editor"><button class="profile-close" aria-label="Schließen">×</button><div class="profile-editor-preview" id="avatarPreview">${esc(avatar)}</div><div><div class="profile-editor-kicker">${p?'PROFIL BEARBEITEN':'NEUES PROFIL'}</div><h2>${p?'Dein Profil.':'Wer kommt dazu?'}</h2><label class="field">Name<input id="profileName" maxlength="30" value="${esc(p?.name||'')}"></label><div class="avatar-picker">${avatarChoices().map(a=>`<button class="${a===avatar?'active':''}" data-avatar="${a}">${a}</button>`).join('')}</div><div class="profile-editor-actions"><button class="btn primary" id="saveProfile">${p?'Speichern':'Profil anlegen'}</button>${p&&state.profiles[0]?.id!==p.id?'<button class="btn danger" id="deleteProfile">Profil löschen</button>':''}<button class="btn ghost" id="cancelProfile">Abbrechen</button></div></div></div>`;
  document.body.append(el);
  let chosen=avatar;
  el.querySelectorAll('.avatar-picker button').forEach(b=>b.onclick=()=>{chosen=b.dataset.avatar;el.querySelectorAll('.avatar-picker button').forEach(x=>x.classList.toggle('active',x===b));el.querySelector('#avatarPreview').textContent=chosen});
  const close=()=>el.remove();el.querySelector('.profile-close').onclick=close;el.querySelector('#cancelProfile').onclick=close;
  el.querySelector('#saveProfile').onclick=async()=>{let name=el.querySelector('#profileName').value.trim();if(!name){toast('Bitte Profilnamen eingeben');return}try{if(p){await api('/api/profiles?id='+p.id,{method:'PUT',body:JSON.stringify({Name:name,Avatar:chosen})})}else{await api('/api/profiles',{method:'POST',body:JSON.stringify({Name:name,Avatar:chosen})})}await reloadProfiles();close();showProfileSwitcher();toast(p?'Profil gespeichert':'Profil angelegt')}catch(e){toast(e.message)}};
  let del=el.querySelector('#deleteProfile');if(del)del.onclick=()=>confirmProfileDelete(p,close)
}
function confirmProfileDelete(p,editorClose){
  let el=document.createElement('div');el.className='profile-confirm';el.innerHTML=`<div><h3>${esc(p.name)} löschen?</h3><p>Der Wiedergabefortschritt dieses Profils wird ebenfalls gelöscht.</p><div><button class="btn danger" id="yes">Löschen</button><button class="btn ghost" id="no">Abbrechen</button></div></div>`;document.body.append(el);el.querySelector('#no').onclick=()=>el.remove();el.querySelector('#yes').onclick=async()=>{try{await api('/api/profiles?id='+p.id,{method:'DELETE'});await reloadProfiles();el.remove();editorClose();showProfileSwitcher();toast('Profil gelöscht')}catch(e){toast(e.message)}}
}

function card(m){let pct=mediaProgress(m),resumable=m.position>20&&m.duration>0&&pct<95;return `<article class="card theater-card" onclick="detail(${m.id})"><div class="poster">${m.poster?`<img loading="lazy" src="${esc(m.poster)}" alt="">`:`<div class="placeholder"><span>▶</span><small>Kein Poster</small></div>`}<div class="poster-shade"></div><button class="favorite-badge ${m.favorite?'active':''}" aria-label="${m.favorite?'Aus meiner Liste entfernen':'Zu meiner Liste'}" title="${m.favorite?'Aus meiner Liste entfernen':'Zu meiner Liste'}" onclick="event.stopPropagation();toggleFavorite(${m.id},${!m.favorite})">${m.favorite?'♥':'♡'}</button><div class="poster-info"><strong>${esc(m.title)}</strong><span>${m.year||'Film'}${resumable?` · ${Math.round(pct)}%`:m.progress>=95?' · gesehen':''}</span></div><button class="poster-play" aria-label="${resumable?'Fortsetzen':'Abspielen'}" onclick="event.stopPropagation();play(${m.id},'${resumable?'resume':'start'}')">▶</button>${m.progress>0&&m.progress<95?`<div class="progress"><i style="width:${Math.min(100,m.progress)}%"></i></div>`:''}${m.progress>=95?'<span class="watched-mark">✓</span>':''}</div></article>`}


function landscapeCard(m,index=0){
  let pct=mediaProgress(m),resumable=m.position>20&&m.duration>0&&pct<95;
  return `<article class="cine-card ${index%7===0?'cine-card-wide':''}" onclick="detail(${m.id})">
    <div class="cine-bg" style="background-image:url('${esc(m.backdrop||m.poster||'')}')"></div>
    <div class="cine-shade"></div>
    <button class="cine-favorite ${m.favorite?'active':''}" onclick="event.stopPropagation();toggleFavorite(${m.id},${!m.favorite})" title="${m.favorite?'Aus meiner Liste':'Zu meiner Liste'}">${m.favorite?'♥':'♡'}</button>
    <div class="cine-copy">
      <div class="cine-topline"><span>${m.year||'Film'}</span>${resumable?`<b>${Math.round(pct)}%</b>`:m.progress>=95?'<b>✓ gesehen</b>':''}</div>
      <h3>${esc(m.title)}</h3>
      <p>${esc(m.overview||'Direkt aus deiner BuddyFlix-Bibliothek.')}</p>
      <div class="cine-actions"><button onclick="event.stopPropagation();play(${m.id},'${resumable?'resume':'start'}')">▶ ${resumable?'Fortsetzen':'Ansehen'}</button><span>ⓘ Details</span></div>
    </div>
    ${resumable?`<div class="cine-progress"><i style="width:${pct}%"></i></div>`:''}
  </article>`
}


function homeSection(title,items,subtitle=''){if(!items.length)return '';return `<section class="home-section theater-shelf"><div class="rowhead"><div><div class="shelf-kicker">${esc(subtitle||'BUDDYFLIX')}</div><h2>${esc(title)}</h2></div><span class="rail-count">${items.length}</span></div><div class="media-rail">${items.map(card).join('')}</div></section>`}

function miniPoster(m,index){if(!m)return '';return `<button class="hero-mini" onclick="setHero(${index})" title="${esc(m.title)}">${m.poster?`<img src="${esc(m.poster)}" alt="">`:'<span>▶</span>'}<i>${esc(m.title)}</i></button>`}

function stopHeroRotation(){clearTimeout(heroTimer);heroTimer=null}
function setHero(index){stopHeroRotation();heroIndex=Math.max(0,index||0);renderHome()}
function shiftHero(delta){let c=heroCandidates();if(!c.length)return;heroIndex=(heroIndex+delta+c.length)%c.length;renderHome()}
function heroCandidates(){let rich=state.media.filter(m=>m.backdrop&&m.poster);let fallback=state.media.filter(m=>m.backdrop||m.poster);return (rich.length?rich:fallback).slice(0,8)}
function scheduleHeroRotation(){stopHeroRotation();let c=heroCandidates();if(c.length<2)return;heroTimer=setTimeout(()=>{heroIndex=(heroIndex+1)%c.length;renderHome()},12000)}
function toggleCinemaMode(){document.body.classList.toggle('cinema-focus');let b=$('#cinema');if(b)b.textContent=document.body.classList.contains('cinema-focus')?'◫ Zurück':'◩ Kino'}
function featureStrip(items){if(!items.length)return '';return `<section class="feature-strip">${items.slice(0,3).map((m,i)=>`<article class="feature-tile" onclick="detail(${m.id})"><div class="feature-bg" style="background-image:url('${esc(m.backdrop||m.poster||'')}')"></div><div class="feature-shade"></div><div class="feature-copy"><span>0${i+1}</span><strong>${esc(m.title)}</strong><small>${m.year||'Film'} · Mehr entdecken</small></div></article>`).join('')}</section>`}

async function toggleFavorite(id,want){
  try{
    await api('/api/media/action',{method:'POST',body:JSON.stringify({media_id:id,action:want?'favorite':'unfavorite'})});
    state.media=await api('/api/media');
    toast(want?'Zu deiner Liste hinzugefügt':'Aus deiner Liste entfernt');
    renderCurrentView();
  }catch(e){toast(e.message)}
}
function renderCurrentView(){
  if(currentView==='mylist')renderMyList();
  else if(currentView==='movies')renderMovies();
  else if(currentView==='home')renderHome();
}
function historyItems(){
  return state.media.filter(m=>m.progress_updated).slice().sort((a,b)=>String(b.progress_updated).localeCompare(String(a.progress_updated)));
}
function renderMyList(){
  currentView='mylist';stopHeroRotation();
  let items=state.media.filter(m=>m.favorite);
  let profile=state.activeProfile||{name:'Dein Profil',avatar:'🍿'};
  let hero=items.find(m=>m.backdrop)||items.find(m=>m.poster)||null;
  $('#content').innerHTML=`<section class="mylist-page">
    <div class="mylist-backdrop ${hero?'has-art':''}" ${hero?`style="background-image:url('${esc(hero.backdrop||hero.poster||'')}')"`:''}></div>
    <div class="mylist-fade"></div>
    <div class="mylist-head">
      <div class="mylist-profile">${esc(profile.avatar||'🍿')}</div>
      <div><div class="section-kicker">PERSÖNLICH FÜR ${esc(String(profile.name||'DEIN PROFIL').toUpperCase())}</div><h1>Meine Liste.</h1><p>${items.length?items.length+' Filme, die du nicht aus den Augen verlieren willst.':'Noch ganz leer. Herzchen drücken und hier sammeln.'}</p></div>
    </div>
    ${items.length?`<div class="mylist-grid">${items.map((m,i)=>landscapeCard(m,i)).join('')}</div>`:`<div class="mylist-empty"><span>♡</span><h2>Hier ist noch Platz.</h2><p>Tippe bei einem Film auf das Herz. BuddyFlix merkt sich deine Auswahl nur für dieses Profil.</p><button class="btn primary" onclick="document.querySelector('[data-v=movies]').click()">Filme entdecken</button></div>`}
  </section>`
}

function pickSomething(){let pool=state.media.filter(m=>!m.missing);if(!pool.length){toast('Noch keine Filme vorhanden');return}let unseen=pool.filter(m=>mediaProgress(m)<1),choices=unseen.length?unseen:pool,m=choices[Math.floor(Math.random()*choices.length)];detail(m.id)}

function renderHome(){
  let continueItems=state.media.filter(m=>m.progress>1&&m.progress<95).slice(0,12);
  let newest=state.media.slice(0,18);
  let unseen=state.media.filter(m=>mediaProgress(m)<1).slice(0,14);
  let classics=state.media.filter(m=>m.year&&m.year<2000).slice(0,14);
  let modern=state.media.filter(m=>m.year>=2010).slice(0,14);
  let favorites=state.media.filter(m=>m.favorite).slice(0,14);
  let history=historyItems().slice(0,14);
  let heroes=heroCandidates();
  if(heroes.length&&heroIndex>=heroes.length)heroIndex=0;
  let hero=heroes[heroIndex]||continueItems.find(m=>m.backdrop)||state.media[0];
  let heroPct=hero?mediaProgress(hero):0,heroResume=hero&&hero.position>20&&hero.duration>0&&heroPct<95;
  let watchedCount=state.media.filter(m=>mediaProgress(m)>=95).length;
  let spotlight=heroes.filter((_,i)=>i!==heroIndex).slice(0,3);
  let showcase=[...state.media.filter(m=>m.backdrop&&m.id!==hero?.id)].slice(0,3);
  $('#content').innerHTML=`
    ${hero?`<section class="immersive-hero" onmouseenter="stopHeroRotation()" onmouseleave="scheduleHeroRotation()">
      <div class="immersive-bg" style="background-image:url('${esc(hero.backdrop||hero.poster||'')}')"></div>
      <div class="immersive-fade"></div>
      <div class="immersive-grain"></div>
      <div class="hero-nav hero-nav-left"><button onclick="shiftHero(-1)" aria-label="Vorheriger Film">‹</button></div>
      <div class="hero-nav hero-nav-right"><button onclick="shiftHero(1)" aria-label="Nächster Film">›</button></div>
      <div class="immersive-copy">
        <div class="immersive-eyebrow"><span>BUDDYFLIX ORIGINAL PICK</span><b>${hero.year||'Film'}</b></div>
        <h1>${esc(hero.title)}</h1>
        <p>${esc(hero.overview||'Deine eigene Filmbibliothek. Direkt vom NAS, ohne Umwege und ohne Ballast.')}</p>
        <div class="immersive-actions">
          <button class="btn primary mega-play" onclick="play(${hero.id},'${heroResume?'resume':'start'}')">▶ ${heroResume?'Fortsetzen':'Jetzt ansehen'}</button>
          <button class="round-action" onclick="detail(${hero.id})" title="Details">ⓘ</button>
          <button class="round-action ${hero.favorite?'favorite-active':''}" onclick="toggleFavorite(${hero.id},${!hero.favorite})" title="${hero.favorite?'Aus meiner Liste':'Zu meiner Liste'}">${hero.favorite?'♥':'♡'}</button>
          <button class="round-action" onclick="pickSomething()" title="Überrasch mich">⤨</button>
        </div>
        ${heroResume?`<div class="immersive-progress"><div><span>Weiter bei ${fmtTime(hero.position)}</span><b>${Math.round(heroPct)}%</b></div><i><em style="width:${heroPct}%"></em></i></div>`:''}
        <div class="hero-dots">${heroes.map((_,i)=>`<button class="${i===heroIndex?'active':''}" onclick="setHero(${i})" aria-label="Spotlight ${i+1}"></button>`).join('')}</div>
      </div>
      <div class="hero-stack"><div class="hero-stack-label">ALS NÄCHSTES</div>${spotlight.map(m=>miniPoster(m,heroes.indexOf(m))).join('')}</div>
      <div class="immersive-stats"><span><b>${state.media.length}</b><small>Filme</small></span><span><b>${watchedCount}</b><small>gesehen</small></span><span><b>${continueItems.length}</b><small>offen</small></span></div>
    </section>`:''}
    <div class="shelf-zone">
      ${featureStrip(showcase)}
      ${homeSection('Weiterschauen',continueItems,'DEIN FORTSCHRITT')}
      ${homeSection('Meine Liste',favorites,'VON DIR GEMERKT')}
      ${homeSection('Zuletzt angesehen',history,'DEINE LETZTEN FILMABENDE')}
      ${homeSection('Frisch eingetroffen',newest,'NEU IN DEINER SAMMLUNG')}
      ${homeSection('Klassiker',classics,'VOR 2000')}
      ${homeSection('Moderne Favoriten',modern,'AB 2010')}
      ${homeSection('Noch ungesehen',unseen,'NOCH NICHT ENTDECKT')}
    </div>
    ${!state.media.length?'<div class="empty home-empty">Noch keine Medien. Lege unter Verwaltung eine Bibliothek an.</div>':''}
  `;
  scheduleHeroRotation()
}

function filteredMovies(){
  let items=state.media.slice();
  if(movieFilter==='unseen')items=items.filter(m=>mediaProgress(m)<1);
  if(movieFilter==='continue')items=items.filter(m=>m.progress>1&&m.progress<95);
  if(movieFilter==='watched')items=items.filter(m=>mediaProgress(m)>=95);
  if(movieDecade!=='all'){let d=Number(movieDecade);items=items.filter(m=>m.year>=d&&m.year<d+10)}
  if(movieSort==='title')items.sort((a,b)=>String(a.title||'').localeCompare(String(b.title||''),'de'));
  if(movieSort==='year')items.sort((a,b)=>(b.year||0)-(a.year||0));
  return items
}
function setMovieFilter(filter){movieFilter=filter;renderMovies()}
function setMovieSort(sort){movieSort=sort;renderMovies()}
function setMovieDecade(decade){movieDecade=decade;renderMovies()}
function setMovieView(view){movieView=view;renderMovies()}
function renderMovies(){
  if(!$('#content'))return;
  let items=filteredMovies(),watched=state.media.filter(m=>mediaProgress(m)>=95).length,continuing=state.media.filter(m=>m.progress>1&&m.progress<95).length;
  let filters=[['all','Alle'],['unseen','Ungesehen'],['continue','Weiterschauen'],['watched','Gesehen']];
  let decades=[1970,1980,1990,2000,2010,2020].filter(d=>state.media.some(m=>m.year>=d&&m.year<d+10));
  $('#content').innerHTML=`<section class="movies-page theater-library">
    <div class="library-aurora"></div>
    <div class="movies-heading">
      <div><div class="section-kicker">BUDDYFLIX ARCHIV</div><h1>Deine Filme.</h1><p>${state.media.length} Titel auf deinem Server · ${watched} gesehen · ${continuing} angefangen</p></div>
      <button class="btn primary surprise-btn" onclick="pickSomething()">⤨ Überrasch mich</button>
    </div>
    <div class="library-toolbar">
      <div class="movie-filterbar">${filters.map(([id,label])=>`<button class="${movieFilter===id?'active':''}" onclick="setMovieFilter('${id}')">${label}</button>`).join('')}</div>
      <div class="library-tools">
        <div class="view-group"><span>Ansicht</span><button class="${movieView==='posters'?'active':''}" onclick="setMovieView('posters')" title="Posterwand">▦</button><button class="${movieView==='cinema'?'active':''}" onclick="setMovieView('cinema')" title="CineWall">▰</button></div>
        <div class="sort-group"><span>Sortieren</span><button class="${movieSort==='recent'?'active':''}" onclick="setMovieSort('recent')">Neu</button><button class="${movieSort==='title'?'active':''}" onclick="setMovieSort('title')">A–Z</button><button class="${movieSort==='year'?'active':''}" onclick="setMovieSort('year')">Jahr</button></div>
      </div>
    </div>
    <div class="decade-bar"><button class="${movieDecade==='all'?'active':''}" onclick="setMovieDecade('all')">Alle Jahre</button>${decades.map(d=>`<button class="${String(movieDecade)===String(d)?'active':''}" onclick="setMovieDecade('${d}')">${String(d).slice(2)}er</button>`).join('')}</div>
    <div class="library-resultline"><strong>${items.length}</strong><span>Treffer in dieser Ansicht</span><em>${movieView==='cinema'?'CineWall':'Posterwand'}</em></div>
    ${movieView==='cinema'
      ? `<div class="cinewall">${items.map((m,i)=>landscapeCard(m,i)).join('')||'<div class="empty movies-empty">In dieser Ansicht gibt es gerade nichts zu sehen.</div>'}</div>`
      : `<div class="movie-grid theater-grid">${items.map(card).join('')||'<div class="empty movies-empty">In dieser Ansicht gibt es gerade nichts zu sehen.</div>'}</div>`}
  </section>`
}

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
  let el=document.createElement('div');el.className='modal detail-modal detail-fullscreen';
  let poster=m.poster?`<div class="detail-poster-xl"><img src="${esc(m.poster)}" alt=""></div>`:'<div class="detail-poster-xl detail-poster-empty"><span>▶</span><small>Kein Poster</small></div>';
  let status=watched?'✓ Gesehen':resumable?'▶ Angefangen':'Noch nicht angesehen';
  let ext=(m.path||'').split('.').pop()?.toUpperCase(),size=m.size?humanSize(m.size):'',source=m.metadata_provider?m.metadata_provider==='thetvdb'?'TheTVDB':m.metadata_provider.toUpperCase():'';
  let tech=[ext,size].filter(Boolean).join(' · ');
  el.innerHTML=`<div class="detail-screen">
    <div class="detail-screen-bg" style="background-image:url('${esc(m.backdrop||m.poster||'')}')"></div>
    <div class="detail-screen-fade"></div>
    <button class="close detail-close" aria-label="Schließen">×</button>
    <div class="detail-screen-inner">
      ${poster}
      <div class="detail-screen-copy">
        <div class="detail-kicker">BUDDYFLIX FEATURE</div>
        <h1>${esc(m.title)}</h1>
        <div class="detail-meta-line"><span>${m.year||'Jahr unbekannt'}</span><span>${status}</span>${m.duration>0?`<span>${fmtTime(m.duration)}</span>`:''}</div>
        <p>${esc(m.overview||'Für diesen Titel wurden noch keine Metadaten geladen. Du kannst ihn trotzdem direkt abspielen oder später identifizieren.')}</p>
        ${resumable?`<div class="detail-progress-xl"><div><b>Weiterschauen</b><span>${fmtTime(m.position)} / ${fmtTime(m.duration)} · ${Math.round(pct)}%</span></div><i><em style="width:${pct}%"></em></i></div>`:''}
        <div class="detail-screen-actions"><button class="btn primary detail-play" id="p">▶ ${resumable?'Fortsetzen':'Abspielen'}</button><button class="btn ghost detail-favorite ${m.favorite?'active':''}" id="favorite">${m.favorite?'♥ In meiner Liste':'♡ Meine Liste'}</button>${resumable?'<button class="btn ghost" id="restart">↺ Von Anfang</button>':''}<button class="btn ghost" id="meta">◎ Identifizieren</button><button class="btn ghost" id="editmedia">✎ Bearbeiten</button></div>
        <div class="detail-tech">${source?`<span>Quelle <b>${esc(source)}</b></span>`:''}${tech?`<span>${esc(tech)}</span>`:''}</div>
      </div>
    </div>
  </div>`;
  document.body.append(el);
  const close=closeWithEscape(el);
  el.querySelector('.detail-close').onclick=close;
  el.querySelector('#p').onclick=()=>{close();play(id,resumable?'resume':'start')};
  el.querySelector('#favorite').onclick=async()=>{close();await toggleFavorite(id,!m.favorite)};
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
