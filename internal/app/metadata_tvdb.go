package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTVDBBaseURL = "https://api4.thetvdb.com/v4"

type TVDBProvider struct {
	apiKey      string
	baseURL     string
	client      *http.Client
	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewTVDBProvider(apiKey string) *TVDBProvider {
	return &TVDBProvider{
		apiKey: strings.TrimSpace(apiKey),
		baseURL: defaultTVDBBaseURL,
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (p *TVDBProvider) Name() string { return "thetvdb" }

func (p *TVDBProvider) bearer() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExpiry) {
		return p.token, nil
	}

	body, _ := json.Marshal(map[string]string{"apikey": p.apiKey})
	req, err := http.NewRequest(http.MethodPost, p.baseURL+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TheTVDB login failed (%d)", resp.StatusCode)
	}

	var raw struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw) != nil || strings.TrimSpace(raw.Data.Token) == "" {
		return "", fmt.Errorf("invalid TheTVDB login response")
	}
	p.token = strings.TrimSpace(raw.Data.Token)
	p.tokenExpiry = time.Now().Add(25 * 24 * time.Hour)
	return p.token, nil
}

func (p *TVDBProvider) get(path string, target any) error {
	token, err := p.bearer()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BuddyFlix/"+Version)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TheTVDB request failed (%d)", resp.StatusCode)
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(target) != nil {
		return fmt.Errorf("invalid TheTVDB response")
	}
	return nil
}

func normalizeTVDBImage(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://") {
		return v
	}
	if strings.HasPrefix(v, "/") {
		return "https://artworks.thetvdb.com" + v
	}
	return "https://artworks.thetvdb.com/banners/" + strings.TrimPrefix(v, "/")
}

func cleanMetadataQuery(title string) string {
	parts := strings.Fields(strings.NewReplacer(".", " ", "_", " ").Replace(title))
	if len(parts) == 0 { return strings.TrimSpace(title) }

	isReleaseToken := func(v string) bool {
		v = strings.ToLower(strings.Trim(v, "[](){}.-_ "))
		if v == "" { return false }
		switch v {
		case "2160p", "1080p", "720p", "576p", "480p", "4k", "uhd",
			"bluray", "brrip", "bdrip", "webrip", "webdl", "web-dl", "web", "hdtv", "dvdrip", "hdrip", "remux",
			"x264", "x265", "h264", "h265", "hevc", "avc", "xvid", "10bit", "8bit",
			"hdr", "hdr10", "hdr10plus", "dolbyvision", "dv",
			"dts", "dtshd", "ac3", "eac3", "aac", "ddp", "truehd", "atmos", "flac",
			"german", "deutsch", "dl", "dubbed", "subbed", "multi", "multilang", "internal",
			"proper", "repack", "unrated", "extended", "directorscut", "retail":
			return true
		}
		if strings.HasPrefix(v, "cd") || strings.HasPrefix(v, "disc") {
			if len(v) > 2 { return true }
		}
		return false
	}

	cut := len(parts)
	for i, p := range parts {
		if isReleaseToken(p) {
			cut = i
			break
		}
	}
	if cut == 0 { return strings.TrimSpace(title) }
	return strings.TrimSpace(strings.Join(parts[:cut], " "))
}

