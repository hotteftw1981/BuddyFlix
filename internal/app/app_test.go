package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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


func TestMetadataMatchExactTitleAuto(t *testing.T) {
	results := []TMDbSearchResult{
		{ID: 7, Title: "Was nicht passt, wird passend gemacht", Year: 2002},
		{ID: 8, Title: "Was nicht passt", Year: 2003},
	}
	got, confidence, auto, ok := chooseMetadataMatch("was nicht passt wird passend gemacht", 2002, results)
	if !ok || !auto || got.ID != 7 || confidence < 95 {
		t.Fatalf("expected confident automatic match, got result=%+v confidence=%d auto=%v ok=%v", got, confidence, auto, ok)
	}
}

func TestMetadataMatchWithoutYearStaysReviewWhenAmbiguous(t *testing.T) {
	results := []TMDbSearchResult{
		{ID: 1, Title: "The Thing", Year: 1982},
		{ID: 2, Title: "The Thing", Year: 2011},
	}
	got, confidence, auto, ok := chooseMetadataMatch("The Thing", 0, results)
	if !ok || auto || got.ID == 0 || confidence < 90 {
		t.Fatalf("expected ambiguous exact title to require review, got result=%+v confidence=%d auto=%v ok=%v", got, confidence, auto, ok)
	}
}

func TestMetadataMatchClearFuzzyWithExactYearAuto(t *testing.T) {
	results := []TMDbSearchResult{{ID: 3, Title: "Zurück in die Zukunft", Year: 1985}}
	got, confidence, auto, ok := chooseMetadataMatch("Zuruck in die Zukunft", 1985, results)
	if !ok || !auto || got.ID != 3 || confidence < 90 {
		t.Fatalf("expected clear fuzzy candidate with matching year to auto-match, got result=%+v confidence=%d auto=%v ok=%v", got, confidence, auto, ok)
	}
}

func TestMetadataMatchCloseCandidatesStillNeedReview(t *testing.T) {
	results := []MetadataCandidate{
		{Provider:"thetvdb", ProviderID:"1", Title:"Halloween", Year:1978},
		{Provider:"thetvdb", ProviderID:"2", Title:"Halloween II", Year:1978},
	}
	got, confidence, auto, ok := chooseMetadataMatch("Halloween I", 1978, results)
	if !ok || auto || got.ProviderID == "" || confidence < 55 {
		t.Fatalf("close candidates must still require review, got result=%+v confidence=%d auto=%v ok=%v", got, confidence, auto, ok)
	}
}

func TestMetadataMatchIgnoresReleaseNoise(t *testing.T) {
	results := []MetadataCandidate{{Provider:"thetvdb", ProviderID:"42", Title:"Some Movie", Year:2001}}
	got, confidence, auto, ok := chooseMetadataMatch("Some Movie 1080p BluRay x264 German", 2001, results)
	if !ok || !auto || got.ProviderID != "42" || confidence < 95 {
		t.Fatalf("release noise should not force manual review, got result=%+v confidence=%d auto=%v ok=%v", got, confidence, auto, ok)
	}
}


type fakeMetadataProvider struct{}

func (fakeMetadataProvider) Name() string { return "fake" }
func (fakeMetadataProvider) SearchMovie(title string, year int) ([]MetadataCandidate, error) {
	return []MetadataCandidate{{Provider:"fake", ProviderID:"42", Title:title, Year:year, ExternalIDs:map[string]string{"fake":"42"}}}, nil
}
func (fakeMetadataProvider) EnrichMovie(c MetadataCandidate) (MetadataCandidate, error) {
	if c.ExternalIDs == nil { c.ExternalIDs = map[string]string{} }
	c.ExternalIDs["imdb"] = "tt0042"
	c.Overview = "Provider-neutral metadata"
	return c, nil
}

func TestMetadataEngineUsesProvider(t *testing.T) {
	engine := NewMetadataEngine(fakeMetadataProvider{})
	if !engine.Available() { t.Fatal("engine should be available") }
	results, err := engine.SearchMovie("Testfilm", 2026)
	if err != nil || len(results) != 1 { t.Fatalf("unexpected search result: %+v err=%v", results, err) }
	if results[0].Provider != "fake" || results[0].ProviderID != "42" { t.Fatalf("provider identity lost: %+v", results[0]) }
	enriched, err := engine.EnrichMovie(results[0])
	if err != nil || enriched.ExternalIDs["imdb"] != "tt0042" { t.Fatalf("external ID enrichment failed: %+v err=%v", enriched, err) }
}

