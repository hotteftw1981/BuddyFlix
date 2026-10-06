package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// MetadataCandidate is provider-neutral. New providers can populate their own
// external IDs without changing the rest of BuddyFlix.
type MetadataCandidate struct {
	ID          int64             `json:"id,omitempty"` // compatibility with the first TMDb UI
	Provider    string            `json:"provider"`
	ProviderID  string            `json:"provider_id"`
	Title       string            `json:"title"`
	Overview    string            `json:"overview"`
	Poster      string            `json:"poster"`
	Backdrop    string            `json:"backdrop"`
	ReleaseDate string            `json:"release_date"`
	Year        int               `json:"year"`
	ExternalIDs     map[string]string `json:"external_ids,omitempty"`
	AlternateTitles []string          `json:"alternate_titles,omitempty"`
}

// Alias keeps older tests/clients source-compatible while the engine becomes provider-neutral.
type TMDbSearchResult = MetadataCandidate

type MetadataProvider interface {
	Name() string
	SearchMovie(title string, year int) ([]MetadataCandidate, error)
	EnrichMovie(candidate MetadataCandidate) (MetadataCandidate, error)
}

type MetadataProviderStatus struct {
	Name             string `json:"name"`
	Configured       bool   `json:"configured"`
	Primary          bool   `json:"primary"`
	CredentialSource string `json:"credential_source,omitempty"`
}

type MetadataEngine struct {
	providers []MetadataProvider
}

func NewMetadataEngine(providers ...MetadataProvider) MetadataEngine {
	clean := make([]MetadataProvider, 0, len(providers))
	for _, p := range providers { if p != nil { clean = append(clean, p) } }
	return MetadataEngine{providers: clean}
}

func (e MetadataEngine) Available() bool { return len(e.providers) > 0 }

func (e MetadataEngine) Status() []MetadataProviderStatus {
	out := make([]MetadataProviderStatus, 0, len(e.providers))
	for i, p := range e.providers {
		out = append(out, MetadataProviderStatus{Name: p.Name(), Configured: true, Primary: i == 0})
	}
	if len(out) == 0 {
		// TMDb is the first built-in provider even when it has no credential yet.
		out = append(out, MetadataProviderStatus{Name: "tmdb", Configured: false, Primary: true})
	}
	return out
}

func (e MetadataEngine) SearchMovie(title string, year int) ([]MetadataCandidate, error) {
	if len(e.providers) == 0 { return nil, fmt.Errorf("no metadata provider configured") }
	var all []MetadataCandidate
	var lastErr error
	for _, p := range e.providers {
		results, err := p.SearchMovie(title, year)
		if err != nil { lastErr = err; continue }
		all = append(all, results...)
		// The first configured provider is primary. If it found candidates,
		// don't fan out to future fallbacks unnecessarily.
		if len(results) > 0 { return all, nil }
	}
	if len(all) == 0 && lastErr != nil { return nil, lastErr }
	return all, nil
}

func (e MetadataEngine) EnrichMovie(candidate MetadataCandidate) (MetadataCandidate, error) {
	if candidate.Provider == "" { return candidate, nil }
	for _, p := range e.providers {
		if p.Name() == candidate.Provider { return p.EnrichMovie(candidate) }
	}
	return candidate, fmt.Errorf("metadata provider %q is not configured", candidate.Provider)
}

type TMDbProvider struct {
	apiKey string
	client *http.Client
}

func NewTMDbProvider(apiKey string) *TMDbProvider {
	return &TMDbProvider{apiKey: strings.TrimSpace(apiKey), client: &http.Client{Timeout: 8 * time.Second}}
}

func (p *TMDbProvider) Name() string { return "tmdb" }

