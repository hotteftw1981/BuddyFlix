package app

import (
 "encoding/json"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "strconv"
 "strings"
)

type artworkSuggestion struct {
 Provider string `json:"provider"`
 Kind string `json:"kind"`
 URL string `json:"url"`
}
func isSafePosterLink(raw string) bool {
 u,err:=url.Parse(strings.TrimSpace(raw))
 if err!=nil || u.Scheme!="https" || u.Hostname()=="" || u.User!=nil || u.Fragment!="" {return false}
 host:=strings.ToLower(u.Hostname())
 if host=="localhost" || strings.HasSuffix(host,".localhost") || strings.HasSuffix(host,".local") {return false}
 if ip:=net.ParseIP(host);ip!=nil && (ip.IsPrivate()||ip.IsLoopback()||ip.IsLinkLocalUnicast()||ip.IsUnspecified()) {return false}
 return true
}

func (s *Server) mediaForArtwork(id int64)(Media,bool){
 s.mu.RLock()
 defer s.mu.RUnlock()
 for _,m:=range s.st.Media {
  if m.ID==id && !m.Missing {return m,true}
 }
 return Media{},false
}

func (s *Server) artworkSuggestions(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet {jsonErr(w,405,"method not allowed");return}
 id,err:=strconv.ParseInt(r.URL.Query().Get("id"),10,64)
 if err!=nil||id<=0{jsonErr(w,400,"invalid media id");return}
 m,ok:=s.mediaForArtwork(id)
 if !ok {jsonErr(w,404,"media not found");return}
 suggestions:=[]artworkSuggestion{}
 seen:=map[string]bool{}
 add:=func(provider,kind,link string){
  if kind!="poster"&&kind!="backdrop"{return}
  if !approvedArtworkURL(link)||seen[link] {return}
  seen[link]=true
  suggestions=append(suggestions,artworkSuggestion{Provider:provider,Kind:kind,URL:link})
 }
 if tmdbKey:=s.tmdbKey();tmdbKey!=""{
  p:=NewTMDbProvider(tmdbKey)
  key:=strings.TrimSpace(m.ExternalIDs["tmdb"])
  if !numericID.MatchString(key)&&imdbPattern.MatchString(m.ExternalIDs["imdb"]){
   if result,e:=tmdbMovieForIMDb(p,m.ExternalIDs["imdb"]);e==nil{key=result}
  }
  if numericID.MatchString(key){
   if item,e:=p.EnrichMovie(MetadataCandidate{Provider:"tmdb",ProviderID:key});e==nil{
    add("TMDb","poster",item.Poster)
    add("TMDb","backdrop",item.Backdrop)
   }
  }
 }
 if tvdbKey:=s.tvdbKey();tvdbKey!=""{
  p:=NewTVDBProvider(tvdbKey)
  key:=strings.TrimSpace(m.ExternalIDs["tvdb"])
  if !numericID.MatchString(key)&&imdbPattern.MatchString(m.ExternalIDs["imdb"]){
   if result,e:=tvdbMovieForIMDb(p,m.ExternalIDs["imdb"]);e==nil{key=result}
  }
  if numericID.MatchString(key){
   if item,e:=p.EnrichMovie(MetadataCandidate{Provider:"thetvdb",ProviderID:key});e==nil{
    add("TheTVDB","poster",item.Poster)
    add("TheTVDB","backdrop",item.Backdrop)
   }
  }
 }
 jsonOut(w,map[string]any{"items":suggestions,"media_id":id})
}

