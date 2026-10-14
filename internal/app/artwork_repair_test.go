package app

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestArtworkSourceURLIsConstrained(t *testing.T){
    accepted:=[]string{
        "https://image.tmdb.org/t/p/w500/poster.jpg",
        "https://artworks.thetvdb.com/banners/v4/movies/12/posters/abc.jpg",
    }
    for _,v:=range accepted{if !approvedArtworkURL(v){t.Fatalf("valid artwork host rejected: %s",v)}}
    rejected:=[]string{
        "file:///etc/passwd","http://127.0.0.1/private","https://artworks.thetvdb.com.evil.invalid/a.jpg",
        "https://image.tmdb.org:8080/a.jpg","https://example.org/anything.jpg",
        "https://image.tmdb.org@localhost/private", "https://image.tmdb.org/a.jpg?token=123",
    }
    for _,v:=range rejected{if approvedArtworkURL(v){t.Fatalf("untrusted artwork host accepted: %s",v)}}
}

func TestArtworkRepairUsesExistingIDsOnly(t *testing.T){
    m:=Media{ID:10,Title:"Definitely wrong title",ExternalIDs:map[string]string{"imdb":"tt0816692"},Kind:"movie"}
    if !artworkCandidate(m){t.Fatal("valid imdb movie must be repair candidate")}
    m.ExternalIDs=map[string]string{"imdb":"1234"}
    if artworkCandidate(m){t.Fatal("invalid imdb must not qualify")}
    m.ExternalIDs=map[string]string{"tmdb":"157336"}
    if !artworkCandidate(m){t.Fatal("tmdb id must qualify")}
    m.Kind="episode"
    if artworkCandidate(m){t.Fatal("episodic art must be excluded from film-only repair")}
}

func TestTVDBFindMovieByExistingIMDbID(t *testing.T){
    fake:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        w.Header().Set("Content-Type","application/json")
        switch r.URL.Path{
        case "/v4/search/remoteid/tt0816692":
            _,_ = w.Write([]byte(`{"data":[{"movie":{"id":123}}]}`))
        case "/v4/movies/123/extended":
            _,_ = w.Write([]byte(`{"data":{"id":123,"image":"https://artworks.thetvdb.com/banners/123/poster.jpg",
            "artworks":[{"image":"https://artworks.thetvdb.com/banners/123/backdrop.jpg","width":1200,"height":670}]}}`))
        case "/v4/movies/123/translations/deu":
            _,_=w.Write([]byte(`{"data":{"name":"Interstellar"}}`))
        default:t.Errorf("unwanted provider request %s",r.URL.Path);w.WriteHeader(404)
        }
    }))
    defer fake.Close()
    p:=NewTVDBProvider("test-key")
    p.baseURL=fake.URL+"/v4"
    p.token="valid-test-token"
    p.tokenExpiry=time.Now().Add(1*time.Hour)
    id,err:=tvdbMovieForIMDb(p,"tt0816692")
    if err!=nil||id!="123"{t.Fatalf("remote id mapping: %q %v",id,err)}
    poster,backdrop:=providerArtwork(Media{ExternalIDs:map[string]string{"imdb":"tt0816692"}},nil,p)
    if !strings.Contains(poster,"123/poster.jpg")||!strings.Contains(backdrop,"123/backdrop.jpg"){
        t.Fatalf("provider artwork was not resolved from imdb: poster %s backdrop %s",poster,backdrop)
    }
}

func TestArtworkRepairRejectsDuplicateRun(t *testing.T){
    s:=newTestServer(t)
    s.artworkMu.Lock()
    s.artworkJob=ArtworkRepairJob{Running:true,Total:2}
    s.artworkMu.Unlock()
    second:=httptest.NewRecorder()
    s.artworkRepair(second,httptest.NewRequest(http.MethodPost,"/api/nfo/artwork/repair",nil))
    if second.Code!=409{t.Fatalf("should not start two image recovery jobs, got %d",second.Code)}
    get:=httptest.NewRecorder()
    s.artworkRepair(get,httptest.NewRequest(http.MethodGet,"/api/nfo/artwork/repair",nil))
    if get.Code!=200||!strings.Contains(get.Body.String(),`"running":true`){t.Fatalf("job status not returned: %d %s",get.Code,get.Body.String())}
}
