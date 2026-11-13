package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Theme: "The Batman" (2022). Rain-soaked Gotham noir: near-black, blood red
// as the one accent, and a sodium-streetlight amber for secondary details
// (key hints, the playing track, timestamps). Each color is adaptive:
// lipgloss picks Dark or Light depending on the terminal background, and the
// Light values are the same hues darkened for legibility on white.
var (
	accent   = lipgloss.AdaptiveColor{Light: "#B3121D", Dark: "#E5383B"} // blood red
	amber    = lipgloss.AdaptiveColor{Light: "#9A5B00", Dark: "#E9A23B"} // sodium streetlight
	dim      = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#5F6672"} // wet asphalt
	text     = lipgloss.AdaptiveColor{Light: "#2B0A0D", Dark: "#F1E9E4"} // bright text on bands
	rowBG    = lipgloss.AdaptiveColor{Light: "#F6D9DC", Dark: "#3A1216"} // dried blood: cursor row
	bandBG   = lipgloss.AdaptiveColor{Light: "#ECECEC", Dark: "#171A20"} // gotham blue-black: header band
	onAccent = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0B0B0D"} // text on a red badge
	canvas   = lipgloss.AdaptiveColor{Light: "#F4F0EC", Dark: "#0B0D12"} // the whole screen: Gotham night
	redDeep  = lipgloss.AdaptiveColor{Light: "#D98C92", Dark: "#7A1B22"} // quiet red for borders
)

var (
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	amberStyle  = lipgloss.NewStyle().Foreground(amber)
	boldStyle   = lipgloss.NewStyle().Bold(true).Foreground(text)

	// headerStyle is the column-header band.
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(amber).Background(bandBG)
	// cursorStyle highlights a whole row: background fill, not just the text.
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(text).Background(rowBG)
	// playingStyle marks the track that is currently playing.
	playingStyle = lipgloss.NewStyle().Foreground(amber)

	// errStyle is a red badge so errors can't be mistaken for the red accent
	// used everywhere else.
	errStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(onAccent).Background(accent)

	modeStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(onAccent).
			Background(accent)

	// tabOn is the active result type: a red pill. tabOff is the rest.
	tabOn  = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(onAccent).Background(accent)
	tabOff = lipgloss.NewStyle().Padding(0, 1).Foreground(dim)

	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(amber)
)

// bgSeq is the escape sequence that switches on the canvas background for
// the terminal's color profile. We let lipgloss render one marker character
// and keep what comes before it, so profile and adaptive-color handling stay
// lipgloss's job. It is empty when colors are off (e.g. in tests), which
// disables painting entirely.
func bgSeq() string {
	const marker = "\x00"
	out := lipgloss.NewStyle().Background(canvas).Render(marker)
	i := strings.Index(out, marker)
	if i <= 0 {
		return ""
	}
	return out[:i]
}

// paint fills the whole w x h screen with the canvas background. Styled
// segments end with a reset ("\x1b[0m") that would clear the background, so
// the sequence is re-applied after every reset; each line is then padded to
// the full width, and blank lines fill the height.
func paint(frame string, w, h int) string {
	bg := bgSeq()
	if bg == "" {
		return frame
	}
	const reset = "\x1b[0m"
	lines := strings.Split(frame, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, l := range lines {
		pad := strings.Repeat(" ", max(w-ansi.StringWidth(l), 0))
		lines[i] = bg + strings.ReplaceAll(l, reset, reset+bg) + pad + reset
	}
	return strings.Join(lines, "\n")
}
