package ui

import "github.com/charmbracelet/lipgloss"

// One accent color on an adaptive palette: lipgloss picks Light or Dark
// depending on the terminal's background.
var (
	accent = lipgloss.AdaptiveColor{Light: "#7D3AC1", Dark: "#B388FF"}
	dim    = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6C6C6C"}
	errCol = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6E6E"}
)

var (
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	headerStyle = lipgloss.NewStyle().Foreground(dim).Bold(true)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	errStyle    = lipgloss.NewStyle().Foreground(errCol).Bold(true)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dim)
	focusedPaneStyle = paneStyle.BorderForeground(accent)

	barStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dim).
			Padding(0, 1)

	modeStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
			Background(accent)
)
