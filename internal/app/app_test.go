package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := Config{
		ListenAddr:   ":0",
		DataDir:      t.TempDir(),
		AdminUser:    "admin",
		AdminPassword:"buddyflix",
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFirstRunSetup(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	s.setupStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}

	body := []byte(`{"server_name":"Wohnzimmer","admin_user":"patrick","password":"supersecret","tmdb_api_key":"abc123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	s.setup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: got %d body=%s", rec.Code, rec.Body.String())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.st.Settings.SetupDone {
		t.Fatal("setup should be completed")
	}
	if s.st.Settings.ServerName != "Wohnzimmer" {
		t.Fatalf("unexpected server name: %s", s.st.Settings.ServerName)
	}
	if s.st.Settings.AdminUser != "patrick" {
		t.Fatalf("unexpected admin user: %s", s.st.Settings.AdminUser)
	}
	if s.st.Settings.TMDBAPIKey != "abc123" {
		t.Fatal("TMDb key not persisted")
	}
}

func TestSetupCannotRunTwice(t *testing.T) {
	s := newTestServer(t)
	body := []byte(`{"server_name":"BuddyFlix","admin_user":"admin","password":"12345678"}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.setup(rec, req)
		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("first setup failed: %s", rec.Body.String())
		}
		if i == 1 && rec.Code != http.StatusConflict {
			t.Fatalf("second setup should conflict, got %d", rec.Code)
		}
	}
}

func TestMediaEditPersists(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = append(s.st.Media, Media{ID: 1, Title: "Old title", Path: "/tmp/movie.mkv"})
	s.st.NextMediaID = 2
	if err := s.saveLocked(); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()

	payload, _ := json.Marshal(map[string]any{
		"title": "New title",
		"year": 2026,
		"overview": "Edited metadata",
		"poster": "https://example.invalid/poster.jpg",
		"backdrop": "https://example.invalid/backdrop.jpg",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/media?id=1", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	s.media(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit failed: %d %s", rec.Code, rec.Body.String())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.st.Media[0].Title != "New title" || s.st.Media[0].Year != 2026 {
		t.Fatalf("media edit not persisted: %+v", s.st.Media[0])
	}
}


func TestMediaActions(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = append(s.st.Media, Media{ID: 1, Title: "Movie"})
	s.st.NextMediaID = 2
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/media/action", bytes.NewBufferString(`{"media_id":1,"action":"mark_watched"}`))
	rec := httptest.NewRecorder()
	s.mediaAction(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mark watched failed: %d %s", rec.Code, rec.Body.String())
	}

	s.mu.RLock()
	p, ok := s.st.Progress[1]
	s.mu.RUnlock()
	if !ok || p.Duration != 1 || p.Position != 1 {
		t.Fatalf("watched state not stored: %+v", p)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/media/action", bytes.NewBufferString(`{"media_id":1,"action":"reset_progress"}`))
	rec = httptest.NewRecorder()
	s.mediaAction(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset failed: %d %s", rec.Code, rec.Body.String())
	}
	s.mu.RLock()
	_, ok = s.st.Progress[1]
	s.mu.RUnlock()
	if ok {
		t.Fatal("progress should have been removed")
	}
}

func TestCleanupMissingMedia(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{
		{ID: 1, Title: "Present"},
		{ID: 2, Title: "Missing", Missing: true},
	}
	s.st.Progress[2] = Progress{Position: 10, Duration: 20}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/media/cleanup", nil)
	rec := httptest.NewRecorder()
	s.mediaCleanup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup failed: %d %s", rec.Code, rec.Body.String())
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.st.Media) != 1 || s.st.Media[0].ID != 1 {
		t.Fatalf("unexpected media after cleanup: %+v", s.st.Media)
	}
	if _, ok := s.st.Progress[2]; ok {
		t.Fatal("progress for removed item should also be removed")
	}
}


func TestScanKeepsMediaWhenLibraryUnavailable(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Libraries = []Library{{ID: 1, Name: "Movies", Path: "/definitely/not/mounted"}}
	s.st.Media = []Media{{ID: 1, LibraryID: 1, Title: "Keep me", Path: "/definitely/not/mounted/movie.mkv"}}
	s.st.NextLibraryID = 2
	s.st.NextMediaID = 2
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	rec := httptest.NewRecorder()
	s.scan(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("scan failed: %d %s", rec.Code, rec.Body.String())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.st.Media) != 1 {
		t.Fatalf("media index changed unexpectedly: %+v", s.st.Media)
	}
	if s.st.Media[0].Missing {
		t.Fatal("unavailable library must not mark existing media as missing")
	}
}


func TestLibrariesGetReturnsEmptyArray(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	s.libraries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("libraries GET failed: %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[]\n" {
		t.Fatalf("expected empty JSON array, got %q", got)
	}
}


func TestMediaGetHidesMissingByDefault(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{
		{ID: 1, Title: "Present", Path: "/present.mkv"},
		{ID: 2, Title: "Missing", Path: "/missing.mkv", Missing: true},
	}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	rec := httptest.NewRecorder()
	s.media(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("media GET failed: %d %s", rec.Code, rec.Body.String())
	}
	var visible []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &visible); err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].ID != 1 {
		t.Fatalf("expected only present media, got %+v", visible)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media?include_missing=1", nil)
	rec = httptest.NewRecorder()
	s.media(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("media GET include_missing failed: %d %s", rec.Code, rec.Body.String())
	}
	var all []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected present and missing media, got %+v", all)
	}
}
