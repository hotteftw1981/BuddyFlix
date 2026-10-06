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
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var Version = "0.1.1-dev"

// BuiltinTMDBAPIKey stays empty in source. Official builds can inject the
// BuddyFlix project credential at link time. Local overrides still win.
var BuiltinTMDBAPIKey = ""

var BuiltinTVDBAPIKey = ""

type Config struct{ ListenAddr, DataDir, AdminUser, AdminPassword, TMDBAPIKey, TVDBAPIKey string }

func ConfigFromEnv() Config {
	return Config{
		env("BUDDYFLIX_LISTEN", ":8096"),
		env("BUDDYFLIX_DATA", "./data"),
		env("BUDDYFLIX_ADMIN_USER", "admin"),
		env("BUDDYFLIX_ADMIN_PASSWORD", "buddyflix"),
		os.Getenv("TMDB_API_KEY"),
		os.Getenv("TVDB_API_KEY"),
	}
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
	ID                 int64              `json:"id"`
	MetadataLocked     bool               `json:"metadata_locked"`
	MetadataState      string             `json:"metadata_state,omitempty"`
	MetadataConfidence int                `json:"metadata_confidence,omitempty"`
	PendingMetadata    *MetadataCandidate `json:"pending_metadata,omitempty"`
	MetadataProvider   string             `json:"metadata_provider,omitempty"`
	ExternalIDs        map[string]string  `json:"external_ids,omitempty"`
	MetadataSources    map[string]string  `json:"metadata_sources,omitempty"`
	MetadataUpdated    string             `json:"metadata_updated,omitempty"`
	Missing            bool              `json:"missing"`
	LibraryID          int64             `json:"library_id"`
	Path               string            `json:"path"`
	Title              string            `json:"title"`
	Year               int               `json:"year"`
	Overview           string            `json:"overview"`
	Poster             string            `json:"poster"`
	Backdrop           string            `json:"backdrop"`
	Runtime            int               `json:"runtime"`
	Added              string            `json:"added"`
	MTime              int64             `json:"mtime"`
	Size               int64             `json:"size"`
	Progress           float64           `json:"progress"`
	Position           float64           `json:"position"`
	Duration           float64           `json:"duration"`
}
type Progress struct {
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Updated  string  `json:"updated"`
}

type MetadataJob struct {
	Running      bool   `json:"running"`
	Total        int    `json:"total"`
	Done         int    `json:"done"`
	AutoMatched  int    `json:"auto_matched"`
	Review       int    `json:"review"`
	NoMatch      int    `json:"no_match"`
	Errors       int    `json:"errors"`
	Skipped      int    `json:"skipped"`
	CurrentTitle string `json:"current_title,omitempty"`
	Started      string `json:"started,omitempty"`
	Finished     string `json:"finished,omitempty"`
	Error        string `json:"error,omitempty"`
}
type Settings struct {
	SetupDone      bool   `json:"setup_done"`
	ServerName     string `json:"server_name"`
	AdminUser      string `json:"admin_user"`
	AdminPassHash  string `json:"admin_pass_hash"`
	TMDBAPIKey     string `json:"tmdb_api_key,omitempty"`
	TVDBAPIKey     string `json:"tvdb_api_key,omitempty"`
}

