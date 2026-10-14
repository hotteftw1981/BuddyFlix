package app

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "sort"
 "strconv"
 "strings"
 "time"
)

type SeriesMetadata struct {
 Title string `json:"title"`
 Year int `json:"year,omitempty"`
 Overview string `json:"overview,omitempty"`
 Poster string `json:"poster,omitempty"`
 Backdrop string `json:"backdrop,omitempty"`
 ExternalIDs map[string]string `json:"external_ids,omitempty"`
 Genres []string `json:"genres,omitempty"`
 Rating string `json:"rating,omitempty"`
 Provider string `json:"provider,omitempty"`
 Updated string `json:"updated,omitempty"`
}
type seriesNFOHit struct {
 Key string `json:"key"`
 Path string `json:"nfo_path"`
 Title string `json:"title"`
 ExternalIDs map[string]string `json:"external_ids"`
 NFO jellyfinNFO `json:"-"`
}
type seriesNFOReport struct {
 SeriesTotal int `json:"series_total"`
 Found int `json:"found"`
 Missing int `json:"missing"`
 Invalid int `json:"invalid"`
 EpisodesFound int `json:"episodes_found"`
 Applied int `json:"applied,omitempty"`
 EpisodesApplied int `json:"episodes_applied,omitempty"`
 ImagesCached int `json:"images_cached,omitempty"`
 Problems []string `json:"problems"`
 Examples []seriesNFOHit `json:"examples"`
}

func seriesKey(v string)string {return strings.ToLower(strings.TrimSpace(v))}
func validContainedPath(root,path string)bool {
 r,err:=filepath.Rel(filepath.Clean(root),filepath.Clean(path))
 return err==nil&&r!=".."&&!strings.HasPrefix(r,".."+string(filepath.Separator))
}
func seriesNFOFile(m Media,libRoot string)string {
 if libRoot==""||m.Path=="" {return ""}
 root:=filepath.Clean(libRoot)
 dir:=filepath.Dir(m.Path)
 if !validContainedPath(root,dir){return ""}
 for validContainedPath(root,dir) {
  candidate:=filepath.Join(dir,"tvshow.nfo")
  if st,err:=os.Lstat(candidate);err==nil&&st.Mode().IsRegular()&&st.Size()<=1<<20 {return candidate}
  if dir==root {break}
  parent:=filepath.Dir(dir)
  if parent==dir {break}
  dir=parent
 }
 return ""
}
func collectSeriesNFO(items []Media,libs []Library)(seriesNFOReport,[]seriesNFOHit,[]nfoHit){
 report:=seriesNFOReport{Examples:[]seriesNFOHit{},Problems:[]string{}}
 libraryRoots:=map[int64]string{}
 for _,l:=range libs{libraryRoots[l.ID]=l.Path}
 grouped:=map[string][]Media{}
 for _,m:=range items {
  if m.Missing||m.Kind!="episode"||m.Path=="" {continue}
  key:=seriesKey(m.SeriesTitle);if key==""{continue}
  grouped[key]=append(grouped[key],m)
 }
 // Resolve series NFO paths once. Large NAS libraries must not trigger O(N²)
 // repeated directory walks when previewing dozens of series.
 seriesPaths:=map[string]map[string]bool{}
 pathOwners:=map[string]map[string]bool{}
 for key,group:=range grouped{
  set:=map[string]bool{}
  for _,ep:=range group {
   p:=seriesNFOFile(ep,libraryRoots[ep.LibraryID])
   if p==""{continue}
   set[p]=true
   if pathOwners[p]==nil{pathOwners[p]=map[string]bool{}}
   pathOwners[p][key]=true
  }
  seriesPaths[key]=set
 }
 keys:=make([]string,0,len(grouped))
 for key:=range grouped {keys=append(keys,key)}
 sort.Strings(keys)
 report.SeriesTotal=len(keys)
 hits:=[]seriesNFOHit{}
 episodeHits:=[]nfoHit{}
 for _,key:=range keys {
  media:=grouped[key]
  paths:=seriesPaths[key]
  if len(paths)==0{report.Missing++;continue}
  if len(paths)!=1 {
   report.Invalid++
   if len(report.Problems)<10{report.Problems=append(report.Problems,key+": Mehrere tvshow.nfo-Dateien; Zuordnung nicht eindeutig")}
   continue
  }
  var filepathNFO string
  for p:=range paths{filepathNFO=p}
  // A tvshow.nfo cannot identify multiple different grouped shows.
  if len(pathOwners[filepathNFO])>1{
   report.Invalid++
   if len(report.Problems)<10{report.Problems=append(report.Problems,key+": tvshow.nfo wird von mehreren Serien geteilt")}
   continue
  }
  n,err:=readNFO(filepathNFO)
  if err!=nil||n.XMLName.Local!="tvshow" {
   report.Invalid++
   if len(report.Problems)<10{
    if err!=nil{report.Problems=append(report.Problems,key+": "+err.Error())}else{
     report.Problems=append(report.Problems,key+": Falscher NFO-Dateityp")}
   }
   continue
  }
  hit:=seriesNFOHit{Key:key,Path:filepathNFO,Title:n.Title,ExternalIDs:nfoIDs(n),NFO:n}
  hits=append(hits,hit);report.Found++
  if len(report.Examples)<12{report.Examples=append(report.Examples,hit)}
  for _,m:=range media {
   path,found,err:=nfoLocation(m)
   if err!=nil||!found{continue}
   ep,err:=readNFO(path)
   if err!=nil||ep.XMLName.Local!="episodedetails"{continue}
   // Episode NFOs are only paired with their exact video filename.
   // Never replace episode media IDs, paths, user progress, or profile data.
   if ep.Season>0&&m.Season>0&&ep.Season!=m.Season{continue}
   if ep.Episode>0&&m.Episode>0&&ep.Episode!=m.Episode{continue}
   episodeHits=append(episodeHits,nfoHit{ID:m.ID,Path:m.Path,File:path,Title:ep.Title,IDs:nfoIDs(ep),
     Document:ep,Poster:nfoArtURL(m,ep,"poster"),Backdrop:nfoArtURL(m,ep,"backdrop")})
   report.EpisodesFound++
  }
 }
 return report,hits,episodeHits
}

