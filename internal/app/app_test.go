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
