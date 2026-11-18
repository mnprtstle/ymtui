package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/queue"
	"github.com/mnprtstle/ymtui/internal/session"
)

// Session snapshots what should survive a restart.
func (m Model) Session() session.State {
	st := session.State{
		Songs:    m.queue.Songs(),
		Index:    m.queue.Position(),
		Position: m.pos,
		Duration: m.dur,
		Repeat:   int(m.queue.Repeat()),
	}
	return st
}

// WithSession restores a saved session: the queue and the track appear in the
// player bar, paused. Nothing is loaded into mpv yet, because that would make
// yt-dlp fetch a stream at every startup. pendingResume marks this state; the
// first "play" loads the track and seeks to the saved position.
func (m Model) WithSession(s session.State) Model {
	if !s.Valid() {
		return m
	}
	song, _ := m.queue.Replace(s.Songs, s.Index)
	m.queue.SetRepeat(queue.Repeat(s.Repeat))
	m.now = &song
	m.pos, m.dur = s.Position, s.Duration
	m.paused = true
	m.pendingResume = true
	return m
}

// startResume loads the restored track. The seek to the saved position waits
// for mpv's "file-loaded" event (see Update): seeking earlier would hit an
// empty player.
func (m *Model) startResume() tea.Cmd {
	m.pendingResume = false
	m.seekOnLoad = m.pos
	m.paused = false
	p := m.player
	url := "https://music.youtube.com/watch?v=" + m.now.VideoID
	return playerCmd("resume", func(ctx context.Context) error {
		if err := p.Load(ctx, url); err != nil {
			return err
		}
		return p.SetPause(ctx, false)
	})
}

// setPause pauses or resumes, treating a restored-but-unloaded track as
// "resume means load it".
func (m *Model) setPause(pause bool) tea.Cmd {
	if m.now == nil {
		return nil
	}
	if m.pendingResume {
		if pause {
			return nil
		}
		return m.startResume()
	}
	p := m.player
	return playerCmd("pause", func(ctx context.Context) error { return p.SetPause(ctx, pause) })
}