func TestApplyMetadataStoresExternalIDsAndSources(t *testing.T) {
	m := Media{ID:1, Title:"Old"}
	candidate := MetadataCandidate{
		Provider:"tmdb", ProviderID:"123", Title:"New", Year:2026,
		Overview:"Overview", Poster:"poster", Backdrop:"backdrop",
		ExternalIDs:map[string]string{"tmdb":"123","imdb":"tt0123"},
	}
	applyMetadataResult(&m, candidate, "auto", 100)
	if m.ExternalIDs["tmdb"] != "123" || m.ExternalIDs["imdb"] != "tt0123" {
		t.Fatalf("external IDs not persisted: %+v", m.ExternalIDs)
	}
	if m.MetadataProvider != "tmdb" || m.MetadataSources["poster"] != "tmdb" || m.MetadataUpdated == "" {
		t.Fatalf("metadata provenance missing: %+v", m)
	}
}


func TestTMDBCredentialPrecedence(t *testing.T) {
	oldBuiltin := BuiltinTMDBAPIKey
	BuiltinTMDBAPIKey = "build-key"
	defer func(){ BuiltinTMDBAPIKey = oldBuiltin }()
	s := newTestServer(t)

	key, source := s.tmdbCredential()
	if key != "build-key" || source != "builtin" {
		t.Fatalf("expected built-in credential, got key=%q source=%q", key, source)
	}
	s.cfg.TMDBAPIKey = "env-key"
	key, source = s.tmdbCredential()
	if key != "env-key" || source != "environment" {
		t.Fatalf("expected environment credential, got key=%q source=%q", key, source)
	}
	s.mu.Lock()
	s.st.Settings.TMDBAPIKey = "override-key"
	s.mu.Unlock()
	key, source = s.tmdbCredential()
	if key != "override-key" || source != "override" {
		t.Fatalf("expected user override, got key=%q source=%q", key, source)
	}
}

func TestSettingsOnlyExposeCredentialSource(t *testing.T) {
	oldBuiltin := BuiltinTMDBAPIKey
	BuiltinTMDBAPIKey = "build-key"
	defer func(){ BuiltinTMDBAPIKey = oldBuiltin }()
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	s.settings(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("settings failed: %d", rec.Code) }
	body := rec.Body.String()
	if bytes.Contains([]byte(body), []byte("build-key")) { t.Fatal("settings leaked build credential") }
	if !bytes.Contains([]byte(body), []byte(`"tmdb_credential_source":"builtin"`)) {
		t.Fatalf("missing credential source status: %s", body)
	}
}


func TestTVDBProviderSearchAndEnrich(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("login method: %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]string{"token": "token-123"},
		})
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-123" {
			t.Fatalf("missing bearer token")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{
			"id":              "movie-1",
			"tvdb_id":         "123",
			"name":            "Test Movie",
			"name_translated": "Testfilm",
			"year":            "2026",
			"overviews":       map[string]string{"deu": "Deutsche Beschreibung"},
			"image_url":       "https://img.example/poster.jpg",
			"remote_ids":      []any{map[string]any{"id": "tt0123456", "sourceName": "IMDB"}},
		}}})
	})
	mux.HandleFunc("/movies/123/extended", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id":        123,
			"name":      "Test Movie",
			"year":      "2026",
			"image":     "https://img.example/base.jpg",
			"remoteIds": []any{map[string]any{"id": "tt0123456", "sourceName": "IMDB"}},
			"artworks":  []any{map[string]any{"image": "https://img.example/back.jpg", "width": 1920, "height": 1080}},
		}})
	})
	mux.HandleFunc("/movies/123/translations/deu", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{
			"name": "Testfilm", "overview": "Lange deutsche Beschreibung",
		}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewTVDBProvider("project-key")
	p.baseURL = srv.URL
	p.client = srv.Client()

	results, err := p.SearchMovie("Testfilm", 2026)
	if err != nil || len(results) != 1 {
		t.Fatalf("search failed: %+v %v", results, err)
	}
	if results[0].Provider != "thetvdb" || results[0].ExternalIDs["imdb"] != "tt0123456" {
		t.Fatalf("unexpected search result: %+v", results[0])
	}

	enriched, err := p.EnrichMovie(results[0])
	if err != nil {
		t.Fatal(err)
	}
	if enriched.Backdrop != "https://img.example/back.jpg" || enriched.Overview != "Lange deutsche Beschreibung" {
		t.Fatalf("unexpected enriched result: %+v", enriched)
	}
}

