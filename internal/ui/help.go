package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpSection groups key bindings under a heading. The help overlay is built
// from this table, so adding a binding here is all it takes to document it.
type helpSection struct {
	title string
	keys  [][2]string // {key, description}
}

var helpSections = []helpSection{
	{"Move", [][2]string{
		{"j/k", "down / up"}, {"gg / G", "top / bottom"}, {"ctrl+d/u", "half page"},
		{"l, enter", "open / play"}, {"h, esc", "back"}, {"tab", "songs / albums / artists"},
		{"ctrl+h/l", "focus sidebar / main"}, {"1 2 3", "search / queue / playlists"},
	}},
	{"Play", [][2]string{
		{"space", "pause"}, {"n / p", "next / previous"}, {"H/L, ←/→", "seek 5 s"},
		{"+ / -", "volume"}, {"s", "shuffle"}, {"r", "cycle repeat"},
	}},
	{"Library", [][2]string{
		{"/", "search"}, {"a", "add to queue"}, {"A", "add to playlist"},
		{"P", "playlists"}, {"c", "new playlist"}, {"Q", "queue view"}, {"d", "remove (dd: delete playlist)"},
	}},
	{"Other", [][2]string{{"?", "this help"}, {"q", "quit"}}},
}

// updateHelp handles keys while the help overlay is open.
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q", "h":
		m.mode = modeNormal
	}
	return m, nil
}

// viewHelp renders the key table as a bordered box centered in w×h.
func viewHelp(w, h int) string {
	var lines []string
	for i, s := range helpSections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, accentStyle.Render("▌")+boldStyle.Render(s.title))
		for _, k := range s.keys {
			lines = append(lines, "  "+keyStyle.Render(pad(k[0], 10))+dimStyle.Render(k[1]))
		}
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// pad right-pads s with spaces to width w, counting display cells.
func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}
