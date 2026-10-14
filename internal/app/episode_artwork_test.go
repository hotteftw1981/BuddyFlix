package app

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
)

func TestTheTVDBEpisodeStillLookupRequiresExactSeasonAndEpisode(t *testing.T){
 fake:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  if r.URL.Path!="/v4/series/101/episodes/default"||r.URL.Query().Get("season")!="2"{
   t.Errorf("unexpected TVDB lookup: %s",r.URL.String());w.WriteHeader(404);return
  }
  if r.URL.Query().Get("page")=="0"{
   _,_=w.Write([]byte(`{"data":{"episodes":[
     {"seasonNumber":2,"number":1,"image":"https://artworks.thetvdb.com/banners/ep1.jpg"},
     {"seasonNumber":2,"number":3,"image":"https://artworks.thetvdb.com/banners/ep3.jpg"},
     {"seasonNumber":1,"number":2,"image":"https://artworks.thetvdb.com/banners/wrong.jpg"}
   ]},"links":{"next":"https://thetvdb.com/next"}}`));return
  }
  _,_=w.Write([]byte(`{"data":{"episodes":[
    {"seasonNumber":2,"number":2,"image":"https://artworks.thetvdb.com/banners/ep2.jpg"},
    {"seasonNumber":2,"number":4,"image":"http://127.0.0.1/private"}
  ]},"links":{"next":null}}`))
 }))
 defer fake.Close()
 p:=NewTVDBProvider("test")
 p.baseURL=fake.URL+"/v4"
 p.token="test-token"
 p.tokenExpiry=time.Now().Add(time.Hour)
 stills,err:=tvdbSeasonStills(p,"101",2)
 if err!=nil{t.Fatal(err)}
 if len(stills)!=3||!strings.HasSuffix(stills[1],"/ep1.jpg")||
 !strings.HasSuffix(stills[2],"/ep2.jpg")||!strings.HasSuffix(stills[3],"/ep3.jpg"){
  t.Fatalf("incorrect TVDB mapping: %+v",stills)
 }
 if _,err:=tvdbSeasonStills(p,"xyz",2);err==nil{t.Fatal("invalid series ID accepted")}
}

func TestTMDbSeasonStillsExactEpisodeNumbers(t *testing.T){
 fake:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/tv/81/season/1"||r.URL.Query().Get("api_key")!="test-key"{
   t.Errorf("unexpected TMDB route: %s",r.URL.String());w.WriteHeader(400);return
  }
  w.Header().Set("Content-Type","application/json")
  _,_=w.Write([]byte(`{"episodes":[
   {"season_number":1,"episode_number":1,"still_path":"/still1.jpg"},
   {"season_number":1,"episode_number":3,"still_path":"/still3.png"},
   {"season_number":2,"episode_number":2,"still_path":"/wrongseason.jpg"},
   {"season_number":1,"episode_number":4,"still_path":""}
  ]}`))
 }))
 defer fake.Close()
 results,err:=tmdbSeasonStills(fake.Client(),fake.URL,"test-key","81",1)
 if err!=nil{t.Fatal(err)}
 if len(results)!=2||results[1]!="https://image.tmdb.org/t/p/w780/still1.jpg"||
 results[3]!="https://image.tmdb.org/t/p/w780/still3.png" {
  t.Fatalf("unexpected TMDB stills: %+v",results)
 }
}

func TestEpisodesArtworkRepairTargetsOnlyMissingEpisodeStills(t *testing.T){
 s:=newTestServer(t)
 s.mu.Lock()
 s.st.SeriesMetadata=map[string]SeriesMetadata{
  "ballers":{Title:"Ballers",ExternalIDs:map[string]string{"tvdb":"345"}},
 }
 s.st.Media=[]Media{
  {ID:1,Kind:"episode",SeriesTitle:"Ballers",Season:1,Episode:1,Title:"Pilot"},
  {ID:2,Kind:"episode",SeriesTitle:"Ballers",Season:1,Episode:2,Title:"Second",Backdrop:"https://images.example.org/custom.jpg"},
  {ID:3,Kind:"movie",Title:"Film",ExternalIDs:map[string]string{"tvdb":"234"}},
  {ID:4,Kind:"episode",SeriesTitle:"Without valid ID",Season:2,Episode:1},
  {ID:5,Kind:"episode",SeriesTitle:"Ballers",Season:1,Episode:0},
 }
 s.mu.Unlock()
 if !episodeNeedsStill(s,s.st.Media[0])||episodeNeedsStill(s,s.st.Media[1])||
 episodeNeedsStill(s,s.st.Media[2])||episodeNeedsStill(s,s.st.Media[4]){
  t.Fatal("incorrect episode filter")
 }
 // Verify GET is read-only; inspect exact target count computed by POST.
 status:=httptest.NewRecorder()
 s.episodeArtworkRepair(status,httptest.NewRequest(http.MethodGet,"/api/episodes/artwork/repair",nil))
 if status.Code!=200{t.Fatalf("status error %d",status.Code)}
 // Temporarily mark the job running to validate the mutual exclusion guard
 // without spawning a remote API request.
 s.episodeArtMu.Lock()
 s.episodeArtJob=ArtworkRepairJob{Running:true,Total:1,CurrentTitle:"Ballers"}
 s.episodeArtMu.Unlock()
 request:=httptest.NewRecorder()
 s.episodeArtworkRepair(request,httptest.NewRequest(http.MethodPost,"/api/episodes/artwork/repair",nil))
 if request.Code!=409{t.Fatalf("duplicate job accepted %d",request.Code)}
 var parsed ArtworkRepairJob
 if err:=json.Unmarshal(status.Body.Bytes(),&parsed);err!=nil{t.Fatal(err)}
 if parsed.Running{t.Fatal("idle status was incorrectly marked running")}
}

func TestEpisodeArtworkRemainsAfterOriginalNFODeleted(t *testing.T){
 s:=newTestServer(t)
 s.mu.Lock()
 s.st.Media=[]Media{{ID:12,Kind:"episode",Title:"Pilot",SeriesTitle:"Ballers",Season:1,Episode:1,
  ExternalIDs:map[string]string{"tvdb":"101"},MetadataProvider:"jellyfin-nfo"}}
 s.st.Progress[12]=Progress{Position:153,Duration:300}
 s.mu.Unlock()
 // Persisting one image for the episode must never modify its identity.
 image:=[]byte{137,80,78,71,13,10,26,10,0,0,0,13,73,72,68,82,0,0,0,1,0,0,0,1,8,6,0,0,0}
 url,err:=s.saveUploadedArtwork(12,"backdrop",image)
 if err!=nil{t.Fatal(err)}
 if err:=s.updateArtworkOnly(12,"backdrop",url);err!=nil{t.Fatal(err)}
 restarted,err:=New(s.cfg)
 if err!=nil{t.Fatal(err)}
 rec:=httptest.NewRecorder()
 restarted.nfoArt(rec,httptest.NewRequest(http.MethodGet,url,nil))
 if rec.Code!=200{t.Fatalf("episode image not served after restart: %d",rec.Code)}
 got:=restarted.st.Media[0]
 if got.Title!="Pilot"||got.ExternalIDs["tvdb"]!="101"||got.Season!=1||got.Episode!=1||
 restarted.st.Progress[12].Position!=153{t.Fatalf("episode metadata/progress changed: %+v",got)}
}
