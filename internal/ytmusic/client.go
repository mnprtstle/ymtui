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
	songsParams   = "EgWKAQIIAWoMEA4QChADEAQQCRAF"
	albumsParams  = "EgWKAQIYAWoMEA4QChADEAQQCRAF"
	artistsParams = "EgWKAQIgAWoMEA4QChADEAQQCRAF"
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
	data, err := c.search(ctx, query, songsParams)
	if err != nil {
		return nil, err
	}
	songs, err := parseSongs(data)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	return songs, nil
}

// SearchAlbums returns albums, singles and EPs matching query.
func (c *Client) SearchAlbums(ctx context.Context, query string) ([]Album, error) {
	data, err := c.search(ctx, query, albumsParams)
	if err != nil {
		return nil, err
	}
	albums, err := parseAlbums(data)
	if err != nil {
		return nil, fmt.Errorf("search albums %q: %w", query, err)
	}
	return albums, nil
}

// SearchArtists returns artists matching query.
func (c *Client) SearchArtists(ctx context.Context, query string) ([]Artist, error) {
	data, err := c.search(ctx, query, artistsParams)
	if err != nil {
		return nil, err
	}
	artists, err := parseArtists(data)
	if err != nil {
		return nil, fmt.Errorf("search artists %q: %w", query, err)
	}
	return artists, nil
}

// Album loads an album page by its browse ID (MPREb_...).
func (c *Client) Album(ctx context.Context, browseID string) (AlbumPage, error) {
	data, err := c.browse(ctx, browseID)
	if err != nil {
		return AlbumPage{}, err
	}
	page, err := parseAlbumPage(data)
	if err != nil {
		return AlbumPage{}, fmt.Errorf("album %q: %w", browseID, err)
	}
	return page, nil
}

// Artist loads an artist page by its channel ID (UC...).
func (c *Client) Artist(ctx context.Context, browseID string) (ArtistPage, error) {
	data, err := c.browse(ctx, browseID)
	if err != nil {
		return ArtistPage{}, err
	}
	page, err := parseArtistPage(data)
	if err != nil {
		return ArtistPage{}, fmt.Errorf("artist %q: %w", browseID, err)
	}
	return page, nil
}

func (c *Client) search(ctx context.Context, query, params string) ([]byte, error) {
	data, err := c.post(ctx, "search", map[string]any{"query": query, "params": params})
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	return data, nil
}

func (c *Client) browse(ctx context.Context, browseID string) ([]byte, error) {
	data, err := c.post(ctx, "browse", map[string]any{"browseId": browseID})
	if err != nil {
		return nil, fmt.Errorf("browse %q: %w", browseID, err)
	}
	return data, nil
}

// post sends one InnerTube request. Every endpoint shares the same envelope
// (the "context" block), so callers pass only their own fields; adding the
// envelope here is what lets Search, Album and Artist stay tiny.
func (c *Client) post(ctx context.Context, endpoint string, fields map[string]any) ([]byte, error) {
	fields["context"] = map[string]any{"client": map[string]string{
		"clientName":    "WEB_REMIX",
		"clientVersion": "1." + time.Now().UTC().Format("20060102") + ".01.00",
		"hl":            "en",
	}}
	body, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/youtubei/v1/"+endpoint+"?alt=json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", defaultBaseURL)
	req.Header.Set("User-Agent", userAgent)
	req.AddCookie(&http.Cookie{Name: "SOCS", Value: "CAI"})

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // Go doesn't close it for us; leaking stalls connection reuse.

	// A 4xx/5xx is not an error from Do, so check it explicitly.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return data, nil
}