func (p *TVDBProvider) searchMovieOnce(title string, year int) ([]MetadataCandidate, error) {
	q := url.Values{"query": {title}, "type": {"movie"}, "limit": {"12"}}
	if year > 0 { q.Set("year", strconv.Itoa(year)) }

	var raw struct {
		Data []struct {
			ID             string            `json:"id"`
			TVDBID         string            `json:"tvdb_id"`
			Name           string            `json:"name"`
			NameTranslated string            `json:"name_translated"`
			Title          string            `json:"title"`
			Aliases        []string          `json:"aliases"`
			Year           string            `json:"year"`
			Overview       string            `json:"overview"`
			Overviews      map[string]string `json:"overviews"`
			Translations   map[string]string `json:"translations"`
			ImageURL       string            `json:"image_url"`
			Poster         string            `json:"poster"`
			RemoteIDs      []struct {
				ID         string `json:"id"`
				SourceName string `json:"sourceName"`
			} `json:"remote_ids"`
		} `json:"data"`
	}
	if err := p.get("/search?"+q.Encode(), &raw); err != nil { return nil, err }

	out := make([]MetadataCandidate, 0, len(raw.Data))
	for _, z := range raw.Data {
		id := strings.TrimSpace(z.TVDBID)
		if id == "" { id = strings.TrimSpace(z.ID) }
		if id == "" { continue }

		name := strings.TrimSpace(z.Translations["deu"])
		if name == "" { name = strings.TrimSpace(z.NameTranslated) }
		if name == "" { name = strings.TrimSpace(z.Name) }
		if name == "" { name = strings.TrimSpace(z.Title) }
		y, _ := strconv.Atoi(strings.TrimSpace(z.Year))
		overview := strings.TrimSpace(z.Overviews["deu"])
		if overview == "" { overview = strings.TrimSpace(z.Overview) }
		poster := normalizeTVDBImage(z.Poster)
		if poster == "" { poster = normalizeTVDBImage(z.ImageURL) }

		external := map[string]string{"tvdb": id}
		for _, rid := range z.RemoteIDs {
			src := strings.ToLower(rid.SourceName)
			switch {
			case strings.Contains(src, "imdb"):
				external["imdb"] = strings.TrimSpace(rid.ID)
			case strings.Contains(src, "movie database"), strings.Contains(src, "tmdb"):
				external["tmdb"] = strings.TrimSpace(rid.ID)
			}
		}

		alts := make([]string, 0, len(z.Aliases)+len(z.Translations)+2)
		for _, a := range z.Aliases {
			a = strings.TrimSpace(a)
			if a != "" && !strings.EqualFold(a, name) { alts = append(alts, a) }
		}
		for _, translated := range z.Translations {
			translated = strings.TrimSpace(translated)
			if translated != "" && !strings.EqualFold(translated, name) { alts = append(alts, translated) }
		}
		if z.Name != "" && !strings.EqualFold(z.Name, name) { alts = append(alts, strings.TrimSpace(z.Name)) }
		if z.Title != "" && !strings.EqualFold(z.Title, name) { alts = append(alts, strings.TrimSpace(z.Title)) }

		out = append(out, MetadataCandidate{
			Provider: p.Name(), ProviderID: id, Title: name, Year: y,
			Overview: overview, Poster: poster, ExternalIDs: external, AlternateTitles: alts,
		})
	}
	return out, nil
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(strings.Join(strings.Fields(item), " "))
		if item == "" { continue }
		key := strings.ToLower(item)
		if seen[key] { continue }
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func germanSearchVariants(title string) []string {
	cleaned := cleanMetadataQuery(title)
	punct := strings.NewReplacer(
		":", " ",
		" - ", " ",
		" – ", " ",
		" — ", " ",
		"&", " und ",
		"'", " ",
		"’", " ",
	).Replace(cleaned)

	short := cleaned
	for _, sep := range []string{":", " - ", " – ", " — "} {
		if i := strings.Index(short, sep); i > 3 {
			short = strings.TrimSpace(short[:i])
			break
		}
	}

	deASCII := cleaned
	if !strings.ContainsAny(deASCII, "äöüÄÖÜ") {
		deASCII = strings.NewReplacer(
			"ae", "ä", "Ae", "Ä", "AE", "Ä",
			"oe", "ö", "Oe", "Ö", "OE", "Ö",
			"ue", "ü", "Ue", "Ü", "UE", "Ü",
		).Replace(deASCII)
	}

	ascii := strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss",
	).Replace(cleaned)

	return uniqueStrings([]string{strings.TrimSpace(title), cleaned, punct, short, deASCII, ascii})
}

func (p *TVDBProvider) germanTranslation(id string) (string, string, error) {
	var tr struct {
		Data struct {
			Name     string `json:"name"`
			Overview string `json:"overview"`
		} `json:"data"`
	}
	if err := p.get("/movies/"+url.PathEscape(id)+"/translations/deu", &tr); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(tr.Data.Name), strings.TrimSpace(tr.Data.Overview), nil
}

