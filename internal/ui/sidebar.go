package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	sidebarWidth    = 26 // total cells, border included
	sidebarMinWidth = 90 // below this terminal width the sidebar collapses
)

// sidebarShown reports whether the terminal is wide enough for the sidebar.
// When it isn't, the keys 1-3 switch views instead.
func (m Model) sidebarShown() bool { return m.width >= sidebarMinWidth }

// paneWidth is the width left for the main pane.
func (m Model) paneWidth() int {
	if m.sidebarShown() {
		return m.width - sidebarWidth
	}
	return m.width
}

// Sidebar entries are, in order: Search, Queue, then one per playlist.
const sidebarFixed = 2

func (m Model) sidebarLen() int {
	if m.playlists == nil {
		return sidebarFixed
	}
	return sidebarFixed + len(m.playlists.Playlists())
}

// sidebarActive is the entry whose content the main pane is showing.
func (m Model) sidebarActive() int {
	if m.showQueue {
		return 1
	}
	if v := m.cur(); v.src == srcPlaylist && m.playlists != nil {
		for i, p := range m.playlists.Playlists() {
			if p.Name == v.playlist {
				return sidebarFixed + i
			}
		}
	}
	return 0
}

// activateSidebar shows entry i in the main pane and moves focus there.
func (m *Model) activateSidebar(i int) {
	m.sidebarFocus = false
	m.showQueue = false
	m.stack = m.stack[:1]
	switch {
	case i == 1:
		m.showQueue = true
	case i >= sidebarFixed && m.playlists != nil:
		if pls := m.playlists.Playlists(); i-sidebarFixed < len(pls) {
			m.openPlaylist(pls[i-sidebarFixed].Name)
		}
	}
	m.scrollToCursor()
}

// updateSidebar handles a key while the sidebar has focus. Navigation keys
// act on the sidebar; the globalKeys (playback, help, quit) are handled by
// updateNormal before we get here; everything else is ignored so that, say,
// "d" can't delete something in a pane that isn't focused.
func (m Model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := m.sidebarLen()
	m.sideCursor = min(m.sideCursor, n-1)
	switch key := msg.String(); key {
	case "j", "down":
		m.sideCursor = min(m.sideCursor+1, n-1)
	case "k", "up":
		m.sideCursor = max(m.sideCursor-1, 0)
	case "g":
		m.sideCursor = 0
	case "G":
		m.sideCursor = n - 1
	case "enter", "l":
		m.activateSidebar(m.sideCursor)
	}
	return m, nil
}

// globalKeys keep working while the sidebar has focus.
var globalKeys = map[string]bool{
	"q": true, "?": true, "/": true, " ": true, "n": true, "p": true, "s": true, "r": true,
	"H": true, "L": true, "left": true, "right": true, "+": true, "=": true, "-": true,
}

// viewSidebar draws the sidebar box, w x h cells.
func (m Model) viewSidebar(w, h int) string {
	inner := w - 4
	type entry struct{ label, count string }
	entries := []entry{{"Search", ""}, {"Queue", ""}}
	if n := m.queue.Len(); n > 0 {
		entries[1].count = fmt.Sprint(n)
	}
	if m.playlists != nil {
		for _, p := range m.playlists.Playlists() {
			entries = append(entries, entry{p.Name, fmt.Sprint(len(p.Songs))})
		}
	}

	active := m.sidebarActive()
	var lines []string
	cursorLine := 0
	for i, e := range entries {
		if i == sidebarFixed {
			lines = append(lines, "", dimStyle.Render("PLAYLISTS"))
		}
		marker := " "
		if i == active {
			marker = "▸"
		}
		countW := lipgloss.Width(e.count)
		line := marker + " " + fit(e.label, max(inner-2-countW-1, 1)) + " " + e.count
		switch {
		case m.sidebarFocus && i == m.sideCursor:
			line = cursorStyle.Render(fit(line, inner))
			cursorLine = len(lines)
		case i == active:
			line = amberStyle.Render(line)
		}
		lines = append(lines, line)
	}
	// Scroll so the cursor line stays visible.
	if body := h - 2; cursorLine >= body {
		lines = lines[cursorLine-body+1:]
	}

	border := lipgloss.TerminalColor(dim)
	if m.sidebarFocus {
		border = accent
	}
	return box("ymtui", "", strings.Split(strings.Join(lines, "\n"), "\n"), w, h, border)
}
