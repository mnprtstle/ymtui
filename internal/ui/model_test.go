package ui

import (
	"context"
	"encoding/json"
	"errors"
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

// fakeCatalog serves canned data and records what was requested.
type fakeCatalog struct {
	albumErr error
	requests []string
}

func (f *fakeCatalog) Search(context.Context, string) ([]ytmusic.Song, error) {
	f.requests = append(f.requests, "songs")
	return []ytmusic.Song{{VideoID: "s1", Title: "S1"}}, nil
}
func (f *fakeCatalog) SearchAlbums(context.Context, string) ([]ytmusic.Album, error) {
	f.requests = append(f.requests, "albums")
	return []ytmusic.Album{{BrowseID: "MPREb_1", Title: "Album One", Artists: "Art", Kind: "Album", Year: "1999"}}, nil
}
func (f *fakeCatalog) SearchArtists(context.Context, string) ([]ytmusic.Artist, error) {
	f.requests = append(f.requests, "artists")
	return []ytmusic.Artist{{BrowseID: "UC1", Name: "Artist One"}}, nil
}
func (f *fakeCatalog) Album(_ context.Context, id string) (ytmusic.AlbumPage, error) {
	f.requests = append(f.requests, "album "+id)
	if f.albumErr != nil {
		return ytmusic.AlbumPage{}, f.albumErr
	}
	return ytmusic.AlbumPage{Songs: []ytmusic.Song{
		{VideoID: "t1", Title: "T1"}, {VideoID: "t2", Title: "T2"},
	}}, nil
}
func (f *fakeCatalog) Artist(_ context.Context, id string) (ytmusic.ArtistPage, error) {
	f.requests = append(f.requests, "artist "+id)
	return ytmusic.ArtistPage{
		Name:     "Artist One",
		TopSongs: []ytmusic.Song{{VideoID: "top1", Title: "Top1"}},
		Albums:   []ytmusic.Album{{BrowseID: "MPREb_1", Title: "Album One", Year: "1999"}},
	}, nil
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func loaded(t *testing.T) (Model, *fakePlayer) {
	t.Helper()
	fp := &fakePlayer{}
	m := New(nil, fp, nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.query = "q"
	next, _ = m.Update(searchResultsMsg{query: "q", tab: tabSongs, rows: songRows([]ytmusic.Song{
		{VideoID: "a", Title: "A"}, {VideoID: "b", Title: "B"}, {VideoID: "c", Title: "C"},
	})})
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
			if m.cur().cursor != tt.wantCur {
				t.Errorf("cursor = %d, want %d", m.cur().cursor, tt.wantCur)
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
	next, _ := m.Update(searchResultsMsg{query: "old", rows: nil})
	if got := len(next.(Model).stack[0].rows); got != 3 {
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

// runCmds feeds keys through Update, then delivers each returned Cmd's message
// back to Update the way Bubble Tea would.
func runCmds(m Model, keys ...string) Model {
	for _, k := range keys {
		next, cmd := m.Update(key(k))
		m = next.(Model)
		if cmd != nil {
			if msg := cmd(); msg != nil {
				next, _ = m.Update(msg)
				m = next.(Model)
			}
		}
	}
	return m
}

// withCatalog is loaded() with a fake catalog, showing one album result.
func withCatalog(t *testing.T) (Model, *fakeCatalog, *fakePlayer) {
	t.Helper()
	fc, fp := &fakeCatalog{}, &fakePlayer{}
	m := New(fc, fp, nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.query, m.tab = "q", tabAlbums
	m.stack[0].header = albumsHeader
	next, _ = m.Update(searchResultsMsg{query: "q", tab: tabAlbums, rows: albumRows([]ytmusic.Album{
		{BrowseID: "MPREb_1", Title: "Album One", Artists: "Art", Kind: "Album", Year: "1999"},
	})})
	return next.(Model), fc, fp
}

func TestBrowseAlbumAndBack(t *testing.T) {
	m, fc, fp := withCatalog(t)

	// l pushes a loading view and fires the load; the result then fills it.
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	if len(m.stack) != 2 || !m.cur().loading || cmd == nil {
		t.Fatalf("after l: depth=%d loading=%v cmd=%v", len(m.stack), m.cur().loading, cmd != nil)
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Error("loading text missing")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.cur().loading || len(m.cur().rows) != 2 {
		t.Fatalf("page not filled: %+v", m.cur())
	}
	if !slices.Equal(fc.requests, []string{"album MPREb_1"}) {
		t.Errorf("requests = %v", fc.requests)
	}
	if v := m.View(); !strings.Contains(v, "Album One") || !strings.Contains(v, "T2") {
		t.Errorf("breadcrumb or tracks missing:\n%s", v)
	}

	// Enter on a track queues the whole album from that track.
	m = run(m, "j", "enter")
	if len(fp.loaded) != 1 || !strings.HasSuffix(fp.loaded[0], "=t2") {
		t.Errorf("loaded %v, want t2", fp.loaded)
	}
	if m.queue.Len() != 2 {
		t.Errorf("queue len = %d, want 2", m.queue.Len())
	}

	// h pops back to the search view with its cursor intact; at the root it
	// does nothing.
	m = run(m, "h")
	if len(m.stack) != 1 {
		t.Errorf("depth after h = %d", len(m.stack))
	}
	m = run(m, "h", "esc")
	if len(m.stack) != 1 {
		t.Errorf("popped the root: depth %d", len(m.stack))
	}
}

func TestBrowseArtistPage(t *testing.T) {
	m, _, fp := withCatalog(t)
	m.tab = tabArtists
	next, _ := m.Update(searchResultsMsg{query: "q", tab: tabArtists, rows: artistRows([]ytmusic.Artist{{BrowseID: "UC1", Name: "Artist One"}})})
	m = next.(Model)

	m = runCmds(m, "l")
	if len(m.stack) != 2 || len(m.cur().rows) != 2 {
		t.Fatalf("artist page: depth=%d rows=%d", len(m.stack), len(m.cur().rows))
	}
	// Row 0 is a song, row 1 an album that opens a deeper view.
	m = runCmds(m, "j", "l")
	if len(m.stack) != 3 || m.cur().id != "MPREb_1" {
		t.Fatalf("album not pushed: depth=%d id=%q", len(m.stack), m.cur().id)
	}
	if v := m.View(); !strings.Contains(v, "Artist One › Album One") {
		t.Errorf("breadcrumb missing:\n%s", v)
	}
	m = run(m, "esc", "k", "enter")
	if len(fp.loaded) != 1 || !strings.HasSuffix(fp.loaded[0], "=top1") {
		t.Errorf("loaded %v, want top1", fp.loaded)
	}
}

func TestStalePageIgnored(t *testing.T) {
	m, _, _ := withCatalog(t)
	next, _ := m.Update(key("l"))
	m = next.(Model)
	m = run(m, "h") // back out before the page arrives
	next, _ = m.Update(pageLoadedMsg{id: "MPREb_1", rows: songRows([]ytmusic.Song{{VideoID: "x", Title: "X"}})})
	m = next.(Model)
	if len(m.stack) != 1 || len(m.stack[0].rows) != 1 || m.stack[0].rows[0].kind != rowAlbum {
		t.Errorf("late page leaked into the search view: %+v", m.stack)
	}
}

func TestPageErrorPopsLoadingView(t *testing.T) {
	m, fc, _ := withCatalog(t)
	fc.albumErr = errors.New("boom")
	m = runCmds(m, "l")
	if len(m.stack) != 1 || m.err == nil {
		t.Errorf("depth=%d err=%v, want back at root with an error", len(m.stack), m.err)
	}
}

func TestTabSwitchesResultType(t *testing.T) {
	m, fc, _ := withCatalog(t)
	m.tab = tabSongs // start on Songs with the query already set
	m = runCmds(m, "tab")
	if m.tab != tabAlbums || len(m.stack[0].rows) != 1 || m.stack[0].rows[0].kind != rowAlbum {
		t.Errorf("tab=%d rows=%+v", m.tab, m.stack[0].rows)
	}
	m = runCmds(m, "tab", "tab") // artists, then wraps to songs
	if m.tab != tabSongs || !slices.Equal(fc.requests, []string{"albums", "artists", "songs"}) {
		t.Errorf("tab=%d requests=%v", m.tab, fc.requests)
	}
	// A result for a tab we left must not land in the list.
	next, _ := m.Update(searchResultsMsg{query: "q", tab: tabArtists, rows: artistRows([]ytmusic.Artist{{Name: "late"}})})
	if got := next.(Model).stack[0].rows; len(got) != 1 || got[0].cells[0] == "late" {
		t.Errorf("stale tab result applied: %+v", got)
	}
}

func TestBrowseViewFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 24}, {45, 15}} {
		m, _, _ := withCatalog(t)
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = runCmds(next.(Model), "l")
		for depth, view := range []string{m.View()} {
			lines := strings.Split(view, "\n")
			if len(lines) != size[1] {
				t.Errorf("%v depth %d: %d lines, want %d", size, depth, len(lines), size[1])
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > size[0] {
					t.Errorf("%v: line %d is %d wide", size, i, w)
				}
			}
		}
	}
}