type Store struct {
	ServerID      string             `json:"server_id"`
	Settings      Settings           `json:"settings"`
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
	metadataMu  sync.Mutex
	metadataJob MetadataJob
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
	s.st.Settings.ServerName = "BuddyFlix"
	s.st.Settings.AdminUser = s.cfg.AdminUser
	h := sha256.Sum256([]byte(s.cfg.AdminPassword))
	s.st.Settings.AdminPassHash = hex.EncodeToString(h[:])
	if s.st.ServerID == "" {
		raw := make([]byte, 12)
		_, _ = rand.Read(raw)
		s.st.ServerID = hex.EncodeToString(raw)
	}
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
	if s.st.ServerID == "" {
		raw := make([]byte, 12)
		_, _ = rand.Read(raw)
		s.st.ServerID = hex.EncodeToString(raw)
		if err := s.saveLocked(); err != nil { return err }
	}
	if s.st.Settings.ServerName == "" {
		s.st.Settings.ServerName = "BuddyFlix"
	}
	if s.st.Settings.AdminUser == "" {
		s.st.Settings.AdminUser = s.cfg.AdminUser
	}
	if s.st.Settings.AdminPassHash == "" {
		h := sha256.Sum256([]byte(s.cfg.AdminPassword))
		s.st.Settings.AdminPassHash = hex.EncodeToString(h[:])
		if err := s.saveLocked(); err != nil { return err }
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
	s.mux.HandleFunc("/api/setup/status", s.setupStatus)
	s.mux.HandleFunc("/api/setup", s.setup)
	s.mux.HandleFunc("/api/info", s.info)
	s.mux.HandleFunc("/api/v1/info", s.info)
	s.mux.HandleFunc("/api/login", s.login)
	s.mux.HandleFunc("/api/logout", s.auth(s.logout))
	s.mux.HandleFunc("/api/health", s.health)
	s.mux.HandleFunc("/api/system", s.auth(s.system))
	s.mux.HandleFunc("/api/settings", s.auth(s.settings))
	s.mux.HandleFunc("/api/password", s.auth(s.password))
	s.mux.HandleFunc("/api/libraries", s.auth(s.libraries))
	s.mux.HandleFunc("/api/scan", s.auth(s.scan))
	s.mux.HandleFunc("/api/media", s.auth(s.media))
	s.mux.HandleFunc("/api/media/action", s.auth(s.mediaAction))
	s.mux.HandleFunc("/api/media/cleanup", s.auth(s.mediaCleanup))
	s.mux.HandleFunc("/api/progress", s.auth(s.progress))
	s.mux.HandleFunc("/api/metadata/search", s.auth(s.metadataSearch))
	s.mux.HandleFunc("/api/metadata/apply", s.auth(s.metadataApply))
	s.mux.HandleFunc("/api/metadata/bulk", s.auth(s.metadataBulk))
	s.mux.HandleFunc("/api/metadata/providers", s.auth(s.metadataProviders))
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
	s.mu.RLock()
	adminUser := s.st.Settings.AdminUser
	adminPassHash := s.st.Settings.AdminPassHash
	s.mu.RUnlock()
	a := sha256.Sum256([]byte(x.Password))
	b, _ := hex.DecodeString(adminPassHash)
	userOK := subtle.ConstantTimeCompare([]byte(x.Username), []byte(adminUser)) == 1
	passOK := len(b) == len(a) && subtle.ConstantTimeCompare(a[:], b) == 1
	if !userOK || !passOK {
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
	jsonOut(w, map[string]any{"ok": true, "user": adminUser})
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
func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	s.mu.RLock()
	done := s.st.Settings.SetupDone
	name := s.st.Settings.ServerName
	s.mu.RUnlock()
	jsonOut(w, map[string]any{"setup_done": done, "server_name": name, "version": Version})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	s.mu.RLock()
	done := s.st.Settings.SetupDone
	s.mu.RUnlock()
	if done {
		jsonErr(w, 409, "setup already completed")
		return
	}
	var x struct {
		ServerName string `json:"server_name"`
		AdminUser  string `json:"admin_user"`
		Password   string `json:"password"`
		TMDBAPIKey string `json:"tmdb_api_key"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	x.ServerName = strings.TrimSpace(x.ServerName)
	x.AdminUser = strings.TrimSpace(x.AdminUser)
	x.TMDBAPIKey = strings.TrimSpace(x.TMDBAPIKey)
	if x.ServerName == "" || x.AdminUser == "" {
		jsonErr(w, 400, "server name and admin user required")
		return
	}
	if len(x.Password) < 8 {
		jsonErr(w, 400, "password must have at least 8 characters")
		return
	}
	h := sha256.Sum256([]byte(x.Password))
	s.mu.Lock()
	s.st.Settings.SetupDone = true
	s.st.Settings.ServerName = x.ServerName
	s.st.Settings.AdminUser = x.AdminUser
	s.st.Settings.AdminPassHash = hex.EncodeToString(h[:])
	s.st.Settings.TMDBAPIKey = x.TMDBAPIKey
	e := s.saveLocked()
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	s.mu.RLock()
	id := s.st.ServerID
	name := s.st.Settings.ServerName
	s.mu.RUnlock()
	jsonOut(w, map[string]any{
		"name": name,
		"server_id": id,
		"version": Version,
		"api_version": "1",
		"product": "BuddyFlix Media Server",
		"features": []string{"direct_play", "range_streaming", "progress", "libraries", "movies"},
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"ok": true, "version": Version})
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.mu.RLock()
	mc, missingCount, lc := 0, 0, len(s.st.Libraries)
	for _, item := range s.st.Media {
		if item.Missing {
			missingCount++
		} else {
			mc++
		}
	}
	s.mu.RUnlock()
	s.scanMu.Lock()
	sc := s.scanRunning
	s.scanMu.Unlock()
	s.mu.RLock()
	serverName := s.st.Settings.ServerName
	s.mu.RUnlock()
	jsonOut(w, map[string]any{"version": Version, "server_name": serverName, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "memory_mb": m.Alloc / 1024 / 1024, "uptime_sec": int(time.Since(s.started).Seconds()), "media": mc, "missing_media": missingCount, "libraries": lc, "scanning": sc, "tmdb": s.tmdbKey() != "", "tvdb": s.tvdbKey() != "", "storage": "embedded-json-v1"})
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.mu.RLock()
		_, tmdbSource := s.tmdbCredentialUnlocked()
		_, tvdbSource := s.tvdbCredentialUnlocked()
		out := map[string]any{
			"server_name": s.st.Settings.ServerName,
			"admin_user": s.st.Settings.AdminUser,
			"server_id": s.st.ServerID,
			"data_dir": s.cfg.DataDir,
			"listen": s.cfg.ListenAddr,
			"tmdb_configured": tmdbSource != "missing",
			"tmdb_credential_source": tmdbSource,
			"tmdb_builtin": tmdbSource == "builtin",
			"tmdb_override": strings.TrimSpace(s.st.Settings.TMDBAPIKey) != "",
			"tvdb_configured": tvdbSource != "missing",
			"tvdb_credential_source": tvdbSource,
			"tvdb_builtin": tvdbSource == "builtin",
			"tvdb_override": strings.TrimSpace(s.st.Settings.TVDBAPIKey) != "",
		}
		s.mu.RUnlock()
		jsonOut(w, out)
	case "PUT":
		var x struct {
			ServerName string `json:"server_name"`
			AdminUser  string `json:"admin_user"`
			TMDBAPIKey *string `json:"tmdb_api_key"`
			TVDBAPIKey *string `json:"tvdb_api_key"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
			jsonErr(w, 400, "invalid json")
			return
		}
		x.ServerName = strings.TrimSpace(x.ServerName)
		x.AdminUser = strings.TrimSpace(x.AdminUser)
		if x.ServerName == "" || x.AdminUser == "" {
			jsonErr(w, 400, "server_name and admin_user required")
			return
		}
		s.mu.Lock()
		s.st.Settings.ServerName = x.ServerName
		s.st.Settings.AdminUser = x.AdminUser
		if x.TMDBAPIKey != nil {
			s.st.Settings.TMDBAPIKey = strings.TrimSpace(*x.TMDBAPIKey)
		}
		if x.TVDBAPIKey != nil {
			s.st.Settings.TVDBAPIKey = strings.TrimSpace(*x.TVDBAPIKey)
		}
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

func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	var x struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	if len(x.New) < 8 {
		jsonErr(w, 400, "new password must have at least 8 characters")
		return
	}
	cur := sha256.Sum256([]byte(x.Current))
	s.mu.RLock()
	stored, _ := hex.DecodeString(s.st.Settings.AdminPassHash)
	s.mu.RUnlock()
	if len(stored) != len(cur) || subtle.ConstantTimeCompare(cur[:], stored) != 1 {
		jsonErr(w, 403, "current password is wrong")
		return
	}
	next := sha256.Sum256([]byte(x.New))
	s.mu.Lock()
	s.st.Settings.AdminPassHash = hex.EncodeToString(next[:])
	e := s.saveLocked()
	s.sessions = map[string]time.Time{}
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "buddyflix_session", Path: "/", MaxAge: -1, HttpOnly: true})
	jsonOut(w, map[string]bool{"ok": true})
}

