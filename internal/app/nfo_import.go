package app

import (
    "encoding/json"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
)

// Jellyfin's NFO Saver writes Kodi-style movie/episode XML beside the video.
// Never rewrite .nfo, video files or playback/profiles during this import.
type nfoUniqueID struct {
    Type string `xml:"type,attr"`
    Value string `xml:",chardata"`
}
type nfoActor struct {
    Name string `xml:"name"`
    Role string `xml:"role"`
}
type jellyfinNFO struct {
    XMLName xml.Name
    Title string `xml:"title"`
    OriginalTitle string `xml:"originaltitle"`
    SortTitle string `xml:"sorttitle"`
    Tagline string `xml:"tagline"`
    Plot string `xml:"plot"`
    Outline string `xml:"outline"`
    Year string `xml:"year"`
    Premiered string `xml:"premiered"`
    Released string `xml:"releasedate"`
    Runtime string `xml:"runtime"`
    Rating string `xml:"rating"`
    MPAA string `xml:"mpaa"`
    ID string `xml:"id"`
    IMDbID string `xml:"imdbid"`
    TMDbID string `xml:"tmdbid"`
    TVDbID string `xml:"tvdbid"`
    UniqueIDs []nfoUniqueID `xml:"uniqueid"`
    Genres []string `xml:"genre"`
    Tags []string `xml:"tag"`
    Studios []string `xml:"studio"`
    Directors []string `xml:"director"`
    Credits []string `xml:"credits"`
    Actors []nfoActor `xml:"actor"`
    Country []string `xml:"country"`
    Thumb string `xml:"thumb"`
    Art struct {
        Poster string `xml:"poster"`
        Fanart string `xml:"fanart"`
        Thumb string `xml:"thumb"`
        Banner string `xml:"banner"`
    } `xml:"art"`
    Fanart struct {
        Thumb string `xml:"thumb"`
    } `xml:"fanart"`
    ShowTitle string `xml:"showtitle"`
    Season int `xml:"season"`
    Episode int `xml:"episode"`
}

type nfoHit struct {
    ID int64 `json:"id"`
    Path string `json:"path"`
    File string `json:"nfo_file"`
    Title string `json:"title"`
    IDs map[string]string `json:"external_ids"`
    Document jellyfinNFO `json:"-"`
    Poster string `json:"-"`
    Backdrop string `json:"-"`
}
type nfoPreview struct {
    Total int `json:"total"`
    Found int `json:"found"`
    Missing int `json:"missing"`
    Invalid int `json:"invalid"`
    NoIDs int `json:"without_ids"`
    Examples []nfoHit `json:"examples"`
    Problems []string `json:"problems"`
    Applied int `json:"applied,omitempty"`
    Failed int `json:"failed,omitempty"`
    ImagesCached int `json:"images_cached,omitempty"`
}
var imdbPattern = regexp.MustCompile(`^tt[0-9]{6,12}$`)
var numericID = regexp.MustCompile(`^[0-9]{1,20}$`)
var providerName = regexp.MustCompile(`^[a-z0-9_-]{2,32}$`)

func nfoIDs(n jellyfinNFO) map[string]string {
    ids:=map[string]string{}
    set:=func(kind,val string) {
        kind=strings.ToLower(strings.TrimSpace(kind))
        kind=strings.TrimSuffix(kind,"id")
        kind=strings.TrimSpace(kind)
        switch kind {case "thetvdb":kind="tvdb";case "themoviedb":kind="tmdb"}
        val=strings.TrimSpace(val)
        if kind=="imdb" && imdbPattern.MatchString(val) { ids["imdb"]=val; return }
        if (kind=="tmdb" || kind=="tvdb") && numericID.MatchString(val) { ids[kind]=val; return }
        if providerName.MatchString(kind) && val!="" && len(val)<128 { ids[kind]=val }
    }
    for _, v:=range n.UniqueIDs { set(v.Type,v.Value) }
    set("imdb",n.IMDbID)
    set("tmdb",n.TMDbID)
    set("tvdb",n.TVDbID)
    if imdbPattern.MatchString(strings.TrimSpace(n.ID)) { set("imdb",n.ID) }
    return ids
}