func (p *TMDbProvider) SearchMovie(title string, year int) ([]MetadataCandidate, error) {
	v := url.Values{"api_key": {p.apiKey}, "query": {title}, "language": {"de-DE"}}
	if year > 0 { v.Set("year", strconv.Itoa(year)) }
	resp, err := p.client.Get("https://api.themoviedb.org/3/search/movie?" + v.Encode())
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, fmt.Errorf("TMDb request failed (%d)", resp.StatusCode) }
	var raw struct {
		Results []struct {
			ID int64 `json:"id"`
			Title string `json:"title"`
			Overview string `json:"overview"`
			PosterPath string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
			ReleaseDate string `json:"release_date"`
		} `json:"results"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&raw) != nil { return nil, fmt.Errorf("invalid TMDb response") }
	out := make([]MetadataCandidate, 0, len(raw.Results))
	for i, z := range raw.Results {
		if i >= 12 { break }
		y := 0
		if len(z.ReleaseDate) >= 4 { y, _ = strconv.Atoi(z.ReleaseDate[:4]) }
		poster, backdrop := "", ""
		if z.PosterPath != "" { poster = "https://image.tmdb.org/t/p/w342" + z.PosterPath }
		if z.BackdropPath != "" { backdrop = "https://image.tmdb.org/t/p/w780" + z.BackdropPath }
		id := strconv.FormatInt(z.ID, 10)
		out = append(out, MetadataCandidate{
			ID: z.ID, Provider: p.Name(), ProviderID: id,
			Title: z.Title, Overview: z.Overview, Poster: poster, Backdrop: backdrop,
			ReleaseDate: z.ReleaseDate, Year: y,
			ExternalIDs: map[string]string{"tmdb": id},
		})
	}
	return out, nil
}

func (p *TMDbProvider) EnrichMovie(candidate MetadataCandidate) (MetadataCandidate, error) {
	id := candidate.ProviderID
	if id == "" && candidate.ID > 0 { id = strconv.FormatInt(candidate.ID, 10) }
	if id == "" { return candidate, nil }
	v := url.Values{"api_key": {p.apiKey}, "language": {"de-DE"}, "append_to_response": {"external_ids"}}
	resp, err := p.client.Get("https://api.themoviedb.org/3/movie/" + url.PathEscape(id) + "?" + v.Encode())
	if err != nil { return candidate, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return candidate, fmt.Errorf("TMDb detail request failed (%d)", resp.StatusCode) }
	var raw struct {
		ID int64 `json:"id"`
		Title string `json:"title"`
		Overview string `json:"overview"`
		PosterPath string `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
		ReleaseDate string `json:"release_date"`
		ExternalIDs struct {
			IMDbID string `json:"imdb_id"`
			WikidataID string `json:"wikidata_id"`
		} `json:"external_ids"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&raw) != nil { return candidate, fmt.Errorf("invalid TMDb detail response") }
	if raw.ID > 0 { candidate.ID = raw.ID; candidate.ProviderID = strconv.FormatInt(raw.ID, 10) }
	candidate.Provider = p.Name()
	if raw.Title != "" { candidate.Title = raw.Title }
	if raw.Overview != "" { candidate.Overview = raw.Overview }
	if raw.ReleaseDate != "" { candidate.ReleaseDate = raw.ReleaseDate }
	if len(raw.ReleaseDate) >= 4 { candidate.Year, _ = strconv.Atoi(raw.ReleaseDate[:4]) }
	if raw.PosterPath != "" { candidate.Poster = "https://image.tmdb.org/t/p/w500" + raw.PosterPath }
	if raw.BackdropPath != "" { candidate.Backdrop = "https://image.tmdb.org/t/p/w1280" + raw.BackdropPath }
	if candidate.ExternalIDs == nil { candidate.ExternalIDs = map[string]string{} }
	candidate.ExternalIDs["tmdb"] = candidate.ProviderID
	if raw.ExternalIDs.IMDbID != "" { candidate.ExternalIDs["imdb"] = raw.ExternalIDs.IMDbID }
	if raw.ExternalIDs.WikidataID != "" { candidate.ExternalIDs["wikidata"] = raw.ExternalIDs.WikidataID }
	return candidate, nil
}

func normalizedTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) { b.WriteRune(r) }
	}
	return b.String()
}

func titleSimilarity(a, b string) int {
	aRunes, bRunes := []rune(normalizedTitle(a)), []rune(normalizedTitle(b))
	if len(aRunes) == 0 || len(bRunes) == 0 { return 0 }
	if string(aRunes) == string(bRunes) { return 100 }
	prev := make([]int, len(bRunes)+1)
	for j := range prev { prev[j] = j }
	for i := 1; i <= len(aRunes); i++ {
		cur := make([]int, len(bRunes)+1)
		cur[0] = i
		for j := 1; j <= len(bRunes); j++ {
			cost := 0
			if aRunes[i-1] != bRunes[j-1] { cost = 1 }
			ins, del, sub := cur[j-1]+1, prev[j]+1, prev[j-1]+cost
			cur[j] = ins
			if del < cur[j] { cur[j] = del }
			if sub < cur[j] { cur[j] = sub }
		}
		prev = cur
	}
	maxLen := len(aRunes)
	if len(bRunes) > maxLen { maxLen = len(bRunes) }
	score := 100 - (prev[len(bRunes)] * 100 / maxLen)
	if score < 0 { return 0 }
	return score
}

func candidateTitleScore(input string, r MetadataCandidate) (int, bool) {
	best := titleSimilarity(input, r.Title)
	exact := normalizedTitle(r.Title) == normalizedTitle(input)
	for _, alt := range r.AlternateTitles {
		score := titleSimilarity(input, alt)
		if score > best { best = score }
		if normalizedTitle(alt) == normalizedTitle(input) { exact = true }
	}
	return best, exact
}

func chooseMetadataMatch(title string, year int, results []MetadataCandidate) (MetadataCandidate, int, bool, bool) {
	if len(results) == 0 { return MetadataCandidate{}, 0, false, false }
	exactCount := 0
	for _, r := range results {
		_, exact := candidateTitleScore(title, r)
		if exact { exactCount++ }
	}

	best, bestScore, bestExact := results[0], -1, false
	for _, r := range results {
		score, exact := candidateTitleScore(title, r)
		if year > 0 && r.Year > 0 {
			if year == r.Year { score += 6 } else { score -= 14 }
		}
		if !exact && score > 94 { score = 94 }
		if score > 100 { score = 100 }
		if score < 0 { score = 0 }
		if score > bestScore {
			best, bestScore, bestExact = r, score, exact
		}
	}
	if bestScore < 55 { return MetadataCandidate{}, bestScore, false, false }
	auto := false
	if bestExact {
		if year > 0 { auto = best.Year == year } else { auto = exactCount == 1 }
	}
	return best, bestScore, auto, true
}