func TestTVDBCredentialPrecedence(t *testing.T) {
	oldBuiltin := BuiltinTVDBAPIKey
	BuiltinTVDBAPIKey = "tvdb-build-key"
	defer func() { BuiltinTVDBAPIKey = oldBuiltin }()

	s := newTestServer(t)
	key, source := s.tvdbCredential()
	if key != "tvdb-build-key" || source != "builtin" {
		t.Fatalf("builtin: %q %q", key, source)
	}

	s.cfg.TVDBAPIKey = "tvdb-env-key"
	key, source = s.tvdbCredential()
	if key != "tvdb-env-key" || source != "environment" {
		t.Fatalf("env: %q %q", key, source)
	}

	s.mu.Lock()
	s.st.Settings.TVDBAPIKey = "tvdb-override"
	s.mu.Unlock()
	key, source = s.tvdbCredential()
	if key != "tvdb-override" || source != "override" {
		t.Fatalf("override: %q %q", key, source)
	}
}


func TestCleanMetadataQueryStripsReleaseNoise(t *testing.T) {
	cases := map[string]string{
		"Dead.Man.on.Campus.1998.1080p.BluRay.x264-GROUP": "Dead Man on Campus 1998",
		"Zurueck in die Zukunft 1080p German DTS": "Zurueck in die Zukunft",
		"Alien_1979_WEB-DL_2160p_HDR": "Alien 1979",
	}
	for in, want := range cases {
		if got := cleanMetadataQuery(in); got != want {
			t.Fatalf("cleanMetadataQuery(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMetadataMatchUsesAlternateTitles(t *testing.T) {
	results := []MetadataCandidate{{
		Provider: "thetvdb",
		ProviderID: "1",
		Title: "The Hangover",
		AlternateTitles: []string{"Hangover"},
		Year: 2009,
	}}
	got, confidence, auto, ok := chooseMetadataMatch("Hangover", 2009, results)
	if !ok || !auto || got.ProviderID != "1" || confidence < 95 {
		t.Fatalf("alternate title should match automatically: %+v %d %v %v", got, confidence, auto, ok)
	}
}

func TestTVDBSearchRetriesCleanedTitleWithoutYear(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"token":"token"}})
	})
	var queries []string
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		q := r.URL.Query().Get("query")
		year := r.URL.Query().Get("year")
		if q == "Some Movie" && year == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data":[]any{map[string]any{
				"tvdb_id":"42","name":"Some Movie","year":"2001",
			}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data":[]any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewTVDBProvider("key")
	p.baseURL = srv.URL
	p.client = srv.Client()
	results, err := p.SearchMovie("Some Movie 1080p BluRay", 2002)
	if err != nil { t.Fatal(err) }
	if len(results) != 1 || results[0].ProviderID != "42" {
		t.Fatalf("expected cleaned yearless fallback hit, got %+v", results)
	}
	if len(queries) < 4 {
		t.Fatalf("expected cleaned and yearless fallback attempts, got %d: %+v", len(queries), queries)
	}
}


func TestTVDBSearchPrefersGermanTranslation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"token":"token"}})
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data":[]any{map[string]any{
			"tvdb_id":"77",
			"name":"The Lives of Others",
			"year":"2006",
			"translations":map[string]string{
				"deu":"Das Leben der Anderen",
				"eng":"The Lives of Others",
			},
		}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewTVDBProvider("key")
	p.baseURL = srv.URL
	p.client = srv.Client()
	results, err := p.SearchMovie("Das Leben der Anderen", 2006)
	if err != nil || len(results) != 1 {
		t.Fatalf("search failed: %+v %v", results, err)
	}
	if results[0].Title != "Das Leben der Anderen" {
		t.Fatalf("expected German title, got %q", results[0].Title)
	}
	_, confidence, auto, ok := chooseMetadataMatch("Das Leben der Anderen", 2006, results)
	if !ok || !auto || confidence < 95 {
		t.Fatalf("German translation should auto-match: confidence=%d auto=%v ok=%v result=%+v", confidence, auto, ok, results[0])
	}
}


func TestGermanSearchVariantsIncludeUmlautForm(t *testing.T) {
	variants := germanSearchVariants("Zurueck in die Zukunft")
	found := false
	for _, v := range variants {
		if v == "Zurück in die Zukunft" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected German umlaut search variant, got %+v", variants)
	}
}