func readNFO(path string) (jellyfinNFO,error) {
    file,err:=os.Open(path); if err!=nil { return jellyfinNFO{},err }
    defer file.Close()
    info,err:=file.Stat(); if err!=nil { return jellyfinNFO{},err }
    if !info.Mode().IsRegular() || info.Size()>1024*1024 { return jellyfinNFO{},fmt.Errorf("NFO too large or not regular") }
    var n jellyfinNFO
    decoder:=xml.NewDecoder(io.LimitReader(file,1024*1024+1))
    decoder.Strict=true
    if err:=decoder.Decode(&n);err!=nil {return jellyfinNFO{},err}
    tag:=strings.ToLower(n.XMLName.Local)
    if tag!="movie" && tag!="episodedetails" { return jellyfinNFO{},fmt.Errorf("unsupported NFO type %q",tag) }
    if strings.TrimSpace(n.Title)=="" && len(nfoIDs(n))==0 {return jellyfinNFO{},fmt.Errorf("NFO contains neither title nor IDs")}
    return n,nil
}

func nfoLocation(m Media) (string,bool,error) {
    dir:=filepath.Dir(m.Path)
    stem:=strings.TrimSuffix(filepath.Base(m.Path),filepath.Ext(m.Path))
    // Filename-based NFO is unambiguous even in folders with many films.
    candidate:=filepath.Join(dir,stem+".nfo")
    if info,err:=os.Stat(candidate);err==nil && info.Mode().IsRegular(){return candidate,true,nil}else if err!=nil&&!os.IsNotExist(err){return "",false,err}
    if strings.EqualFold(m.Kind,"episode") {return "",false,nil}
    // movie.nfo is safe only when exactly one playable video lives in the folder.
    candidate=filepath.Join(dir,"movie.nfo")
    if _,err:=os.Stat(candidate);os.IsNotExist(err){return "",false,nil}else if err!=nil{return "",false,err}
    entries,err:=os.ReadDir(dir);if err!=nil{return "",false,err}
    count:=0
    for _, e:=range entries {if !e.IsDir() && videoExt[strings.ToLower(filepath.Ext(e.Name()))]{count++}}
    if count!=1{return "",false,nil}
    return candidate,true,nil
}

func nfoImagePath(m Media,n jellyfinNFO,kind string) (string,bool) {
    dir:=filepath.Dir(m.Path)
    stem:=strings.TrimSuffix(filepath.Base(m.Path),filepath.Ext(m.Path))
    candidates:=[]string{}
    if kind=="poster" {
        candidates=append(candidates,n.Art.Poster,n.Art.Thumb,n.Thumb,
            stem+"-poster.jpg",stem+"-poster.png",stem+"-poster.webp",
            "poster.jpg","poster.png","poster.webp","folder.jpg","folder.png","folder.webp")
    } else {
        candidates=append(candidates,n.Art.Fanart,n.Fanart.Thumb,
            stem+"-fanart.jpg",stem+"-fanart.png",stem+"-backdrop.jpg",
            "fanart.jpg","fanart.png","backdrop.jpg","background.jpg","backdrop.png")
    }
    for _,candidate:=range candidates {
        candidate=strings.TrimSpace(candidate)
        if candidate=="" {continue}
        // Only display files stored in the movie's own folder.
        name:=filepath.Base(candidate)
        if name=="."||name==".."||name=="" {continue}
        path:=filepath.Join(dir,name)
        ext:=strings.ToLower(filepath.Ext(path))
        if ext!=".jpg"&&ext!=".jpeg"&&ext!=".png"&&ext!=".webp" {continue}
        info,err:=os.Stat(path)
        if err!=nil||!info.Mode().IsRegular()||info.Size()>20*1024*1024 {continue}
        // Refuse symlinks (including the last component) which might escape the library.
        link,err:=os.Lstat(path);if err!=nil||!link.Mode().IsRegular(){continue}
        return path,true
    }
    return "",false
}