func artworkFileExt(data []byte)(string,error){
 switch http.DetectContentType(data) {
 case "image/jpeg":return ".jpg",nil
 case "image/png":return ".png",nil
 case "image/webp":return ".webp",nil
 }
 return "",fmt.Errorf("file must be JPEG, PNG or WebP")
}
func (s *Server) saveUploadedArtwork(id int64,kind string,content []byte)(string,error){
 if id<1||(kind!="poster"&&kind!="backdrop") {return "",fmt.Errorf("invalid artwork target")}
 if len(content)==0||len(content)>12*1024*1024{return "",fmt.Errorf("image exceeds 12 MiB limit")}
 ext,err:=artworkFileExt(content);if err!=nil{return "",err}
 dir:=filepath.Join(s.cfg.DataDir,"nfo-art")
 if err:=os.MkdirAll(dir,0700);err!=nil{return "",err}
 file,err:=os.CreateTemp(dir,".art-upload-*")
 if err!=nil{return "",err}
 name:=file.Name();defer os.Remove(name)
 _,err=file.Write(content)
 if err==nil{err=file.Sync()}
 if ce:=file.Close();err==nil{err=ce}
 if err!=nil{return "",err}
 if err:=os.Chmod(name,0600);err!=nil{return "",err}
 stem:=strconv.FormatInt(id,10)+"-"+kind
 if err:=os.Rename(name,filepath.Join(dir,stem+ext));err!=nil{return "",err}
 for _,other:=range []string{".jpg",".jpeg",".png",".webp"}{
  if other!=ext{_ = os.Remove(filepath.Join(dir,stem+other))}
 }
 return "/api/nfo/art?id="+strconv.FormatInt(id,10)+"&type="+kind,nil
}

func (s *Server) updateArtworkOnly(id int64,kind,newURL string)error{
 s.mu.Lock()
 defer s.mu.Unlock()
 for i:=range s.st.Media{
  m:=&s.st.Media[i]
  if m.ID!=id||m.Missing{continue}
  original:=*m
  if kind=="poster"{m.Poster=newURL}else{m.Backdrop=newURL}
  if err:=s.saveLocked();err!=nil{*m=original;return err}
  return nil
 }
 return fmt.Errorf("media no longer exists")
}

func (s *Server) artworkManual(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost{jsonErr(w,405,"method not allowed");return}
 var request struct {
  ID int64 `json:"id"`
  Kind string `json:"kind"`
  URL string `json:"url"`
 }
 if json.NewDecoder(io.LimitReader(r.Body,8192)).Decode(&request)!=nil||request.ID<1||(request.Kind!="poster"&&request.Kind!="backdrop"){
  jsonErr(w,400,"invalid artwork selection");return
 }
 request.URL=strings.TrimSpace(request.URL)
 if !isSafePosterLink(request.URL){jsonErr(w,400,"only public HTTPS image URLs are supported");return}
 if _,exists:=s.mediaForArtwork(request.ID);!exists {jsonErr(w,404,"media not found");return}
 final:=request.URL
 local:=false
 if approvedArtworkURL(final){
  copied,err:=s.persistRemoteArtwork(request.ID,request.Kind,final)
  if err!=nil{jsonErr(w,502,"artwork download failed: "+err.Error());return}
  final=copied;local=true
 }
 if err:=s.updateArtworkOnly(request.ID,request.Kind,final);err!=nil{jsonErr(w,500,err.Error());return}
 jsonOut(w,map[string]any{"ok":true,"stored_locally":local,"url":final})
}

func (s *Server) artworkUpload(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost{jsonErr(w,405,"method not allowed");return}
 id,err:=strconv.ParseInt(r.URL.Query().Get("id"),10,64)
 kind:=r.URL.Query().Get("kind")
 if err!=nil||id<1||(kind!="poster"&&kind!="backdrop"){jsonErr(w,400,"invalid upload target");return}
 if _,ok:=s.mediaForArtwork(id);!ok{jsonErr(w,404,"media not found");return}
 const limit = 12*1024*1024
 r.Body=http.MaxBytesReader(w,r.Body,limit+1024*1024)
 if err:=r.ParseMultipartForm(2<<20);err!=nil{jsonErr(w,413,"image upload too large or malformed");return}
 file,_,err:=r.FormFile("image")
 if err!=nil{jsonErr(w,400,"image field required");return}
 defer file.Close()
 content,err:=io.ReadAll(io.LimitReader(file,limit+1))
 if err!=nil||len(content)>limit{jsonErr(w,413,"image exceeds 12 MiB");return}
 stored,err:=s.saveUploadedArtwork(id,kind,content)
 if err!=nil{jsonErr(w,400,err.Error());return}
 if err:=s.updateArtworkOnly(id,kind,stored);err!=nil{jsonErr(w,500,err.Error());return}
 jsonOut(w,map[string]any{"ok":true,"stored_locally":true,"url":stored})
}
