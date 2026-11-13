package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/player"
	"github.com/mnprtstle/ymtui/internal/store"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// Catalog and Player are the slices of ytmusic.Client and player.Player the
// UI needs. In Go, interfaces are satisfied implicitly and belong to the
// consumer: neither of those packages knows these interfaces exist, and tests
// can pass fakes.
type Catalog interface {
	Search(ctx context.Context, query string) ([]ytmusic.Song, error)
	SearchAlbums(ctx context.Context, query string) ([]ytmusic.Album, error)
	SearchArtists(ctx context.Context, query string) ([]ytmusic.Artist, error)
	Album(ctx context.Context, browseID string) (ytmusic.AlbumPage, error)
	Artist(ctx context.Context, browseID string) (ytmusic.ArtistPage, error)
}

type Player interface {
	Load(ctx context.Context, url string) error
	SetPause(ctx context.Context, paused bool) error
	Seek(ctx context.Context, secs float64) error
	SetVolume(ctx context.Context, percent float64) error
}

// PlaylistStore is the slice of store.Store the UI needs.
type PlaylistStore interface {
	Playlists() []store.Playlist
	Create(name string) error
	Delete(name string) error
	AddSong(name string, song ytmusic.Song) (added bool, err error)
	RemoveSong(name string, idx int) error
}

// Messages. Each is a distinct type so Update can type-switch on them.
type (
	searchResultsMsg struct {
		query string
		tab   tab
		rows  []row
	}
	// pageLoadedMsg carries an album or artist page; id says which request it
	// answers, so a page the user already backed out of can be dropped.
	pageLoadedMsg struct {
		id   string
		rows []row
	}
	pageErrMsg struct {
		id  string
		err error
	}
	// playerEventMsg wraps an mpv event delivered through program.Send.
	playerEventMsg struct{ ev player.Event }
	// errMsg reports a failed Cmd.
	errMsg struct{ err error }
)

const cmdTimeout = 30 * time.Second // loadfile can wait on yt-dlp

// searchCmd returns a tea.Cmd. Bubble Tea runs it in its own goroutine, so the
// blocking network call never stalls Update.
func searchCmd(c Catalog, query string, t tab) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		var (
			rows []row
			err  error
		)
		switch t {
		case tabAlbums:
			var albums []ytmusic.Album
			albums, err = c.SearchAlbums(ctx, query)
			rows = albumRows(albums)
		case tabArtists:
			var artists []ytmusic.Artist
			artists, err = c.SearchArtists(ctx, query)
			rows = artistRows(artists)
		default:
			var songs []ytmusic.Song
			songs, err = c.Search(ctx, query)
			rows = songRows(songs)
		}
		if err != nil {
			return errMsg{fmt.Errorf("search %q: %w", query, err)}
		}
		return searchResultsMsg{query: query, tab: t, rows: rows}
	}
}

// albumCmd and artistCmd load a page. Unlike a search failure, a page failure
// names the page so Update can remove the "Loading…" view it left behind.
func albumCmd(c Catalog, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		page, err := c.Album(ctx, id)
		if err != nil {
			return pageErrMsg{id, fmt.Errorf("load album: %w", err)}
		}
		return pageLoadedMsg{id: id, rows: songRows(page.Songs)}
	}
}

func artistCmd(c Catalog, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		page, err := c.Artist(ctx, id)
		if err != nil {
			return pageErrMsg{id, fmt.Errorf("load artist: %w", err)}
		}
		return pageLoadedMsg{id: id, rows: artistPageRows(page)}
	}
}

// playerCmd runs one player call and reports only failures (nil msg = nothing).
func playerCmd(what string, f func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := f(ctx); err != nil {
			return errMsg{fmt.Errorf("%s: %w", what, err)}
		}
		return nil
	}
}

// Pump forwards mpv events to the program. Run it in a goroutine once the
// program exists; it returns when the events channel is closed.
func Pump(p *tea.Program, events <-chan player.Event) {
	for ev := range events {
		p.Send(playerEventMsg{ev})
	}
}

// Observed mpv properties, with the IDs used for ObserveProperty.
var ObservedProperties = []string{"time-pos", "duration", "pause", "volume", "media-title"}
