/* BuddyFlix Next — standalone web client. No dependency on legacy app.js. */
(() => {
"use strict";
const root=document.getElementById("app");
const state={
  page:"start",previous:"filme",adminTab:"overview",movieFilter:"all",movieSort:"new",
  query:"",seriesQuery:"",season:null,detail:null,show:null,
  media:[],shows:[],libraries:[],profiles:[],profile:null,system:null,settings:null,
  providers:[],job:null,allMedia:[],loading:false,setup:false,authed:false
};
const E=s=>String(s??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
const num=n=>Number(n||0).toLocaleString("de-DE");
const pct=m=>Math.max(0,Math.min(100,Number(m?.progress||0)));
const duration=s=>{s=Math.max(0,Math.floor(Number(s)||0));return [Math.floor(s/3600),Math.floor((s%3600)/60),s%60].filter((n,i)=>i>0||n>0).map((n,i)=>i?String(n).padStart(2,"0"):String(n)).join(":")||"0:00"};
const q=(s,parent=document)=>parent.querySelector(s);
const qa=(s,parent=document)=>[...parent.querySelectorAll(s)];
const data=(obj)=>JSON.stringify(obj);
const art=(m,wide=false)=>wide?(m?.backdrop||m?.poster||""):(m?.poster||m?.backdrop||"");
const img=(src,cls,alt)=>src?'<img class="'+cls+'" src="'+E(src)+'" alt="'+E(alt||"")+'" loading="lazy" decoding="async">':'<div class="'+cls+' art-missing"><span>▶</span><small>BuddyFlix</small></div>';
const normalize=(x)=>String(x||"").toLocaleLowerCase("de");
const isEpisode=m=>m?.kind==="episode";
const movies=()=>state.media.filter(m=>!isEpisode(m));
const watched=m=>pct(m)>=95;
const inProgress=m=>pct(m)>1&&pct(m)<95;
const continued=()=>state.media.filter(inProgress).sort((a,b)=>String(b.progress_updated||"").localeCompare(String(a.progress_updated||"")));
const favorites=()=>state.media.filter(m=>m.favorite);
const mById=id=>state.media.find(m=>m.id===Number(id))||state.allMedia.find(m=>m.id===Number(id));
const sByKey=key=>state.shows.find(s=>s.key===key);
const episodes=s=>(s?.seasons||[]).flatMap(x=>x.episodes||[]).sort((a,b)=>(a.season-b.season)||(a.episode-b.episode));
const percentWatched=s=>s.episode_count?Math.min(100,Math.round(100*s.watched_count/s.episode_count)):0;
const errorMessage=e=>e?.message||"Unbekannter Fehler";
async function api(path,options={}){
  const headers={"Accept":"application/json",...(options.body?{"Content-Type":"application/json"}:{}),...(options.headers||{})};
  const res=await fetch(path,{credentials:"same-origin",...options,headers});
  const text=await res.text();
  let parsed={};try{parsed=JSON.parse(text)}catch{}
  if(!res.ok){
    if(res.status===401&&state.authed){state.authed=false;state.page="login";render()}
    throw new Error(parsed.error||"HTTP "+res.status);
  }
  return parsed;
}
function toast(text,type="info"){
  let host=q("#toasts");if(!host){host=document.createElement("div");host.id="toasts";document.body.append(host)}
  const node=document.createElement("div");node.className="toast "+type;node.textContent=text;host.append(node);
  setTimeout(()=>node.remove(),4000);
}
function error(e){toast(errorMessage(e),"error")}
async function loadData(){
  const [system,media,shows,libraries,profileInfo]=await Promise.all([
    api("/api/system"),api("/api/media"),api("/api/series"),api("/api/libraries"),api("/api/profiles")
  ]);
  Object.assign(state,{system,media,shows,libraries,profiles:profileInfo.profiles||[],
    profile:(profileInfo.profiles||[]).find(p=>p.id===profileInfo.active_id)||(profileInfo.profiles||[])[0]||null,authed:true});
}
async function boot(){
  try{
    const info=await api("/api/setup/status");
    if(!info.setup_done){state.page="setup";state.setup=true;render();return}
    await loadData();state.page="start";render();
    const p=await api("/api/profiles");
    if(state.profiles.length>1&&!p.selected)showProfiles();
  }catch(e){state.page="login";render()}
}
const logo='<img src="/assets/buddyflix-logo.png" alt="BuddyFlix" class="bf-logo">';
const symbol={start:"◈",filme:"▦",serien:"▤",weiter:"↗",merkliste:"♡",admin:"⚙"};
const pages=[["start","Übersicht"],["filme","Filme"],["serien","Serien"],["weiter","Weiterschauen"],["merkliste","Merkliste"]];
const btn=(text,action="",cls="",attrs="")=>'<button type="button" class="button '+cls+'" data-action="'+E(action)+'" '+attrs+'>'+text+'</button>';
function shell(content){
  const page=state.page;
  const active=page==="detail"?state.previous:page==="show"?"serien":page;
  root.innerHTML='<div class="appframe"><header class="global-nav"><div class="nav-brand">'+logo+'<span class="edition">MEDIA SPACE</span></div>'+
    '<nav class="nav-items" aria-label="BuddyFlix Bereiche">'+pages.map(([id,label])=>
      '<button class="nav-link '+(active===id?"selected":"")+'" data-page="'+id+'" '+(active===id?'aria-current="page"':"")+'><span class="nav-symbol">'+symbol[id]+'</span><span>'+label+'</span></button>'
    ).join("")+'</nav><div class="global-tools"><label class="search-field"><span>⌕</span><input id="globalSearch" type="search" placeholder="In deiner Sammlung suchen" value="'+E(state.query)+'" aria-label="Suche"></label>'+
    '<button class="icon-button admin-link '+(active==="admin"?"selected":"")+'" data-page="admin" title="Verwaltung" aria-label="Verwaltung">⚙</button>'+
    '<button class="profile-trigger" data-action="profiles" title="Profil wechseln"><span class="profile-avatar">'+E(state.profile?.avatar||"🍿")+'</span><span>'+E(state.profile?.name||"Profil")+'</span><b>⌄</b></button></div></header>'+
    '<main class="main-stage" id="mainStage">'+content+'</main>'+
    '<footer class="site-foot"><span>DEIN KINO. DEINE REGELN.</span><span>'+E(state.system?.server_name||"BuddyFlix")+' · '+E(state.system?.version||"dev")+'</span></footer></div>';
}
function basePage(title,tagline,extra=""){
  return '<div class="page-intro"><div><div class="eyebrow">'+E(tagline)+'</div><h1>'+E(title)+'</h1></div>'+extra+'</div>';
}
function smallArt(m,wide=false){
  return '<div class="tile-art">'+img(art(m,wide),"tile-image",m.title)+
   (inProgress(m)?'<div class="tile-progress"><i style="width:'+pct(m)+'%"></i></div>':"")+
   (watched(m)?'<span class="tile-done">✓</span>':"")+'</div>';
}
function tile(m,kind="portrait"){
  return '<article class="media-tile '+kind+'" tabindex="0" role="button" data-action="detail" data-id="'+m.id+'" aria-label="'+E(m.title)+'">'+
    smallArt(m,kind==="landscape")+
    '<div class="tile-label"><strong>'+E(m.title)+'</strong><span>'+
    E(isEpisode(m)?(m.series_title||"Serie")+' · S'+String(m.season||0).padStart(2,"0")+"E"+String(m.episode||0).padStart(2,"0"):[m.year||"",m.runtime?m.runtime+" Min.":""].filter(Boolean).join(" · "))+
    '</span></div></article>';
}
function seriesTile(s){
  const e=s.next_episode||episodes(s)[0];
  return '<article class="media-tile show-tile" tabindex="0" role="button" data-action="show" data-key="'+E(s.key)+'">'+
    '<div class="tile-art">'+img(art(s),"tile-image",s.title)+'<span class="show-count">'+num(s.season_count)+' Staffeln</span>'+
    (s.watched_count?'<div class="tile-progress"><i style="width:'+percentWatched(s)+'%"></i></div>':"")+'</div>'+
    '<div class="tile-label"><strong>'+E(s.title)+'</strong><span>'+num(s.episode_count)+' Folgen · '+num(s.watched_count)+' gesehen</span></div></article>';
}
function horizontal(items,kind="portrait",className=""){
  return '<div class="horizontal-track '+className+'">'+items.map(m=>kind==="series"?seriesTile(m):tile(m,kind)).join("")+'</div>';
}
function sectionHead(title,subtitle,view){
 return '<div class="block-heading"><div><h2>'+E(title)+'</h2><p>'+E(subtitle)+'</p></div>'+
  (view?'<button class="inline-link" data-page="'+E(view)+'">Alle ansehen →</button>':"")+'</div>';
}
function start(){
  const arr=movies(),resume=continued(),fresh=arr.slice(0,8),showArr=state.shows.slice(0,7),pick=arr.find(m=>m.backdrop)||arr[0];
  const watchedCount=arr.filter(watched).length;
  const last=resume[0]||fresh[0];
  const feature=pick?'<article class="featured"><div class="featured-image">'+img(art(pick,true),"fill-art",pick.title)+'</div>'+
    '<div class="featured-copy"><div class="eyebrow light">HEUTE IM KINO</div><h3>'+E(pick.title)+'</h3><p>'+E(pick.overview||"Dein nächster Filmabend wartet.")+'</p>'+
    '<div class="feature-meta">'+E(pick.year||"Film")+' · '+(watched(pick)?"Schon gesehen":"Bereit zum Abspielen")+'</div>'+
    btn("▶ Jetzt ansehen","play","light",'data-id="'+pick.id+'"')+' '+btn("Details","detail","subtle",'data-id="'+pick.id+'"')+'</div></article>':
    '<div class="empty-state">Noch keine Filme. Lege in der Verwaltung deine erste Bibliothek an.</div>';
  return '<div class="dashboard">'+
    '<div class="welcome"><div><div class="eyebrow">BUDDYFLIX / DEIN MEDIENRAUM</div><h1>Ein guter Abend<br>beginnt <i>hier.</i></h1><p>Willkommen zurück, '+E(state.profile?.name||"Filmfan")+'. Das läuft auf deinem eigenen Server.</p></div>'+
    '<div class="welcome-stats"><div><b>'+num(arr.length)+'</b><span>Filme</span></div><div><b>'+num(state.shows.length)+'</b><span>Serien</span></div><div><b>'+num(watchedCount)+'</b><span>Gesehen</span></div></div></div>'+
    '<div class="dashboard-bento"><div class="bento-feature">'+feature+'</div>'+
    '<section class="resume-pod"><div class="eyebrow">DEIN NÄCHSTER KLICK</div><h2>Wo warst du?</h2>'+
    (last?'<div class="resume-poster">'+img(art(last,true),"fill-art",last.title)+'</div>'+
      '<div class="resume-info"><h3>'+E(last.title)+'</h3><p>'+(inProgress(last)?Math.round(pct(last))+"% angesehen":"Frisch in deiner Bibliothek")+'</p>'+
      (inProgress(last)?'<div class="progress-rule"><i style="width:'+pct(last)+'%"></i></div>':"")+
      btn(inProgress(last)?"▶ Fortsetzen":"▶ Abspielen","play","primary full",'data-id="'+last.id+'" '+(inProgress(last)?'data-mode="resume"':""))+'</div>':
      '<p>Noch nichts abgespielt.</p>')+'</section></div>'+
    '<div class="dashboard-overview"><div class="overview-title">'+sectionHead("Frisch auf deinem Server","Neue Filme zum Entdecken","filme")+'</div>'+
    (fresh.length?'<div class="dashboard-posters">'+fresh.slice(0,6).map(m=>tile(m)).join("")+'</div>':'<div class="empty-state">Noch keine Filme in der Bibliothek.</div>')+'</div>'+
    (showArr.length?'<section class="dashboard-section">'+sectionHead("Serien, die bleiben","Staffeln und Folgen auf einen Blick","serien")+horizontal(showArr,"series")+'</section>':"")+
    (resume.length>1?'<section class="dashboard-section">'+sectionHead("Weiterschauen","Du bist noch nicht fertig","weiter")+horizontal(resume.slice(1,9),"landscape")+'</section>':"")+
    '</div>';
}
function catalog(){
  const all=movies(),sort=state.movieSort,filter=state.movieFilter,search=normalize(state.query.trim());
  let filtered=all.filter(m=>(!search||normalize(m.title+" "+(m.overview||"")).includes(search))&&
    (filter==="all"||filter==="unseen"&&pct(m)<1||filter==="watched"&&watched(m)||filter==="ongoing"&&inProgress(m)));
  if(sort==="title")filtered.sort((a,b)=>a.title.localeCompare(b.title,"de"));
  if(sort==="year")filtered.sort((a,b)=>(b.year||0)-(a.year||0));
  if(sort==="new")filtered.sort((a,b)=>String(b.added||"").localeCompare(String(a.added||"")));
  const filters=[["all","Alle Filme"],["unseen","Ungesehen"],["ongoing","Angefangen"],["watched","Gesehen"]];
  return '<div class="content-page">'+basePage("Filme","DEINE BIBLIOTHEK",'<span class="count-motif">'+num(filtered.length)+' / '+num(all.length)+'</span>')+
    '<div class="catalog-top"><div class="filter-chips">'+filters.map(([id,label])=>'<button class="chip '+(filter===id?"active":"")+'" data-filter="'+id+'">'+label+'</button>').join("")+'</div>'+
    '<label class="sort-label">Sortieren <select id="movieSort"><option value="new" '+(sort==="new"?"selected":"")+'>Neueste</option>'+
    '<option value="title" '+(sort==="title"?"selected":"")+'>Titel A–Z</option><option value="year" '+(sort==="year"?"selected":"")+'>Jahr</option></select></label></div>'+
    (filtered.length?'<div class="poster-grid">'+filtered.map(m=>tile(m)).join("")+'</div>':'<div class="empty-state">Keine Filme für diese Auswahl. Versuch einen anderen Filter.</div>')+'</div>';
}
function showsPage(){
  const items=state.shows.filter(s=>!state.query||normalize(s.title).includes(normalize(state.query)));
  return '<div class="content-page">'+basePage("Serienwelten","STAFFEL FÜR STAFFEL",'<span class="count-motif">'+num(items.length)+' Serien</span>')+
    '<p class="page-desc">Jede Geschichte. Jede Staffel. Dein persönlicher Fortschritt.</p>'+
    (items.length?'<div class="series-gallery">'+items.map(seriesTile).join("")+'</div>':'<div class="empty-state">Noch keine Serien – oder keine Treffer für deine Suche.</div>')+'</div>';
}
function continuePage(){
  const items=continued().filter(m=>!state.query||normalize(m.title).includes(normalize(state.query)));
  const history=state.media.filter(m=>watched(m)).sort((a,b)=>String(b.progress_updated||"").localeCompare(String(a.progress_updated||""))).slice(0,12);
  return '<div class="content-page">'+basePage("Bleib dran.","DEIN FORTSCHRITT")+
    '<p class="page-desc">Hier geht es genau dort weiter, wo du aufgehört hast.</p>'+
    (items.length?'<div class="continue-grid">'+items.map(m=>'<article class="continue-item"><button class="continue-art" data-action="play" data-id="'+m.id+'" data-mode="resume">'+img(art(m,true),"fill-art",m.title)+'<span class="overlay-play">▶</span></button>'+
      '<div class="continue-info"><div class="eyebrow">'+(isEpisode(m)?"SERIE":"FILM")+'</div><h3>'+E(m.title)+'</h3><p>'+E(isEpisode(m)?m.series_title||"":m.year||"")+'</p>'+
      '<div class="progress-rule"><i style="width:'+pct(m)+'%"></i></div><small>'+Math.round(pct(m))+'% angesehen</small>'+
      btn("▶ Weiterschauen","play","primary",'data-id="'+m.id+'" data-mode="resume"')+'</div></article>').join("")+'</div>':'<div class="empty-state">Noch nichts angefangen. Such dir deinen nächsten Film aus.</div>')+
    (history.length?'<section class="dashboard-section">'+sectionHead("Schon gesehen","Dein Filmverlauf")+horizontal(history)+'</section>':"")+'</div>';
}
function listPage(){
 const items=favorites().filter(m=>!state.query||normalize(m.title).includes(normalize(state.query)));
 return '<div class="content-page">'+basePage("Für später.","DEINE MERKLISTE",'<span class="count-motif">'+num(items.length)+' Titel</span>')+
 '<p class="page-desc">Alles, was du nicht aus den Augen verlieren willst. Nur für dein Profil.</p>'+
 (items.length?'<div class="poster-grid">'+items.map(m=>tile(m)).join("")+'</div>':'<div class="empty-state"><div class="empty-symbol">♡</div><h3>Deine Liste ist noch leer.</h3><p>Ein Herz in der Filmansicht merkt Titel hier vor.</p>'+btn("Filme entdecken","goto","primary",'data-target="filme"')+'</div>')+'</div>';
}
function detailPage(){
 const m=mById(state.detail);if(!m)return catalog();
 const resume=inProgress(m);
 return '<div class="detail-page"><button class="back-link" data-action="back">← Zurück zur Sammlung</button>'+
   '<div class="detail-layout"><div class="detail-visual">'+img(art(m,true),"fill-art",m.title)+'</div>'+
   '<section class="detail-copy"><div class="eyebrow">'+(isEpisode(m)?"EPISODE":"AUS DEINEM ARCHIV")+'</div>'+
   '<h1>'+E(m.title)+'</h1><div class="detail-pills">'+
     [m.year,isEpisode(m)?"S"+m.season+"E"+m.episode:"Film",m.runtime?m.runtime+" Min.":null,watched(m)?"✓ Gesehen":resume?"Angefangen":null].filter(Boolean).map(s=>'<span>'+E(s)+'</span>').join("")+
   '</div><p class="detail-description">'+E(m.overview||"Noch keine Beschreibung hinterlegt. Den Film kannst du trotzdem abspielen oder später identifizieren.")+'</p>'+
   (resume?'<div class="detail-progress"><span>Dein Fortschritt · '+Math.round(pct(m))+'%</span><div class="progress-rule"><i style="width:'+pct(m)+'%"></i></div></div>':"")+
   '<div class="detail-controls">'+btn(resume?"▶ Fortsetzen":"▶ Abspielen","play","primary",'data-id="'+m.id+'" '+(resume?'data-mode="resume"':""))+
   (resume?btn("↺ Von Anfang","play","",'data-id="'+m.id+'" data-mode="start"'):"")+
   btn(m.favorite?"♥ Gemerkt":"♡ Merken","favorite","",'data-id="'+m.id+'"')+
   btn(watched(m)?"✓ Als ungesehen":"✓ Als gesehen","watch","",'data-id="'+m.id+'"')+'</div>'+
   '<div class="detail-divider"></div><h2>Zum Titel</h2>'+
   '<div class="detail-extras"><div><small>Quelle</small><strong>'+E(m.metadata_provider||"Lokal")+'</strong></div>'+
   '<div><small>Format</small><strong>'+E(String(m.path||"").split(".").pop().toUpperCase())+'</strong></div></div>'+
   '<div class="detail-secondary">'+btn("Metadaten suchen","identify","",'data-id="'+m.id+'"')+
   btn("Titel bearbeiten","editMedia","",'data-id="'+m.id+'"')+'</div></section></div></div>';
}
function showPage(){
 const s=sByKey(state.show);if(!s)return showsPage();
 const seasons=s.seasons||[],selected=seasons.find(x=>x.number===state.season)||seasons[0],eps=selected?.episodes||[];
 const next=s.next_episode||episodes(s)[0];
 return '<div class="show-page"><button class="back-link" data-page="serien">← Alle Serien</button>'+
   '<div class="show-head"><div class="show-visual">'+img(art(s,true),"fill-art",s.title)+'</div>'+
   '<div class="show-heading"><span class="eyebrow">DEINE SERIENWELT</span><h1>'+E(s.title)+'</h1><p>'+num(s.season_count)+' Staffeln · '+num(s.episode_count)+' Folgen · '+num(s.watched_count)+' gesehen</p>'+
   '<div class="progress-rule"><i style="width:'+percentWatched(s)+'%"></i></div>'+
   (next?btn(inProgress(next)?"▶ Nächste Folge fortsetzen":"▶ Nächste Folge","play","primary",'data-id="'+next.id+'" '+(inProgress(next)?'data-mode="resume"':"")):"")+'</div></div>'+
   '<div class="episode-heading"><div><span class="eyebrow">EPISODEN</span><h2>Staffeln & Folgen</h2></div><div class="season-choices">'+seasons.map(x=>'<button class="chip '+(selected?.number===x.number?"active":"")+'" data-season="'+x.number+'">'+(x.number?"Staffel "+x.number:"Extras")+'</button>').join("")+'</div></div>'+
   '<div class="episode-list">'+eps.map(ep=>'<article class="episode-item"><button class="episode-image" data-action="play" data-id="'+ep.id+'">'+img(art(ep,true),"fill-art",ep.title)+'<span class="overlay-play">▶</span></button>'+
   '<div class="episode-content"><small>S'+String(ep.season||0).padStart(2,"0")+' · E'+String(ep.episode||0).padStart(2,"0")+'</small><h3>'+E(ep.title)+'</h3>'+
   '<p>'+E(ep.overview||"Eine Folge deiner Serie.")+'</p>'+(inProgress(ep)?'<div class="progress-rule"><i style="width:'+pct(ep)+'%"></i></div>':"")+'</div>'+
   '<div class="episode-actions">'+(watched(ep)?'<small>✓ Gesehen</small>':"")+btn("▶","play","",'data-id="'+ep.id+'" '+(inProgress(ep)?'data-mode="resume"':""))+
   btn("ⓘ","detail","",'data-id="'+ep.id+'"')+'</div></article>').join("")+'</div></div>';
}

/* Independent management workspace: sections, not a single 10000px-long page. */
const tabs=[["overview","Zentrale"],["libraries","Bibliotheken"],["metadata","Metadaten"],["medien","Medienpflege"],["profiles","Profile"],["settings","System"]];
function admin(){
 const t=state.adminTab,nav='<div class="workspace-tabs">'+tabs.map(([key,label])=>'<button class="'+(t===key?"current":"")+'" data-admintab="'+key+'">'+label+'</button>').join("")+'</div>';
 const c=state.system||{},cfg=state.settings||{};
 const incomplete=state.allMedia.filter(m=>!m.missing&&(!m.poster||!m.overview));
 const reviews=state.allMedia.filter(m=>m.metadata_state==="review"&&m.pending_metadata);
 let body="";
 if(t==="overview")body='<div class="workspace-tiles">'+
   [['Bibliotheken',state.libraries.length,'libraries'],['Indizierte Medien',state.allMedia.length,'medien'],['Metadaten offen',incomplete.length,'metadata'],['Bitte prüfen',reviews.length,'metadata']].map(([title,v,to])=>
     '<button class="workspace-tile" data-admintab="'+to+'"><span>'+title+'</span><b>'+num(v)+'</b><em>Öffnen ↗</em></button>').join("")+
   '</div><div class="workspace-panels"><section class="workspace-panel"><span class="eyebrow">DEIN SERVER</span><h2>'+E(c.server_name||cfg.server_name||"BuddyFlix")+'</h2>'+
   '<p>Verwaltung ohne unnötige Umwege. Links die Medien, rechts die Technik.</p>'+
   '<div class="panel-buttons">'+btn("Bibliotheken scannen","scan","primary")+btn("Metadaten starten","bulkMeta")+'</div></section>'+
   '<section class="workspace-panel"><span class="eyebrow">SYSTEMSTATUS</span><h2>Alles im Blick.</h2>'+
   '<dl class="mini-facts"><dt>Version</dt><dd>'+E(c.version||"dev")+'</dd><dt>Plattform</dt><dd>'+E(c.os||"")+" / "+E(c.arch||"")+'</dd>'+
   '<dt>Speicherbedarf</dt><dd>'+num(c.memory_mb)+' MB RAM</dd><dt>Bibliotheks-Scan</dt><dd>'+(c.scanning?"Läuft":"Bereit")+'</dd></dl></section></div>';
 if(t==="libraries")body='<div class="workspace-panel"><div class="panel-heading"><div><h2>Deine Medienorte</h2><p>BuddyFlix scannt nur diese Ordner. Medien auf dem NAS werden nicht verändert.</p></div>'+
   btn("＋ Bibliothek hinzufügen","libraryNew","primary")+'</div>'+
   (state.libraries.length?'<div class="library-rows">'+state.libraries.map(l=>'<article class="library-row"><div class="library-icon">'+(l.type==="shows"?"▤":"▦")+'</div>'+
   '<div><strong>'+E(l.name)+'</strong><p>'+E(l.path)+'</p><small>'+E(l.type||"movies")+'</small></div>'+
   '<div class="row-buttons">'+btn("Bearbeiten","libraryEdit","",'data-id="'+l.id+'"')+btn("Entfernen","libraryDelete","danger",'data-id="'+l.id+'"')+'</div></article>').join("")+'</div>':
   '<div class="empty-state">Noch keine Bibliotheken angelegt.</div>')+
   '<div class="panel-buttons">'+btn("↻ Alle Bibliotheken scannen","scan","primary")+'</div></div>';
 if(t==="metadata")body='<div class="workspace-panels"><section class="workspace-panel"><div class="eyebrow">IDENTIFIZIERUNG</div><h2>Metadaten-Zentrale</h2>'+
  '<p>Automatische Suche für Filme ohne Bilder und Beschreibung. Unsichere Vorschläge landen zur manuellen Prüfung.</p>'+
  '<div class="provider-pills">'+state.providers.map(p=>'<span class="provider-pill">'+E(p.name||"Provider")+' · '+(p.configured?"bereit":"nicht eingerichtet")+'</span>').join("")+'</div>'+
  btn("Metadatenlauf starten","bulkMeta","primary")+
  '<div id="jobStatus">'+jobMarkup()+'</div></section>'+
  '<section class="workspace-panel"><div class="eyebrow">OFFENE AUFGABEN</div><h2>Jetzt wichtig</h2>'+
  '<div class="task-line"><b>'+num(reviews.length)+'</b><span>Treffer zur Prüfung</span></div>'+
  '<div class="task-line"><b>'+num(incomplete.length)+'</b><span>Medien ohne vollständige Metadaten</span></div>'+
  btn("Zur Medienpflege","gotoAdmin","",'data-tab="medien"')+
  '<p class="attribution">Metadata provided by <a href="https://thetvdb.com" target="_blank" rel="noopener noreferrer">TheTVDB</a>.</p></section></div>';
 if(t==="medien")body='<div class="workspace-panel"><div class="panel-heading"><div><h2>Medienpflege</h2><p>Metadaten zuordnen, Titel bearbeiten und verwaiste Einträge kontrollieren.</p></div>'+
   '<div class="row-buttons">'+btn("Fehlende bereinigen","cleanup","danger")+'</div></div>'+
   '<div class="data-toolbar"><label>Anzeige <select id="adminFilter"><option value="review">Bitte prüfen</option><option value="incomplete">Ohne Metadaten</option><option value="missing">Fehlende Datei</option><option value="all">Alle Medien</option></select></label>'+
   '<span>'+num(state.allMedia.length)+' Medien insgesamt</span></div><div id="adminMediaList">'+adminMediaList("review")+'</div></div>';
 if(t==="profiles")body='<div class="workspace-panel"><div class="panel-heading"><div><h2>Profile verwalten</h2><p>Fortschritt und Merkliste sind je Profil getrennt.</p></div>'+
    btn("＋ Neues Profil","profileNew","primary")+'</div><div class="profile-work-grid">'+state.profiles.map(p=>
    '<div class="profile-work-card"><span>'+E(p.avatar||"🎬")+'</span><strong>'+E(p.name)+'</strong><div>'+
    btn("Auswählen","profileSelect","",'data-id="'+p.id+'"')+
    btn("Bearbeiten","profileEdit","",'data-id="'+p.id+'"')+
    (state.profiles[0]?.id!==p.id?btn("Löschen","profileDelete","danger",'data-id="'+p.id+'"'):"")+
    '</div></div>').join("")+'</div></div>';
 if(t==="settings")body='<div class="workspace-panels"><section class="workspace-panel"><div class="eyebrow">SERVER</div><h2>Grundkonfiguration</h2>'+
   '<form id="serverForm" class="stack-form"><label>Servername<input name="server_name" required value="'+E(cfg.server_name||"BuddyFlix")+'"></label>'+
   '<label>Administrator<input name="admin_user" required value="'+E(cfg.admin_user||"admin")+'"></label>'+
   '<p>Server-ID: <code>'+E(cfg.server_id||"")+'</code></p>'+
   '<button class="button primary">Speichern</button></form></section>'+
   '<section class="workspace-panel"><div class="eyebrow">SICHERHEIT</div><h2>Passwort ändern</h2>'+
   '<form id="passwordForm" class="stack-form"><label>Aktuelles Passwort<input type="password" name="current" required autocomplete="current-password"></label>'+
   '<label>Neues Passwort<input type="password" name="next" minlength="8" required autocomplete="new-password"></label>'+
   '<label>Wiederholen<input type="password" name="repeat" minlength="8" required autocomplete="new-password"></label>'+
   '<button class="button primary">Passwort ändern</button></form></section>'+
   '<section class="workspace-panel"><div class="eyebrow">METADATEN</div><h2>TheTVDB</h2><p>BuddyFlix nutzt den konfigurierten TheTVDB-Zugang. Optional kannst du einen eigenen Schlüssel hinterlegen.</p>'+
   '<form id="tvdbForm" class="stack-form"><label>Eigener API-Key<input type="password" name="key" placeholder="Optionaler eigener Schlüssel"></label>'+
   '<button class="button primary">Override speichern</button></form>'+
   (cfg.tvdb_override?btn("Override entfernen","tvdbClear","danger"):"")+'</section>'+
   '<section class="workspace-panel"><div class="eyebrow">TECHNIK</div><h2>Systemdetails</h2>'+
   '<dl class="mini-facts"><dt>Listen-Adresse</dt><dd>'+E(cfg.listen||"")+'</dd>'+
   '<dt>Datenordner</dt><dd>'+E(cfg.data_dir||"")+'</dd><dt>CPU Threads</dt><dd>'+num(c.cpus)+'</dd>'+
   '<dt>Version</dt><dd>'+E(c.version||"")+'</dd></dl>'+
   '<div class="panel-buttons">'+btn("Abmelden","logout")+'</div></section></div>';
 return '<div class="workspace">'+basePage("BuddyFlix Studio","SYSTEM / VERWALTUNG")+
 nav+'<div class="workspace-body">'+body+'</div></div>';
}
function adminMediaList(filter){
 const items=state.allMedia.filter(m=>filter==="review"?m.metadata_state==="review"&&m.pending_metadata:
   filter==="incomplete"?!m.poster||!m.overview:
   filter==="missing"?m.missing:true).slice(0,180);
 return items.length?'<div class="admin-media-rows">'+items.map(m=>'<div class="admin-media-row">'+img(m.poster,"admin-thumb",m.title)+
 '<div class="admin-media-title"><b>'+E(m.title)+'</b><span>'+E(m.metadata_state||"Ohne Zuordnung")+' · '+E(m.path||"")+'</span></div>'+
 '<div class="row-buttons">'+(m.pending_metadata?btn("Vorschlag übernehmen","applyPending","primary",'data-id="'+m.id+'"'):"")+
 btn("Suchen","identify","",'data-id="'+m.id+'"')+btn("Bearbeiten","editMedia","",'data-id="'+m.id+'"')+'</div></div>').join("")+'</div>':
 '<div class="empty-state">Keine passenden Medien. Hier ist alles erledigt.</div>';
}
function jobMarkup(){
 const j=state.job;
 if(!j)return '<p class="muted">Noch kein Metadatenlauf gestartet.</p>';
 const n=Math.min(100,Math.round(100*(j.done||0)/Math.max(1,j.total||0)));
 return '<div class="job-box"><div><strong>'+E(j.running?"Suche läuft …":"Letzter Durchlauf")+'</strong><span>'+n+'%</span></div>'+
 '<div class="progress-rule"><i style="width:'+n+'%"></i></div>'+
 '<p>'+num(j.done)+' / '+num(j.total)+' geprüft · '+num(j.auto_matched)+' automatisch · '+num(j.review)+' prüfen</p>'+
 (j.error?'<p class="error-text">'+E(j.error)+'</p>':"")+'</div>';
}
async function loadAdmin(){
 const [settings,providers,all,job]=await Promise.all([
  api("/api/settings"),api("/api/metadata/providers"),api("/api/media?include_missing=1"),api("/api/metadata/bulk")
 ]);
 Object.assign(state,{settings,providers,allMedia:all,job});
}
function modal(body,cls=""){
 closeModal();
 const wrap=document.createElement("div");wrap.id="dialogOverlay";wrap.className="overlay";
 wrap.innerHTML='<div class="dialog '+cls+'" role="dialog" aria-modal="true" aria-label="BuddyFlix Dialog"><button class="close-dialog" data-action="close">×</button>'+body+'</div>';
 document.body.append(wrap);q("button,input,select",wrap)?.focus();
}
function closeModal(){q("#dialogOverlay")?.remove()}
function confirmBox(title,detail,run,label="Bestätigen"){
 modal('<div class="eyebrow">BESTÄTIGUNG</div><h2>'+E(title)+'</h2><p>'+E(detail)+'</p>'+
 '<div class="dialog-actions"><button class="button" data-action="close">Abbrechen</button><button class="button danger" id="confirmYes">'+E(label)+'</button></div>');
 q("#confirmYes").onclick=async()=>{try{await run();closeModal()}catch(e){error(e)}};
}
function showProfiles(){
 modal('<div class="eyebrow">PERSÖNLICH</div><h2>Wer schaut?</h2><p>Deine Wiedergabe. Deine Liste. Dein Profil.</p>'+
 '<div class="profile-chooser">'+state.profiles.map(p=>'<button class="profile-choice" data-action="profileSelect" data-id="'+p.id+'"><span>'+E(p.avatar||"🎬")+'</span><b>'+E(p.name)+'</b>'+
 (state.profile?.id===p.id?'<small>Aktiv</small>':"")+'</button>').join("")+
 (state.profiles.length<8?'<button class="profile-choice add-profile" data-action="profileNew"><span>＋</span><b>Hinzufügen</b></button>':"")+'</div>'+
 '<div class="dialog-actions">'+btn("Profile verwalten","gotoAdmin","",'data-tab="profiles"')+'</div>',"wide-dialog");
}
function profileEditor(p){
 const avatar=p?.avatar||"🎬";
 modal('<div class="eyebrow">PROFIL</div><h2>'+(p?"Profil bearbeiten":"Neues Profil")+'</h2>'+
 '<form id="profileForm" class="stack-form" data-id="'+(p?.id||"")+'">'+
 '<label>Name<input name="name" required maxlength="30" value="'+E(p?.name||"")+'"></label>'+
 '<label>Avatar auswählen<select name="avatar">'+["🎬","🍿","😎","🚀","🦊","🐼","⭐","🎧"].map(a=>'<option '+(a===avatar?"selected":"")+'>'+a+'</option>').join("")+'</select></label>'+
 '<div class="dialog-actions"><button type="button" class="button" data-action="close">Abbrechen</button><button class="button primary">Speichern</button></div></form>');
}
function libraryEditor(l){
 modal('<div class="eyebrow">BIBLIOTHEK</div><h2>'+(l?"Bibliothek bearbeiten":"Bibliothek hinzufügen")+'</h2>'+
 '<form id="libraryForm" class="stack-form" data-id="'+(l?.id||"")+'">'+
 '<label>Name<input name="name" required value="'+E(l?.name||"")+'" placeholder="Meine Filme"></label>'+
 '<label>Typ<select name="type">'+[["movies","Filme"],["shows","Serien"],["other","Sonstige Videos"]].map(([x,y])=>'<option value="'+x+'" '+(l?.type===x?"selected":"")+'>'+y+'</option>').join("")+'</select></label>'+
 '<label>Ordner auf dem NAS<input name="path" required value="'+E(l?.path||"")+'" placeholder="/share/Multimedia/Filme"></label>'+
 '<p>Der Pfad muss auf dem QNAP bereits existieren.</p>'+
 '<div class="dialog-actions"><button type="button" class="button" data-action="close">Abbrechen</button><button class="button primary">Speichern</button></div></form>');
}
function mediaEditor(m){
 modal('<div class="eyebrow">MEDIENEDITOR</div><h2>'+E(m.title)+'</h2>'+
 '<form id="mediaForm" class="stack-form" data-id="'+m.id+'">'+
 '<label>Titel<input name="title" required value="'+E(m.title)+'"></label>'+
 '<label>Jahr<input name="year" type="number" min="1888" max="2100" value="'+E(m.year||"")+'"></label>'+
 '<label>Inhaltsangabe<textarea name="overview" rows="4">'+E(m.overview||"")+'</textarea></label>'+
 '<label>Poster-URL<input name="poster" value="'+E(m.poster||"")+'"></label>'+
 '<label>Hintergrundbild-URL<input name="backdrop" value="'+E(m.backdrop||"")+'"></label>'+
 '<div class="dialog-actions"><button class="button" type="button" data-action="close">Abbrechen</button><button class="button primary">Speichern</button></div></form>');
}
async function identify(m){
 modal('<div class="eyebrow">METADATEN</div><h2>Film identifizieren</h2>'+
 '<form id="lookupForm" class="lookup-form" data-id="'+m.id+'"><label>Titel<input name="q" required value="'+E(m.title)+'"></label>'+
 '<label>Jahr<input name="year" type="number" value="'+E(m.year||"")+'"></label><button class="button primary">Suchen</button></form>'+
 '<div id="lookupResults" class="lookup-results"><p>Suche einen passenden Titel aus.</p></div>',"wide-dialog");
}
let metadataResults=[];
async function showSearchResults(form){
 const host=q("#lookupResults");host.innerHTML='<p>Suche läuft …</p>';
 const m=Number(form.dataset.id);
 try{
  metadataResults=await api("/api/metadata/search?q="+encodeURIComponent(form.q.value)+"&year="+encodeURIComponent(form.year.value||""));
  host.innerHTML=metadataResults.length?metadataResults.slice(0,20).map((x,i)=>
   '<div class="lookup-row">'+img(x.poster,"lookup-image",x.title)+'<div><b>'+E(x.title)+'</b><p>'+
   E(x.year||"")+' · '+E(x.provider||"TheTVDB")+'</p></div>'+
   btn("Übernehmen","applyResult","primary",'data-id="'+m+'" data-index="'+i+'"')+'</div>').join(""):
   '<div class="empty-state">Keine Treffer. Versuche einen anderen Filmtitel.</div>';
 }catch(e){host.innerHTML='<p class="error-text">'+E(errorMessage(e))+'</p>'}
}
async function refresh({renderNow=true}={}){
 await loadData();if(state.page==="admin")await loadAdmin();if(renderNow)render();
}
function loginScreen(mode){
 const setup=mode==="setup";
 root.innerHTML='<div class="auth-layout"><aside class="auth-presentation"><div class="auth-brand">'+logo+'</div>'+
  '<div class="auth-copy"><div class="eyebrow">DEIN EIGENES KINO</div><h1>Alles, was du liebst.<br><i>Nur für dich.</i></h1>'+
  '<p>Deine Filme, deine Serien, dein Zuhause für gute Geschichten.</p></div>'+
  '<div class="auth-footer">BUDDYFLIX · PRIVATE MEDIA SPACE</div></aside>'+
  '<section class="auth-panel"><div class="auth-card"><div class="eyebrow">'+(setup?"ERSTEINRICHTUNG":"WILLKOMMEN ZURÜCK")+'</div>'+
  '<h2>'+(setup?"Mach es zu deinem.":"Schön, dass du da bist.")+'</h2><p>'+(setup?"Richte deinen Medienserver einmalig ein.":"Einloggen und direkt weiterschauen.")+'</p>'+
  (setup?'<form id="setupForm" class="stack-form"><label>Servername<input name="server_name" required value="BuddyFlix"></label>'+
   '<label>Administrator<input name="admin_user" required value="admin"></label><label>Passwort<input type="password" name="password" required minlength="8"></label>'+
   '<label>Passwort wiederholen<input type="password" name="repeat" required minlength="8"></label>'+
   '<button class="button primary full">BuddyFlix einrichten</button></form>':
   '<form id="loginForm" class="stack-form"><label>Benutzer<input name="username" required autocomplete="username" autofocus></label>'+
   '<label>Passwort<input type="password" name="password" required autocomplete="current-password"></label>'+
   '<button class="button primary full">Anmelden →</button></form>')+
   '<p class="auth-hint">Deine Mediathek bleibt auf deinem Server.</p></div></section></div>';
}
function render(){
 if(state.page==="login"||state.page==="setup"){loginScreen(state.page);return}
 const page=state.page;
 const map={start,filme:catalog,serien:showsPage,weiter:continuePage,merkliste:listPage,detail:detailPage,show:showPage,admin};
 shell((map[page]||start)());
}
function route(page){
 if(!state.authed)return;
 state.query=page==="filme"?state.query:"";
 state.page=page;
 if(page==="admin")loadAdmin().then(()=>render()).catch(error);
 else render();
 window.scrollTo({top:0,behavior:"instant"});
}
async function doMediaAction(id,action){
 await api("/api/media/action",{method:"POST",body:data({media_id:Number(id),action})});
 await refresh();toast("Änderung gespeichert","success");
}
function openPlayer(m,mode){
 const node=document.createElement("div");
 node.className="player-view";node.id="bfPlayer";
 node.innerHTML='<video controls autoplay playsinline preload="metadata" src="/stream/'+m.id+'"></video>'+
  '<div class="player-top"><div><b>'+E(m.title)+'</b><span>'+E(isEpisode(m)?m.series_title||"":m.year||"")+'</span></div>'+
  '<button aria-label="Player schließen" id="closePlayer">×</button></div>'+
  '<div id="playerError" class="player-error" hidden>Der Film konnte nicht gestartet werden. Prüfe, ob das Format im Browser unterstützt wird.</div>';
 document.body.append(node);
 const video=q("video",node);let closed=false,last=0;
 const save=async()=>{const p=video.currentTime,d=video.duration;if(!Number.isFinite(d)||d<=0)return;
  try{await api("/api/progress",{method:"POST",keepalive:true,body:data({media_id:m.id,Position:p,Duration:d})})}catch{}};
 const close=async()=>{
  if(closed)return;closed=true;const next=isEpisode(m)?nextEpisode(m):null;
  document.removeEventListener("keydown",escape);
  video.pause();await save();node.remove();await refresh();if(next&&video.ended)toast("Nächste Folge: "+next.title);
 };
 const escape=e=>{if(e.key==="Escape")close()};
 document.addEventListener("keydown",escape);
 q("#closePlayer",node).onclick=close;
 video.addEventListener("loadedmetadata",()=>{if(mode==="resume"&&m.position>20&&m.position<video.duration-20)video.currentTime=m.position});
 video.addEventListener("timeupdate",()=>{if(Date.now()-last>12000){last=Date.now();save()}});
 video.addEventListener("seeked",save);
 video.addEventListener("pause",save);
 video.addEventListener("ended",close);
 video.addEventListener("error",()=>{q("#playerError",node).hidden=false});
 video.play().catch(()=>{});
}
function nextEpisode(m){const s=state.shows.find(x=>x.title===m.series_title);if(!s)return null;return episodes(s).find(x=>x.season>m.season||x.season===m.season&&x.episode>m.episode)}
async function handleAction(btn){
 const action=btn.dataset.action,id=Number(btn.dataset.id||0);
 try{
 switch(action){
 case "close":closeModal();break;
 case "goto":closeModal();route(btn.dataset.target);break;
 case "gotoAdmin":closeModal();state.adminTab=btn.dataset.tab||"overview";route("admin");break;
 case "profiles":showProfiles();break;
 case "back":route(state.previous||"filme");break;
 case "detail":state.previous=["start","filme","weiter","merkliste"].includes(state.page)?state.page:"filme";state.detail=id;route("detail");break;
 case "show":state.show=btn.dataset.key;state.season=null;route("show");break;
 case "play":{const m=mById(id);if(m)openPlayer(m,btn.dataset.mode||"start");else toast("Film nicht gefunden","error");break}
 case "favorite":{const m=mById(id);await doMediaAction(id,m?.favorite?"unfavorite":"favorite");break}
 case "watch":{const m=mById(id);await doMediaAction(id,watched(m)?"mark_unwatched":"mark_watched");break}
 case "scan":{const x=await api("/api/scan",{method:"POST"});toast(num(x.files_seen)+" Dateien gefunden","success");await refresh();break}
 case "bulkMeta":{state.job=await api("/api/metadata/bulk",{method:"POST"});toast("Metadatenlauf gestartet","success");render();pollJob();break}
 case "cleanup":confirmBox("Fehlende Einträge bereinigen?","Die Medien werden nur aus dem Index entfernt, nicht vom NAS.",async()=>{
   await api("/api/media/cleanup",{method:"POST"});await refresh();toast("Bereinigung abgeschlossen","success")},"Bereinigen");break;
 case "libraryNew":libraryEditor(null);break;
 case "libraryEdit":libraryEditor(state.libraries.find(l=>l.id===id));break;
 case "libraryDelete":{const l=state.libraries.find(l=>l.id===id);if(!l)return;
  confirmBox("Bibliothek entfernen?","Auch die Indexeinträge der Bibliothek \""+l.name+"\" werden entfernt; Dateien bleiben erhalten.",async()=>{
    await api("/api/libraries?id="+id,{method:"DELETE"});await refresh();toast("Bibliothek entfernt","success")},"Entfernen");break}
 case "profileNew":profileEditor(null);break;
 case "profileEdit":profileEditor(state.profiles.find(p=>p.id===id));break;
 case "profileSelect":await api("/api/profiles/select",{method:"POST",body:data({profile_id:id})});
  closeModal();await refresh();route("start");break;
 case "profileDelete":{const p=state.profiles.find(p=>p.id===id);if(!p)return;
  confirmBox("Profil löschen?","Fortschritt und Merkliste von "+p.name+" gehen verloren.",async()=>{
    await api("/api/profiles?id="+id,{method:"DELETE"});await refresh();toast("Profil gelöscht","success")},"Profil löschen");break}
 case "editMedia":{const m=mById(id)||((await api("/api/media?id="+id+"&include_missing=1"))[0]);if(m)mediaEditor(m);break}
 case "identify":{const m=mById(id);if(m)identify(m);break}
 case "applyPending":{const m=mById(id);if(!m?.pending_metadata)return;await api("/api/metadata/apply",{method:"POST",body:data({media_id:id,result:m.pending_metadata})});await refresh();toast("Metadaten übernommen","success");break}
 case "applyResult":{const item=metadataResults[Number(btn.dataset.index)];if(!item)return;
  await api("/api/metadata/apply",{method:"POST",body:data({media_id:id,result:item})});closeModal();await refresh();toast("Metadaten gespeichert","success");break}
 case "tvdbClear":confirmBox("Override entfernen?","BuddyFlix nutzt danach wieder den verfügbaren Projektzugang.",async()=>{
  await api("/api/settings",{method:"PUT",body:data({server_name:state.settings.server_name,admin_user:state.settings.admin_user,tvdb_api_key:""})});await refresh();toast("Override entfernt","success")},"Entfernen");break;
 case "logout":await api("/api/logout",{method:"POST"});state.authed=false;state.page="login";render();break;
 }
 }catch(e){error(e)}
}
let pollHandle=null;
async function pollJob(){
 if(pollHandle)clearTimeout(pollHandle);
 try{state.job=await api("/api/metadata/bulk");if(state.page==="admin"&&state.adminTab==="metadata")q("#jobStatus")?.replaceChildren(htmlNode(jobMarkup()));
 if(state.job.running)pollHandle=setTimeout(pollJob,2500);else if(state.page==="admin"){await refresh()}
 }catch(e){error(e)}
}
function htmlNode(html){const d=document.createElement("div");d.innerHTML=html;return d}
document.addEventListener("click",e=>{
 const routeTarget=e.target.closest("[data-page]");if(routeTarget){route(routeTarget.dataset.page);return}
 const tab=e.target.closest("[data-admintab]");if(tab){state.adminTab=tab.dataset.admintab;render();return}
 const filter=e.target.closest("[data-filter]");if(filter){state.movieFilter=filter.dataset.filter;render();return}
 const season=e.target.closest("[data-season]");if(season){state.season=Number(season.dataset.season);render();return}
 const action=e.target.closest("[data-action]");if(action){handleAction(action)}
});
document.addEventListener("keydown",e=>{
 if(e.key==="Escape"&&q("#dialogOverlay")){closeModal();return}
 if((e.key==="Enter"||e.key===" ")&&e.target.matches("[role=button][data-action]")){e.preventDefault();handleAction(e.target)}
});
document.addEventListener("change",e=>{
 if(e.target.id==="movieSort"){state.movieSort=e.target.value;render()}
 if(e.target.id==="adminFilter")q("#adminMediaList").innerHTML=adminMediaList(e.target.value)
});
document.addEventListener("input",e=>{
 if(e.target.id==="globalSearch"){state.query=e.target.value;
  const main=q("#mainStage");
  if(state.page!=="filme"&&state.page!=="serien"&&state.page!=="merkliste"&&state.page!=="weiter"){state.page="filme"}
  if(main)main.innerHTML=({filme:catalog,serien:showsPage,merkliste:listPage,weiter:continuePage}[state.page]||catalog)();
  qa(".nav-link").forEach(b=>b.classList.toggle("selected",b.dataset.page===state.page));
 }
});
document.addEventListener("submit",async e=>{
 const form=e.target;if(!form.id)return;e.preventDefault();
 const val=k=>form.elements.namedItem(k)?.value||"";
 const send=(path,method,body)=>api(path,{method,body:data(body)});
 try{
 switch(form.id){
 case "loginForm":await send("/api/login","POST",{Username:val("username"),Password:val("password")});await loadData();state.page="start";render();break;
 case "setupForm":
  if(val("password")!==val("repeat"))throw Error("Die Passwörter stimmen nicht überein");
  await send("/api/setup","POST",{server_name:val("server_name"),admin_user:val("admin_user"),password:val("password")});
  state.page="login";render();toast("Einrichtung abgeschlossen","success");break;
 case "profileForm":{const id=Number(form.dataset.id);
  await send("/api/profiles"+(id?"?id="+id:""),id?"PUT":"POST",{Name:val("name"),Avatar:val("avatar")});
  closeModal();await refresh();toast("Profil gespeichert","success");break}
 case "libraryForm":{const id=Number(form.dataset.id);
  await send("/api/libraries"+(id?"?id="+id:""),id?"PUT":"POST",{name:val("name"),path:val("path"),type:val("type")});
  closeModal();await refresh();toast("Bibliothek gespeichert","success");break}
 case "mediaForm":await send("/api/media?id="+Number(form.dataset.id),"PUT",{title:val("title"),year:Number(val("year"))||0,
  overview:val("overview"),poster:val("poster"),backdrop:val("backdrop")});
  closeModal();await refresh();toast("Titel gespeichert","success");break;
 case "lookupForm":await showSearchResults(form);break;
 case "serverForm":await send("/api/settings","PUT",{server_name:val("server_name"),admin_user:val("admin_user")});
  await refresh();toast("Einstellungen gespeichert","success");break;
 case "tvdbForm":if(!val("key").trim())throw Error("Bitte einen Schlüssel eingeben");
  await send("/api/settings","PUT",{server_name:state.settings.server_name,admin_user:state.settings.admin_user,tvdb_api_key:val("key").trim()});
  await refresh();toast("TheTVDB-Zugang aktualisiert","success");break;
 case "passwordForm":if(val("next")!==val("repeat"))throw Error("Die neuen Passwörter stimmen nicht überein");
  await send("/api/password","POST",{current:val("current"),new:val("next")});
  state.authed=false;state.page="login";render();toast("Passwort geändert. Bitte neu anmelden.","success");break;
 }
 }catch(e){error(e)}
});
boot();
})();