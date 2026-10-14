package app

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

// ArtworkRepairJob is separate from title matching: only previously stored
// provider IDs are used. NFO data, media files and profiles are never changed.
type ArtworkRepairJob struct {
    Running bool `json:"running"`
    Total int `json:"total"`
    Done int `json:"done"`
    Posters int `json:"posters"`
    Backdrops int `json:"backdrops"`
    Unresolved int `json:"unresolved"`
    Errors int `json:"errors"`
    Skipped int `json:"skipped"`
    CurrentTitle string `json:"current_title,omitempty"`
    Warning string `json:"warning,omitempty"`
    Started string `json:"started,omitempty"`
    Finished string `json:"finished,omitempty"`
}

func approvedArtworkURL(value string) bool {
    parsed,err:=url.Parse(strings.TrimSpace(value))
    if err!=nil || parsed.Scheme!="https"||parsed.User!=nil||parsed.RawQuery!=""||parsed.Fragment!="" {return false}
    if parsed.Port()!="" && parsed.Port()!="443" {return false}
    switch strings.ToLower(parsed.Hostname()){
    case "image.tmdb.org","artworks.thetvdb.com":
        return true
    }
    return false
}

func (s *Server) persistRemoteArtwork(id int64,kind,source string) (string,error) {
    if !approvedArtworkURL(source) {return "",fmt.Errorf("untrusted artwork host")}
    if id<1||(kind!="poster"&&kind!="backdrop"){return "",fmt.Errorf("invalid artwork target")}
    client:=&http.Client{
        Timeout:18*time.Second,
        CheckRedirect:func(req *http.Request,via []*http.Request) error {
            if len(via)>=3||!approvedArtworkURL(req.URL.String()){return fmt.Errorf("artwork redirect is not allowed")}
            return nil
        },
    }
    resp,err:=client.Get(source)
    if err!=nil{return "",err}
    defer resp.Body.Close()
    if resp.StatusCode!=http.StatusOK{return "",fmt.Errorf("artwork server HTTP %d",resp.StatusCode)}
    const maxBytes=12*1024*1024
    body,err:=io.ReadAll(io.LimitReader(resp.Body,maxBytes+1))
    if err!=nil{return "",err}
    if len(body)>maxBytes||len(body)==0{return "",fmt.Errorf("artwork is empty or too large")}
    contentType:=http.DetectContentType(body)
    ext:=""
    switch {
    case strings.HasPrefix(contentType,"image/jpeg"):ext=".jpg"
    case strings.HasPrefix(contentType,"image/png"):ext=".png"
    case strings.HasPrefix(contentType,"image/webp"):ext=".webp"
    default:return "",fmt.Errorf("response is not a JPEG/PNG/WebP image")}
    directory:=filepath.Join(s.cfg.DataDir,"nfo-art")
    if err:=os.MkdirAll(directory,0700);err!=nil{return "",err}
    file,err:=os.CreateTemp(directory,".art-*")
    if err!=nil{return "",err}
    temp:=file.Name()
    defer os.Remove(temp)
    _,err=file.Write(body)
    if err==nil{err=file.Sync()}
    closeErr:=file.Close()
    if err==nil{err=closeErr}
    if err!=nil{return "",err}
    if err:=os.Chmod(temp,0600);err!=nil{return "",err}
    name:=strconv.FormatInt(id,10)+"-"+kind
    final:=filepath.Join(directory,name+ext)
    if err:=os.Rename(temp,final);err!=nil{return "",err}
    for _,other:=range []string{".jpg",".jpeg",".png",".webp"}{
        if other!=ext {_=os.Remove(filepath.Join(directory,name+other))}
    }
    return "/api/nfo/art?id="+strconv.FormatInt(id,10)+"&type="+kind,nil
}

// An exact IMDb ID lookup on TheTVDB avoids reintroducing the old title
// matching errors. Only a uniquely returned movie can be used.
func tvdbMovieForIMDb(p *TVDBProvider, imdb string) (string,error) {
    if !imdbPattern.MatchString(imdb){return "",fmt.Errorf("invalid IMDb id")}
    var response struct {
        Data []struct{
            Movie *struct{ID int64 `json:"id"`} `json:"movie"`
        } `json:"data"`
    }
    if err:=p.get("/search/remoteid/"+url.PathEscape(imdb),&response);err!=nil{return "",err}
    unique:=map[int64]bool{}
    for _,result:=range response.Data {
        if result.Movie!=nil&&result.Movie.ID>0 {unique[result.Movie.ID]=true}
    }
    if len(unique)!=1{return "",fmt.Errorf("IMDb ID cannot be linked to exactly one TheTVDB movie")}
    for id:=range unique{return strconv.FormatInt(id,10),nil}
    return "",fmt.Errorf("no TheTVDB movie")
}

