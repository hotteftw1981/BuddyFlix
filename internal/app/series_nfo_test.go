package app

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func TestJellyfinSeriesNFOFullImportPreservesProgress(t *testing.T){
 s:=newTestServer(t)
 root:=t.TempDir()
 show:=filepath.Join(root,"Dark (2017)")
 season:=filepath.Join(show,"Season 01")
 if err:=os.MkdirAll(season,0755);err!=nil{t.Fatal(err)}
 video:=filepath.Join(season,"Dark S01E01.mkv")
 if err:=os.WriteFile(video,[]byte("video"),0644);err!=nil{t.Fatal(err)}
 seriesDoc:=`<tvshow>
<title>Dark</title><year>2017</year><plot>A German science fiction series.</plot>
<uniqueid type="imdb">tt5753856</uniqueid><uniqueid type="tvdb">325731</uniqueid><uniqueid type="tmdb">70523</uniqueid>
<genre>Drama</genre><genre>Science Fiction</genre>
</tvshow>`
 if err:=os.WriteFile(filepath.Join(show,"tvshow.nfo"),[]byte(seriesDoc),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(show,"poster.jpg"),[]byte("series-poster"),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(show,"fanart.jpg"),[]byte("series-backdrop"),0644);err!=nil{t.Fatal(err)}
 episodeDoc:=`<episodedetails><title>Geheimnisse</title><showtitle>Dark</showtitle>
<season>1</season><episode>1</episode><plot>Episode synopsis</plot>
<uniqueid type="tvdb">1234567</uniqueid></episodedetails>`
 if err:=os.WriteFile(filepath.Join(season,"Dark S01E01.nfo"),[]byte(episodeDoc),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(season,"Dark S01E01-thumb.jpg"),[]byte("episode-still"),0644);err!=nil{t.Fatal(err)}
 s.mu.Lock()
 s.st.Libraries=[]Library{{ID:2,Path:root,Type:"shows"}}
 s.st.Media=[]Media{{ID:71,LibraryID:2,Path:video,Kind:"episode",Title:"Episode 1",EpisodeTitle:"Episode 1",SeriesTitle:"Dark",Season:1,Episode:1}}
 s.st.Progress[71]=Progress{Position:56,Duration:93}
 s.st.ProfileFavorites[1]=map[int64]bool{71:true}
 s.mu.Unlock()
 preview:=httptest.NewRecorder()
 s.seriesNFOImport(preview,httptest.NewRequest(http.MethodGet,"/api/nfo/series",nil))
 if preview.Code!=200{t.Fatalf("preview failed %d %s",preview.Code,preview.Body.String())}
 var p seriesNFOReport
 if err:=json.Unmarshal(preview.Body.Bytes(),&p);err!=nil{t.Fatal(err)}
 if p.SeriesTotal!=1||p.Found!=1||p.EpisodesFound!=1{t.Fatalf("wrong series preview %+v",p)}
 if len(s.st.SeriesMetadata)!=0{t.Fatal("preview must not alter state")}
 rejected:=httptest.NewRecorder()
 s.seriesNFOImport(rejected,httptest.NewRequest(http.MethodPost,"/api/nfo/series",bytes.NewBufferString(`{}`)))
 if rejected.Code!=400{t.Fatal("series import must require explicit confirmation")}
 applied:=httptest.NewRecorder()
 s.seriesNFOImport(applied,httptest.NewRequest(http.MethodPost,"/api/nfo/series",bytes.NewBufferString(`{"confirm":true}`)))
 if applied.Code!=200{t.Fatalf("import failed %d: %s",applied.Code,applied.Body.String())}
 var result seriesNFOReport
 if err:=json.Unmarshal(applied.Body.Bytes(),&result);err!=nil{t.Fatal(err)}
 if result.Applied!=1||result.EpisodesApplied!=1||result.ImagesCached<3{t.Fatalf("unexpected import results %+v",result)}
 s.mu.RLock()
 m:=s.st.SeriesMetadata["dark"]
 ep:=s.st.Media[0]
 progress:=s.st.Progress[71];favorite:=s.st.ProfileFavorites[1][71]
 s.mu.RUnlock()
 if m.Title!="Dark"||m.Year!=2017||m.ExternalIDs["tvdb"]!="325731"||m.Overview==""||m.Poster==""||m.Backdrop==""{
  t.Fatalf("series metadata incomplete: %+v",m)
 }
 if ep.Title!="Geheimnisse"||ep.EpisodeTitle!="Geheimnisse"||ep.ExternalIDs["tvdb"]!="1234567"||ep.Backdrop==""{
  t.Fatalf("episode metadata not imported: %+v",ep)
 }
 if progress.Position!=56||!favorite||ep.ID!=71||ep.Path!=video{t.Fatal("import unexpectedly changed episode progress or identity")}
 seriesAPI:=httptest.NewRecorder()
 s.series(seriesAPI,httptest.NewRequest(http.MethodGet,"/api/series",nil))
 if seriesAPI.Code!=200||!strings.Contains(seriesAPI.Body.String(),`"poster":"/api/series/art?key=dark&type=poster"`){
  // URL HTML escaping depends on Go encoder; check semantic payload instead.
  var views []SeriesView
  if err:=json.Unmarshal(seriesAPI.Body.Bytes(),&views);err!=nil||len(views)!=1||views[0].Poster==""||views[0].Title!="Dark"{t.Fatalf("series view mismatch %s",seriesAPI.Body.String())}
 }
 for _,tc:=range []struct{kind,text string}{{"poster","series-poster"},{"backdrop","series-backdrop"}}{
  serve:=httptest.NewRecorder()
  s.seriesArt(serve,httptest.NewRequest(http.MethodGet,"/api/series/art?key=dark&type="+tc.kind,nil))
  if serve.Code!=200||serve.Body.String()!=tc.text{t.Fatalf("%s not served: %d %s",tc.kind,serve.Code,serve.Body.String())}
 }
 // Critical end-to-end safety: remove the original NFOs and artwork.
 for _,file:=range []string{filepath.Join(show,"tvshow.nfo"),filepath.Join(show,"poster.jpg"),filepath.Join(show,"fanart.jpg"),
 filepath.Join(season,"Dark S01E01.nfo"),filepath.Join(season,"Dark S01E01-thumb.jpg")}{
  if err:=os.Remove(file);err!=nil{t.Fatal(err)}
 }
 restarted,err:=New(s.cfg);if err!=nil{t.Fatal(err)}
 art:=httptest.NewRecorder()
 restarted.seriesArt(art,httptest.NewRequest(http.MethodGet,"/api/series/art?key=dark&type=poster",nil))
 if art.Code!=200||art.Body.String()!="series-poster"{t.Fatalf("imported series artwork lost after deleting source: %d",art.Code)}
 episodeImage:=httptest.NewRecorder()
 restarted.nfoArt(episodeImage,httptest.NewRequest(http.MethodGet,"/api/nfo/art?id=71&type=backdrop",nil))
 if episodeImage.Code!=200||episodeImage.Body.String()!="episode-still"{t.Fatalf("episode still lost after NFO deletion: %d %q",episodeImage.Code,episodeImage.Body.String())}
 restarted.mu.RLock()
 saved:=restarted.st.SeriesMetadata["dark"]
 afterEp:=restarted.st.Media[0]
 restarted.mu.RUnlock()
 if saved.ExternalIDs["imdb"]!="tt5753856"||afterEp.Title!="Geheimnisse"{t.Fatal("series metadata not persisted")}
}

