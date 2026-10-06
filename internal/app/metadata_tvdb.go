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

func (p *TVDBProvider) SearchMovie(title string, year int) ([]MetadataCandidate, error) {
	q := url.Values{"query": {title}, "type": {"movie"}, "limit": {"12"}}
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}

	var raw struct {
		Data []struct {
			ID             string            `json:"id"`
			TVDBID         string            `json:"tvdb_id"`
			Name           string            `json:"name"`
			NameTranslated string            `json:"name_translated"`
			Title          string            `json:"title"`
			Year           string            `json:"year"`
			Overview       string            `json:"overview"`
			Overviews      map[string]string `json:"overviews"`
			ImageURL       string            `json:"image_url"`
			Poster         string            `json:"poster"`
			RemoteIDs      []struct {
				ID         string `json:"id"`
				SourceName string `json:"sourceName"`
			} `json:"remote_ids"`
		} `json:"data"`
	}
	if err := p.get("/search?"+q.Encode(), &raw); err != nil {
		return nil, err
	}

	out := make([]MetadataCandidate, 0, len(raw.Data))
	for _, z := range raw.Data {
		id := strings.TrimSpace(z.TVDBID)
		if id == "" {
			id = strings.TrimSpace(z.ID)
		}
		if id == "" {
			continue
		}

		name := strings.TrimSpace(z.NameTranslated)
		if name == "" {
			name = strings.TrimSpace(z.Name)
		}
		if name == "" {
			name = strings.TrimSpace(z.Title)
		}
		y, _ := strconv.Atoi(strings.TrimSpace(z.Year))

		overview := strings.TrimSpace(z.Overviews["deu"])
		if overview == "" {
			overview = strings.TrimSpace(z.Overview)
		}

		poster := normalizeTVDBImage(z.Poster)
		if poster == "" {
			poster = normalizeTVDBImage(z.ImageURL)
		}

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

		out = append(out, MetadataCandidate{
			Provider:    p.Name(),
			ProviderID:  id,
			Title:       name,
			Year:        y,
			Overview:    overview,
			Poster:      poster,
			ExternalIDs: external,
		})
	}
	return out, nil
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

	var tr struct {
		Data struct {
			Name     string `json:"name"`
			Overview string `json:"overview"`
		} `json:"data"`
	}
	if err := p.get("/movies/"+url.PathEscape(id)+"/translations/deu", &tr); err == nil {
		if strings.TrimSpace(tr.Data.Name) != "" {
			candidate.Title = strings.TrimSpace(tr.Data.Name)
		}
		if strings.TrimSpace(tr.Data.Overview) != "" {
			candidate.Overview = strings.TrimSpace(tr.Data.Overview)
		}
	}

	return candidate, nil
}
