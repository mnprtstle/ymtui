package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Colors are off under `go test` (no TTY), so force truecolor, and restore
// the global profile afterwards so other tests keep plain output.
func TestPaintCoversScreen(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	m, _, st := withStore(t)
	st.Create("P")
	bg := bgSeq()
	if bg == "" {
		t.Fatal("no background sequence under truecolor")
	}
	for name, mm := range map[string]Model{"results": m, "overlay": press(m, "A")} {
		lines := strings.Split(mm.View(), "\n")
		if len(lines) != m.height {
			t.Errorf("%s: %d lines, want %d", name, len(lines), m.height)
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != m.width {
				t.Errorf("%s line %d: width %d, want %d", name, i, w, m.width)
			}
			// After every reset the background must be switched back on.
			for _, seg := range strings.Split(l, "\x1b[0m")[1:] {
				if seg != "" && !strings.HasPrefix(seg, bg) {
					t.Errorf("%s line %d: segment %q lost the background", name, i, seg)
				}
			}
		}
	}
}