func nfoArtURL(m Media,n jellyfinNFO,kind string) string {
    value:=""
    if kind=="poster" {value=n.Art.Poster; if value==""{value=n.Art.Thumb};if value==""{value=n.Thumb}
    }else{value=n.Art.Fanart;if value==""{value=n.Fanart.Thumb}}
    value=strings.TrimSpace(value)
    if strings.HasPrefix(value,"https://") || strings.HasPrefix(value,"http://") {return value}
    if _,ok:=nfoImagePath(m,n,kind);ok {
        return "/api/nfo/art?id="+strconv.FormatInt(m.ID,10)+"&type="+kind
    }
    return ""
}

func buildNFOPreview(items []Media) (nfoPreview,[]nfoHit) {
    report:=nfoPreview{Examples:[]nfoHit{},Problems:[]string{}}
    hits:=[]nfoHit{}
    for _,m:=range items {
        if m.Missing || m.Path=="" {continue}
        // NFO does not identify videos automatically; the source file path does.
        if m.Kind=="other" {continue}
        report.Total++
        file,exists,err:=nfoLocation(m)
        if err!=nil {
            report.Invalid++
            if len(report.Problems)<8{report.Problems=append(report.Problems,filepath.Base(m.Path)+": "+err.Error())}
            continue
        }
        if !exists {report.Missing++;continue}
        n,err:=readNFO(file)
        if err!=nil {
            report.Invalid++
            if len(report.Problems)<8{report.Problems=append(report.Problems,filepath.Base(file)+": "+err.Error())}
            continue
        }
        // A movie.nfo must never be applied to an episode, and vice versa.
        if (m.Kind=="episode") != (strings.EqualFold(n.XMLName.Local,"episodedetails")) {
            report.Invalid++
            if len(report.Problems)<8{report.Problems=append(report.Problems,filepath.Base(file)+": NFO type does not match media")}
            continue
        }
        ids:=nfoIDs(n)
        if len(ids)==0 {report.NoIDs++}
        hit:=nfoHit{ID:m.ID,Path:m.Path,File:file,Title:n.Title,IDs:ids,Document:n,
            Poster:nfoArtURL(m,n,"poster"),Backdrop:nfoArtURL(m,n,"backdrop")}
        hits=append(hits,hit)
        report.Found++
        if len(report.Examples)<12 {report.Examples=append(report.Examples,hit)}
    }
    return report,hits
}

func cleanedList(items []string) []string {
    out:=[]string{}
    seen:=map[string]bool{}
    for _,v:=range items {
        v=strings.TrimSpace(v)
        if v==""||seen[v]{continue}
        seen[v]=true;out=append(out,v)
    }
    return out
}
func importedNFO(m Media,h nfoHit) Media {
    n:=h.Document
    // Deliberately reset BuddyFlix's old assignments and metadata only for an NFO hit.
    m.MetadataState="nfo"
    m.MetadataConfidence=100
    m.PendingMetadata=nil
    m.MetadataProvider="jellyfin-nfo"
    m.MetadataUpdated=time.Now().Format(time.RFC3339)
    m.MetadataLocked=true // Prevent later title-search jobs from replacing the imported assignment.
    m.ExternalIDs=h.IDs
    m.MetadataSources=map[string]string{}
    originalTitle:=strings.TrimSpace(n.Title)
    if originalTitle!="" {m.Title=originalTitle;m.MetadataSources["title"]="jellyfin-nfo"}
    m.OriginalTitle=strings.TrimSpace(n.OriginalTitle)
    m.Tagline=strings.TrimSpace(n.Tagline)
    m.SortTitle=strings.TrimSpace(n.SortTitle)
    m.Premiered=strings.TrimSpace(n.Premiered)
    if m.Premiered=="" {m.Premiered=strings.TrimSpace(n.Released)}
    m.Overview=strings.TrimSpace(n.Plot)
    if m.Overview=="" {m.Overview=strings.TrimSpace(n.Outline)}
    m.Year=0
    if year,err:=strconv.Atoi(strings.TrimSpace(n.Year));err==nil {m.Year=year}
    if m.Year==0&&len(m.Premiered)>=4 {m.Year,_=strconv.Atoi(m.Premiered[:4])}
    m.Poster=h.Poster
    m.Backdrop=h.Backdrop
    m.Genres=cleanedList(n.Genres)
    m.Tags=cleanedList(n.Tags)
    m.Studios=cleanedList(n.Studios)
    m.Directors=cleanedList(n.Directors)
    m.Writers=cleanedList(n.Credits)
    m.Countries=cleanedList(n.Country)
    m.Actors=[]string{}
    for _,a:=range n.Actors {if a.Name!="" {m.Actors=append(m.Actors,strings.TrimSpace(a.Name))}}
    m.Actors=cleanedList(m.Actors)
    m.Rating=strings.TrimSpace(n.Rating)
    m.ContentRating=strings.TrimSpace(n.MPAA)
    if minutes,err:=strconv.Atoi(strings.TrimSpace(n.Runtime));err==nil&&minutes>0 {m.Runtime=minutes}else{m.Runtime=0}
    for _,key:=range []string{"year","overview","poster","backdrop","original_title","tagline","genres","actors","studios","directors","writers","rating","runtime"}{
        m.MetadataSources[key]="jellyfin-nfo"
    }
    // Do not touch m.ID, m.Path, m.LibraryID, user progress, or profile favorites.
    return m
}