func (s *Server) libraries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.mu.RLock()
		x := append([]Library{}, s.st.Libraries...)
		s.mu.RUnlock()
		jsonOut(w, x)
	case "POST", "PUT":
		var x Library
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
			jsonErr(w, 400, "invalid json")
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Path = filepath.Clean(strings.TrimSpace(x.Path))
		if r.Method == "PUT" {
			x.ID, _ = strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		}
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
			if l.Path == x.Path && l.ID != x.ID {
				s.mu.Unlock()
				jsonErr(w, 400, "library path already exists")
				return
			}
		}
		x.Updated = time.Now().Format(time.RFC3339)
		if r.Method == "PUT" {
			found := false
			for i := range s.st.Libraries {
				if s.st.Libraries[i].ID == x.ID {
					s.st.Libraries[i] = x
					found = true
					break
				}
			}
			if !found {
				s.mu.Unlock()
				jsonErr(w, 404, "library not found")
				return
			}
		} else {
			x.ID = s.st.NextLibraryID
			s.st.NextLibraryID++
			s.st.Libraries = append(s.st.Libraries, x)
		}
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

	type scannedFile struct {
		path string
		info fs.FileInfo
	}

	seen, updated, skippedSystemDirs := 0, 0, 0
	skipped := make([]string, 0)

	for _, lib := range libs {
		root, err := os.Stat(lib.Path)
		if err != nil || !root.IsDir() {
			skipped = append(skipped, lib.Name)
			continue
		}

		files := make([]scannedFile, 0)
		walkFailed := false
		skippedDirs := 0
		skipDir := func(name string) bool {
			lower := strings.ToLower(name)
			if strings.HasPrefix(name, ".") {
				return true
			}
			switch lower {
			case "@recycle", "@recycle.bin", "@transcode", "@recently-snapshot", "@eadir":
				return true
			}
			return false
		}
		err = filepath.WalkDir(lib.Path, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				walkFailed = true
				return nil
			}
			if d.IsDir() {
				if path != lib.Path && skipDir(d.Name()) {
					skippedDirs++
					return filepath.SkipDir
				}
				return nil
			}
			if !videoExt[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				walkFailed = true
				return nil
			}
			files = append(files, scannedFile{path: path, info: info})
			return nil
		})
		if err != nil {
			walkFailed = true
		}
		skippedSystemDirs += skippedDirs

		foundPaths := make(map[string]struct{}, len(files))
		s.mu.Lock()
		for _, file := range files {
			seen++
			foundPaths[file.path] = struct{}{}
			title, year := titleFromPath(file.path)
			found := -1
			for i := range s.st.Media {
				if s.st.Media[i].Path == file.path {
					found = i
					break
				}
			}
			if found >= 0 {
				s.st.Media[found].LibraryID = lib.ID
				s.st.Media[found].Missing = false
				if strings.TrimSpace(s.st.Media[found].Title) == "" {
					s.st.Media[found].Title = title
				}
				if s.st.Media[found].Year == 0 {
					s.st.Media[found].Year = year
				}
				s.st.Media[found].MTime = file.info.ModTime().Unix()
				s.st.Media[found].Size = file.info.Size()
			} else {
				s.st.Media = append(s.st.Media, Media{
					ID: s.st.NextMediaID, LibraryID: lib.ID, Path: file.path,
					Title: title, Year: year, Added: time.Now().Format(time.RFC3339),
					MTime: file.info.ModTime().Unix(), Size: file.info.Size(),
				})
				s.st.NextMediaID++
			}
			updated++
		}

		if !walkFailed {
			for i := range s.st.Media {
				if s.st.Media[i].LibraryID != lib.ID {
					continue
				}
				_, exists := foundPaths[s.st.Media[i].Path]
				s.st.Media[i].Missing = !exists
			}
			for i := range s.st.Libraries {
				if s.st.Libraries[i].ID == lib.ID {
					s.st.Libraries[i].Updated = time.Now().Format(time.RFC3339)
				}
			}
		} else {
			skipped = append(skipped, lib.Name)
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

	jsonOut(w, map[string]any{
		"ok": true,
		"files_seen": seen,
		"updated": updated,
		"skipped_system_dirs": skippedSystemDirs,
		"skipped_libraries": skipped,
	})
}
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if r.Method == "PUT" {
		if id < 1 {
			jsonErr(w, 400, "id required")
			return
		}
		var x struct {
			Title    string `json:"title"`
			Year     int    `json:"year"`
			Overview string `json:"overview"`
			Poster   string `json:"poster"`
			Backdrop string `json:"backdrop"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
			jsonErr(w, 400, "invalid json")
			return
		}
		x.Title = strings.TrimSpace(x.Title)
		if x.Title == "" {
			jsonErr(w, 400, "title required")
			return
		}
		s.mu.Lock()
		found := false
		for i := range s.st.Media {
			if s.st.Media[i].ID == id {
				s.st.Media[i].Title = x.Title
				s.st.Media[i].Year = x.Year
				s.st.Media[i].Overview = strings.TrimSpace(x.Overview)
				s.st.Media[i].Poster = strings.TrimSpace(x.Poster)
				s.st.Media[i].Backdrop = strings.TrimSpace(x.Backdrop)
				s.st.Media[i].PendingMetadata = nil
				if s.st.Media[i].Overview != "" || s.st.Media[i].Poster != "" || s.st.Media[i].Backdrop != "" {
					s.st.Media[i].MetadataState = "manual"
					s.st.Media[i].MetadataConfidence = 100
					s.st.Media[i].MetadataProvider = "manual"
					s.st.Media[i].MetadataUpdated = time.Now().Format(time.RFC3339)
					if s.st.Media[i].MetadataSources == nil { s.st.Media[i].MetadataSources = map[string]string{} }
					s.st.Media[i].MetadataSources["title"] = "manual"
					s.st.Media[i].MetadataSources["year"] = "manual"
					s.st.Media[i].MetadataSources["overview"] = "manual"
					s.st.Media[i].MetadataSources["poster"] = "manual"
					s.st.Media[i].MetadataSources["backdrop"] = "manual"
				} else {
					s.st.Media[i].MetadataState = ""
					s.st.Media[i].MetadataConfidence = 0
					s.st.Media[i].MetadataProvider = ""
				}
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			jsonErr(w, 404, "media not found")
			return
		}
		e := s.saveLocked()
		s.mu.Unlock()
		if e != nil {
			jsonErr(w, 500, e.Error())
			return
		}
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	includeMissing := r.URL.Query().Get("include_missing") == "1"
	s.mu.RLock()
	out := make([]Media, 0)
	for _, m := range s.st.Media {
		if id > 0 && m.ID != id {
			continue
		}
		if !includeMissing && m.Missing {
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
	jsonOut(w, out)
}

func (s *Server) mediaAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	var x struct { MediaID int64 `json:"media_id"`; Action string `json:"action"` }
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil || x.MediaID < 1 { jsonErr(w, 400, "invalid json"); return }
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.st.Media {
		if s.st.Media[i].ID != x.MediaID { continue }
		switch x.Action {
		case "reset_progress", "mark_unwatched":
			delete(s.st.Progress, x.MediaID)
		case "mark_watched":
			s.st.Progress[x.MediaID] = Progress{Position: 1, Duration: 1, Updated: time.Now().Format(time.RFC3339)}
		case "lock_metadata":
			s.st.Media[i].MetadataLocked = true
		case "unlock_metadata":
			s.st.Media[i].MetadataLocked = false
		default:
			jsonErr(w, 400, "unknown action"); return
		}
		if err := s.saveLocked(); err != nil { jsonErr(w, 500, err.Error()); return }
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	jsonErr(w, 404, "media not found")
}

func (s *Server) mediaCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	s.mu.Lock()
	kept := s.st.Media[:0]
	removed := 0
	for _, m := range s.st.Media {
		if m.Missing { delete(s.st.Progress, m.ID); removed++; continue }
		kept = append(kept, m)
	}
	s.st.Media = kept
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil { jsonErr(w, 500, err.Error()); return }
	jsonOut(w, map[string]any{"ok": true, "removed": removed})
}

func (s *Server) metadataSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" { jsonErr(w, 405, "method not allowed"); return }
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if q == "" { jsonErr(w, 400, "query required"); return }
	engine := s.metadataEngine()
	if !engine.Available() { jsonErr(w, 400, "no metadata provider configured"); return }
	out, err := engine.SearchMovie(q, year)
	if err != nil { jsonErr(w, 502, err.Error()); return }
	jsonOut(w, out)
}

func (s *Server) metadataProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" { jsonErr(w, 405, "method not allowed"); return }
	engine := s.metadataEngine()
	status := engine.Status()
	_, tmdbSource := s.tmdbCredential()
	_, tvdbSource := s.tvdbCredential()
	for i := range status {
		if status[i].Name == "tmdb" { status[i].CredentialSource = tmdbSource }
		if status[i].Name == "thetvdb" { status[i].CredentialSource = tvdbSource }
	}
	jsonOut(w, status)
}

func mediaHasMetadata(m Media) bool {
	return strings.TrimSpace(m.Overview) != "" || strings.TrimSpace(m.Poster) != "" || strings.TrimSpace(m.Backdrop) != ""
}

func applyMetadataResult(m *Media, result MetadataCandidate, state string, confidence int) {
	if strings.TrimSpace(result.Title) != "" { m.Title = strings.TrimSpace(result.Title) }
	if result.Year > 0 { m.Year = result.Year }
	m.Overview = strings.TrimSpace(result.Overview)
	m.Poster = strings.TrimSpace(result.Poster)
	m.Backdrop = strings.TrimSpace(result.Backdrop)
	m.PendingMetadata = nil
	m.MetadataState = state
	m.MetadataConfidence = confidence
	m.MetadataProvider = result.Provider
	m.MetadataUpdated = time.Now().Format(time.RFC3339)
	if len(result.ExternalIDs) > 0 {
		if m.ExternalIDs == nil { m.ExternalIDs = map[string]string{} }
		for k, v := range result.ExternalIDs {
			if strings.TrimSpace(v) != "" { m.ExternalIDs[k] = strings.TrimSpace(v) }
		}
	}
	if m.MetadataSources == nil { m.MetadataSources = map[string]string{} }
	source := result.Provider
	if source == "" { source = "unknown" }
	for _, field := range []string{"title","year","overview","poster","backdrop"} { m.MetadataSources[field] = source }
}

func (s *Server) metadataJobSnapshot() MetadataJob {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	return s.metadataJob
}

func (s *Server) metadataBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, s.metadataJobSnapshot())
		return
	}
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	engine := s.metadataEngine()
	if !engine.Available() { jsonErr(w, 400, "no metadata provider configured"); return }

	s.metadataMu.Lock()
	if s.metadataJob.Running {
		s.metadataMu.Unlock()
		jsonErr(w, 409, "metadata scan already running")
		return
	}
	s.mu.RLock()
	ids := make([]int64, 0)
	for _, m := range s.st.Media {
		if m.Missing || m.MetadataLocked || mediaHasMetadata(m) { continue }
		ids = append(ids, m.ID)
	}
	s.mu.RUnlock()
	s.metadataJob = MetadataJob{Running: len(ids) > 0, Total: len(ids), Started: time.Now().Format(time.RFC3339)}
	if len(ids) == 0 { s.metadataJob.Finished = time.Now().Format(time.RFC3339) }
	job := s.metadataJob
	s.metadataMu.Unlock()
	if len(ids) > 0 { go s.runMetadataBulk(ids, engine) }
	jsonOut(w, job)
}

func (s *Server) runMetadataBulk(ids []int64, engine MetadataEngine) {
	consecutiveErrors := 0
	for n, id := range ids {
		s.mu.RLock()
		var item Media
		found := false
		for _, m := range s.st.Media {
			if m.ID == id { item = m; found = true; break }
		}
		s.mu.RUnlock()
		if !found || item.Missing || item.MetadataLocked || mediaHasMetadata(item) {
			s.metadataMu.Lock()
			s.metadataJob.Done++
			s.metadataJob.Skipped++
			s.metadataMu.Unlock()
			continue
		}

		s.metadataMu.Lock()
		s.metadataJob.CurrentTitle = item.Title
		s.metadataMu.Unlock()

		results, err := engine.SearchMovie(item.Title, item.Year)
		if err != nil {
			consecutiveErrors++
			s.mu.Lock()
			for i := range s.st.Media {
				if s.st.Media[i].ID == id {
					s.st.Media[i].MetadataState = "error"
					s.st.Media[i].PendingMetadata = nil
					s.st.Media[i].MetadataConfidence = 0
					break
				}
			}
			s.mu.Unlock()
			s.metadataMu.Lock()
			s.metadataJob.Done++
			s.metadataJob.Errors++
			s.metadataMu.Unlock()
			if consecutiveErrors >= 5 {
				s.metadataMu.Lock()
				s.metadataJob.Error = "Metadatenanbieter sind momentan nicht zuverlässig erreichbar. Lauf wurde nach mehreren Fehlern gestoppt."
				s.metadataMu.Unlock()
				break
			}
			time.Sleep(650 * time.Millisecond)
			continue
		}
		consecutiveErrors = 0

		candidate, confidence, auto, hasCandidate := chooseMetadataMatch(item.Title, item.Year, results)

		// Second-stage resolver: only spend extra API calls on ambiguous results.
		// TheTVDB's search endpoint does not always include German translations,
		// so fetch them for the top candidates before asking the user to review.
		if !auto && len(results) > 0 {
			if refined, refineErr := engine.RefineMovieCandidates(results); refineErr == nil {
				results = refined
				candidate, confidence, auto, hasCandidate = chooseMetadataMatch(item.Title, item.Year, results)
			}
		}

		resultKind := "no_match"
		if auto {
			if enriched, enrichErr := engine.EnrichMovie(candidate); enrichErr == nil { candidate = enriched }
		}
		s.mu.Lock()
		for i := range s.st.Media {
			m := &s.st.Media[i]
			if m.ID != id { continue }
			if m.Missing || m.MetadataLocked || mediaHasMetadata(*m) {
				resultKind = "skipped"
				break
			}
			if auto {
				applyMetadataResult(m, candidate, "auto", confidence)
				resultKind = "auto"
			} else if hasCandidate {
				copyCandidate := candidate
				m.PendingMetadata = &copyCandidate
				m.MetadataState = "review"
				m.MetadataConfidence = confidence
				resultKind = "review"
			} else {
				m.PendingMetadata = nil
				m.MetadataState = "no_match"
				m.MetadataConfidence = 0
			}
			break
		}
		if n%10 == 9 { _ = s.saveLocked() }
		s.mu.Unlock()

		s.metadataMu.Lock()
		s.metadataJob.Done++
		switch resultKind {
		case "auto": s.metadataJob.AutoMatched++
		case "review": s.metadataJob.Review++
		case "no_match": s.metadataJob.NoMatch++
		case "skipped": s.metadataJob.Skipped++
		}
		s.metadataMu.Unlock()
		time.Sleep(220 * time.Millisecond)
	}

	s.mu.Lock()
	_ = s.saveLocked()
	s.mu.Unlock()
	s.metadataMu.Lock()
	s.metadataJob.Running = false
	s.metadataJob.CurrentTitle = ""
	s.metadataJob.Finished = time.Now().Format(time.RFC3339)
	s.metadataMu.Unlock()
}

func (s *Server) metadataApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	var x struct {
		MediaID int64 `json:"media_id"`
		Result MetadataCandidate `json:"result"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil || x.MediaID < 1 {
		jsonErr(w, 400, "invalid json")
		return
	}
	if strings.TrimSpace(x.Result.Title) == "" {
		jsonErr(w, 400, "result title required")
		return
	}
	engine := s.metadataEngine()
	if enriched, err := engine.EnrichMovie(x.Result); err == nil { x.Result = enriched }
	s.mu.Lock()
	found := false
	for i := range s.st.Media {
		if s.st.Media[i].ID != x.MediaID { continue }
		applyMetadataResult(&s.st.Media[i], x.Result, "matched", 100)
		found = true
		break
	}
	if !found {
		s.mu.Unlock()
		jsonErr(w, 404, "media not found")
		return
	}
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil { jsonErr(w, 500, err.Error()); return }
	jsonOut(w, map[string]bool{"ok": true})
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
func (s *Server) tmdbCredentialUnlocked() (string, string) {
	if key := strings.TrimSpace(s.st.Settings.TMDBAPIKey); key != "" { return key, "override" }
	if key := strings.TrimSpace(s.cfg.TMDBAPIKey); key != "" { return key, "environment" }
	if key := strings.TrimSpace(BuiltinTMDBAPIKey); key != "" { return key, "builtin" }
	return "", "missing"
}

func (s *Server) tmdbCredential() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tmdbCredentialUnlocked()
}

func (s *Server) tmdbKeyUnlocked() string {
	key, _ := s.tmdbCredentialUnlocked()
	return key
}

func (s *Server) tmdbKey() string {
	key, _ := s.tmdbCredential()
	return key
}

func (s *Server) tvdbCredentialUnlocked() (string, string) {
	if key := strings.TrimSpace(s.st.Settings.TVDBAPIKey); key != "" { return key, "override" }
	if key := strings.TrimSpace(s.cfg.TVDBAPIKey); key != "" { return key, "environment" }
	if key := strings.TrimSpace(BuiltinTVDBAPIKey); key != "" { return key, "builtin" }
	return "", "missing"
}

func (s *Server) tvdbCredential() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tvdbCredentialUnlocked()
}