func TestJellyfinSeriesNFORejectsWrongOrSharedNFO(t *testing.T){
 dir:=t.TempDir()
 first:=filepath.Join(dir,"A S01E01.mkv")
 second:=filepath.Join(dir,"B S01E01.mkv")
 for _,p:=range []string{first,second}{if err:=os.WriteFile(p,[]byte("video"),0644);err!=nil{t.Fatal(err)}}
 if err:=os.WriteFile(filepath.Join(dir,"tvshow.nfo"),[]byte("<tvshow><title>Wrong for both</title></tvshow>"),0644);err!=nil{t.Fatal(err)}
 report,series,_:=collectSeriesNFO([]Media{
 {ID:1,LibraryID:5,Path:first,Kind:"episode",SeriesTitle:"Show A"},
 {ID:2,LibraryID:5,Path:second,Kind:"episode",SeriesTitle:"Show B"}},
 []Library{{ID:5,Path:dir,Type:"shows"}})
 if report.Found!=0||report.Invalid!=2||len(series)!=0{t.Fatalf("shared root NFO must not be imported: %+v",report)}
}

func TestJellyfinSeriesArtworkByKnownIDs(t *testing.T){
 s:=newTestServer(t)
 s.st.SeriesMetadata=map[string]SeriesMetadata{"test show":{Title:"Test Show",ExternalIDs:map[string]string{"tvdb":"12"}}}
 s.seriesArtMu.Lock()
 s.seriesArtJob=ArtworkRepairJob{Running:true,Total:2}
 s.seriesArtMu.Unlock()
 rec:=httptest.NewRecorder()
 s.seriesArtworkRepair(rec,httptest.NewRequest(http.MethodPost,"/api/series/artwork/repair",nil))
 if rec.Code!=409{t.Fatalf("overlapping series background jobs should be rejected: %d",rec.Code)}
 if !numericID.MatchString(s.st.SeriesMetadata["test show"].ExternalIDs["tvdb"]){t.Fatal("series ID missing")}
 if _,ok:=s.seriesArtLocation("test show","poster");ok{t.Fatal("nonexistent artwork must not appear")}
 // Ensure async job state stays stable on GET.
 got:=httptest.NewRecorder()
 s.seriesArtworkRepair(got,httptest.NewRequest(http.MethodGet,"/api/series/artwork/repair",nil))
 if got.Code!=200||!strings.Contains(got.Body.String(),`"running":true`){t.Fatalf("status unavailable %s",got.Body.String())}
 _=time.Now()
}