// Art is copied to BuddyFlix's own persistent data folder before committing the
// metadata. The copied images continue working if all Jellyfin NFOs and artwork
// are subsequently removed from the movie directory.
func (s *Server) cacheNFOArt(m Media, n jellyfinNFO, kind string) (string, bool, error) {
    src,local:=nfoImagePath(m,n,kind)
    if !local {
        return nfoArtURL(m,n,kind),false,nil
    }
    dir:=filepath.Join(s.cfg.DataDir,"nfo-art")
    if err:=os.MkdirAll(dir,0700);err!=nil {return "",false,err}
    ext:=strings.ToLower(filepath.Ext(src))
    name:=strconv.FormatInt(m.ID,10)+"-"+kind
    final:=filepath.Join(dir,name+ext)
    in,err:=os.Open(src);if err!=nil{return "",false,err}
    defer in.Close()
    // The file may change between preview and import. Never cache anything
    // except a regular image within the configured maximum size.
    stat,err:=in.Stat()
    if err!=nil{return "",false,err}
    if !stat.Mode().IsRegular()||stat.Size()>20*1024*1024 {return "",false,fmt.Errorf("image is not regular or too large: %s",filepath.Base(src))}
    temp,err:=os.CreateTemp(dir,".import-*")
    if err!=nil{return "",false,err}
    tmpName:=temp.Name()
    defer os.Remove(tmpName)
    count,copyErr:=io.Copy(temp,io.LimitReader(in,20*1024*1024+1))
    if syncErr:=temp.Sync();copyErr==nil {copyErr=syncErr}
    if closeErr:=temp.Close();copyErr==nil {copyErr=closeErr}
    if copyErr!=nil{return "",false,copyErr}
    if count>20*1024*1024 {return "",false,fmt.Errorf("image exceeds 20 MiB")}
    if err:=os.Chmod(tmpName,0600);err!=nil{return "",false,err}
    if err:=os.Rename(tmpName,final);err!=nil{return "",false,err}
    for _,oldExt:=range []string{".jpg",".jpeg",".png",".webp"} {
        if oldExt!=ext {_=os.Remove(filepath.Join(dir,name+oldExt))}
    }
    return "/api/nfo/art?id="+strconv.FormatInt(m.ID,10)+"&type="+kind,true,nil
}

// Reads only an image that BuddyFlix previously copied into its own DataDir.
// No NFO or media-folder lookup is required for this path.
func (s *Server) cachedArt(id int64,kind string) (string,bool) {
    if id<1||(kind!="poster"&&kind!="backdrop"){return "",false}
    base:=filepath.Join(s.cfg.DataDir,"nfo-art",strconv.FormatInt(id,10)+"-"+kind)
    for _,ext:=range []string{".jpg",".jpeg",".png",".webp"} {
        filename:=base+ext
        st,err:=os.Lstat(filename)
        if err==nil&&st.Mode().IsRegular()&&st.Size()<=20*1024*1024{return filename,true}
    }
    return "",false
}

