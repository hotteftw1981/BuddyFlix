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
)

func TestJellyfinNFOPreviewAndImport(t *testing.T) {
    s:=newTestServer(t)
    dir:=t.TempDir()
    movie:=filepath.Join(dir,"Interstellar (2014).mkv")
    nfo:=filepath.Join(dir,"Interstellar (2014).nfo")
    if err:=os.WriteFile(movie,[]byte("placeholder"),0644);err!=nil{t.Fatal(err)}
    doc:=`<?xml version="1.0" encoding="utf-8"?>
<movie>
  <title>Interstellar</title><originaltitle>Interstellar</originaltitle>
  <year>2014</year><plot>A journey through space.</plot>
  <rating>8.6</rating><mpaa>PG-13</mpaa>
  <runtime>169</runtime><genre>Science Fiction</genre><genre>Drama</genre>
  <director>Christopher Nolan</director><actor><name>Matthew McConaughey</name><role>Cooper</role></actor>
  <uniqueid type="imdb" default="true">tt0816692</uniqueid>
  <uniqueid type="tmdb">157336</uniqueid>
  <uniqueid type="tvdb">435</uniqueid>
</movie>`
    if err:=os.WriteFile(nfo,[]byte(doc),0644);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(filepath.Join(dir,"poster.jpg"),[]byte("test-poster"),0644);err!=nil{t.Fatal(err)}
    s.mu.Lock()
    s.st.Media=[]Media{
        {ID:41,Path:movie,Title:"Wrong Guess",Year:2002,Poster:"https://old.invalid/wrong.jpg",Overview:"wrong",
            ExternalIDs:map[string]string{"tmdb":"999"},MetadataState:"auto",MetadataLocked:true},
        {ID:42,Path:filepath.Join(dir,"Not Exported.mkv"),Title:"Leave Alone",ExternalIDs:map[string]string{"imdb":"tt9999999"}},
    }
    s.st.Progress[41]=Progress{Position:120,Duration:300}
    s.st.ProfileFavorites[1]=map[int64]bool{41:true}
    s.mu.Unlock()

    preview:=httptest.NewRecorder()
    s.nfoImport(preview,httptest.NewRequest(http.MethodGet,"/api/nfo/import",nil))
    if preview.Code!=200 {t.Fatalf("preview: %v %s",preview.Code,preview.Body.String())}
    var p nfoPreview
    if err:=json.Unmarshal(preview.Body.Bytes(),&p);err!=nil{t.Fatal(err)}
    if p.Total!=2||p.Found!=1||p.Missing!=1||p.NoIDs!=0||p.Applied!=0 {t.Fatalf("bad preview: %+v",p)}
    if p.Examples[0].IDs["imdb"]!="tt0816692" {t.Fatal("IMDb ID not detected")}

    s.mu.RLock()
    if s.st.Media[0].Title!="Wrong Guess" {t.Fatal("preview mutated data")}
    s.mu.RUnlock()

    reject:=httptest.NewRecorder()
    s.nfoImport(reject,httptest.NewRequest(http.MethodPost,"/api/nfo/import",bytes.NewBufferString(`{}`)))
    if reject.Code!=400 {t.Fatal("POST without explicit confirmation must fail")}

    result:=httptest.NewRecorder()
    s.nfoImport(result,httptest.NewRequest(http.MethodPost,"/api/nfo/import",bytes.NewBufferString(`{"confirm":true}`)))
    if result.Code!=200 {t.Fatalf("import: %v %s",result.Code,result.Body.String())}
    var after nfoPreview
    if err:=json.Unmarshal(result.Body.Bytes(),&after);err!=nil{t.Fatal(err)}
    if after.Applied!=1 {t.Fatalf("expected one import, got %+v",after)}
    s.mu.RLock()
    imported:=s.st.Media[0]; untouched:=s.st.Media[1]
    progress:=s.st.Progress[41]; fav:=s.st.ProfileFavorites[1][41]
    s.mu.RUnlock()
    if imported.Title!="Interstellar" || imported.Year!=2014 || imported.ExternalIDs["imdb"]!="tt0816692" ||
       imported.ExternalIDs["tmdb"]!="157336" || imported.Overview!="A journey through space." ||
       len(imported.Genres)!=2 || imported.Runtime!=169 || imported.MetadataState!="nfo" || !imported.MetadataLocked {
        t.Fatalf("NFO import incorrect: %+v",imported)
    }
    if imported.ExternalIDs["tmdb"]=="999" || imported.Poster=="" || !strings.HasPrefix(imported.Poster,"/api/nfo/art") {t.Fatal("stale mapping or art")}
    if untouched.Title!="Leave Alone" || untouched.ExternalIDs["imdb"]!="tt9999999" {t.Fatal("item without NFO was changed")}
    if imported.ID!=41||imported.Path!=movie||progress.Position!=120||!fav {t.Fatal("core media, progress or favorites changed")}
    art:=httptest.NewRecorder()
    s.nfoArt(art,httptest.NewRequest(http.MethodGet,"/api/nfo/art?id=41&type=poster",nil))
    if art.Code!=200||art.Body.String()!="test-poster" {t.Fatalf("art endpoint failed: %d %s",art.Code,art.Body.String())}
    if err:=os.Remove(nfo);err!=nil {t.Fatal(err)}
    if err:=os.Remove(filepath.Join(dir,"poster.jpg"));err!=nil {t.Fatal(err)}
    // Imported artwork must survive deletion of BOTH the NFO and Jellyfin's
    // original poster. The poster is served from BuddyFlix's private DataDir.
    restarted,err:=New(s.cfg)
    if err!=nil {t.Fatal(err)}
    artAfterDelete:=httptest.NewRecorder()
    restarted.nfoArt(artAfterDelete,httptest.NewRequest(http.MethodGet,"/api/nfo/art?id=41&type=poster",nil))
    if artAfterDelete.Code!=200||artAfterDelete.Body.String()!="test-poster" {
        t.Fatalf("NFO deletion broke cached poster: %d %s",artAfterDelete.Code,artAfterDelete.Body.String())
    }
    restarted.mu.RLock()
    saved:=restarted.st.Media[0]
    restarted.mu.RUnlock()
    if saved.Title!="Interstellar"||saved.ExternalIDs["imdb"]!="tt0816692"||saved.Overview!="A journey through space." {
        t.Fatalf("imported metadata lost after NFO deletion: %+v",saved)
    }

}

