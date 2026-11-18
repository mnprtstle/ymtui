package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

func TestSidebarNavigation(t *testing.T) {
	m, _, st := withStore(t)
	st.Create("road trip")
	st.AddSong("road trip", ytmusic.Song{VideoID: "z", Title: "Z"})

	tests := []struct {
		name      string
		keys      []string
		wantFocus bool
		wantQueue bool
		wantTitle string // title of the top view; "" = search root
	}{
		{"starts in main pane", nil, false, false, ""},
		{"ctrl+h focuses sidebar", []string{"ctrl+h"}, true, false, ""},
		{"ctrl+l returns to main", []string{"ctrl+h", "ctrl+l"}, false, false, ""},
		{"enter on Queue", []string{"ctrl+h", "j", "enter"}, false, true, ""},
		{"enter on a playlist", []string{"ctrl+h", "j", "j", "enter"}, false, false, "road trip"},
		{"main keys are ignored while sidebar has focus", []string{"ctrl+h", "Q"}, true, false, ""},
		{"1/2/3 switch views", []string{"2"}, false, true, ""},
		{"3 opens playlists", []string{"3"}, false, false, "Playlists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := press(m, tt.keys...)
			if got.sidebarFocus != tt.wantFocus || got.showQueue != tt.wantQueue {
				t.Errorf("focus=%v queue=%v, want %v %v", got.sidebarFocus, got.showQueue, tt.wantFocus, tt.wantQueue)
			}
			if tt.wantTitle != "" && got.cur().title != tt.wantTitle {
				t.Errorf("view = %q, want %q", got.cur().title, tt.wantTitle)
			}
		})
	}
}

func TestSidebarRendersAndCollapses(t *testing.T) {
	m, _, st := withStore(t)
	st.Create("road trip")

	if v := ansi.Strip(m.View()); !strings.Contains(v, "PLAYLISTS") || !strings.Contains(v, "road trip") {
		t.Error("wide terminal should show the sidebar with playlists")
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	narrow := next.(Model)
	if strings.Contains(ansi.Strip(narrow.View()), "PLAYLISTS") {
		t.Error("narrow terminal should collapse the sidebar")
	}
	if got := press(narrow, "ctrl+h"); got.sidebarFocus {
		t.Error("a hidden sidebar must not take focus")
	}

	// Shrinking while the sidebar has focus hands focus back.
	focused := press(m, "ctrl+h")
	next, _ = focused.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	if next.(Model).sidebarFocus {
		t.Error("focus should return to the main pane when the sidebar hides")
	}

	// No line may exceed the terminal width.
	for _, line := range strings.Split(m.View(), "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("line is %d cells wide, terminal is %d", w, m.width)
		}
	}
}
