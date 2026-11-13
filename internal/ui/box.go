package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// box draws a btop-style frame, w x h cells in total, with the title set into
// the top border and an optional right-hand label:
//
//	╭─┤ Songs ├──────────────────────┤ 3/25 ├─╮
//
// lipgloss borders can't hold text, so the frame is drawn by hand. Body lines
// are truncated or padded to the inner width (w-4: border plus one space of
// padding on each side) and the body is padded with blank lines to h-2.
func box(title, right string, body []string, w, h int, border lipgloss.TerminalColor) string {
	b := lipgloss.NewStyle().Foreground(border)

	left := b.Render("╭─┤") + boldStyle.Render(" "+title+" ") + b.Render("├")
	end := b.Render("─╮")
	if right != "" {
		end = b.Render("┤") + amberStyle.Render(" "+right+" ") + b.Render("├─╮")
	}
	fill := w - ansi.StringWidth(left) - ansi.StringWidth(end)
	if fill < 0 { // too narrow for the right label: drop it
		end = b.Render("─╮")
		fill = max(w-ansi.StringWidth(left)-ansi.StringWidth(end), 0)
	}
	top := left + b.Render(strings.Repeat("─", fill)) + end

	out := []string{ansi.Truncate(top, w, "")}
	side := b.Render("│")
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		out = append(out, side+" "+fit(line, w-4)+" "+side)
	}
	out = append(out, b.Render("╰"+strings.Repeat("─", max(w-2, 0))+"╯"))
	return strings.Join(out, "\n")
}

// gradient maps 0..1 onto deep red → red → amber, the color ramp btop uses
// for its meters (there: green → yellow → red).
func gradient(f float64) lipgloss.Color {
	stops := [3][3]float64{{122, 27, 34}, {229, 56, 59}, {233, 162, 59}} // dark terminals
	if !lipgloss.HasDarkBackground() {
		stops = [3][3]float64{{217, 140, 146}, {179, 18, 29}, {154, 91, 0}}
	}
	f = min(max(f, 0), 1) * 2 // 0..2: which pair of stops, and how far along
	i := min(int(f), 1)
	t := f - float64(i)
	var c [3]int
	for k := range c {
		c[k] = int(math.Round(stops[i][k] + (stops[i+1][k]-stops[i][k])*t))
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]))
}

// volumeMeter is a stepped block meter: ▁▂▃▄▅▆▇█, the lit part colored along
// the gradient, the rest dim.
func volumeMeter(volume float64) string {
	steps := []rune("▁▂▃▄▅▆▇█")
	lit := int(math.Round(min(max(volume, 0), 100) / 100 * float64(len(steps))))
	var sb strings.Builder
	for i, r := range steps {
		if i < lit {
			sb.WriteString(lipgloss.NewStyle().Foreground(gradient(float64(i) / float64(len(steps)-1))).Render(string(r)))
		} else {
			sb.WriteString(dimStyle.Render(string(r)))
		}
	}
	return sb.String()
}