func (p *TVDBProvider) RefineMovieCandidates(candidates []MetadataCandidate) ([]MetadataCandidate, error) {
	limit := len(candidates)
	if limit > 5 { limit = 5 }
	out := append([]MetadataCandidate(nil), candidates...)
	var lastErr error

	for i := 0; i < limit; i++ {
		id := strings.TrimSpace(out[i].ProviderID)
		if id == "" { continue }
		name, overview, err := p.germanTranslation(id)
		if err != nil {
			lastErr = err
			continue
		}
		if name != "" {
			found := strings.EqualFold(name, out[i].Title)
			for _, alt := range out[i].AlternateTitles {
				if strings.EqualFold(name, alt) { found = true; break }
			}
			if !found { out[i].AlternateTitles = append(out[i].AlternateTitles, name) }
		}
		if overview != "" && out[i].Overview == "" { out[i].Overview = overview }
	}

	if lastErr != nil && len(out) == 0 { return candidates, lastErr }
	return out, nil
}

func (p *TVDBProvider) SearchMovie(title string, year int) ([]MetadataCandidate, error) {
	variants := germanSearchVariants(title)
	var lastErr error

	for _, query := range variants {
		results, err := p.searchMovieOnce(query, year)
		if err != nil {
			lastErr = err
			continue
		}
		if len(results) > 0 { return results, nil }
	}

	if year > 0 {
		limit := len(variants)
		if limit > 3 { limit = 3 }
		for _, query := range variants[:limit] {
			results, err := p.searchMovieOnce(query, 0)
			if err != nil {
				lastErr = err
				continue
			}
			if len(results) > 0 { return results, nil }
		}
	}

	if lastErr != nil { return nil, lastErr }
	return []MetadataCandidate{}, nil
}

func (p *TVDBProvider) EnrichMovie(candidate MetadataCandidate) (MetadataCandidate, error) {
	id := strings.TrimSpace(candidate.ProviderID)
	if id == "" && candidate.ExternalIDs != nil {
		id = strings.TrimSpace(candidate.ExternalIDs["tvdb"])
	}
	if id == "" {
		return candidate, nil
	}

	var raw struct {
		Data struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Year      string `json:"year"`
			Image     string `json:"image"`
			RemoteIDs []struct {
				ID         string `json:"id"`
				SourceName string `json:"sourceName"`
			} `json:"remoteIds"`
			Artworks []struct {
				Image  string  `json:"image"`
				Width  int     `json:"width"`
				Height int     `json:"height"`
				Score  float64 `json:"score"`
			} `json:"artworks"`
		} `json:"data"`
	}
	if err := p.get("/movies/"+url.PathEscape(id)+"/extended", &raw); err != nil {
		return candidate, err
	}

	candidate.Provider = p.Name()
	candidate.ProviderID = id
	if candidate.ExternalIDs == nil {
		candidate.ExternalIDs = map[string]string{}
	}
	candidate.ExternalIDs["tvdb"] = id

	if candidate.Title == "" && raw.Data.Name != "" {
		candidate.Title = strings.TrimSpace(raw.Data.Name)
	}
	if candidate.Year == 0 {
		candidate.Year, _ = strconv.Atoi(strings.TrimSpace(raw.Data.Year))
	}
	if candidate.Poster == "" {
		candidate.Poster = normalizeTVDBImage(raw.Data.Image)
	}

	for _, rid := range raw.Data.RemoteIDs {
		src := strings.ToLower(rid.SourceName)
		switch {
		case strings.Contains(src, "imdb"):
			candidate.ExternalIDs["imdb"] = strings.TrimSpace(rid.ID)
		case strings.Contains(src, "movie database"), strings.Contains(src, "tmdb"):
			candidate.ExternalIDs["tmdb"] = strings.TrimSpace(rid.ID)
		}
	}

	bestPortraitArea, bestLandscapeArea := 0, 0
	for _, a := range raw.Data.Artworks {
		if a.Width <= 0 || a.Height <= 0 {
			continue
		}
		img := normalizeTVDBImage(a.Image)
		if img == "" {
			continue
		}
		area := a.Width * a.Height
		if a.Height > a.Width && area > bestPortraitArea {
			bestPortraitArea = area
			if candidate.Poster == "" {
				candidate.Poster = img
			}
		}
		if a.Width*10 >= a.Height*13 && area > bestLandscapeArea {
			bestLandscapeArea = area
			candidate.Backdrop = img
		}
	}

	if name, overview, err := p.germanTranslation(id); err == nil {
		if name != "" { candidate.Title = name }
		if overview != "" { candidate.Overview = overview }
	}

	return candidate, nil
}