func tmdbMovieForIMDb(p *TMDbProvider, imdb string) (string,error){
    if !imdbPattern.MatchString(imdb){return "",fmt.Errorf("invalid IMDb id")}
    v:=url.Values{"api_key":{p.apiKey},"external_source":{"imdb_id"}}
    req,err:=http.NewRequest(http.MethodGet,"https://api.themoviedb.org/3/find/"+url.PathEscape(imdb)+"?"+v.Encode(),nil)
    if err!=nil{return "",err}
    resp,err:=p.client.Do(req)
    if err!=nil{return "",err}
    defer resp.Body.Close()
    if resp.StatusCode!=200{return "",fmt.Errorf("TMDb IMDb lookup failed (%d)",resp.StatusCode)}
    var result struct{
        Movies []struct{ID int64 `json:"id"`} `json:"movie_results"`
    }
    if err:=json.NewDecoder(io.LimitReader(resp.Body,2<<20)).Decode(&result);err!=nil{return "",err}
    if len(result.Movies)!=1||result.Movies[0].ID<=0{return "",fmt.Errorf("IMDb ID does not identify one TMDb movie")}
    return strconv.FormatInt(result.Movies[0].ID,10),nil
}

func providerArtwork(m Media,tmdb *TMDbProvider,tvdb *TVDBProvider)(string,string) {
    if tmdb!=nil {
        id:=strings.TrimSpace(m.ExternalIDs["tmdb"])
        if !numericID.MatchString(id)&&imdbPattern.MatchString(m.ExternalIDs["imdb"]){
            if found,err:=tmdbMovieForIMDb(tmdb,m.ExternalIDs["imdb"]);err==nil{id=found}
        }
        if numericID.MatchString(id){
            if result,err:=tmdb.EnrichMovie(MetadataCandidate{Provider:"tmdb",ProviderID:id});err==nil&&result.Poster!="" {
                return result.Poster,result.Backdrop
            }
        }
    }
    if tvdb!=nil {
        id:=strings.TrimSpace(m.ExternalIDs["tvdb"])
        if !numericID.MatchString(id)&&imdbPattern.MatchString(m.ExternalIDs["imdb"]){
            if found,err:=tvdbMovieForIMDb(tvdb,m.ExternalIDs["imdb"]);err==nil{id=found}
        }
        if numericID.MatchString(id){
            if result,err:=tvdb.EnrichMovie(MetadataCandidate{Provider:"thetvdb",ProviderID:id});err==nil {
                return result.Poster,result.Backdrop
            }
        }
    }
    return "",""
}

func artworkCandidate(m Media) bool {
    if m.Missing||m.Kind=="other"||m.Kind=="episode"{return false}
    if m.ExternalIDs==nil {return false}
    return imdbPattern.MatchString(m.ExternalIDs["imdb"])||
        numericID.MatchString(m.ExternalIDs["tmdb"])||
        numericID.MatchString(m.ExternalIDs["tvdb"])
}

func (s *Server) artworkRepair(w http.ResponseWriter,r *http.Request){
    if r.Method!=http.MethodGet&&r.Method!=http.MethodPost{jsonErr(w,405,"method not allowed");return}
    s.artworkMu.Lock()
    if r.Method==http.MethodGet {
        snapshot:=s.artworkJob
        s.artworkMu.Unlock()
        jsonOut(w,snapshot)
        return
    }
    if s.artworkJob.Running {
        s.artworkMu.Unlock()
        jsonErr(w,409,"artwork repair already running")
        return
    }
    s.mu.RLock()
    items:=append([]Media(nil),s.st.Media...)
    s.mu.RUnlock()
    targets:=[]Media{}
    for _,m:=range items {
        if !artworkCandidate(m) {continue}
        _,hasPoster:=s.cachedArt(m.ID,"poster")
        _,hasBackdrop:=s.cachedArt(m.ID,"backdrop")
        if !hasPoster||!hasBackdrop{targets=append(targets,m)}
    }
    warning:=""
    if s.tmdbKey()==""&&s.tvdbKey()=="" {
        warning="Kein TMDb- oder TheTVDB-Zugang eingerichtet. Externe NFO-Bildlinks können trotzdem versucht werden."
    }
    s.artworkJob=ArtworkRepairJob{
        Running:len(targets)>0,Total:len(targets),Warning:warning,
        Started:time.Now().Format(time.RFC3339),
    }
    if len(targets)==0{s.artworkJob.Finished=time.Now().Format(time.RFC3339)}
    snapshot:=s.artworkJob
    s.artworkMu.Unlock()
    if len(targets)>0{go s.runArtworkRepair(targets)}
    jsonOut(w,snapshot)
}

