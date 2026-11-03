package ytmusic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://music.youtube.com"
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0"
	// songsParams is InnerTube's encoded "filter: songs" search option
	// (from ytmusicapi).
	songsParams = "EgWKAQIIAWoMEA4QChADEAQQCRAF"
	// maxBody caps how much of a response we read, as a guard against a
	// runaway server. Real search responses are under 1 MB.
	maxBody = 16 << 20
)

// Client calls the YouTube Music InnerTube API.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient returns a Client using the real YouTube Music endpoint.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}, baseURL: defaultBaseURL}
}

// newTestClient points a Client at another server (tests use httptest).
func newTestClient(h *http.Client, baseURL string) *Client {
	return &Client{http: h, baseURL: baseURL}
}

// Search returns songs matching query.
func (c *Client) Search(ctx context.Context, query string) ([]Song, error) {
	body, err := json.Marshal(map[string]any{
		"query":  query,
		"params": songsParams,
		"context": map[string]any{"client": map[string]string{
			"clientName":    "WEB_REMIX",
			"clientVersion": "1." + time.Now().UTC().Format("20060102") + ".01.00",
			"hl":            "en",
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/youtubei/v1/search?alt=json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", defaultBaseURL)
	req.Header.Set("User-Agent", userAgent)
	req.AddCookie(&http.Cookie{Name: "SOCS", Value: "CAI"})

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer resp.Body.Close() // Go doesn't close it for us; leaking stalls connection reuse.

	// A 4xx/5xx is not an error from Do, so check it explicitly.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search %q: unexpected status %s", query, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("search %q: read response: %w", query, err)
	}
	songs, err := parseSongs(data)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	return songs, nil
}