func TestTVDBRefineCandidatesAddsGermanTitle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"token":"token"}})
	})
	mux.HandleFunc("/movies/42/translations/deu", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{
			"name":"Die nackte Kanone",
			"overview":"Deutsche Beschreibung",
		}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewTVDBProvider("key")
	p.baseURL = srv.URL
	p.client = srv.Client()

	refined, err := p.RefineMovieCandidates([]MetadataCandidate{{
		Provider:"thetvdb",
		ProviderID:"42",
		Title:"The Naked Gun",
		Year:1988,
	}})
	if err != nil || len(refined) != 1 {
		t.Fatalf("refine failed: %+v %v", refined, err)
	}
	found := false
	for _, alt := range refined[0].AlternateTitles {
		if alt == "Die nackte Kanone" { found = true }
	}
	if !found {
		t.Fatalf("German title was not added: %+v", refined[0])
	}
	_, confidence, auto, ok := chooseMetadataMatch("Die nackte Kanone", 1988, refined)
	if !ok || !auto || confidence < 95 {
		t.Fatalf("refined German title should auto-match: confidence=%d auto=%v ok=%v", confidence, auto, ok)
	}
}


func TestFreshInstallCreatesDefaultProfile(t *testing.T) {
	s := newTestServer(t)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.st.Profiles) != 1 {
		t.Fatalf("expected one default profile, got %+v", s.st.Profiles)
	}
	if s.st.Profiles[0].Name != "Hauptprofil" || s.st.Profiles[0].ID != 1 {
		t.Fatalf("unexpected default profile: %+v", s.st.Profiles[0])
	}
	if s.st.NextProfileID != 2 {
		t.Fatalf("expected next profile id 2, got %d", s.st.NextProfileID)
	}
}

func TestProfilesKeepWatchProgressSeparate(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{{ID:1, Title:"Movie"}}
	s.st.NextMediaID = 2
	s.st.Profiles = append(s.st.Profiles, Profile{ID:2, Name:"Gast", Avatar:"🎬"})
	s.st.NextProfileID = 3
	s.st.ProfileProgress[2] = map[int64]Progress{}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/progress", bytes.NewBufferString(`{"media_id":1,"Position":25,"Duration":100}`))
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"2"})
	rec := httptest.NewRecorder()
	s.progress(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("progress failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"2"})
	rec = httptest.NewRecorder()
	s.media(rec, req)
	var guest []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &guest); err != nil { t.Fatal(err) }
	if len(guest) != 1 || guest[0].Progress != 25 {
		t.Fatalf("guest progress missing: %+v", guest)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec = httptest.NewRecorder()
	s.media(rec, req)
	var main []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &main); err != nil { t.Fatal(err) }
	if len(main) != 1 || main[0].Progress != 0 {
		t.Fatalf("main profile should stay untouched: %+v", main)
	}
}

func TestProfileCreateSelectAndDelete(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", bytes.NewBufferString(`{"Name":"Patrick","Avatar":"🚀"}`))
	rec := httptest.NewRecorder()
	s.profiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile create failed: %d %s", rec.Code, rec.Body.String())
	}
	var created Profile
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil { t.Fatal(err) }
	if created.ID != 2 || created.Name != "Patrick" {
		t.Fatalf("unexpected created profile: %+v", created)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/profiles/select", bytes.NewBufferString(`{"profile_id":2}`))
	rec = httptest.NewRecorder()
	s.selectProfile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile select failed: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "buddyflix_profile" || cookies[0].Value != "2" {
		t.Fatalf("profile cookie missing: %+v", cookies)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/profiles?id=2", nil)
	rec = httptest.NewRecorder()
	s.profiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile delete failed: %d %s", rec.Code, rec.Body.String())
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.st.Profiles) != 1 {
		t.Fatalf("profile was not deleted: %+v", s.st.Profiles)
	}
}


func TestFavoritesAreProfileSpecific(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{{ID:1, Title:"Movie"}}
	s.st.NextMediaID = 2
	s.st.Profiles = append(s.st.Profiles, Profile{ID:2, Name:"Gast", Avatar:"🎬"})
	s.st.NextProfileID = 3
	s.st.ProfileProgress[2] = map[int64]Progress{}
	s.st.ProfileFavorites[2] = map[int64]bool{}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/media/action", bytes.NewBufferString(`{"media_id":1,"action":"favorite"}`))
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"2"})
	rec := httptest.NewRecorder()
	s.mediaAction(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("favorite failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"2"})
	rec = httptest.NewRecorder()
	s.media(rec, req)
	var guest []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &guest); err != nil { t.Fatal(err) }
	if len(guest) != 1 || !guest[0].Favorite {
		t.Fatalf("guest favorite missing: %+v", guest)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec = httptest.NewRecorder()
	s.media(rec, req)
	var main []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &main); err != nil { t.Fatal(err) }
	if len(main) != 1 || main[0].Favorite {
		t.Fatalf("favorite leaked into main profile: %+v", main)
	}
}

