package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/player"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// Searcher and Player are the slices of ytmusic.Client and player.Player the
// UI needs. In Go, interfaces are satisfied implicitly and belong to the
// consumer: neither of those packages knows these interfaces exist, and tests
// can pass fakes.
type Searcher interface {
	Search(ctx context.Context, query string) ([]ytmusic.Song, error)
}

type Player interface {
	Load(ctx context.Context, url string) error
	SetPause(ctx context.Context, paused bool) error
	Seek(ctx context.Context, secs float64) error
	SetVolume(ctx context.Context, percent float64) error
}

// Messages. Each is a distinct type so Update can type-switch on them.
type (
	searchResultsMsg struct {
		query string
		songs []ytmusic.Song
	}
	// playerEventMsg wraps an mpv event delivered through program.Send.
	playerEventMsg struct{ ev player.Event }
	// errMsg reports a failed Cmd.
	errMsg struct{ err error }
)

const cmdTimeout = 30 * time.Second // loadfile can wait on yt-dlp

// searchCmd returns a tea.Cmd. Bubble Tea runs it in its own goroutine, so the
// blocking network call never stalls Update.
func searchCmd(s Searcher, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		songs, err := s.Search(ctx, query)
		if err != nil {
			return errMsg{fmt.Errorf("search %q: %w", query, err)}
		}
		return searchResultsMsg{query: query, songs: songs}
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
