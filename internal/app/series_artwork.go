package app

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "time"
)

func tmdbSeriesArtwork(p *TMDbProvider,id string)(string,string,error){
 if !numericID.MatchString(id){return "","",fmt.Errorf("invalid TMDb series id")}
 params:=url.Values{"api_key":{p.apiKey},"language":{"de-DE"}}
 addr:="https://api.themoviedb.org/3/tv/"+url.PathEscape(id)+"?"+params.Encode()
 resp,err:=p.client.Get(addr)
 if err!=nil{return "","",err}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return "","",fmt.Errorf("TMDb series artwork HTTP %d",resp.StatusCode)}
 var result struct{
  Poster string `json:"poster_path"`
  Backdrop string `json:"backdrop_path"`
 }
 if err:=json.NewDecoder(io.LimitReader(resp.Body,2<<20)).Decode(&result);err!=nil{return "","",err}
 poster,backdrop:="",""
 if strings.HasPrefix(result.Poster,"/"){poster="https://image.tmdb.org/t/p/w500"+result.Poster}
 if strings.HasPrefix(result.Backdrop,"/"){backdrop="https://image.tmdb.org/t/p/w1280"+result.Backdrop}
 return poster,backdrop,nil
}
func tvdbSeriesArtwork(p *TVDBProvider,id string)(string,string,error){
 if !numericID.MatchString(id){return "","",fmt.Errorf("invalid TheTVDB series id")}
 var response struct{
  Data struct{
   Image string `json:"image"`
   Artworks []struct{
    Image string `json:"image"`
    Width int `json:"width"`
    Height int `json:"height"`
    Score float64 `json:"score"`
   } `json:"artworks"`
  } `json:"data"`
 }
 if err:=p.get("/series/"+url.PathEscape(id)+"/extended",&response);err!=nil{return "","",err}
 poster:=normalizeTVDBImage(response.Data.Image)
 backdrop:=""
 for _,a:=range response.Data.Artworks{
  if a.Width<=0||a.Height<=0 {continue}
  target:=normalizeTVDBImage(a.Image)
  if a.Height>a.Width && poster=="" {poster=target}
  if a.Width*10>=a.Height*13&&backdrop==""{backdrop=target}
 }
 return poster,backdrop,nil
}
func seriesArtworkURLs(m SeriesMetadata,tmdb *TMDbProvider,tvdb *TVDBProvider)(string,string){
 poster,backdrop:="",""
 if tmdb!=nil&&numericID.MatchString(m.ExternalIDs["tmdb"]){
  poster,backdrop,_=tmdbSeriesArtwork(tmdb,m.ExternalIDs["tmdb"])
 }
 if tvdb!=nil&&(poster==""||backdrop=="")&&numericID.MatchString(m.ExternalIDs["tvdb"]){
  p,b,err:=tvdbSeriesArtwork(tvdb,m.ExternalIDs["tvdb"])
  if err==nil{
   if poster==""{poster=p}
   if backdrop==""{backdrop=b}
  }
 }
 return poster,backdrop
}
func (s *Server) fetchSeriesArtwork(key,kind,sourceURL string)(string,error){
 if !approvedArtworkURL(sourceURL){return "",fmt.Errorf("unsupported artwork URL")}
 client:=&http.Client{
  Timeout:18*time.Second,
  CheckRedirect:func(req *http.Request,previous []*http.Request)error{
   if len(previous)>=3||!approvedArtworkURL(req.URL.String()){return fmt.Errorf("artwork redirect rejected")}
   return nil
  },
 }
 resp,err:=client.Get(sourceURL)
 if err!=nil{return "",err}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return "",fmt.Errorf("artwork HTTP %d",resp.StatusCode)}
 data,err:=io.ReadAll(io.LimitReader(resp.Body,(12<<20)+1))
 if err!=nil{return "",err}
 if len(data)==0||len(data)>12<<20{return "",fmt.Errorf("image is empty or exceeds 12 MiB")}
 ext:=""
 switch http.DetectContentType(data){
 case "image/jpeg":ext=".jpg"
 case "image/png":ext=".png"
 case "image/webp":ext=".webp"
 default:return "",fmt.Errorf("response is not JPEG/PNG/WebP")
 }
 dir:=filepath.Join(s.cfg.DataDir,"series-art")
 if err:=os.MkdirAll(dir,0700);err!=nil{return "",err}
 f,err:=os.CreateTemp(dir,".series-remote-*")
 if err!=nil{return "",err}
 tmp:=f.Name()
 defer os.Remove(tmp)
 _,err=f.Write(data)
 if err==nil{err=f.Sync()}
 if closeErr:=f.Close();err==nil{err=closeErr}
 if err!=nil{return "",err}
 if err:=os.Chmod(tmp,0600);err!=nil{return "",err}
 stem:=seriesArtStem(key,kind)
 if err:=os.Rename(tmp,filepath.Join(dir,stem+ext));err!=nil{return "",err}
 for _,other:=range []string{".jpg",".jpeg",".png",".webp"}{
  if other!=ext{_ = os.Remove(filepath.Join(dir,stem+other))}
 }
 return seriesArtURL(key,kind),nil
}
func (s *Server) seriesArtworkRepair(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet&&r.Method!=http.MethodPost{jsonErr(w,405,"method not allowed");return}
 s.seriesArtMu.Lock()
 if r.Method==http.MethodGet {
  result:=s.seriesArtJob
  s.seriesArtMu.Unlock()
  jsonOut(w,result);return
 }
 if s.seriesArtJob.Running{
  s.seriesArtMu.Unlock()
  jsonErr(w,409,"series artwork repair already running");return
 }
 s.mu.RLock()
 items:=map[string]SeriesMetadata{}
 for key,m:=range s.st.SeriesMetadata{items[key]=m}
 s.mu.RUnlock()
 targets:=map[string]SeriesMetadata{}
 for key,m:=range items{
  if _,has:=s.seriesArtLocation(key,"poster");!has{targets[key]=m;continue}
  if _,has:=s.seriesArtLocation(key,"backdrop");!has{targets[key]=m}
 }
 warning:=""
 if s.tmdbKey()==""&&s.tvdbKey()==""{
  warning="Keine API-Zugänge konfiguriert. Bildlinks aus tvshow.nfo können trotzdem geladen werden."
 }
 s.seriesArtJob=ArtworkRepairJob{Running:len(targets)>0,Total:len(targets),
  Started:time.Now().Format(time.RFC3339),Warning:warning}
 if len(targets)==0{s.seriesArtJob.Finished=time.Now().Format(time.RFC3339)}
 report:=s.seriesArtJob
 s.seriesArtMu.Unlock()
 if len(targets)>0{go s.runSeriesArtworkRepair(targets)}
 jsonOut(w,report)
}
func (s *Server) runSeriesArtworkRepair(items map[string]SeriesMetadata){
 var tmdb *TMDbProvider
 var tvdb *TVDBProvider
 if key:=s.tmdbKey();key!=""{tmdb=NewTMDbProvider(key)}
 if key:=s.tvdbKey();key!=""{tvdb=NewTVDBProvider(key)}
 for key:=range items {
  s.mu.RLock()
  original,ok:=s.st.SeriesMetadata[key]
  s.mu.RUnlock()
  if !ok {s.seriesArtMu.Lock();s.seriesArtJob.Done++;s.seriesArtJob.Skipped++;s.seriesArtMu.Unlock();continue}
  s.seriesArtMu.Lock();s.seriesArtJob.CurrentTitle=original.Title;s.seriesArtMu.Unlock()
  _,hasPoster:=s.seriesArtLocation(key,"poster")
  _,hasBackdrop:=s.seriesArtLocation(key,"backdrop")
  poster,backdrop:="",""
  if !hasPoster&&approvedArtworkURL(original.Poster){poster=original.Poster}
  if !hasBackdrop&&approvedArtworkURL(original.Backdrop){backdrop=original.Backdrop}
  if (!hasPoster&&poster=="")||(!hasBackdrop&&backdrop==""){
   p,b:=seriesArtworkURLs(original,tmdb,tvdb)
   if poster==""{poster=p}
   if backdrop==""{backdrop=b}
  }
  newPoster,newBackdrop:="",""
  errs:=0
  if !hasPoster&&poster!=""{
   if value,err:=s.fetchSeriesArtwork(key,"poster",poster);err==nil{newPoster=value}else{errs++}
  }
  if !hasBackdrop&&backdrop!=""{
   if value,err:=s.fetchSeriesArtwork(key,"backdrop",backdrop);err==nil{newBackdrop=value}else{errs++}
  }
  if newPoster!=""||newBackdrop!=""{
   s.mu.Lock()
   current,exists:=s.st.SeriesMetadata[key]
   if exists{
    previous:=current
    // Never overwrite user changes or a newer NFO import.
    if newPoster!=""&&current.Poster==original.Poster{current.Poster=newPoster}
    if newBackdrop!=""&&current.Backdrop==original.Backdrop{current.Backdrop=newBackdrop}
    s.st.SeriesMetadata[key]=current
    if err:=s.saveLocked();err!=nil{s.st.SeriesMetadata[key]=previous;errs++}
   }
   s.mu.Unlock()
  }
  s.seriesArtMu.Lock()
  s.seriesArtJob.Done++
  if newPoster!=""{s.seriesArtJob.Posters++}
  if newBackdrop!=""{s.seriesArtJob.Backdrops++}
  if !hasPoster&&newPoster==""{s.seriesArtJob.Unresolved++}
  s.seriesArtJob.Errors+=errs
  s.seriesArtMu.Unlock()
  time.Sleep(200*time.Millisecond)
 }
 s.seriesArtMu.Lock()
 s.seriesArtJob.Running=false
 s.seriesArtJob.CurrentTitle=""
 s.seriesArtJob.Finished=time.Now().Format(time.RFC3339)
 s.seriesArtMu.Unlock()
}