func (s *Server) nfoImport(w http.ResponseWriter,r *http.Request) {
    if r.Method!=http.MethodGet && r.Method!=http.MethodPost {jsonErr(w,405,"method not allowed");return}
    s.mu.RLock()
    items:=append([]Media(nil),s.st.Media...)
    s.mu.RUnlock()
    preview,hits:=buildNFOPreview(items)
    if r.Method==http.MethodGet {jsonOut(w,preview);return}

    // A confirmation flag prevents accidental destructive imports from generic clients.
    var request struct{ Confirm bool `json:"confirm"` }
    if json.NewDecoder(io.LimitReader(r.Body,2048)).Decode(&request)!=nil || !request.Confirm {
        jsonErr(w,400,"explicit confirm=true required");return
    }
    // No NFO found = change absolutely nothing.
    if len(hits)==0 {jsonOut(w,preview);return}
    byID:=make(map[int64]nfoHit,len(hits))
    mediaByID:=make(map[int64]Media,len(items))
    for _,m:=range items {mediaByID[m.ID]=m}
    for _,h:=range hits {
        m:=mediaByID[h.ID]
        poster,localPoster,err:=s.cacheNFOArt(m,h.Document,"poster")
        if err!=nil {
            preview.Failed++
            if len(preview.Problems)<8 {preview.Problems=append(preview.Problems,filepath.Base(m.Path)+": Poster konnte nicht gesichert werden: "+err.Error())}
            continue
        }
        backdrop,localBackdrop,err:=s.cacheNFOArt(m,h.Document,"backdrop")
        if err!=nil {
            preview.Failed++
            if len(preview.Problems)<8 {preview.Problems=append(preview.Problems,filepath.Base(m.Path)+": Hintergrundbild konnte nicht gesichert werden: "+err.Error())}
            continue
        }
        if localPoster {preview.ImagesCached++}
        if localBackdrop {preview.ImagesCached++}
        h.Poster=poster
        h.Backdrop=backdrop
        byID[h.ID]=h
    }
    s.mu.Lock()
    original:=s.st.Media
    updated:=append([]Media(nil),original...)
    for i,m:=range updated {
        if h,ok:=byID[m.ID];ok && m.Path==h.Path && !m.Missing {
            updated[i]=importedNFO(m,h)
            preview.Applied++
        }
    }
    s.st.Media=updated
    if err:=s.saveLocked();err!=nil {
        s.st.Media=original
        s.mu.Unlock()
        jsonErr(w,500,"NFO import could not be saved: "+err.Error())
        return
    }
    s.mu.Unlock()
    jsonOut(w,preview)
}

func (s *Server) nfoArt(w http.ResponseWriter,r *http.Request) {
    if r.Method!=http.MethodGet {jsonErr(w,405,"method not allowed");return}
    id,err:=strconv.ParseInt(r.URL.Query().Get("id"),10,64)
    kind:=r.URL.Query().Get("type")
    if err!=nil||id<1||(kind!="poster"&&kind!="backdrop"){jsonErr(w,400,"invalid art request");return}
    s.mu.RLock()
    var m Media
    found:=false
    for _,x:=range s.st.Media {if x.ID==id && !x.Missing {m=x;found=true;break}}
    s.mu.RUnlock()
    if !found||m.Path=="" {http.NotFound(w,r);return}
    img,ok:=s.cachedArt(id,kind)
    if !ok {
        // Compatibility fallback for images imported by earlier dev.123.
        // Re-import while the NFOs still exist to create permanent copies.
        file,exists,err:=nfoLocation(m)
        if err!=nil||!exists {http.NotFound(w,r);return}
        n,err:=readNFO(file)
        if err!=nil {http.NotFound(w,r);return}
        img,ok=nfoImagePath(m,n,kind)
        if !ok {http.NotFound(w,r);return}
    }
    f,err:=os.Open(img);if err!=nil {http.NotFound(w,r);return}
    defer f.Close()
    info,err:=f.Stat()
    if err!=nil||!info.Mode().IsRegular(){http.NotFound(w,r);return}
    switch strings.ToLower(filepath.Ext(img)){
    case ".png":w.Header().Set("Content-Type","image/png")
    case ".webp":w.Header().Set("Content-Type","image/webp")
    default:w.Header().Set("Content-Type","image/jpeg")
    }
    w.Header().Set("Cache-Control","private, max-age=120")
    http.ServeContent(w,r,filepath.Base(img),info.ModTime(),f)
}
