package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Version = "0.1.0"

type Config struct{ ListenAddr, DataDir, AdminUser, AdminPassword, TMDBAPIKey string }

func ConfigFromEnv() Config {
	return Config{env("BUDDYFLIX_LISTEN", ":8096"), env("BUDDYFLIX_DATA", "./data"), env("BUDDYFLIX_ADMIN_USER", "admin"), env("BUDDYFLIX_ADMIN_PASSWORD", "buddyflix"), os.Getenv("TMDB_API_KEY")}
}
func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}

type Library struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Updated string `json:"updated"`
}
type Media struct {
	ID        int64   `json:"id"`
	LibraryID int64   `json:"library_id"`
	Path      string  `json:"path"`
	Title     string  `json:"title"`
	Year      int     `json:"year"`
	Overview  string  `json:"overview"`
	Poster    string  `json:"poster"`
	Backdrop  string  `json:"backdrop"`
	Runtime   int     `json:"runtime"`
	Added     string  `json:"added"`
	MTime     int64   `json:"mtime"`
	Size      int64   `json:"size"`
	Progress  float64 `json:"progress"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
}
type Progress struct {
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Updated  string  `json:"updated"`
}
type Store struct {
	NextLibraryID int64              `json:"next_library_id"`
	NextMediaID   int64              `json:"next_media_id"`
	Libraries     []Library          `json:"libraries"`
	Media         []Media            `json:"media"`
	Progress      map[int64]Progress `json:"progress"`
}

type Server struct {
	cfg         Config
	mux         *http.ServeMux
	started     time.Time
	mu          sync.RWMutex
	st          Store
	sessions    map[string]time.Time
	scanMu      sync.Mutex
	scanRunning bool
}

func New(cfg Config) (*Server, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux(), started: time.Now(), sessions: map[string]time.Time{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}
func (s *Server) dbPath() string { return filepath.Join(s.cfg.DataDir, "buddyflix.json") }
func (s *Server) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st = Store{NextLibraryID: 1, NextMediaID: 1, Progress: map[int64]Progress{}}
	b, err := os.ReadFile(s.dbPath())
	if os.IsNotExist(err) {
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &s.st); err != nil {
		return err
	}
	if s.st.Progress == nil {
		s.st.Progress = map[int64]Progress{}
	}
	if s.st.NextLibraryID < 1 {
		s.st.NextLibraryID = 1
	}
	if s.st.NextMediaID < 1 {
		s.st.NextMediaID = 1
	}
	return nil
}
func (s *Server) saveLocked() error {
	b, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.dbPath() + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath())
}
func (s *Server) routes() {
	s.mux.HandleFunc("/api/login", s.login)
	s.mux.HandleFunc("/api/logout", s.auth(s.logout))
	s.mux.HandleFunc("/api/health", s.health)
	s.mux.HandleFunc("/api/system", s.auth(s.system))
	s.mux.HandleFunc("/api/libraries", s.auth(s.libraries))
	s.mux.HandleFunc("/api/scan", s.auth(s.scan))
	s.mux.HandleFunc("/api/media", s.auth(s.media))
	s.mux.HandleFunc("/api/progress", s.auth(s.progress))
	s.mux.HandleFunc("/api/metadata/", s.auth(s.metadata))
	s.mux.HandleFunc("/stream/", s.auth(s.stream))
	s.mux.HandleFunc("/", s.static)
}
func (s *Server) Run() error {
	return http.ListenAndServe(s.cfg.ListenAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		s.mux.ServeHTTP(w, r)
	}))
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func jsonErr(w http.ResponseWriter, c int, m string) {
	w.WriteHeader(c)
	jsonOut(w, map[string]string{"error": m})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	var x struct{ Username, Password string }
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	a := sha256.Sum256([]byte(x.Username + "\x00" + x.Password))
	b := sha256.Sum256([]byte(s.cfg.AdminUser + "\x00" + s.cfg.AdminPassword))
	if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		jsonErr(w, 401, "invalid credentials")
		return
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	t := hex.EncodeToString(raw)
	s.mu.Lock()
	s.sessions[t] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "buddyflix_session", Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	jsonOut(w, map[string]any{"ok": true, "user": s.cfg.AdminUser})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("buddyflix_session"); e == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "buddyflix_session", Path: "/", MaxAge: -1, HttpOnly: true})
	jsonOut(w, map[string]bool{"ok": true})
}
func (s *Server) auth(n http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("buddyflix_session")
		if e != nil {
			jsonErr(w, 401, "unauthorized")
			return
		}
		s.mu.Lock()
		exp, ok := s.sessions[c.Value]
		if ok && time.Now().After(exp) {
			delete(s.sessions, c.Value)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			jsonErr(w, 401, "unauthorized")
			return
		}
		n(w, r)
	}
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"ok": true, "version": Version})
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.mu.RLock()
	mc, lc := len(s.st.Media), len(s.st.Libraries)
	s.mu.RUnlock()
	s.scanMu.Lock()
	sc := s.scanRunning
	s.scanMu.Unlock()
	jsonOut(w, map[string]any{"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "memory_mb": m.Alloc / 1024 / 1024, "uptime_sec": int(time.Since(s.started).Seconds()), "media": mc, "libraries": lc, "scanning": sc, "tmdb": s.cfg.TMDBAPIKey != "", "storage": "embedded-json-v1"})
}
func (s *Server) libraries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.mu.RLock()
		x := append([]Library(nil), s.st.Libraries...)
		s.mu.RUnlock()
		jsonOut(w, x)
	case "POST":
		var x Library
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
			jsonErr(w, 400, "invalid json")
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Path = filepath.Clean(strings.TrimSpace(x.Path))
		if x.Type == "" {
			x.Type = "movies"
		}
		if x.Name == "" || x.Path == "" {
			jsonErr(w, 400, "name and path required")
			return
		}
		st, e := os.Stat(x.Path)
		if e != nil || !st.IsDir() {
			jsonErr(w, 400, "path does not exist or is not a directory")
			return
		}
		s.mu.Lock()
		for _, l := range s.st.Libraries {
			if l.Path == x.Path {
				s.mu.Unlock()
				jsonErr(w, 400, "library path already exists")
				return
			}
		}
		x.ID = s.st.NextLibraryID
		s.st.NextLibraryID++
		x.Updated = time.Now().Format(time.RFC3339)
		s.st.Libraries = append(s.st.Libraries, x)
		e = s.saveLocked()
		s.mu.Unlock()
		if e != nil {
			jsonErr(w, 500, e.Error())
			return
		}
		jsonOut(w, map[string]any{"ok": true, "id": x.ID})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		s.mu.Lock()
		libs := s.st.Libraries[:0]
		for _, l := range s.st.Libraries {
			if l.ID != id {
				libs = append(libs, l)
			}
		}
		s.st.Libraries = libs
		med := s.st.Media[:0]
		for _, m := range s.st.Media {
			if m.LibraryID != id {
				med = append(med, m)
			} else {
				delete(s.st.Progress, m.ID)
			}
		}
		s.st.Media = med
		e := s.saveLocked()
		s.mu.Unlock()
		if e != nil {
			jsonErr(w, 500, e.Error())
			return
		}
		jsonOut(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, 405, "method not allowed")
	}
}

var videoExt = map[string]bool{".mp4": true, ".mkv": true, ".m4v": true, ".mov": true, ".avi": true, ".webm": true, ".ts": true, ".m2ts": true}
var yearRx = regexp.MustCompile(`(?i)[\(\[\. _-](19\d{2}|20\d{2})[\)\]\. _-]`)

func titleFromPath(p string) (string, int) {
	n := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	n = strings.ReplaceAll(strings.ReplaceAll(n, ".", " "), "_", " ")
	y := 0
	m := yearRx.FindStringSubmatch(" " + n + " ")
	if len(m) > 1 {
		y, _ = strconv.Atoi(m[1])
		if i := strings.Index(n, m[1]); i > 0 {
			n = n[:i]
		}
	}
	return strings.TrimSpace(strings.Join(strings.Fields(n), " ")), y
}
func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	s.scanMu.Lock()
	if s.scanRunning {
		s.scanMu.Unlock()
		jsonErr(w, 409, "scan already running")
		return
	}
	s.scanRunning = true
	s.scanMu.Unlock()
	defer func() { s.scanMu.Lock(); s.scanRunning = false; s.scanMu.Unlock() }()
	s.mu.RLock()
	libs := append([]Library(nil), s.st.Libraries...)
	s.mu.RUnlock()
	seen, updated := 0, 0
	for _, lib := range libs {
		_ = filepath.WalkDir(lib.Path, func(path string, d fs.DirEntry, e error) error {
			if e != nil || d.IsDir() || !videoExt[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			seen++
			info, e := d.Info()
			if e != nil {
				return nil
			}
			title, year := titleFromPath(path)
			s.mu.Lock()
			found := -1
			for i := range s.st.Media {
				if s.st.Media[i].Path == path {
					found = i
					break
				}
			}
			if found >= 0 {
				s.st.Media[found].LibraryID = lib.ID
				s.st.Media[found].Title = title
				s.st.Media[found].Year = year
				s.st.Media[found].MTime = info.ModTime().Unix()
				s.st.Media[found].Size = info.Size()
			} else {
				s.st.Media = append(s.st.Media, Media{ID: s.st.NextMediaID, LibraryID: lib.ID, Path: path, Title: title, Year: year, Added: time.Now().Format(time.RFC3339), MTime: info.ModTime().Unix(), Size: info.Size()})
				s.st.NextMediaID++
			}
			updated++
			s.mu.Unlock()
			return nil
		})
		s.mu.Lock()
		for i := range s.st.Libraries {
			if s.st.Libraries[i].ID == lib.ID {
				s.st.Libraries[i].Updated = time.Now().Format(time.RFC3339)
			}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	e := s.saveLocked()
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	jsonOut(w, map[string]any{"ok": true, "files_seen": seen, "updated": updated})
}
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	s.mu.RLock()
	out := make([]Media, 0)
	for _, m := range s.st.Media {
		if id > 0 && m.ID != id {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.Title), q) {
			continue
		}
		if p, ok := s.st.Progress[m.ID]; ok {
			m.Position = p.Position
			m.Duration = p.Duration
			if p.Duration > 0 {
				m.Progress = p.Position / p.Duration * 100
			}
		}
		out = append(out, m)
	}
	s.mu.RUnlock()
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > 500 {
		out = out[:500]
	}
	jsonOut(w, out)
}
func (s *Server) progress(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	var x struct {
		MediaID  int64 `json:"media_id"`
		Position float64
		Duration float64
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil || x.MediaID < 1 {
		jsonErr(w, 400, "invalid json")
		return
	}
	s.mu.Lock()
	s.st.Progress[x.MediaID] = Progress{x.Position, x.Duration, time.Now().Format(time.RFC3339)}
	e := s.saveLocked()
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/stream/"), 10, 64)
	s.mu.RLock()
	p := ""
	for _, m := range s.st.Media {
		if m.ID == id {
			p = m.Path
			break
		}
	}
	s.mu.RUnlock()
	if p == "" {
		http.NotFound(w, r)
		return
	}
	f, e := os.Open(p)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(p)))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, filepath.Base(p), st.ModTime(), f)
}
func (s *Server) metadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	if s.cfg.TMDBAPIKey == "" {
		jsonErr(w, 400, "TMDB_API_KEY not configured")
		return
	}
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/metadata/"), 10, 64)
	s.mu.RLock()
	var cur Media
	ok := false
	for _, m := range s.st.Media {
		if m.ID == id {
			cur = m
			ok = true
			break
		}
	}
	s.mu.RUnlock()
	if !ok {
		jsonErr(w, 404, "not found")
		return
	}
	v := url.Values{"api_key": {s.cfg.TMDBAPIKey}, "query": {cur.Title}, "language": {"de-DE"}}
	if cur.Year > 0 {
		v.Set("year", strconv.Itoa(cur.Year))
	}
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/search/movie?"+v.Encode(), nil)
	resp, e := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if e != nil {
		jsonErr(w, 502, e.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		jsonErr(w, 502, "TMDb request failed")
		return
	}
	var raw struct {
		Results []struct {
			Title        string `json:"title"`
			Overview     string `json:"overview"`
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
			ReleaseDate  string `json:"release_date"`
		} `json:"results"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil || len(raw.Results) == 0 {
		jsonErr(w, 404, "no match")
		return
	}
	z := raw.Results[0]
	y := cur.Year
	if len(z.ReleaseDate) >= 4 {
		if yy, e := strconv.Atoi(z.ReleaseDate[:4]); e == nil {
			y = yy
		}
	}
	poster, back := "", ""
	if z.PosterPath != "" {
		poster = "https://image.tmdb.org/t/p/w500" + z.PosterPath
	}
	if z.BackdropPath != "" {
		back = "https://image.tmdb.org/t/p/w1280" + z.BackdropPath
	}
	s.mu.Lock()
	for i := range s.st.Media {
		if s.st.Media[i].ID == id {
			s.st.Media[i].Title = z.Title
			s.st.Media[i].Year = y
			s.st.Media[i].Overview = z.Overview
			s.st.Media[i].Poster = poster
			s.st.Media[i].Backdrop = back
		}
	}
	e = s.saveLocked()
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
	if p == "." || p == "" {
		p = "index.html"
	}
	roots := []string{"web", filepath.Join(filepath.Dir(os.Args[0]), "web")}
	for _, root := range roots {
		b, e := os.ReadFile(filepath.Join(root, p))
		if e == nil {
			if ct := mime.TypeByExtension(filepath.Ext(p)); ct != "" {
				w.Header().Set("Content-Type", ct)
			}
			_, _ = w.Write(b)
			return
		}
	}
	if p != "index.html" {
		for _, root := range roots {
			if b, e := os.ReadFile(filepath.Join(root, "index.html")); e == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(b)
				return
			}
		}
	}
	http.NotFound(w, r)
}
