package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	barHeight    = 5 // player bar: 3 content lines + border
	statusHeight = 1
	paneChrome   = 2 + 2 // pane border + (input line, column header)
)

// rows is how many result rows fit in the pane.
func (m Model) rows() int {
	return max(m.height-barHeight-statusHeight-paneChrome-2, 1)
}

func (m Model) View() string {
	if m.width == 0 {
		return "" // no WindowSizeMsg yet
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.viewPane(), m.viewBar(), m.viewStatus())
}

func (m Model) viewPane() string {
	inner := m.width - 2
	lines := []string{m.input.View(), m.header(inner - 2)}
	lines = append(lines, m.resultRows(inner-2)...)

	style := paneStyle
	if m.mode == modeNormal {
		style = focusedPaneStyle
	}
	return style.
		Width(inner).
		Height(m.height-barHeight-statusHeight-2).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// columns returns the widths of title, artist and album for a content width.
// Album is dropped first when narrow, then artist (width 0 = hidden).
func columns(w int) (title, artist, album int) {
	const fixed = 2 + 3 + 5 + 2*3 // marker, #, time, gaps
	avail := w - fixed
	switch {
	case w < 50:
		return max(avail, 5), 0, 0
	case w < 70:
		return avail * 55 / 100, avail - avail*55/100, 0
	}
	title = avail * 40 / 100
	artist = avail * 30 / 100
	return title, artist, avail - title - artist
}

func (m Model) header(w int) string {
	return headerStyle.Render(m.rowText(w, " ", "#", "Title", "Artist", "Album", "Time"))
}

// rowText lays out one table row. Hidden columns are skipped.
func (m Model) rowText(w int, marker, num, title, artist, album, dur string) string {
	tw, aw, bw := columns(w)
	parts := []string{fit(marker, 1) + " ", fit(num, 3), fit(title, tw)}
	if aw > 0 {
		parts = append(parts, fit(artist, aw))
	}
	if bw > 0 {
		parts = append(parts, fit(album, bw))
	}
	parts = append(parts, fmt.Sprintf("%5s", dur))
	return strings.Join(parts, " ")
}

func (m Model) resultRows(w int) []string {
	if len(m.songs) == 0 {
		msg := "No results yet. Press / to search."
		if m.searching {
			msg = "Searching…"
		}
		return []string{dimStyle.Render(msg)}
	}
	end := min(m.offset+m.rows(), len(m.songs))
	out := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		s := m.songs[i]
		marker := " "
		if m.now != nil && m.now.VideoID == s.VideoID {
			marker = "♪"
		}
		line := m.rowText(w, marker, fmt.Sprint(i+1), s.Title, s.Artists, s.Album, s.Duration)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		out = append(out, line)
	}
	return out
}

func (m Model) viewBar() string {
	w := m.width - 4 // border + padding
	title, sub := "Nothing playing", ""
	switch {
	case m.now != nil:
		title = m.now.Title
		sub = m.now.Artists
		if m.now.Album != "" {
			sub += " · " + m.now.Album
		}
	case m.title != "":
		title = m.title
	}
	state := "▶"
	if m.paused {
		state = "⏸"
	}
	right := fmt.Sprintf("%s  %d%%", state, int(m.volume))

	titleW := max(w-ansi.StringWidth(right)-1, 1)
	line1 := boldStyle.Render(fit(title, titleW)) + " " + dimStyle.Render(right)
	line2 := dimStyle.Render(fit(sub, w))
	return barStyle.Width(m.width - 2).Render(line1 + "\n" + line2 + "\n" + m.progress(w))
}

func (m Model) progress(w int) string {
	left, right := clock(m.pos), clock(m.dur)
	barW := max(w-len(left)-len(right)-2, 4)
	frac := 0.0
	if m.dur > 0 {
		frac = min(m.pos/m.dur, 1)
	}
	filled := int(frac * float64(barW-1))
	bar := accentStyle.Render(strings.Repeat("━", filled)+"●") +
		dimStyle.Render(strings.Repeat("─", barW-filled-1))
	return left + " " + bar + " " + right
}

func (m Model) viewStatus() string {
	label, hints := " NORMAL ", "j/k move  enter play  space pause  H/L seek  +/- volume  / search  q quit"
	if m.mode == modeSearch {
		label, hints = " SEARCH ", "enter search  esc cancel"
	}
	left := modeStyle.Render(strings.TrimSpace(label))
	var text string
	switch {
	case m.err != nil:
		text = errStyle.Render(m.err.Error())
	case m.searching:
		text = dimStyle.Render("searching…")
	default:
		text = dimStyle.Render(hints)
	}
	return fit(left+" "+text, m.width)
}

// fit truncates s to w cells (with …) and pads it with spaces to exactly w.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

func clock(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
