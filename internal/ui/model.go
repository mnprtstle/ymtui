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

	"github.com/mnprtstle/ymtui/internal/queue"
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

	queue            *queue.Queue
	showQueue        bool // toggled with Q: queue list instead of results
	qCursor, qOffset int
	notice           string // transient status message, cleared on next key

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
	return Model{search: s, player: p, input: in, volume: 100, queue: queue.New(nil)}
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
		m.err, m.notice = nil, ""
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
		// Only a natural end ("eof") advances. Our own loadfile replacing a
		// track also fires end-file, with reason "stop"; reacting to that
		// would skip a song on every manual change.
		if msg.ev.Name == "end-file" && msg.ev.Reason == "eof" {
			return m, m.advance(true)
		}
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
			m.moveCursor(-m.listLen())
		} else {
			m.pendingG = true
		}
	case "G":
		m.moveCursor(m.listLen())
	case "enter", "l":
		return m, m.playSelected()
	case "Q":
		m.showQueue = !m.showQueue
		m.scrollToCursor()
	case "a":
		if !m.showQueue && len(m.songs) > 0 {
			s := m.songs[m.cursor]
			m.queue.Add(s)
			m.notice = "added: " + s.Title
		}
	case "d":
		if m.showQueue {
			m.queue.Remove(m.qCursor)
			m.moveCursor(0) // clamp
		}
	case "n":
		return m, m.advance(false)
	case "p":
		return m, m.previous()
	case "s":
		on := m.queue.ToggleShuffle()
		m.notice = "shuffle " + map[bool]string{true: "on", false: "off"}[on]
	case "r":
		m.notice = "repeat " + m.queue.CycleRepeat().String()
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

// listLen is the length of whichever list is on screen.
func (m *Model) listLen() int {
	if m.showQueue {
		return m.queue.Len()
	}
	return len(m.songs)
}

// activeList returns pointers to the cursor and scroll offset of the list on
// screen. Pointers let one function edit either pair of fields.
func (m *Model) activeList() (cursor, offset *int) {
	if m.showQueue {
		return &m.qCursor, &m.qOffset
	}
	return &m.cursor, &m.offset
}

func (m *Model) moveCursor(delta int) {
	n := m.listLen()
	if n == 0 {
		return
	}
	c, o := m.activeList()
	*c = min(max(*c+delta, 0), n-1)
	m.scroll(*c, o)
}

// scrollToCursor re-clamps both lists, e.g. after a resize or view switch.
func (m *Model) scrollToCursor() {
	m.scroll(m.cursor, &m.offset)
	m.scroll(m.qCursor, &m.qOffset)
}

func (m *Model) scroll(cursor int, offset *int) {
	rows := m.rows()
	if cursor < *offset {
		*offset = cursor
	} else if cursor >= *offset+rows {
		*offset = cursor - rows + 1
	}
	*offset = max(*offset, 0)
}

// playSelected handles Enter. On results it replaces the queue with the whole
// result list, starting at the cursor, so playback continues down the list.
// In the queue view it jumps to the selected entry.
func (m *Model) playSelected() tea.Cmd {
	if m.showQueue {
		if song, ok := m.queue.Jump(m.qCursor); ok {
			return m.playSong(song)
		}
		return nil
	}
	if song, ok := m.queue.Replace(m.songs, m.cursor); ok {
		return m.playSong(song)
	}
	return nil
}

// advance plays the next queued song. auto is true for end-of-track.
func (m *Model) advance(auto bool) tea.Cmd {
	if song, ok := m.queue.Next(auto); ok {
		return m.playSong(song)
	}
	return nil
}

// previous restarts the song if it has played for a few seconds, otherwise
// goes back one.
func (m *Model) previous() tea.Cmd {
	if m.pos > 3 && m.now != nil {
		return m.seek(-m.pos) // seek is relative
	}
	if song, ok := m.queue.Prev(); ok {
		return m.playSong(song)
	}
	return nil
}

func (m *Model) playSong(song ytmusic.Song) tea.Cmd {
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
