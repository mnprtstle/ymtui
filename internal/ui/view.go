package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mnprtstle/ymtui/internal/queue"
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
	var lines []string
	if m.mode == modePicker || m.mode == modeName {
		lines = []string{m.viewOverlay(inner-2, m.height-barHeight-statusHeight-4)}
	} else if m.showQueue {
		lines = append(lines, m.queueTitle(), m.header(inner-2))
		lines = append(lines, m.queueRows(inner-2)...)
	} else {
		lines = append(lines, m.topLine(inner-2), m.header(inner-2))
		lines = append(lines, m.resultRows(inner-2)...)
	}

	style := paneStyle
	if m.mode != modeSearch {
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

// tabsWidth is the width of the rendered tab strip: names plus 2-space gaps.
const tabsWidth = 2 + len("Songs") + 2 + len("Albums") + 2 + len("Artists")

// topLine is the first pane line: the search box and result tabs at the root,
// a breadcrumb ("Radiohead › OK Computer") once the user has drilled in.
func (m Model) topLine(w int) string {
	if len(m.stack) > 1 {
		var crumbs []string
		for _, v := range m.stack[1:] {
			crumbs = append(crumbs, v.title)
		}
		return fit(boldStyle.Render(strings.Join(crumbs, " › ")), w)
	}
	var tabs []string
	for i, name := range tabNames {
		if tab(i) == m.tab {
			tabs = append(tabs, accentStyle.Bold(true).Underline(true).Render(name))
		} else {
			tabs = append(tabs, dimStyle.Render(name))
		}
	}
	return fit(m.input.View()+"  "+strings.Join(tabs, "  "), w)
}

func (m Model) header(w int) string {
	h := m.stack[len(m.stack)-1].header
	return headerStyle.Render(m.rowText(w, " ", "#", h[0], h[1], h[2], h[3]))
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
	v := m.stack[len(m.stack)-1]
	if len(v.rows) == 0 {
		msg := "Nothing here."
		switch {
		case v.src == srcPlaylists:
			msg = "No playlists yet. Press c to create one, or A on a song."
		case v.src == srcPlaylist:
			msg = "Empty playlist. Press A on any song to add it."
		case v.loading:
			msg = "Loading…"
		case len(m.stack) == 1 && m.searching:
			msg = "Searching…"
		case len(m.stack) == 1 && m.query == "":
			msg = "No results yet. Press / to search."
		case len(m.stack) == 1:
			msg = "No results."
		}
		return []string{dimStyle.Render(msg)}
	}
	end := min(v.offset+m.rows(), len(v.rows))
	out := make([]string, 0, end-v.offset)
	for i := v.offset; i < end; i++ {
		r := v.rows[i]
		marker := " "
		switch {
		case r.kind == rowSong && m.now != nil && m.now.VideoID == r.song.VideoID:
			marker = "♪"
		case r.kind != rowSong:
			marker = "›" // opens a page
		}
		line := m.rowText(w, marker, fmt.Sprint(i+1), r.cells[0], r.cells[1], r.cells[2], r.cells[3])
		if i == v.cursor {
			line = cursorStyle.Render(line)
		}
		out = append(out, line)
	}
	return out
}

func (m Model) queueTitle() string {
	return boldStyle.Render("Queue") + dimStyle.Render(fmt.Sprintf("  %d tracks", m.queue.Len()))
}

func (m Model) queueRows(w int) []string {
	songs := m.queue.Songs()
	if len(songs) == 0 {
		return []string{dimStyle.Render("Queue is empty. Enter on a result plays it; a adds it.")}
	}
	cur := m.queue.Position()
	end := min(m.qOffset+m.rows(), len(songs))
	out := make([]string, 0, end-m.qOffset)
	for i := m.qOffset; i < end; i++ {
		s := songs[i]
		marker := " "
		if i == cur {
			marker = "♪"
		}
		line := m.rowText(w, marker, fmt.Sprint(i+1), s.Title, s.Artists, s.Album, s.Duration)
		if i == m.qCursor {
			line = cursorStyle.Render(line)
		}
		out = append(out, line)
	}
	return out
}

// queueTag summarises shuffle/repeat/position for the player bar.
func (m Model) queueTag() string {
	var parts []string
	if m.queue.Shuffle() {
		parts = append(parts, "⇄")
	}
	switch m.queue.Repeat() {
	case queue.RepeatAll:
		parts = append(parts, "↻")
	case queue.RepeatOne:
		parts = append(parts, "↻1")
	}
	if p := m.queue.Position(); p >= 0 {
		parts = append(parts, fmt.Sprintf("%d/%d", p+1, m.queue.Len()))
	}
	return strings.Join(parts, " ")
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
	if tag := m.queueTag(); tag != "" {
		right = tag + "  " + right
	}

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
	label, hints := " NORMAL ", "j/k move  l open/play  h back  tab type  a queue  A playlist  P playlists  n/p next/prev  s shuffle  r repeat  Q queue  space pause  / search  q quit"
	switch v := m.cur(); {
	case m.showQueue:
		hints = "j/k move  enter play  d remove  A playlist  n/p next/prev  s shuffle  r repeat  Q/h results  q quit"
	case v.src == srcPlaylists:
		hints = "j/k move  l open  a queue all  c new  d d delete  h back  q quit"
	case v.src == srcPlaylist:
		hints = "j/k move  enter play  a queue  d remove  A other playlist  h back  q quit"
	}
	switch m.mode {
	case modeSearch:
		label, hints = " SEARCH ", "enter search  esc cancel"
	case modePicker:
		label, hints = " PLAYLIST ", "j/k move  enter choose  esc cancel"
	case modeName:
		label, hints = " NAME ", "enter create  esc cancel"
	}
	left := modeStyle.Render(strings.TrimSpace(label))
	var text string
	switch {
	case m.err != nil:
		text = errStyle.Render(m.err.Error())
	case m.notice != "":
		text = accentStyle.Render(m.notice)
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
