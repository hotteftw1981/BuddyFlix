package app

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestPlaybackProbeDiagnosesAC3AndMKV(t *testing.T){
 dir:=t.TempDir()
 ffprobe:=filepath.Join(dir,"ffprobe-test")
 script:="#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"},{\"codec_type\":\"audio\",\"codec_name\":\"ac3\"}],\"format\":{\"duration\":\"412.50\"}}'\n"
 if err:=os.WriteFile(ffprobe,[]byte(script),0755);err!=nil{t.Fatal(err)}
 ffmpeg:=filepath.Join(dir,"ffmpeg-test")
 if err:=os.WriteFile(ffmpeg,[]byte("#!/bin/sh\nexit 0\n"),0755);err!=nil{t.Fatal(err)}
 t.Setenv("BUDDYFLIX_FFPROBE",ffprobe)
 t.Setenv("BUDDYFLIX_FFMPEG",ffmpeg)
 info:=inspectPlayback("/media/S01E01.mkv")
 if !info.FFmpegAvailable||!info.FFprobeAvailable||info.VideoCodec!="h264"||info.AudioCodec!="ac3"||
 info.Recommended!="compat"||!info.CanRepairAudio||info.Duration!=412.5{
  t.Fatalf("expected audio repair recommendation, got %+v",info)
 }
 if info.Container!="mkv"{t.Fatal("file extension was not preserved")}
}

func TestPlaybackCheckDoesNotLeakOtherFiles(t *testing.T){
 s:=newTestServer(t)
 s.st.Media=[]Media{{ID:3,Title:"Allowed",Path:"/private/allowed.mp4"}}
 notFound:=httptest.NewRecorder()
 s.playbackCheck(notFound,httptest.NewRequest(http.MethodGet,"/api/playback/check?id=999",nil))
 if notFound.Code!=404{t.Fatalf("must reject unknown media IDs, got %d",notFound.Code)}
 bad:=httptest.NewRecorder()
 s.playbackCheck(bad,httptest.NewRequest(http.MethodGet,"/api/playback/check?id=../../etc/passwd",nil))
 if bad.Code!=400{t.Fatalf("must reject nonnumeric media IDs: %d",bad.Code)}
 data:=httptest.NewRecorder()
 s.playbackCheck(data,httptest.NewRequest(http.MethodGet,"/api/playback/check?id=3",nil))
 if data.Code!=200{t.Fatalf("expected supported indexed media, got %d",data.Code)}
 var parsed playbackInfo
 if err:=json.Unmarshal(data.Body.Bytes(),&parsed);err!=nil{t.Fatal(err)}
 if parsed.ID!=3{t.Fatal("diagnostics returned wrong media")}
}

func TestFFmpegArgumentsCopyVideoButConvertAudio(t *testing.T){
 args:=audioRemuxArgs("/media/a TV series' tricky.mkv",76.5)
 str:=strings.Join(args," ")
 if !strings.Contains(str,"-c:v copy")||!strings.Contains(str,"-c:a aac")||
 !strings.Contains(str,"-ac 2")||!strings.Contains(str,"-f mp4 pipe:1")||
 !strings.Contains(str,"-ss 76.500") {t.Fatalf("remux flags invalid %s",str)}
}

func TestAudioCompatEndpointWithFakeFFmpegAndBusyLimit(t *testing.T){
 s:=newTestServer(t)
 dir:=t.TempDir()
 video:=filepath.Join(dir,"Movie.mkv")
 if err:=os.WriteFile(video,[]byte("movie-placeholder"),0644);err!=nil{t.Fatal(err)}
 ffmpeg:=filepath.Join(dir,"ffmpeg-test")
 if err:=os.WriteFile(ffmpeg,[]byte("#!/bin/sh\nprintf 'FAKE-FMP4-STREAM'\n"),0755);err!=nil{t.Fatal(err)}
 t.Setenv("BUDDYFLIX_FFMPEG",ffmpeg)
 // Use a nonexistent probe so stream validation does not depend on runner codecs.
 t.Setenv("BUDDYFLIX_FFPROBE",filepath.Join(dir,"none"))
 s.st.Media=[]Media{{ID:45,Path:video,Kind:"movie",Title:"Movie"}}
 request:=httptest.NewRequest(http.MethodGet,"/stream/compat/45?start=25",nil)
 rec:=httptest.NewRecorder()
 s.streamAudioCompatible(rec,request)
 if rec.Code!=200||rec.Body.String()!="FAKE-FMP4-STREAM"||
 rec.Header().Get("Content-Type")!="video/mp4"{
  t.Fatalf("compatible stream was not served %d %s",rec.Code,rec.Body.String())
 }
 // Concurrent-stream limit is designed for the small QNAP ARM CPU.
 audioRemuxSlots<-struct{}{}
 busy:=httptest.NewRecorder()
 s.streamAudioCompatible(busy,httptest.NewRequest(http.MethodGet,"/stream/compat/45",nil))
 <-audioRemuxSlots
 if busy.Code!=429{t.Fatalf("ARM concurrency limit missing: %d",busy.Code)}
 invalid:=httptest.NewRecorder()
 s.streamAudioCompatible(invalid,httptest.NewRequest(http.MethodGet,"/stream/compat/45?start=-10",nil))
 if invalid.Code!=400{t.Fatalf("invalid seek accepted: %d",invalid.Code)}
}
