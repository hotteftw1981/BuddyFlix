package app

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "math"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "time"
)

// On older ARMHF NAS systems ffmpeg is optional. Direct Play works without it.
// A single audio-remux session is permitted to protect the weak ARMv7 CPU.
var audioRemuxSlots=make(chan struct{},1)

type playbackInfo struct {
 ID int64 `json:"id"`
 Container string `json:"container"`
 AudioCodec string `json:"audio_codec,omitempty"`
 VideoCodec string `json:"video_codec,omitempty"`
 Duration float64 `json:"duration,omitempty"`
 FFmpegAvailable bool `json:"ffmpeg_available"`
 FFprobeAvailable bool `json:"ffprobe_available"`
 CanRepairAudio bool `json:"can_repair_audio"`
 Recommended string `json:"recommended"`
 Reason string `json:"reason"`
}

func binaryPath(program string)string{
 env:="BUDDYFLIX_FFMPEG"
 if program=="ffprobe"{env="BUDDYFLIX_FFPROBE"}
 if value:=strings.TrimSpace(os.Getenv(env));value!=""{
  if strings.ContainsRune(value,os.PathSeparator){
   if f,err:=os.Stat(value);err==nil&&f.Mode().IsRegular()&&f.Mode()&0111!=0{return value}
  }else if p,err:=exec.LookPath(value);err==nil{return p}
 }
 if f,err:=exec.LookPath(program);err==nil{return f}
 candidates:=[]string{
  "/usr/local/bin/"+program,"/usr/bin/"+program,"/opt/bin/"+program,
  "/share/CACHEDEV1_DATA/.qpkg/ffmpeg/"+program,
  "/share/CACHEDEV1_DATA/.qpkg/Entware/bin/"+program,
  "/share/MD0_DATA/.qpkg/ffmpeg/"+program,
 }
 // Jellyfin is installed on the same NAS. Its bundled ffmpeg may be reusable
 // even while Jellyfin is stopped. Never require its service to be running.
 for _,share:=range []string{"/share/CACHEDEV1_DATA","/share/CACHEDEV2_DATA","/share/MD0_DATA","/share/HDA_DATA"}{
  roots,err:=filepath.Glob(filepath.Join(share,".qpkg","[Jj]ellyfin*"))
  if err!=nil{continue}
  for _,root:=range roots{
   for _,relative:=range []string{program,"bin/"+program,"ffmpeg/"+program,
     "jellyfin-ffmpeg/"+program,"jellyfin-ffmpeg/usr/bin/"+program,
     "bin/jellyfin-ffmpeg/"+program}{
    candidates=append(candidates,filepath.Join(root,relative))
   }
  }
 }
 for _,p:=range candidates{
  if f,err:=os.Stat(p);err==nil&&f.Mode().IsRegular()&&f.Mode()&0111!=0{return p}
 }
 return ""
}
func (s *Server) pathForPlayback(id int64)(string,bool){
 if id<1{return "",false}
 s.mu.RLock()
 defer s.mu.RUnlock()
 for _,m:=range s.st.Media{
  if m.ID==id&&!m.Missing&&m.Path!="" {return m.Path,true}
 }
 return "",false
}
func inspectPlayback(path string) playbackInfo{
 info:=playbackInfo{Container:strings.TrimPrefix(strings.ToLower(filepath.Ext(path)),"."),
  Recommended:"direct",Reason:"Direct Play"}
 info.FFmpegAvailable=binaryPath("ffmpeg")!=""
 ffprobe:=binaryPath("ffprobe")
 info.FFprobeAvailable=ffprobe!=""
 if ffprobe!=""{
  ctx,cancel:=context.WithTimeout(context.Background(),7*time.Second)
  defer cancel()
  cmd:=exec.CommandContext(ctx,ffprobe,"-v","error","-show_entries",
   "stream=codec_type,codec_name:format=duration","-of","json",path)
  out,err:=cmd.Output()
  if err==nil{
   var result struct{
    Streams []struct{Type string `json:"codec_type"`;Name string `json:"codec_name"`} `json:"streams"`
    Format struct{Duration string `json:"duration"`} `json:"format"`
   }
   if json.Unmarshal(out,&result)==nil{
    for _,stream:=range result.Streams{
     if stream.Type=="video"&&info.VideoCodec==""{info.VideoCodec=strings.ToLower(stream.Name)}
     if stream.Type=="audio"&&info.AudioCodec==""{info.AudioCodec=strings.ToLower(stream.Name)}
    }
    if duration,err:=strconv.ParseFloat(result.Format.Duration,64);err==nil&&duration>0&&!math.IsInf(duration,0)&&!math.IsNaN(duration){
     info.Duration=duration
    }
   }
  }
 }
 if !audioCompatibleVideo(info.VideoCodec) {
  info.Reason="Video-Codec "+info.VideoCodec+": Audio-Konvertierung allein reicht hier nicht aus."
  info.CanRepairAudio=false
  return info
 }
 info.CanRepairAudio=info.FFmpegAvailable
 unsupportedAudio:=info.AudioCodec!=""&&info.AudioCodec!="aac"&&info.AudioCodec!="mp3"&&info.AudioCodec!="opus"
 needsRemux:=info.Container=="mkv"||info.Container=="avi"||info.Container=="ts"||info.Container=="m2ts"
 if unsupportedAudio||needsRemux {
  if info.FFmpegAvailable{
   info.Recommended="compat"
   info.Reason="AAC-Stereo mit unverändertem Video (bei HEVC abhängig von der Browserunterstützung)."
  }else{
   info.Recommended="direct"
   info.Reason="Tonspur/Container vermutlich nicht browserkompatibel. FFmpeg wurde auf dem QNAP nicht gefunden."
  }
 }else if info.AudioCodec=="" {
  info.Reason="Tonspur unbekannt: bei stummem Video Audio-Kompatibilitätsmodus versuchen."
 }
 return info
}