func (s *Server) tvdbKey() string {
	key, _ := s.tvdbCredential()
	return key
}

func (s *Server) metadataEngine() MetadataEngine {
	providers := make([]MetadataProvider, 0, 2)
	if key := s.tvdbKey(); key != "" { providers = append(providers, NewTVDBProvider(key)) }
	if key := s.tmdbKey(); key != "" { providers = append(providers, NewTMDbProvider(key)) }
	return NewMetadataEngine(providers...)
}

// Legacy one-click metadata endpoint kept for older clients. It now uses the provider engine.
func (s *Server) metadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/metadata/"), 10, 64)
	s.mu.RLock()
	var cur Media
	ok := false
	for _, m := range s.st.Media {
		if m.ID == id { cur = m; ok = true; break }
	}
	s.mu.RUnlock()
	if !ok { jsonErr(w, 404, "not found"); return }
	engine := s.metadataEngine()
	if !engine.Available() { jsonErr(w, 400, "no metadata provider configured"); return }
	results, err := engine.SearchMovie(cur.Title, cur.Year)
	if err != nil { jsonErr(w, 502, err.Error()); return }
	candidate, confidence, _, hasCandidate := chooseMetadataMatch(cur.Title, cur.Year, results)
	if !hasCandidate { jsonErr(w, 404, "no match"); return }
	if enriched, enrichErr := engine.EnrichMovie(candidate); enrichErr == nil { candidate = enriched }
	s.mu.Lock()
	for i := range s.st.Media {
		if s.st.Media[i].ID == id {
			applyMetadataResult(&s.st.Media[i], candidate, "matched", confidence)
			break
		}
	}
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil { jsonErr(w, 500, err.Error()); return }
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
	if p == "index.html" || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
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