func TestJellyfinNFOImageCacheReportsMissingSource(t *testing.T) {
    s:=newTestServer(t)
    dir:=t.TempDir()
    video:=filepath.Join(dir,"movie.mkv")
    if err:=os.WriteFile(video,[]byte("video"),0644);err!=nil {t.Fatal(err)}
    m:=Media{ID:71,Path:video,Kind:"movie"}
    n:=jellyfinNFO{}
    url,local,err:=s.cacheNFOArt(m,n,"poster")
    if err!=nil||local||url!="" {t.Fatalf("expected no image, got %q %v %v",url,local,err)}
    if _,ok:=s.cachedArt(71,"poster");ok {t.Fatal("empty image cache must stay empty")}
}

func TestJellyfinNFOMovieFallbackAvoidsAmbiguousFolder(t *testing.T) {
    dir:=t.TempDir()
    first:=filepath.Join(dir,"Film A.mkv")
    second:=filepath.Join(dir,"Film B.mkv")
    for _,file:=range []string{first,second} {if err:=os.WriteFile(file,[]byte("movie"),0644);err!=nil{t.Fatal(err)}}
    if err:=os.WriteFile(filepath.Join(dir,"movie.nfo"),[]byte("<movie><title>Film A</title></movie>"),0644);err!=nil{t.Fatal(err)}
    if _,ok,_:=nfoLocation(Media{Path:first,Kind:"movie"});ok{t.Fatal("movie.nfo cannot be assigned to both videos")}
    if err:=os.Remove(second);err!=nil{t.Fatal(err)}
    if path,ok,err:=nfoLocation(Media{Path:first,Kind:"movie"});err!=nil||!ok||filepath.Base(path)!="movie.nfo"{t.Fatalf("single-movie fallback failed: %s %v %v",path,ok,err)}
}

func TestJellyfinNFORejectsInvalidAndWrongType(t *testing.T) {
    dir:=t.TempDir()
    movie:=filepath.Join(dir,"Movie.mkv")
    nfo:=filepath.Join(dir,"Movie.nfo")
    if err:=os.WriteFile(movie,[]byte("media"),0644);err!=nil{t.Fatal(err)}
    s:=newTestServer(t)
    s.st.Media=[]Media{{ID:7,Path:movie,Title:"Existing",ExternalIDs:map[string]string{"imdb":"tt1234567"},Kind:"movie"}}
    if err:=os.WriteFile(nfo,[]byte("<movie><title>"),0644);err!=nil{t.Fatal(err)}
    p,h:=buildNFOPreview(s.st.Media)
    if p.Invalid!=1||len(h)!=0 {t.Fatalf("invalid NFO accepted: %+v",p)}
    if err:=os.WriteFile(nfo,[]byte("<episodedetails><title>Not a movie</title></episodedetails>"),0644);err!=nil{t.Fatal(err)}
    p,h=buildNFOPreview(s.st.Media)
    if p.Invalid!=1||len(h)!=0 {t.Fatalf("wrong NFO type accepted: %+v",p)}
}

func TestJellyfinNFOEpisodeWithMatchingFilename(t *testing.T) {
    dir:=t.TempDir()
    video:=filepath.Join(dir,"Pilot S01E01.mkv")
    nfo:=filepath.Join(dir,"Pilot S01E01.nfo")
    if err:=os.WriteFile(video,[]byte("video"),0644);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(nfo,[]byte(`<episodedetails><title>Pilot</title><showtitle>Show</showtitle><season>1</season><episode>1</episode><uniqueid type="tvdb">123</uniqueid></episodedetails>`),0644);err!=nil{t.Fatal(err)}
    p,h:=buildNFOPreview([]Media{{ID:1,Path:video,Kind:"episode",SeriesTitle:"Show"}})
    if p.Found!=1||h[0].IDs["tvdb"]!="123" {t.Fatalf("episode NFO failed: %+v",p)}
}