func (s *Server) runArtworkRepair(items []Media){
    var tmdb *TMDbProvider
    var tvdb *TVDBProvider
    if key:=s.tmdbKey();key!=""{tmdb=NewTMDbProvider(key)}
    if key:=s.tvdbKey();key!=""{tvdb=NewTVDBProvider(key)}
    for _,item:=range items {
        s.artworkMu.Lock()
        s.artworkJob.CurrentTitle=item.Title
        s.artworkMu.Unlock()

        // If another operation changed the media, never overwrite its artwork.
        s.mu.RLock()
        var latest Media
        exists:=false
        for _,m:=range s.st.Media {
            if m.ID==item.ID&&!m.Missing {latest=m;exists=true;break}
        }
        s.mu.RUnlock()
        if !exists {
            s.artworkMu.Lock();s.artworkJob.Done++;s.artworkJob.Skipped++;s.artworkMu.Unlock()
            continue
        }
        posterPresent:=false
        backdropPresent:=false
        if _,ok:=s.cachedArt(latest.ID,"poster");ok{posterPresent=true}
        if _,ok:=s.cachedArt(latest.ID,"backdrop");ok{backdropPresent=true}
        posterURL,backdropURL:="",""
        // Existing NFO remote artwork can be downloaded without provider API keys.
        if !posterPresent&&approvedArtworkURL(latest.Poster){posterURL=latest.Poster}
        if !backdropPresent&&approvedArtworkURL(latest.Backdrop){backdropURL=latest.Backdrop}
        if (!posterPresent&&posterURL=="")||(!backdropPresent&&backdropURL==""){
            providerPoster,providerBackdrop:=providerArtwork(latest,tmdb,tvdb)
            if posterURL=="" {posterURL=providerPoster}
            if backdropURL=="" {backdropURL=providerBackdrop}
        }
        poster,backdrop:="",""
        var imageErrs int
        if !posterPresent&&approvedArtworkURL(posterURL){
            if path,err:=s.persistRemoteArtwork(latest.ID,"poster",posterURL);err==nil{poster=path}else{imageErrs++}
        }
        if !backdropPresent&&approvedArtworkURL(backdropURL){
            if path,err:=s.persistRemoteArtwork(latest.ID,"backdrop",backdropURL);err==nil{backdrop=path}else{imageErrs++}
        }
        changed:=false
        if poster!=""||backdrop!=""{
            s.mu.Lock()
            for i:=range s.st.Media {
                m:=&s.st.Media[i]
                if m.ID!=latest.ID||m.Missing{continue}
                // Only replace the URL of the still-unmodified source item.
                if poster!=""&&m.Poster==latest.Poster{m.Poster=poster;changed=true}
                if backdrop!=""&&m.Backdrop==latest.Backdrop{m.Backdrop=backdrop;changed=true}
                break
            }
            if changed {if err:=s.saveLocked();err!=nil{imageErrs++}}
            s.mu.Unlock()
        }
        s.artworkMu.Lock()
        s.artworkJob.Done++
        if poster!=""{s.artworkJob.Posters++}
        if backdrop!=""{s.artworkJob.Backdrops++}
        if !posterPresent&&poster==""{s.artworkJob.Unresolved++}
        if imageErrs>0{s.artworkJob.Errors+=imageErrs}
        s.artworkMu.Unlock()
        // Keep API requests gentle for TheTVDB Free users.
        time.Sleep(200*time.Millisecond)
    }
    s.artworkMu.Lock()
    s.artworkJob.Running=false
    s.artworkJob.CurrentTitle=""
    s.artworkJob.Finished=time.Now().Format(time.RFC3339)
    s.artworkMu.Unlock()
}