func TestMediaReturnsProgressUpdatedForHistory(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{{ID:1, Title:"Movie"}}
	s.st.Progress[1] = Progress{Position:50, Duration:100, Updated:"2026-10-07T18:00:00+02:00"}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec := httptest.NewRecorder()
	s.media(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("media failed: %d", rec.Code) }
	var items []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ProgressUpdated == "" || items[0].Progress != 50 {
		t.Fatalf("history metadata missing: %+v", items)
	}
}


func TestEpisodeFromPathCommonPatterns(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "media", "Shows")
	series, season, episode, title := episodeFromPath(root, filepath.Join(root, "Dark", "Staffel 1", "Dark.S01E03.Gestern.und.Heute.mkv"))
	if series != "Dark" || season != 1 || episode != 3 || title != "Gestern und Heute" {
		t.Fatalf("unexpected SxxExx parse: series=%q season=%d episode=%d title=%q", series, season, episode, title)
	}

	series, season, episode, title = episodeFromPath(root, filepath.Join(root, "The Office", "Season 02", "02x05 Halloween.mp4"))
	if series != "The Office" || season != 2 || episode != 5 || title != "Halloween" {
		t.Fatalf("unexpected x pattern parse: series=%q season=%d episode=%d title=%q", series, season, episode, title)
	}
}

func TestSeriesAPIgroupsSeasonsAndUsesProfileProgress(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{
		{ID:1, Kind:"episode", SeriesTitle:"Dark", Season:1, Episode:1, Title:"Geheimnisse"},
		{ID:2, Kind:"episode", SeriesTitle:"Dark", Season:1, Episode:2, Title:"Lügen"},
		{ID:3, Kind:"episode", SeriesTitle:"Dark", Season:2, Episode:1, Title:"Anfänge und Enden"},
		{ID:4, Kind:"movie", Title:"Ein Film"},
	}
	s.st.Progress[1] = Progress{Position:1, Duration:1, Updated:"2026-10-07T17:00:00Z"}
	s.st.Progress[2] = Progress{Position:50, Duration:100, Updated:"2026-10-07T18:00:00Z"}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/series", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec := httptest.NewRecorder()
	s.series(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("series failed: %d %s", rec.Code, rec.Body.String())
	}
	var out []SeriesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil { t.Fatal(err) }
	if len(out) != 1 {
		t.Fatalf("expected one series, got %+v", out)
	}
	got := out[0]
	if got.Title != "Dark" || got.SeasonCount != 2 || got.EpisodeCount != 3 || got.WatchedCount != 1 || got.ContinueCount != 1 {
		t.Fatalf("unexpected series summary: %+v", got)
	}
	if got.NextEpisode == nil || got.NextEpisode.ID != 2 || got.NextEpisode.Progress != 50 {
		t.Fatalf("expected in-progress episode as next episode: %+v", got.NextEpisode)
	}
	if len(got.Seasons) != 2 || len(got.Seasons[0].Episodes) != 2 {
		t.Fatalf("unexpected seasons: %+v", got.Seasons)
	}
}

func TestShowsLibraryScannerMarksEpisodes(t *testing.T) {
	s := newTestServer(t)
	root := t.TempDir()
	seasonDir := filepath.Join(root, "Dark", "Staffel 01")
	if err := os.MkdirAll(seasonDir, 0755); err != nil { t.Fatal(err) }
	file := filepath.Join(seasonDir, "Dark.S01E01.Geheimnisse.mkv")
	if err := os.WriteFile(file, []byte("video"), 0644); err != nil { t.Fatal(err) }

	s.mu.Lock()
	s.st.Libraries = []Library{{ID:1, Name:"Serien", Path:root, Type:"shows"}}
	s.st.NextLibraryID = 2
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
		t.Fatalf("expected one episode, got %+v", s.st.Media)
	}
	m := s.st.Media[0]
	if m.Kind != "episode" || m.SeriesTitle != "Dark" || m.Season != 1 || m.Episode != 1 || m.Title != "Geheimnisse" {
		t.Fatalf("episode metadata not parsed: %+v", m)
	}
}