func (s *Server) playbackCheck(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{jsonErr(w,405,"method not allowed");return}
 id,err:=strconv.ParseInt(r.URL.Query().Get("id"),10,64)
 if err!=nil||id<=0{jsonErr(w,400,"invalid media id");return}
 path,found:=s.pathForPlayback(id)
 if !found{jsonErr(w,404,"media not found");return}
 result:=inspectPlayback(path)
 result.ID=id
 jsonOut(w,result)
}
func audioCompatibleVideo(codec string)bool {
 switch strings.ToLower(codec){
 case "", "h264","avc1","hevc","h265","hvc1":return true
 default:return false
 }
}
func audioRemuxArgsForCodec(path string,start float64,codec string)[]string{
 args:=audioRemuxArgs(path,start)
 if codec=="hevc"||codec=="h265"||codec=="hvc1" {
  // HEVC from MKV must be tagged hvc1 in fragmented MP4 on Chromium/Windows.
  for i:=range args {
   if args[i]=="-movflags"{
    result:=append([]string{},args[:i]...)
    result=append(result,"-tag:v","hvc1")
    return append(result,args[i:]...)
   }
  }
 }
 return args
}
func audioRemuxArgs(path string,start float64)[]string{
 args:=[]string{"-hide_banner","-nostdin","-loglevel","error"}
 if start>0{
  args=append(args,"-ss",strconv.FormatFloat(start,'f',3,64))
 }
 args=append(args,"-i",path,
  "-map","0:v:0","-map","0:a:0?",
  "-c:v","copy","-c:a","aac","-ac","2","-ar","48000","-b:a","160k",
  "-movflags","+frag_keyframe+empty_moov+default_base_moof",
  "-max_interleave_delta","0","-f","mp4","pipe:1")
 return args
}
func (s *Server) streamAudioCompatible(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet {jsonErr(w,405,"method not allowed");return}
 id,err:=strconv.ParseInt(strings.TrimPrefix(r.URL.Path,"/stream/compat/"),10,64)
 if err!=nil||id<=0{jsonErr(w,400,"invalid media id");return}
 path,found:=s.pathForPlayback(id)
 if !found{http.NotFound(w,r);return}
 ffmpeg:=binaryPath("ffmpeg")
 if ffmpeg==""{jsonErr(w,503,"FFmpeg fehlt auf dem QNAP. Audio-Kompatibilitätsmodus nicht verfügbar.");return}
 start:=0.0
 if value:=r.URL.Query().Get("start");value!=""{
  val,e:=strconv.ParseFloat(value,64)
  if e!=nil||math.IsNaN(val)||math.IsInf(val,0)||val<0||val>172800 {
   jsonErr(w,400,"invalid start time");return
  }
  start=val
 }
 select{
 case audioRemuxSlots<-struct{}{}:defer func(){<-audioRemuxSlots}()
 default:jsonErr(w,429,"Ein Audiostream läuft bereits. Bitte andere Wiedergabe schließen.");return
 }
 // A diagnostic probe can prevent a second unsupported video format from
 // producing a confusing silent stream. We do not block if ffprobe is absent.
 codec:=""
 if binaryPath("ffprobe")!=""{
  media:=inspectPlayback(path)
  codec=media.VideoCodec
  if !audioCompatibleVideo(codec){
   jsonErr(w,415,"Video-Codec "+codec+" kann nicht ausschließlich mit einer Audio-Reparatur umgewandelt werden.");return
  }
 }
 ctx,cancel:=context.WithCancel(r.Context())
 defer cancel()
 cmd:=exec.CommandContext(ctx,ffmpeg,audioRemuxArgsForCodec(path,start,codec)...)
 var stderr bytes.Buffer
 cmd.Stderr=&stderr
 stdout,err:=cmd.StdoutPipe()
 if err!=nil{jsonErr(w,500,"Audio-Stream konnte nicht geöffnet werden.");return}
 if err:=cmd.Start();err!=nil{jsonErr(w,500,"FFmpeg konnte nicht gestartet werden: "+err.Error());return}
 w.Header().Set("Content-Type","video/mp4")
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("X-Accel-Buffering","no")
 w.Header().Set("Accept-Ranges","none")
 _,copyErr:=io.Copy(w,stdout)
 cancel()
 waitErr:=cmd.Wait()
 if waitErr!=nil&&r.Context().Err()==nil{
  log.Printf("BuddyFlix audio stream id=%d: %v: %s",id,waitErr,stderr.String())
 }
 if copyErr!=nil&&r.Context().Err()==nil{
  log.Printf("BuddyFlix audio stream connection id=%d: %v",id,copyErr)
 }
}

// Only called during diagnostics, never by an ordinary /stream request.
func (s *Server) ffmpegInfo(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{jsonErr(w,405,"method not allowed");return}
 jsonOut(w,map[string]any{
  "ffmpeg_available":binaryPath("ffmpeg")!="",
  "ffprobe_available":binaryPath("ffprobe")!="",
  "hint":fmt.Sprintf("Installiere eine ARMHF-kompatible FFmpeg-Version auf dem QNAP oder konfiguriere BUDDYFLIX_FFMPEG. Aktueller Suchpfad: %s",os.Getenv("PATH")),
 })
}
