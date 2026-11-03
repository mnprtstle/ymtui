// Package ui holds the Bubble Tea models. It depends on the other internal
// packages (through small interfaces); they never depend on it.
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
)

// Model is the whole UI state. Bubble Tea follows the Elm architecture:
// Update(msg) returns a new state, View() renders it, and nothing else
// touches the state.
type Model struct {
	search Searcher
	player Player

	width, height int
	mode          mode
	input         textinput.Model

	query     string
	songs     []ytmusic.Song
	cursor    int
	offset    int // index of the first visible row
	pendingG  bool
	searching bool

	now      *ytmusic.Song // track we asked mpv to play
	title    string        // mpv's media-title, used until we have a Song
	pos, dur float64       // seconds
	paused   bool
	volume   float64
	err      error
}

// New builds the initial model.
func New(s Searcher, p Player) Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "press / to search"
	in.PromptStyle = accentStyle
	in.CharLimit = 200
	return Model{search: s, player: p, input: in, volume: 100}
}

// Init runs once at startup; returning nil means no initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles one message. It must never block: slow work is returned as
// a tea.Cmd, whose result arrives later as another message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(msg.Width-8, 10)
		m.scrollToCursor()
		return m, nil

	case tea.KeyMsg:
		m.err = nil
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.mode == modeSearch {
			return m.updateSearch(msg)
		}
		return m.updateNormal(msg)

	case searchResultsMsg:
		if msg.query != m.query {
			return m, nil // a newer search superseded this one
		}
		m.searching = false
		m.songs, m.cursor, m.offset = msg.songs, 0, 0
		return m, nil

	case playerEventMsg:
		m.applyEvent(msg)
		return m, nil

	case errMsg:
		m.searching = false
		m.err = msg.err
		return m, nil
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		q := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if q == "" {
			return m, nil
		}
		m.query, m.searching = q, true
		return m, searchCmd(m.search, q)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	wasG := m.pendingG
	m.pendingG = false

	switch key {
	case "q":
		return m, tea.Quit
	case "/":
		m.mode = modeSearch
		return m, m.input.Focus()
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "ctrl+d":
		m.moveCursor(m.rows() / 2)
	case "ctrl+u":
		m.moveCursor(-m.rows() / 2)
	case "g":
		if wasG {
			m.moveCursor(-len(m.songs))
		} else {
			m.pendingG = true
		}
	case "G":
		m.moveCursor(len(m.songs))
	case "enter", "l":
		return m, m.playSelected()
	case " ":
		p := m.player
		pause := !m.paused
		return m, playerCmd("pause", func(ctx context.Context) error { return p.SetPause(ctx, pause) })
	case "H", "left":
		return m, m.seek(-5)
	case "L", "right":
		return m, m.seek(5)
	case "+", "=":
		return m, m.changeVolume(5)
	case "-":
		return m, m.changeVolume(-5)
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	if len(m.songs) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.songs)-1)
	m.scrollToCursor()
}

func (m *Model) scrollToCursor() {
	rows := m.rows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(m.offset, 0)
}

func (m *Model) playSelected() tea.Cmd {
	if len(m.songs) == 0 {
		return nil
	}
	song := m.songs[m.cursor]
	m.now = &song
	m.pos, m.dur = 0, 0
	p := m.player
	url := "https://music.youtube.com/watch?v=" + song.VideoID
	return playerCmd("play", func(ctx context.Context) error { return p.Load(ctx, url) })
}

func (m Model) seek(secs float64) tea.Cmd {
	p := m.player
	return playerCmd("seek", func(ctx context.Context) error { return p.Seek(ctx, secs) })
}

func (m *Model) changeVolume(delta float64) tea.Cmd {
	m.volume = min(max(m.volume+delta, 0), 100)
	p, v := m.player, m.volume
	return playerCmd("volume", func(ctx context.Context) error { return p.SetVolume(ctx, v) })
}

// applyEvent folds an mpv event into the state. Property values arrive as
// raw JSON; "null" (property unavailable) decodes without error and leaves
// the zero value, which is what we want.
func (m *Model) applyEvent(msg playerEventMsg) {
	ev := msg.ev
	switch ev.Name {
	case "property-change":
		switch ev.Property {
		case "time-pos":
			var f float64
			json.Unmarshal(ev.Data, &f)
			m.pos = f
		case "duration":
			var f float64
			json.Unmarshal(ev.Data, &f)
			m.dur = f
		case "pause":
			json.Unmarshal(ev.Data, &m.paused)
		case "volume":
			json.Unmarshal(ev.Data, &m.volume)
		case "media-title":
			json.Unmarshal(ev.Data, &m.title)
		}
	case "end-file":
		if ev.Reason == "error" {
			m.err = fmt.Errorf("playback failed: %s", ev.FileError)
		}
	}
}
