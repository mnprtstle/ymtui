package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/mpris"
)

// Media is the slice of mpris.Server the UI needs; nil means "no media keys".
type Media interface {
	SetState(mpris.State)
}

// MediaMsg carries a command from a D-Bus client (media key, playerctl).
type MediaMsg struct{ Cmd mpris.Command }

// WithMedia attaches an MPRIS server whose state follows the model.
func (m Model) WithMedia(media Media) Model {
	m.media = media
	return m
}

// mediaState snapshots what remote controls should see.
func (m Model) mediaState() mpris.State {
	st := mpris.State{
		HasTrack: m.now != nil,
		Playing:  !m.paused,
		Length:   secs(m.dur),
		Position: secs(m.pos),
		Volume:   m.volume / 100,
		CanNext:  m.queue.Len() > 0,
		CanPrev:  m.queue.Len() > 0,
	}
	if m.now != nil {
		st.VideoID, st.Title, st.Artists, st.Album = m.now.VideoID, m.now.Title, m.now.Artists, m.now.Album
	}
	return st
}

func secs(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

// handleMedia runs a remote command. Play/pause only mean something once a
// song is loaded; before that there is nothing to resume.
func (m *Model) handleMedia(c mpris.Command) tea.Cmd {
	p := m.player
	setPause := func(pause bool) tea.Cmd {
		if m.now == nil {
			return nil
		}
		return playerCmd("pause", func(ctx context.Context) error { return p.SetPause(ctx, pause) })
	}
	switch c.Kind {
	case mpris.Play:
		return setPause(false)
	case mpris.Pause, mpris.Stop:
		return setPause(true)
	case mpris.PlayPause:
		return setPause(!m.paused)
	case mpris.Next:
		return m.advance(false)
	case mpris.Previous:
		return m.previous()
	case mpris.SeekBy:
		if m.now == nil {
			return nil
		}
		return m.seek(c.Offset.Seconds())
	case mpris.SetVolume:
		return m.changeVolume(c.Volume*100 - m.volume)
	}
	return nil
}