func TestProgressNormalizesCompletionAndRejectsUnknownMedia(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{{ID:1, Title:"Movie", Path:"/movie.mkv"}}
	s.st.NextMediaID = 2
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/progress", bytes.NewBufferString(`{"media_id":1,"Position":96,"Duration":100}`))
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec := httptest.NewRecorder()
	s.progress(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("progress failed: %d %s", rec.Code, rec.Body.String())
	}

	s.mu.RLock()
	p := s.st.Progress[1]
	s.mu.RUnlock()
	if p.Position != 100 || p.Duration != 100 || p.Updated == "" {
		t.Fatalf("near-end progress should be normalized to watched: %+v", p)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/progress", bytes.NewBufferString(`{"media_id":999,"Position":10,"Duration":100}`))
	rec = httptest.NewRecorder()
	s.progress(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown media should return 404, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSeriesExposesLastActivityAndNextUnwatchedEpisode(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Media = []Media{
		{ID:1, Kind:"episode", SeriesTitle:"Dark", Season:1, Episode:1, Title:"Geheimnisse"},
		{ID:2, Kind:"episode", SeriesTitle:"Dark", Season:1, Episode:2, Title:"Lügen"},
	}
	s.st.Progress[1] = Progress{Position:100, Duration:100, Updated:"2026-10-07T18:30:00Z"}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/series", nil)
	req.AddCookie(&http.Cookie{Name:"buddyflix_profile", Value:"1"})
	rec := httptest.NewRecorder()
	s.series(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("series failed: %d %s", rec.Code, rec.Body.String())
	}

	var out []SeriesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil { t.Fatal(err) }
	if len(out) != 1 { t.Fatalf("expected one series, got %+v", out) }
	if out[0].LastActivity != "2026-10-07T18:30:00Z" {
		t.Fatalf("last activity missing: %+v", out[0])
	}
	if out[0].NextEpisode == nil || out[0].NextEpisode.ID != 2 || out[0].NextEpisode.Progress != 0 {
		t.Fatalf("expected next unwatched episode after completed episode: %+v", out[0].NextEpisode)
	}
}


func TestDeviceLoginBearerAuthAndProfileHeader(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Settings.SetupDone = true
	s.st.Media = []Media{{ID:1, Title:"Movie", Path:"/movie.mkv"}}
	s.st.NextMediaID = 2
	s.st.Profiles = append(s.st.Profiles, Profile{ID:2, Name:"Gast", Avatar:"🎬"})
	s.st.NextProfileID = 3
	s.st.ProfileProgress[2] = map[int64]Progress{1:{Position:40, Duration:100, Updated:"2026-10-07T20:00:00Z"}}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/device/login", bytes.NewBufferString(`{"username":"admin","password":"buddyflix","device_name":"Fire TV Wohnzimmer"}`))
	rec := httptest.NewRecorder()
	s.deviceLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("device login failed: %d %s", rec.Code, rec.Body.String())
	}
	var login map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil { t.Fatal(err) }
	token, _ := login["token"].(string)
	if token == "" { t.Fatalf("missing device token: %+v", login) }

	req = httptest.NewRequest(http.MethodGet, "/api/media?id=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-BuddyFlix-Profile", "2")
	rec = httptest.NewRecorder()
	s.auth(s.media)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer media request failed: %d %s", rec.Code, rec.Body.String())
	}
	var items []Media
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Progress != 40 {
		t.Fatalf("profile header was not applied: %+v", items)
	}

	s.mu.RLock()
	if len(s.st.DeviceTokens) != 1 || s.st.DeviceTokens[0].TokenHash == token {
		s.mu.RUnlock()
		t.Fatal("device token must be stored hashed")
	}
	s.mu.RUnlock()
}

func TestDeviceLogoutRevokesBearerToken(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	s.st.Settings.SetupDone = true
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/device/login", bytes.NewBufferString(`{"username":"admin","password":"buddyflix","device_name":"Fire TV"}`))
	rec := httptest.NewRecorder()
	s.deviceLogin(rec, req)
	var login map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &login)
	token, _ := login["token"].(string)
	if token == "" { t.Fatal("missing token") }

	req = httptest.NewRequest(http.MethodPost, "/api/v1/device/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.auth(s.deviceLogout)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.auth(s.media)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token should be unauthorized, got %d", rec.Code)
	}
}
