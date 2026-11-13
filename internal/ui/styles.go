package ui

import "github.com/charmbracelet/lipgloss"

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

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dim)
	focusedPaneStyle = paneStyle.BorderForeground(accent)

	barStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(redDeep).
			Padding(0, 1)

	modeStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(onAccent).
			Background(accent)

	// tabOn is the active result type: a red pill. tabOff is the rest.
	tabOn  = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(onAccent).Background(accent)
	tabOff = lipgloss.NewStyle().Padding(0, 1).Foreground(dim)

	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(amber)
)
