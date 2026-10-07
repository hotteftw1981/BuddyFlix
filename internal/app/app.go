package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"math"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
	ProgressUpdated    string            `json:"progress_updated,omitempty"`
	Favorite           bool              `json:"favorite,omitempty"`
	Kind               string            `json:"kind,omitempty"`
	SeriesTitle        string            `json:"series_title,omitempty"`
	Season             int               `json:"season,omitempty"`
	Episode            int               `json:"episode,omitempty"`
	EpisodeTitle       string            `json:"episode_title,omitempty"`
}
type Progress struct {
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Updated  string  `json:"updated"`
}

type Profile struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	Created string `json:"created"`
}

type SeriesSeason struct {
	Number   int     `json:"number"`
	Episodes []Media `json:"episodes"`
}

type SeriesView struct {
	Key            string         `json:"key"`
	Title          string         `json:"title"`
	SeasonCount    int            `json:"season_count"`
	EpisodeCount   int            `json:"episode_count"`
	WatchedCount   int            `json:"watched_count"`
	ContinueCount  int            `json:"continue_count"`
	Poster         string         `json:"poster,omitempty"`
	Backdrop       string         `json:"backdrop,omitempty"`
	LastActivity   string         `json:"last_activity,omitempty"`
	NextEpisode    *Media         `json:"next_episode,omitempty"`
	Seasons        []SeriesSeason `json:"seasons"`
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

type DeviceToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"token_hash"`
	Created   string `json:"created"`
}

type Store struct {
	ServerID        string                       `json:"server_id"`
	Settings        Settings                     `json:"settings"`
	NextLibraryID   int64                        `json:"next_library_id"`
	NextMediaID     int64                        `json:"next_media_id"`
	NextProfileID   int64                        `json:"next_profile_id"`
	Libraries       []Library                    `json:"libraries"`
	Media           []Media                      `json:"media"`
	Profiles        []Profile                    `json:"profiles,omitempty"`
	Progress        map[int64]Progress           `json:"progress"`
	ProfileProgress  map[int64]map[int64]Progress `json:"profile_progress,omitempty"`
	ProfileFavorites map[int64]map[int64]bool     `json:"profile_favorites,omitempty"`
	DeviceTokens     []DeviceToken                 `json:"device_tokens,omitempty"`
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
	s.st = Store{NextLibraryID: 1, NextMediaID: 1, NextProfileID: 1, Progress: map[int64]Progress{}, ProfileProgress: map[int64]map[int64]Progress{}, ProfileFavorites: map[int64]map[int64]bool{}}
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
		s.st.Profiles = []Profile{{ID: 1, Name: "Hauptprofil", Avatar: "🍿", Created: time.Now().Format(time.RFC3339)}}
		s.st.NextProfileID = 2
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
	if s.st.ProfileProgress == nil {
		s.st.ProfileProgress = map[int64]map[int64]Progress{}
	}
	if s.st.ProfileFavorites == nil {
		s.st.ProfileFavorites = map[int64]map[int64]bool{}
	}
	if len(s.st.Profiles) == 0 {
		s.st.Profiles = []Profile{{ID: 1, Name: "Hauptprofil", Avatar: "🍿", Created: time.Now().Format(time.RFC3339)}}
		s.st.NextProfileID = 2
		if err := s.saveLocked(); err != nil { return err }
	} else if s.st.NextProfileID < 1 {
		var maxID int64
		for _, p := range s.st.Profiles { if p.ID > maxID { maxID = p.ID } }
		s.st.NextProfileID = maxID + 1
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
	if s.st.NextProfileID < 1 {
		s.st.NextProfileID = 1
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
	s.mux.HandleFunc("/api/v1/device/login", s.deviceLogin)
	s.mux.HandleFunc("/api/v1/device/logout", s.auth(s.deviceLogout))
	s.mux.HandleFunc("/api/v1/devices", s.auth(s.devices))
	s.mux.HandleFunc("/api/logout", s.auth(s.logout))
	s.mux.HandleFunc("/api/health", s.health)
	s.mux.HandleFunc("/api/system", s.auth(s.system))
	s.mux.HandleFunc("/api/settings", s.auth(s.settings))
	s.mux.HandleFunc("/api/password", s.auth(s.password))
	s.mux.HandleFunc("/api/profiles", s.auth(s.profiles))
	s.mux.HandleFunc("/api/profiles/select", s.auth(s.selectProfile))
	s.mux.HandleFunc("/api/libraries", s.auth(s.libraries))
	s.mux.HandleFunc("/api/scan", s.auth(s.scan))
	s.mux.HandleFunc("/api/media", s.auth(s.media))
	s.mux.HandleFunc("/api/series", s.auth(s.series))
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
	go s.discoveryLoop()
	return http.ListenAndServe(s.cfg.ListenAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		s.mux.ServeHTTP(w, r)
	}))
}

func (s *Server) discoveryPort() int {
	addr := strings.TrimSpace(s.cfg.ListenAddr)
	if _, port, err := net.SplitHostPort(addr); err == nil {
		if n, convErr := strconv.Atoi(port); convErr == nil && n > 0 && n <= 65535 { return n }
	}
	if strings.HasPrefix(addr, ":") {
		if n, err := strconv.Atoi(strings.TrimPrefix(addr, ":")); err == nil && n > 0 && n <= 65535 { return n }
	}
	return 8096
}

func (s *Server) discoveryLoop() {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 8097})
	if err != nil { return }
	defer conn.Close()
	buf := make([]byte, 512)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil { return }
		if strings.TrimSpace(string(buf[:n])) != "BUDDYFLIX_DISCOVER_V1" { continue }
		s.mu.RLock()
		payload := map[string]any{
			"product": "BuddyFlix Media Server",
			"server_name": s.st.Settings.ServerName,
			"server_id": s.st.ServerID,
			"port": s.discoveryPort(),
		}
		s.mu.RUnlock()
		body, err := json.Marshal(payload)
		if err != nil { continue }
		_, _ = conn.WriteToUDP(body, remote)
	}
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func jsonErr(w http.ResponseWriter, c int, m string) {
	w.WriteHeader(c)
	jsonOut(w, map[string]string{"error": m})
}
func (s *Server) credentialsOK(username, password string) bool {
	s.mu.RLock()
	adminUser := s.st.Settings.AdminUser
	adminPassHash := s.st.Settings.AdminPassHash
	s.mu.RUnlock()
	a := sha256.Sum256([]byte(password))
	b, _ := hex.DecodeString(adminPassHash)
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(adminUser)) == 1
	passOK := len(b) == len(a) && subtle.ConstantTimeCompare(a[:], b) == 1
	return userOK && passOK
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
	if !s.credentialsOK(x.Username, x.Password) {
		jsonErr(w, 401, "invalid credentials")
		return
	}
	s.mu.RLock()
	adminUser := s.st.Settings.AdminUser
	s.mu.RUnlock()
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	t := hex.EncodeToString(raw)
	s.mu.Lock()
	s.sessions[t] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "buddyflix_session", Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	jsonOut(w, map[string]any{"ok": true, "user": adminUser})
}
func (s *Server) deviceLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	var x struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil {
		jsonErr(w, 400, "invalid json")
		return
	}
	x.DeviceName = strings.TrimSpace(x.DeviceName)
	if x.DeviceName == "" { x.DeviceName = "Fire TV" }
	if len([]rune(x.DeviceName)) > 60 { jsonErr(w, 400, "device name too long"); return }
	s.mu.RLock()
	setupDone := s.st.Settings.SetupDone
	s.mu.RUnlock()
	if !setupDone { jsonErr(w, 409, "server setup incomplete"); return }
	if !s.credentialsOK(strings.TrimSpace(x.Username), x.Password) {
		jsonErr(w, 401, "invalid credentials")
		return
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	idRaw := make([]byte, 8)
	_, _ = rand.Read(idRaw)
	device := DeviceToken{
		ID: hex.EncodeToString(idRaw),
		Name: x.DeviceName,
		TokenHash: hex.EncodeToString(hash[:]),
		Created: time.Now().Format(time.RFC3339),
	}
	s.mu.Lock()
	if len(s.st.DeviceTokens) >= 20 {
		s.st.DeviceTokens = append([]DeviceToken(nil), s.st.DeviceTokens[len(s.st.DeviceTokens)-19:]...)
	}
	s.st.DeviceTokens = append(s.st.DeviceTokens, device)
	err := s.saveLocked()
	serverID := s.st.ServerID
	serverName := s.st.Settings.ServerName
	s.mu.Unlock()
	if err != nil { jsonErr(w, 500, err.Error()); return }
	jsonOut(w, map[string]any{
		"ok": true,
		"token": token,
		"device_id": device.ID,
		"server_id": serverID,
		"server_name": serverName,
	})
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 8 || !strings.EqualFold(h[:7], "Bearer ") { return "" }
	return strings.TrimSpace(h[7:])
}

func (s *Server) bearerAuthorized(r *http.Request) bool {
	token := bearerToken(r)
	if token == "" { return false }
	hash := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(hash[:])
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.st.DeviceTokens {
		if subtle.ConstantTimeCompare([]byte(d.TokenHash), []byte(want)) == 1 { return true }
	}
	return false
}

func (s *Server) deviceLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	token := bearerToken(r)
	if token == "" { jsonErr(w, 400, "bearer token required"); return }
	hash := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(hash[:])
	s.mu.Lock()
	next := s.st.DeviceTokens[:0]
	for _, d := range s.st.DeviceTokens {
		if subtle.ConstantTimeCompare([]byte(d.TokenHash), []byte(want)) == 1 { continue }
		next = append(next, d)
	}
	s.st.DeviceTokens = next
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil { jsonErr(w, 500, err.Error()); return }
	jsonOut(w, map[string]bool{"ok": true})
}

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.mu.RLock()
		out := make([]map[string]string, 0, len(s.st.DeviceTokens))
		for _, d := range s.st.DeviceTokens {
			out = append(out, map[string]string{"id": d.ID, "name": d.Name, "created": d.Created})
		}
		s.mu.RUnlock()
		jsonOut(w, out)
	case "DELETE":
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" { jsonErr(w, 400, "id required"); return }
		s.mu.Lock()
		next := s.st.DeviceTokens[:0]
		found := false
		for _, d := range s.st.DeviceTokens {
			if d.ID == id { found = true; continue }
			next = append(next, d)
		}
		if !found { s.mu.Unlock(); jsonErr(w, 404, "device not found"); return }
		s.st.DeviceTokens = next
		err := s.saveLocked()
		s.mu.Unlock()
		if err != nil { jsonErr(w, 500, err.Error()); return }
		jsonOut(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, 405, "method not allowed")
	}
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
		if s.bearerAuthorized(r) {
			n(w, r)
			return
		}
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
		"features": []string{"direct_play", "range_streaming", "progress", "libraries", "movies", "series", "profiles", "device_tokens"},
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
	profileCount := len(s.st.Profiles)
	s.mu.RUnlock()
	jsonOut(w, map[string]any{"version": Version, "server_name": serverName, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "memory_mb": m.Alloc / 1024 / 1024, "uptime_sec": int(time.Since(s.started).Seconds()), "media": mc, "missing_media": missingCount, "libraries": lc, "profiles": profileCount, "scanning": sc, "tmdb": s.tmdbKey() != "", "tvdb": s.tvdbKey() != "", "storage": "embedded-json-v1"})
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
	s.sessions = map[string]time.Time{}
	s.st.DeviceTokens = nil
	e := s.saveLocked()
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "buddyflix_session", Path: "/", MaxAge: -1, HttpOnly: true})
	jsonOut(w, map[string]bool{"ok": true})
}

func (s *Server) activeProfileID(r *http.Request) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.st.Profiles) == 0 { return 0 }
	if raw := strings.TrimSpace(r.Header.Get("X-BuddyFlix-Profile")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			for _, p := range s.st.Profiles {
				if p.ID == id { return id }
			}
		}
	}
	if c, err := r.Cookie("buddyflix_profile"); err == nil {
		if id, err := strconv.ParseInt(c.Value, 10, 64); err == nil {
			for _, p := range s.st.Profiles {
				if p.ID == id { return id }
			}
		}
	}
	return s.st.Profiles[0].ID
}

func (s *Server) profileProgressLocked(profileID int64) map[int64]Progress {
	if len(s.st.Profiles) == 0 { return s.st.Progress }
	defaultID := s.st.Profiles[0].ID
	if profileID == 0 || profileID == defaultID {
		if s.st.Progress == nil { s.st.Progress = map[int64]Progress{} }
		return s.st.Progress
	}
	if s.st.ProfileProgress == nil { s.st.ProfileProgress = map[int64]map[int64]Progress{} }
	if s.st.ProfileProgress[profileID] == nil { s.st.ProfileProgress[profileID] = map[int64]Progress{} }
	return s.st.ProfileProgress[profileID]
}

func (s *Server) profileFavoritesLocked(profileID int64) map[int64]bool {
	if s.st.ProfileFavorites == nil { s.st.ProfileFavorites = map[int64]map[int64]bool{} }
	if s.st.ProfileFavorites[profileID] == nil { s.st.ProfileFavorites[profileID] = map[int64]bool{} }
	return s.st.ProfileFavorites[profileID]
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		active := s.activeProfileID(r)
		s.mu.RLock()
		items := append([]Profile(nil), s.st.Profiles...)
		s.mu.RUnlock()
		_, cookieErr := r.Cookie("buddyflix_profile")
		selected := cookieErr == nil || strings.TrimSpace(r.Header.Get("X-BuddyFlix-Profile")) != ""
		jsonOut(w, map[string]any{"profiles": items, "active_id": active, "selected": selected})
	case "POST":
		var x struct { Name, Avatar string }
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil { jsonErr(w, 400, "invalid json"); return }
		x.Name = strings.TrimSpace(x.Name); x.Avatar = strings.TrimSpace(x.Avatar)
		if x.Name == "" { jsonErr(w, 400, "profile name required"); return }
		if len([]rune(x.Name)) > 30 { jsonErr(w, 400, "profile name too long"); return }
		if x.Avatar == "" { x.Avatar = "🎬" }
		if len([]rune(x.Avatar)) > 8 { jsonErr(w, 400, "avatar too long"); return }
		s.mu.Lock()
		if len(s.st.Profiles) >= 8 { s.mu.Unlock(); jsonErr(w, 400, "maximum of 8 profiles reached"); return }
		p := Profile{ID: s.st.NextProfileID, Name: x.Name, Avatar: x.Avatar, Created: time.Now().Format(time.RFC3339)}
		s.st.NextProfileID++
		s.st.Profiles = append(s.st.Profiles, p)
		if s.st.ProfileProgress == nil { s.st.ProfileProgress = map[int64]map[int64]Progress{} }
		s.st.ProfileProgress[p.ID] = map[int64]Progress{}
		s.profileFavoritesLocked(p.ID)
		err := s.saveLocked()
		s.mu.Unlock()
		if err != nil { jsonErr(w, 500, err.Error()); return }
		jsonOut(w, p)
	case "PUT":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var x struct { Name, Avatar string }
		if id < 1 || json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil { jsonErr(w, 400, "invalid profile"); return }
		x.Name = strings.TrimSpace(x.Name); x.Avatar = strings.TrimSpace(x.Avatar)
		if x.Name == "" || len([]rune(x.Name)) > 30 { jsonErr(w, 400, "invalid profile name"); return }
		if x.Avatar == "" { x.Avatar = "🎬" }
		s.mu.Lock()
		found := false
		for i := range s.st.Profiles {
			if s.st.Profiles[i].ID == id {
				s.st.Profiles[i].Name = x.Name
				s.st.Profiles[i].Avatar = x.Avatar
				found = true
				break
			}
		}
		if !found { s.mu.Unlock(); jsonErr(w, 404, "profile not found"); return }
		err := s.saveLocked()
		s.mu.Unlock()
		if err != nil { jsonErr(w, 500, err.Error()); return }
		jsonOut(w, map[string]bool{"ok": true})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		s.mu.Lock()
		if len(s.st.Profiles) <= 1 { s.mu.Unlock(); jsonErr(w, 400, "at least one profile is required"); return }
		if len(s.st.Profiles) > 0 && id == s.st.Profiles[0].ID { s.mu.Unlock(); jsonErr(w, 400, "main profile cannot be deleted"); return }
		found := false
		next := s.st.Profiles[:0]
		for _, p := range s.st.Profiles {
			if p.ID == id { found = true; continue }
			next = append(next, p)
		}
		if !found { s.mu.Unlock(); jsonErr(w, 404, "profile not found"); return }
		s.st.Profiles = next
		delete(s.st.ProfileProgress, id)
		delete(s.st.ProfileFavorites, id)
		err := s.saveLocked()
		s.mu.Unlock()
		if err != nil { jsonErr(w, 500, err.Error()); return }
		jsonOut(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, 405, "method not allowed")
	}
}

func (s *Server) selectProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	var x struct { ProfileID int64 `json:"profile_id"` }
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil || x.ProfileID < 1 { jsonErr(w, 400, "invalid profile"); return }
	s.mu.RLock()
	found := false
	for _, p := range s.st.Profiles { if p.ID == x.ProfileID { found = true; break } }
	s.mu.RUnlock()
	if !found { jsonErr(w, 404, "profile not found"); return }
	http.SetCookie(w, &http.Cookie{Name:"buddyflix_profile", Value:strconv.FormatInt(x.ProfileID,10), Path:"/", SameSite:http.SameSiteLaxMode, MaxAge:31536000})
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
				for pid := range s.st.ProfileProgress { delete(s.st.ProfileProgress[pid], m.ID) }
				for pid := range s.st.ProfileFavorites { delete(s.st.ProfileFavorites[pid], m.ID) }
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
var episodeRx = regexp.MustCompile(`(?i)(?:^|[ ._\-])S(\d{1,2})E(\d{1,3})(?:[ ._\-]|$)`)
var episodeAltRx = regexp.MustCompile(`(?i)(?:^|[ ._\-])(\d{1,2})x(\d{1,3})(?:[ ._\-]|$)`)

func cleanEpisodeText(v string) string {
	v = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(v)
	return strings.TrimSpace(strings.Join(strings.Fields(v), " "))
}

func episodeFromPath(root, p string) (series string, season, episode int, episodeTitle string) {
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	normalized := strings.NewReplacer(".", " ", "_", " ").Replace(base)
	loc := episodeRx.FindStringSubmatchIndex(normalized)
	rx := episodeRx
	if loc == nil {
		loc = episodeAltRx.FindStringSubmatchIndex(normalized)
		rx = episodeAltRx
	}
	if loc != nil {
		m := rx.FindStringSubmatch(normalized[loc[0]:loc[1]])
		if len(m) >= 3 {
			season, _ = strconv.Atoi(m[1])
			episode, _ = strconv.Atoi(m[2])
		}
		prefix := cleanEpisodeText(normalized[:loc[0]])
		suffix := cleanEpisodeText(normalized[loc[1]:])
		if prefix != "" { series = prefix }
		if suffix != "" { episodeTitle = cleanLocalMetadataTitle(suffix) }
	}
	rel, err := filepath.Rel(root, p)
	if err == nil {
		parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
		if len(parts) > 1 {
			first := cleanEpisodeText(parts[0])
			if first != "" && (series == "" || strings.HasPrefix(strings.ToLower(series), "staffel ") || strings.HasPrefix(strings.ToLower(series), "season ")) {
				series, _ = titleFromPath(first)
			}
		}
	}
	if series == "" {
		parent := cleanEpisodeText(filepath.Base(filepath.Dir(p)))
		lower := strings.ToLower(parent)
		if !strings.HasPrefix(lower, "staffel ") && !strings.HasPrefix(lower, "season ") {
			series, _ = titleFromPath(parent)
		}
	}
	if series == "" { series = "Unbekannte Serie" }
	if episodeTitle == "" && episode > 0 { episodeTitle = "Episode " + strconv.Itoa(episode) }
	return strings.TrimSpace(series), season, episode, strings.TrimSpace(episodeTitle)
}

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
			kind := "movie"
			seriesTitle, seasonNo, episodeNo, episodeTitle := "", 0, 0, ""
			switch strings.ToLower(strings.TrimSpace(lib.Type)) {
			case "shows", "series", "tv":
				kind = "episode"
				seriesTitle, seasonNo, episodeNo, episodeTitle = episodeFromPath(lib.Path, file.path)
				if episodeTitle != "" { title = episodeTitle }
				year = 0
			case "other":
				kind = "other"
			}
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
				s.st.Media[found].Kind = kind
				s.st.Media[found].SeriesTitle = seriesTitle
				s.st.Media[found].Season = seasonNo
				s.st.Media[found].Episode = episodeNo
				s.st.Media[found].EpisodeTitle = episodeTitle
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
					Kind: kind, SeriesTitle: seriesTitle, Season: seasonNo, Episode: episodeNo, EpisodeTitle: episodeTitle,
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
	profileID := s.activeProfileID(r)
	s.mu.Lock()
	progressMap := s.profileProgressLocked(profileID)
	favorites := s.profileFavoritesLocked(profileID)
	s.mu.Unlock()
	s.mu.RLock()
	out := make([]Media, 0)
	for _, m := range s.st.Media {
		if id > 0 && m.ID != id {
			continue
		}
		if !includeMissing && m.Missing {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.Title), q) && !strings.Contains(strings.ToLower(m.SeriesTitle), q) {
			continue
		}
		if p, ok := progressMap[m.ID]; ok {
			m.Position = p.Position
			m.Duration = p.Duration
			m.ProgressUpdated = p.Updated
			if p.Duration > 0 {
				m.Progress = p.Position / p.Duration * 100
			}
		}
		m.Favorite = favorites[m.ID]
		out = append(out, m)
	}
	s.mu.RUnlock()
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	jsonOut(w, out)
}

func (s *Server) series(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" { jsonErr(w, 405, "method not allowed"); return }
	profileID := s.activeProfileID(r)
	s.mu.Lock()
	progressMap := s.profileProgressLocked(profileID)
	favorites := s.profileFavoritesLocked(profileID)
	s.mu.Unlock()

	s.mu.RLock()
	episodes := make([]Media, 0)
	for _, m := range s.st.Media {
		if m.Missing || m.Kind != "episode" { continue }
		if p, ok := progressMap[m.ID]; ok {
			m.Position = p.Position
			m.Duration = p.Duration
			m.ProgressUpdated = p.Updated
			if p.Duration > 0 { m.Progress = p.Position / p.Duration * 100 }
		}
		m.Favorite = favorites[m.ID]
		episodes = append(episodes, m)
	}
	s.mu.RUnlock()

	sort.SliceStable(episodes, func(i, j int) bool {
		a, b := episodes[i], episodes[j]
		if strings.EqualFold(a.SeriesTitle, b.SeriesTitle) {
			if a.Season == b.Season {
				if a.Episode == b.Episode { return a.Title < b.Title }
				return a.Episode < b.Episode
			}
			return a.Season < b.Season
		}
		return strings.ToLower(a.SeriesTitle) < strings.ToLower(b.SeriesTitle)
	})

	type builder struct {
		view SeriesView
		seasons map[int][]Media
	}
	groups := map[string]*builder{}
	order := make([]string, 0)
	for _, ep := range episodes {
		title := strings.TrimSpace(ep.SeriesTitle)
		if title == "" { title = "Unbekannte Serie" }
		key := strings.ToLower(title)
		b := groups[key]
		if b == nil {
			b = &builder{view: SeriesView{Key:key, Title:title}, seasons:map[int][]Media{}}
			groups[key] = b
			order = append(order, key)
		}
		b.view.EpisodeCount++
		if ep.Progress >= 95 { b.view.WatchedCount++ }
		if ep.Progress > 1 && ep.Progress < 95 { b.view.ContinueCount++ }
		if ep.ProgressUpdated != "" && ep.ProgressUpdated > b.view.LastActivity { b.view.LastActivity = ep.ProgressUpdated }
		if b.view.Poster == "" && ep.Poster != "" { b.view.Poster = ep.Poster }
		if b.view.Backdrop == "" && ep.Backdrop != "" { b.view.Backdrop = ep.Backdrop }
		b.seasons[ep.Season] = append(b.seasons[ep.Season], ep)
	}

	sort.Strings(order)
	out := make([]SeriesView, 0, len(order))
	for _, key := range order {
		b := groups[key]
		seasonNos := make([]int, 0, len(b.seasons))
		for n := range b.seasons { seasonNos = append(seasonNos, n) }
		sort.Ints(seasonNos)
		for _, n := range seasonNos {
			b.view.Seasons = append(b.view.Seasons, SeriesSeason{Number:n, Episodes:b.seasons[n]})
		}
		b.view.SeasonCount = len(b.view.Seasons)
		for si := range b.view.Seasons {
			for ei := range b.view.Seasons[si].Episodes {
				ep := b.view.Seasons[si].Episodes[ei]
				if ep.Progress > 1 && ep.Progress < 95 {
					copy := ep
					b.view.NextEpisode = &copy
					break
				}
			}
			if b.view.NextEpisode != nil { break }
		}
		if b.view.NextEpisode == nil {
			for si := range b.view.Seasons {
				for ei := range b.view.Seasons[si].Episodes {
					ep := b.view.Seasons[si].Episodes[ei]
					if ep.Progress < 95 {
						copy := ep
						b.view.NextEpisode = &copy
						break
					}
				}
				if b.view.NextEpisode != nil { break }
			}
		}
		out = append(out, b.view)
	}
	jsonOut(w, out)
}

func (s *Server) mediaAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { jsonErr(w, 405, "method not allowed"); return }
	profileID := s.activeProfileID(r)
	var x struct { MediaID int64 `json:"media_id"`; Action string `json:"action"` }
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&x) != nil || x.MediaID < 1 { jsonErr(w, 400, "invalid json"); return }
	s.mu.Lock()
	defer s.mu.Unlock()
	progressMap := s.profileProgressLocked(profileID)
	favorites := s.profileFavoritesLocked(profileID)
	for i := range s.st.Media {
		if s.st.Media[i].ID != x.MediaID { continue }
		switch x.Action {
		case "reset_progress", "mark_unwatched":
			delete(progressMap, x.MediaID)
		case "mark_watched":
			progressMap[x.MediaID] = Progress{Position: 1, Duration: 1, Updated: time.Now().Format(time.RFC3339)}
		case "favorite":
			favorites[x.MediaID] = true
		case "unfavorite":
			delete(favorites, x.MediaID)
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
		if m.Missing {
			delete(s.st.Progress, m.ID)
			for pid := range s.st.ProfileProgress { delete(s.st.ProfileProgress[pid], m.ID) }
			for pid := range s.st.ProfileFavorites { delete(s.st.ProfileFavorites[pid], m.ID) }
			removed++
			continue
		}
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
		if m.Missing || m.MetadataLocked || m.Kind == "episode" || mediaHasMetadata(m) { continue }
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
	if math.IsNaN(x.Position) || math.IsInf(x.Position, 0) || math.IsNaN(x.Duration) || math.IsInf(x.Duration, 0) || x.Duration <= 0 {
		jsonErr(w, 400, "invalid progress")
		return
	}
	if x.Position < 0 { x.Position = 0 }
	if x.Position > x.Duration { x.Position = x.Duration }
	if x.Position/x.Duration >= 0.95 || (x.Position >= 60 && x.Duration-x.Position <= 30) {
		x.Position = x.Duration
	}

	profileID := s.activeProfileID(r)
	s.mu.Lock()
	found := false
	for _, m := range s.st.Media {
		if m.ID == x.MediaID && !m.Missing { found = true; break }
	}
	if !found {
		s.mu.Unlock()
		jsonErr(w, 404, "media not found")
		return
	}
	progressMap := s.profileProgressLocked(profileID)
	progressMap[x.MediaID] = Progress{Position:x.Position, Duration:x.Duration, Updated:time.Now().Format(time.RFC3339)}
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
