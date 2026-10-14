package app

import (
 "bytes"
 "encoding/json"
 "mime/multipart"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestPosterUploadKeepsMovieMappingAndProgress(t *testing.T){
 s:=newTestServer(t)
 s.mu.Lock()
 s.st.Media=[]Media{{ID:21,Title:"Film",Path:"/media/film.mkv",Overview:"Correct description",
   ExternalIDs:map[string]string{"imdb":"tt0816692","tmdb":"157336"},MetadataState:"nfo",MetadataLocked:true}}
 s.st.Progress[21]=Progress{Position:123,Duration:400}
 s.mu.Unlock()
 // A minimal PNG with the correct signature and IHDR header.
 png:=[]byte{137,80,78,71,13,10,26,10,0,0,0,13,73,72,68,82,0,0,0,1,0,0,0,1,8,6,0,0,0}
 var body bytes.Buffer
 w:=multipart.NewWriter(&body)
 file,err:=w.CreateFormFile("image","poster.png")
 if err!=nil{t.Fatal(err)}
 if _,err:=file.Write(png);err!=nil{t.Fatal(err)}
 if err:=w.Close();err!=nil{t.Fatal(err)}
 req:=httptest.NewRequest(http.MethodPost,"/api/artwork/upload?id=21&kind=poster",&body)
 req.Header.Set("Content-Type",w.FormDataContentType())
 rec:=httptest.NewRecorder()
 s.artworkUpload(rec,req)
 if rec.Code!=200{t.Fatalf("upload failed: %d %s",rec.Code,rec.Body.String())}
 s.mu.RLock()
 m:=s.st.Media[0]
 progress:=s.st.Progress[21]
 s.mu.RUnlock()
 if m.Poster!="/api/nfo/art?id=21&type=poster"||m.Overview!="Correct description"||
 m.MetadataState!="nfo"||!m.MetadataLocked||m.ExternalIDs["imdb"]!="tt0816692"||progress.Position!=123{
  t.Fatalf("poster edit unexpectedly rewrote metadata or progress: %+v",m)
 }
 path,exists:=s.cachedArt(21,"poster")
 if !exists{t.Fatal("poster must be saved inside private BuddyFlix cache")}
 saved,err:=os.ReadFile(path);if err!=nil||!bytes.Equal(saved,png){t.Fatal("cached image mismatch")}
 restarted,err:=New(s.cfg);if err!=nil{t.Fatal(err)}
 got:=httptest.NewRecorder()
 restarted.nfoArt(got,httptest.NewRequest(http.MethodGet,m.Poster,nil))
 if got.Code!=200||!bytes.Equal(got.Body.Bytes(),png){t.Fatalf("poster unavailable after server restart: %d",got.Code)}
}

func TestPosterManualHTTPSKeepsIDs(t *testing.T){
 s:=newTestServer(t)
 s.st.Media=[]Media{{ID:4,Title:"Original",ExternalIDs:map[string]string{"tvdb":"555"},Overview:"untouched",Kind:"movie"}}
 req:=httptest.NewRequest(http.MethodPost,"/api/artwork/manual",strings.NewReader(
  `{"id":4,"kind":"poster","url":"https://images.example.org/poster.jpg"}`))
 rec:=httptest.NewRecorder()
 s.artworkManual(rec,req)
 if rec.Code!=200{t.Fatalf("manual poster failed: %d %s",rec.Code,rec.Body.String())}
 m:=s.st.Media[0]
 if m.Poster!="https://images.example.org/poster.jpg"||m.Title!="Original"||m.Overview!="untouched"||m.ExternalIDs["tvdb"]!="555"{
  t.Fatalf("manual URL changed assignment: %+v",m)
 }
 if _,ok:=s.cachedArt(4,"poster");ok{t.Fatal("external arbitrary URL must not be silently downloaded")}
}

func TestPosterEndpointsRejectUnsafeURLAndBadFile(t *testing.T){
 for _,u:=range []string{"http://localhost:80/p.jpg","https://127.0.0.1/image.png",
  "https://localhost/image.jpg","javascript:alert(1)","file:///etc/passwd","https://user:pass@example.com/image.jpg"}{
  if isSafePosterLink(u){t.Fatalf("unsafe poster link accepted: %s",u)}
 }
 s:=newTestServer(t)
 s.st.Media=[]Media{{ID:9,Title:"Keep existing",Poster:"https://images.example.org/old.jpg"}}
 var buf bytes.Buffer
 mw:=multipart.NewWriter(&buf)
 file,err:=mw.CreateFormFile("image","bad.txt");if err!=nil{t.Fatal(err)}
 _,_=file.Write([]byte("not an image"))
 if err:=mw.Close();err!=nil{t.Fatal(err)}
 req:=httptest.NewRequest(http.MethodPost,"/api/artwork/upload?id=9&kind=poster",&buf)
 req.Header.Set("Content-Type",mw.FormDataContentType())
 rec:=httptest.NewRecorder()
 s.artworkUpload(rec,req)
 if rec.Code!=400 {t.Fatalf("bad upload should fail, status %d",rec.Code)}
 if s.st.Media[0].Poster!="https://images.example.org/old.jpg"{t.Fatal("bad upload changed poster")}
 if _,err:=os.Stat(filepath.Join(s.cfg.DataDir,"nfo-art","9-poster.jpg"));!os.IsNotExist(err){
  t.Fatalf("unexpected cached file after invalid image: %v",err)
 }
}

func TestPosterSuggestionEndpointHandlesNoProviders(t *testing.T){
 s:=newTestServer(t)
 s.st.Media=[]Media{{ID:41,Title:"Known Movie",ExternalIDs:map[string]string{"imdb":"tt0816692"}}}
 rec:=httptest.NewRecorder()
 s.artworkSuggestions(rec,httptest.NewRequest(http.MethodGet,"/api/artwork/suggestions?id=41",nil))
 if rec.Code!=200{t.Fatalf("suggestions: %d %s",rec.Code,rec.Body.String())}
 var parsed struct{Items []artworkSuggestion `json:"items"`}
 if err:=json.Unmarshal(rec.Body.Bytes(),&parsed);err!=nil{t.Fatal(err)}
 if len(parsed.Items)!=0 {t.Fatal("no provider credentials should not fabricate results")}
 rec=httptest.NewRecorder()
 s.artworkSuggestions(rec,httptest.NewRequest(http.MethodGet,"/api/artwork/suggestions?id=100",nil))
 if rec.Code!=404{t.Fatalf("missing movie must be 404, got %d",rec.Code)}
}
