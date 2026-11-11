package ui

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mnprtstle/ymtui/internal/player"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

type fakePlayer struct{ loaded []string }

func (f *fakePlayer) Load(_ context.Context, url string) error {
	f.loaded = append(f.loaded, url)
	return nil
}
func (f *fakePlayer) SetPause(context.Context, bool) error     { return nil }
func (f *fakePlayer) Seek(context.Context, float64) error      { return nil }
func (f *fakePlayer) SetVolume(context.Context, float64) error { return nil }

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func loaded(t *testing.T) (Model, *fakePlayer) {
	t.Helper()
	fp := &fakePlayer{}
	m := New(nil, fp)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.query = "q"
	next, _ = m.Update(searchResultsMsg{query: "q", songs: []ytmusic.Song{
		{VideoID: "a", Title: "A"}, {VideoID: "b", Title: "B"}, {VideoID: "c", Title: "C"},
	}})
	return next.(Model), fp
}

func TestNavigationAndPlay(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		wantCur  int
		wantLoad string
	}{
		{"down", []string{"j"}, 1, ""},
		{"clamp top", []string{"k"}, 0, ""},
		{"bottom", []string{"G"}, 2, ""},
		{"gg", []string{"G", "g", "g"}, 0, ""},
		{"single g does nothing", []string{"G", "g", "k"}, 1, ""},
		{"play", []string{"j", "enter"}, 1, "https://music.youtube.com/watch?v=b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fp := loaded(t)
			for _, k := range tt.keys {
				next, cmd := m.Update(key(k))
				m = next.(Model)
				if cmd != nil {
					cmd() // run Cmds inline; the fakes don't block
				}
			}
			if m.cursor != tt.wantCur {
				t.Errorf("cursor = %d, want %d", m.cursor, tt.wantCur)
			}
			got := ""
			if len(fp.loaded) > 0 {
				got = fp.loaded[0]
			}
			if got != tt.wantLoad {
				t.Errorf("loaded %q, want %q", got, tt.wantLoad)
			}
		})
	}
}

func TestStaleSearchIgnored(t *testing.T) {
	m, _ := loaded(t)
	m.query = "new"
	next, _ := m.Update(searchResultsMsg{query: "old", songs: nil})
	if got := len(next.(Model).songs); got != 3 {
		t.Errorf("stale results replaced songs: %d", got)
	}
}

func TestPlayerEvents(t *testing.T) {
	m, _ := loaded(t)
	for _, ev := range []player.Event{
		{Name: "property-change", Property: "time-pos", Data: json.RawMessage("12.5")},
		{Name: "property-change", Property: "duration", Data: json.RawMessage("200")},
		{Name: "property-change", Property: "pause", Data: json.RawMessage("true")},
	} {
		next, _ := m.Update(playerEventMsg{ev})
		m = next.(Model)
	}
	if m.pos != 12.5 || m.dur != 200 || !m.paused {
		t.Errorf("got pos=%v dur=%v paused=%v", m.pos, m.dur, m.paused)
	}
	next, _ := m.Update(playerEventMsg{player.Event{Name: "end-file", Reason: "error", FileError: "boom"}})
	if next.(Model).err == nil {
		t.Error("end-file error not surfaced")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 24}, {45, 15}} {
		m, _ := loaded(t)
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		next, _ = m.Update(playerEventMsg{player.Event{Name: "property-change", Property: "duration", Data: json.RawMessage("200")}})
		view := next.(Model).View()
		lines := strings.Split(view, "\n")
		if len(lines) != size[1] {
			t.Errorf("%v: %d lines, want %d", size, len(lines), size[1])
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Errorf("%v: line %d is %d wide", size, i, w)
			}
		}
	}
}

func TestFit(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"abcdef", 4, "abc…"},
		{"日本語", 4, "日… "},
		{"x", 0, ""},
	}
	for _, tt := range tests {
		if got := fit(tt.in, tt.w); got != tt.want {
			t.Errorf("fit(%q,%d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}

// run feeds keys through Update, executing returned Cmds inline.
func run(m Model, keys ...string) Model {
	for _, k := range keys {
		next, cmd := m.Update(key(k))
		m = next.(Model)
		if cmd != nil {
			cmd()
		}
	}
	return m
}

func endFile(m Model, reason string) Model {
	next, cmd := m.Update(playerEventMsg{player.Event{Name: "end-file", Reason: reason}})
	if cmd != nil {
		cmd()
	}
	return next.(Model)
}

func TestQueueAdvance(t *testing.T) {
	const base = "https://music.youtube.com/watch?v="
	tests := []struct {
		name   string
		keys   []string
		reason string // end-file reason fired after keys; "" = none
		want   []string
	}{
		{"enter queues results", []string{"enter"}, "", []string{base + "a"}},
		{"eof advances", []string{"enter"}, "eof", []string{base + "a", base + "b"}},
		{"stop is ignored", []string{"enter"}, "stop", []string{base + "a"}},
		{"error is ignored", []string{"enter"}, "error", []string{base + "a"}},
		{"n skips", []string{"enter", "n"}, "", []string{base + "a", base + "b"}},
		{"p goes back", []string{"j", "enter", "p"}, "", []string{base + "b", base + "a"}},
		{"n at end does nothing", []string{"G", "enter", "n"}, "", []string{base + "c"}},
		{"repeat one replays on eof", []string{"enter", "r", "r"}, "eof", []string{base + "a", base + "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fp := loaded(t)
			m = run(m, tt.keys...)
			if tt.reason != "" {
				m = endFile(m, tt.reason)
			}
			if !slices.Equal(fp.loaded, tt.want) {
				t.Errorf("loaded %v, want %v", fp.loaded, tt.want)
			}
		})
	}
}

func TestQueueView(t *testing.T) {
	m, fp := loaded(t)
	m = run(m, "j", "a") // add B to an empty queue
	if m.queue.Len() != 1 || !strings.Contains(m.notice, "added") {
		t.Fatalf("add failed: len=%d notice=%q", m.queue.Len(), m.notice)
	}
	m = run(m, "j", "a", "Q") // add C, open queue
	if !m.showQueue || !strings.Contains(m.View(), "Queue") {
		t.Fatal("queue view not shown")
	}
	m = run(m, "j", "enter") // play entry 2 (C)
	if len(fp.loaded) != 1 || !strings.HasSuffix(fp.loaded[0], "=c") {
		t.Errorf("loaded %v, want c", fp.loaded)
	}
	m = run(m, "d", "d", "d") // remove past the end: clamps, no panic
	if m.queue.Len() != 0 || m.qCursor != 0 {
		t.Errorf("len=%d cursor=%d after removing all", m.queue.Len(), m.qCursor)
	}
	if !strings.Contains(m.View(), "Queue is empty") {
		t.Error("empty queue message missing")
	}
}