func seriesMetadataFromNFO(n jellyfinNFO)SeriesMetadata{
 year,_:=strconv.Atoi(strings.TrimSpace(n.Year))
 if year==0&&len(n.Premiered)>=4{year,_=strconv.Atoi(n.Premiered[:4])}
 overview:=strings.TrimSpace(n.Plot)
 if overview==""{overview=strings.TrimSpace(n.Outline)}
 return SeriesMetadata{Title:strings.TrimSpace(n.Title),Year:year,Overview:overview,
  ExternalIDs:nfoIDs(n),Genres:cleanedList(n.Genres),Rating:strings.TrimSpace(n.Rating),
  Provider:"jellyfin-nfo",Updated:time.Now().Format(time.RFC3339)}
}

func seriesArtStem(key,kind string)string{
 h:=sha256.Sum256([]byte(seriesKey(key)))
 return hex.EncodeToString(h[:12])+"-"+kind
}
func (s *Server) seriesArtLocation(key,kind string)(string,bool){
 if key==""||(kind!="poster"&&kind!="backdrop"){return "",false}
 base:=filepath.Join(s.cfg.DataDir,"series-art",seriesArtStem(key,kind))
 for _,ext:=range []string{".jpg",".jpeg",".png",".webp"}{
  f:=base+ext
  if info,err:=os.Lstat(f);err==nil&&info.Mode().IsRegular()&&info.Size()<=20<<20{return f,true}
 }
 return "",false
}
func seriesArtURL(key,kind string)string{
 return "/api/series/art?key="+url.QueryEscape(key)+"&type="+kind
}
func (s *Server) saveSeriesArtwork(key,kind,sourcePath string)(string,error){
 if seriesKey(key)==""||(kind!="poster"&&kind!="backdrop"){return "",fmt.Errorf("invalid series artwork target")}
 srcInfo,err:=os.Lstat(sourcePath)
 if err!=nil||!srcInfo.Mode().IsRegular()||srcInfo.Size()>20<<20{return "",fmt.Errorf("invalid local artwork")}
 ext:=strings.ToLower(filepath.Ext(sourcePath))
 if ext!=".jpg"&&ext!=".jpeg"&&ext!=".png"&&ext!=".webp"{return "",fmt.Errorf("unsupported image extension")}
 src,err:=os.Open(sourcePath);if err!=nil{return "",err}
 defer src.Close()
 dir:=filepath.Join(s.cfg.DataDir,"series-art")
 if err:=os.MkdirAll(dir,0700);err!=nil{return "",err}
 out,err:=os.CreateTemp(dir,".series-*");if err!=nil{return "",err}
 tmp:=out.Name()
 defer os.Remove(tmp)
 bytesWritten,writeErr:=io.Copy(out,io.LimitReader(src,20<<20+1))
 if writeErr==nil{writeErr=out.Sync()}
 if closeErr:=out.Close();writeErr==nil{writeErr=closeErr}
 if writeErr!=nil{return "",writeErr}
 if bytesWritten>20<<20{return "",fmt.Errorf("image too large")}
 if err:=os.Chmod(tmp,0600);err!=nil{return "",err}
 stem:=seriesArtStem(key,kind)
 if err:=os.Rename(tmp,filepath.Join(dir,stem+ext));err!=nil{return "",err}
 for _,other:=range []string{".jpg",".jpeg",".png",".webp"}{
  if other!=ext{_ = os.Remove(filepath.Join(dir,stem+other))}
 }
 return seriesArtURL(key,kind),nil
}
func localSeriesArt(n jellyfinNFO,nfoPath,kind string)(string,bool){
 // Use the show's own folder. Fall back to poster.jpg, folder.jpg, fanart.jpg etc.
 dummy:=Media{Path:filepath.Join(filepath.Dir(nfoPath),"tvshow.mkv"),Kind:"movie"}
 return nfoImagePath(dummy,n,kind)
}
func (s *Server) seriesNFOImport(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet&&r.Method!=http.MethodPost{jsonErr(w,405,"method not allowed");return}
 s.mu.RLock()
 media:=append([]Media(nil),s.st.Media...)
 libs:=append([]Library(nil),s.st.Libraries...)
 s.mu.RUnlock()
 report,hits,episodes:=collectSeriesNFO(media,libs)
 if r.Method==http.MethodGet{jsonOut(w,report);return}
 var input struct{Confirm bool `json:"confirm"`}
 if err:=json.NewDecoder(io.LimitReader(r.Body,2048)).Decode(&input);err!=nil||!input.Confirm {
  jsonErr(w,400,"confirm=true required");return
 }
 if len(hits)==0 {jsonOut(w,report);return}
 // Do not reset existing series artwork if no new artwork can be imported.
 s.mu.RLock()
 existing:=map[string]SeriesMetadata{}
 for k,v:=range s.st.SeriesMetadata {existing[k]=v}
 s.mu.RUnlock()
 prepared:=map[string]SeriesMetadata{}
 for _,hit:=range hits{
  meta:=seriesMetadataFromNFO(hit.NFO)
  meta.Poster=existing[hit.Key].Poster
  meta.Backdrop=existing[hit.Key].Backdrop
  for _,kind:=range []string{"poster","backdrop"}{
   path,found:=localSeriesArt(hit.NFO,hit.Path,kind)
   if found{
    dest,err:=s.saveSeriesArtwork(hit.Key,kind,path)
    if err!=nil{
     if len(report.Problems)<10{report.Problems=append(report.Problems,hit.Title+": "+err.Error())}
    }else{
     if kind=="poster"{meta.Poster=dest}else{meta.Backdrop=dest}
     report.ImagesCached++
    }
   }else{
    candidate:=""
    if kind=="poster"{candidate=strings.TrimSpace(hit.NFO.Art.Poster);if candidate==""{candidate=strings.TrimSpace(hit.NFO.Thumb)}
    }else{candidate=strings.TrimSpace(hit.NFO.Art.Fanart)}
    if approvedArtworkURL(candidate){
     if kind=="poster"&&meta.Poster==""{meta.Poster=candidate}
     if kind=="backdrop"&&meta.Backdrop==""{meta.Backdrop=candidate}
    }
   }
  }
  prepared[hit.Key]=meta
 }
 byID:=map[int64]nfoHit{}
 mediaByID:=map[int64]Media{}
 for _,m:=range media{mediaByID[m.ID]=m}
 for _,hit:=range episodes{
  m:=mediaByID[hit.ID]
  // Episode backdrops are snapshots; never use generic show posters as thumbnails.
  if src,ok:=episodeArtworkPath(m,hit.Document);ok{
   copyURL,err:=s.copyEpisodeArtwork(m,src)
   if err==nil{hit.Backdrop=copyURL;report.ImagesCached++}
  }
  byID[hit.ID]=hit
 }
 s.mu.Lock()
 oldMedia:=s.st.Media;oldMeta:=s.st.SeriesMetadata
 updated:=append([]Media(nil),oldMedia...)
 mapped:=map[string]SeriesMetadata{}
 for k,v:=range oldMeta{mapped[k]=v}
 for key,meta:=range prepared{mapped[key]=meta;report.Applied++}
 for i,m:=range updated{
  hit,ok:=byID[m.ID]
  if !ok||m.Path!=hit.Path||m.Missing||m.Kind!="episode"{continue}
  out:=importedNFO(m,hit)
  if hit.Document.Title!=""{out.EpisodeTitle=hit.Document.Title}
  if hit.Document.Season>0{out.Season=hit.Document.Season}
  if hit.Document.Episode>0{out.Episode=hit.Document.Episode}
  // Preserve existing artwork when no episode-specific image was found.
  if out.Poster==""{out.Poster=m.Poster}
  if out.Backdrop==""{out.Backdrop=m.Backdrop}
  updated[i]=out
  report.EpisodesApplied++
 }
 s.st.SeriesMetadata=mapped
 s.st.Media=updated
 if err:=s.saveLocked();err!=nil{
  s.st.SeriesMetadata=oldMeta;s.st.Media=oldMedia
  s.mu.Unlock()
  jsonErr(w,500,err.Error());return
 }
 s.mu.Unlock()
 jsonOut(w,report)
}
func episodeArtworkPath(m Media,n jellyfinNFO)(string,bool){
 dir:=filepath.Dir(m.Path)
 stem:=strings.TrimSuffix(filepath.Base(m.Path),filepath.Ext(m.Path))
 options:=[]string{n.Thumb,n.Art.Thumb,stem+"-thumb.jpg",stem+"-thumb.png",stem+"-thumb.webp",
 stem+".jpg",stem+".png",stem+".webp"}
 for _,o:=range options{
  name:=filepath.Base(strings.TrimSpace(o))
  if name==""||name=="."||name==".."{continue}
  filename:=filepath.Join(dir,name)
  ext:=strings.ToLower(filepath.Ext(filename))
  if ext!=".jpg"&&ext!=".jpeg"&&ext!=".png"&&ext!=".webp"{continue}
  info,err:=os.Lstat(filename)
  if err==nil&&info.Mode().IsRegular()&&info.Size()<=20<<20{return filename,true}
 }
 return "",false
}
func (s *Server) copyEpisodeArtwork(m Media,path string)(string,error){
 // Populate the same persistent location served by /api/nfo/art.
 n:=jellyfinNFO{}
 n.Art.Fanart=filepath.Base(path)
 return s.cacheEpisodeBackdrop(m,n)
}
func (s *Server) cacheEpisodeBackdrop(m Media,n jellyfinNFO)(string,error){
 image,ok:=episodeArtworkPath(m,n)
 if !ok{return "",fmt.Errorf("no episode image")}
 // Reuse exact movie cache mechanism, with the episode image explicitly set.
 dummy:=jellyfinNFO{}
 dummy.Art.Fanart=filepath.Base(image)
 saved,_,err:=s.cacheNFOArt(m,dummy,"backdrop")
 return saved,err
}
func (s *Server) seriesArt(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{jsonErr(w,405,"method not allowed");return}
 key:=seriesKey(r.URL.Query().Get("key"))
 kind:=r.URL.Query().Get("type")
 s.mu.RLock()
 _,exists:=s.st.SeriesMetadata[key]
 s.mu.RUnlock()
 if !exists {http.NotFound(w,r);return}
 img,ok:=s.seriesArtLocation(key,kind)
 if !ok{http.NotFound(w,r);return}
 f,err:=os.Open(img)
 if err!=nil{http.NotFound(w,r);return}
 defer f.Close()
 info,err:=f.Stat();if err!=nil||!info.Mode().IsRegular(){http.NotFound(w,r);return}
 switch strings.ToLower(filepath.Ext(img)){
 case ".png":w.Header().Set("Content-Type","image/png")
 case ".webp":w.Header().Set("Content-Type","image/webp")
 default:w.Header().Set("Content-Type","image/jpeg")
 }
 w.Header().Set("Cache-Control","private, max-age=120")
 http.ServeContent(w,r,filepath.Base(img),info.ModTime(),f)
}
