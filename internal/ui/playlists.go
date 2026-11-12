package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mnprtstle/ymtui/internal/store"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// picker is the "add to playlist" overlay: a list of playlists with a final
// "New playlist…" entry. While it is open the model is in modePicker and
// every key goes to updatePicker instead of the normal handler. This is how
// Bubble Tea does modal UI: a mode field, and Update dispatches on it.
type picker struct {
	song   ytmusic.Song
	lists  []store.Playlist
	cursor int // len(lists) is the "New playlist…" entry
}

// newPlaylistEntry is the picker index of the "New playlist…" line.
func (p picker) newPlaylistEntry() int { return len(p.lists) }

// openPicker shows the overlay for adding song to a playlist.
func (m *Model) openPicker(song ytmusic.Song) {
	m.picker = picker{song: song, lists: m.playlists.Playlists()}
	m.mode = modePicker
}

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.picker
	switch msg.String() {
	case "esc", "q", "h":
		m.mode = modeNormal
	case "j", "down":
		p.cursor = min(p.cursor+1, p.newPlaylistEntry())
	case "k", "up":
		p.cursor = max(p.cursor-1, 0)
	case "enter", "l":
		if p.cursor == p.newPlaylistEntry() {
			return m, m.startNaming(&p.song)
		}
		m.mode = modeNormal
		m.addToPlaylist(p.lists[p.cursor].Name, p.song)
	}
	return m, nil
}

// startNaming switches to the name prompt. If song is non-nil it is added to
// the playlist once created. (A pointer is the usual Go way to say "optional
// value": nil means absent.)
func (m *Model) startNaming(song *ytmusic.Song) tea.Cmd {
	m.mode = modeName
	m.naming = song
	m.nameInput.SetValue("")
	return m.nameInput.Focus()
}

func (m Model) updateName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.nameInput.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		m.mode = modeNormal
		m.nameInput.Blur()
		if name == "" {
			return m, nil
		}
		if err := m.playlists.Create(name); err != nil {
			m.err = fmt.Errorf("create playlist: %w", err)
			return m, nil
		}
		if m.naming != nil {
			m.addToPlaylist(name, *m.naming)
		} else {
			m.notice = "created playlist: " + name
			m.refreshPlaylists()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// addToPlaylist saves song and reports the outcome in the status line.
func (m *Model) addToPlaylist(name string, song ytmusic.Song) {
	added, err := m.playlists.AddSong(name, song)
	switch {
	case err != nil:
		m.err = fmt.Errorf("add to playlist: %w", err)
	case !added:
		m.notice = fmt.Sprintf("already in %s: %s", name, song.Title)
	default:
		m.notice = fmt.Sprintf("added to %s: %s", name, song.Title)
	}
	m.refreshPlaylists()
}

// openPlaylists replaces the view stack with the playlist list, like "/"
// does for search: top-level destinations always start from the root.
func (m *Model) openPlaylists() {
	m.stack, m.showQueue = m.stack[:1], false
	m.push(listView{
		title:  "Playlists",
		src:    srcPlaylists,
		header: listsHeader,
		rows:   playlistRows(m.playlists.Playlists()),
	})
}

// openPlaylist pushes the songs of one playlist.
func (m *Model) openPlaylist(name string) {
	for _, p := range m.playlists.Playlists() {
		if p.Name == name {
			m.push(listView{title: p.Name, src: srcPlaylist, playlist: p.Name, header: songsHeader, rows: songRows(p.Songs)})
			return
		}
	}
}

// refreshPlaylists rebuilds every view backed by the store after a change,
// and keeps each cursor inside its (possibly shorter) list.
func (m *Model) refreshPlaylists() {
	pls := m.playlists.Playlists()
	for i := range m.stack {
		v := &m.stack[i]
		switch v.src {
		case srcPlaylists:
			v.rows = playlistRows(pls)
		case srcPlaylist:
			if j := slices.IndexFunc(pls, func(p store.Playlist) bool { return p.Name == v.playlist }); j >= 0 {
				v.rows = songRows(pls[j].Songs)
			}
		}
		v.cursor = min(v.cursor, max(len(v.rows)-1, 0))
	}
	m.scrollToCursor()
}

// selectedSong is the song under the cursor in the queue or the current
// list, if the cursor is on one.
func (m *Model) selectedSong() (ytmusic.Song, bool) {
	if m.showQueue {
		songs := m.queue.Songs()
		if m.qCursor < len(songs) {
			return songs[m.qCursor], true
		}
		return ytmusic.Song{}, false
	}
	if v := m.cur(); v.cursor < len(v.rows) && v.rows[v.cursor].kind == rowSong {
		return v.rows[v.cursor].song, true
	}
	return ytmusic.Song{}, false
}

// queueSelected handles "a": queue the selected song, or every song of the
// selected playlist.
func (m *Model) queueSelected() {
	if m.showQueue {
		return
	}
	v := m.cur()
	if v.cursor >= len(v.rows) {
		return
	}
	switch r := v.rows[v.cursor]; r.kind {
	case rowSong:
		m.queue.Add(r.song)
		m.notice = "added: " + r.song.Title
	case rowPlaylist:
		for _, p := range m.playlists.Playlists() {
			if p.Name == r.id {
				m.queue.Add(p.Songs...)
				m.notice = fmt.Sprintf("queued %d tracks from %s", len(p.Songs), p.Name)
			}
		}
	}
}

// deleteSelected handles "d" outside the queue. In a playlist it removes the
// song. In the playlist list it deletes the playlist, but only on a second
// press, since that loses data. armed is the playlist the previous key armed.
func (m *Model) deleteSelected(armed string) {
	v := m.cur()
	if v.cursor >= len(v.rows) {
		return
	}
	switch v.src {
	case srcPlaylist:
		if err := m.playlists.RemoveSong(v.playlist, v.cursor); err != nil {
			m.err = err
			return
		}
		m.refreshPlaylists()
	case srcPlaylists:
		name := v.rows[v.cursor].id
		if name != armed {
			m.pendingDelete = name
			m.notice = fmt.Sprintf("press d again to delete %q", name)
			return
		}
		if err := m.playlists.Delete(name); err != nil {
			m.err = err
			return
		}
		m.notice = "deleted playlist: " + name
		m.refreshPlaylists()
	}
}

func newNameInput() textinput.Model {
	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = "playlist name"
	in.PromptStyle = accentStyle
	in.CharLimit = 60
	in.Width = 30
	return in
}

// viewOverlay renders the picker or name prompt as a bordered box centered in
// the pane area (w x h).
func (m Model) viewOverlay(w, h int) string {
	var lines []string
	switch m.mode {
	case modePicker:
		p := m.picker
		lines = append(lines, boldStyle.Render("Add to playlist"), dimStyle.Render(fit(p.song.Title, 34)), "")
		for i, pl := range p.lists {
			lines = append(lines, pickLine(i == p.cursor, fmt.Sprintf("%s (%d)", pl.Name, len(pl.Songs))))
		}
		lines = append(lines, pickLine(p.cursor == p.newPlaylistEntry(), "+ New playlist…"))
	case modeName:
		lines = append(lines, boldStyle.Render("New playlist"), "", m.nameInput.View())
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func pickLine(selected bool, label string) string {
	label = fit(label, 34)
	if selected {
		return cursorStyle.Render("▌ " + label)
	}
	return "  " + label
}
