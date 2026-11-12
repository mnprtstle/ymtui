package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mnprtstle/ymtui/internal/store"
)

// withStore is loaded() plus a real store in a temp dir. The store is a
// concrete type satisfying PlaylistStore, so no fake is needed.
func withStore(t *testing.T) (Model, *fakePlayer, *store.Store) {
	t.Helper()
	m, fp := loaded(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "playlists.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.playlists = st
	return m, fp, st
}

// press feeds keys to the model and runs any returned Cmd. Cmds run in a
// goroutine with a short timeout because the text input's cursor-blink Cmd
// sleeps ~0.5s; our fakes return instantly, so anything slower is ignored.
func press(m Model, keys ...string) Model {
	for _, k := range keys {
		next, cmd := m.Update(key(k))
		m = next.(Model)
		if cmd != nil {
			done := make(chan struct{})
			go func() { cmd(); close(done) }()
			select {
			case <-done:
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	return m
}

func ids(p store.Playlist) string {
	var out []string
	for _, s := range p.Songs {
		out = append(out, s.VideoID)
	}
	return strings.Join(out, ",")
}

func TestPickerCreatesAndAdds(t *testing.T) {
	m, _, st := withStore(t)
	// Cursor on song "a": A opens the picker, which has only "New playlist…".
	m = press(m, "A")
	if m.mode != modePicker {
		t.Fatalf("mode = %v, want picker", m.mode)
	}
	m = press(m, "enter") // New playlist…
	if m.mode != modeName {
		t.Fatalf("mode = %v, want name prompt", m.mode)
	}
	m = press(m, "Chill", "enter")
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want normal", m.mode)
	}
	pls := st.Playlists()
	if len(pls) != 1 || pls[0].Name != "Chill" || ids(pls[0]) != "a" {
		t.Errorf("playlists = %+v", pls)
	}
}

func TestPickerExistingPlaylist(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		want       string // video IDs in "P"
		wantNotice string
	}{
		{"add", []string{"A", "enter"}, "a", "added to P"},
		{"cursor song", []string{"j", "A", "enter"}, "b", "added to P"},
		{"duplicate", []string{"A", "enter", "A", "enter"}, "a", "already in P"},
		{"cancel", []string{"A", "esc"}, "", ""},
		{"cancel from name prompt", []string{"A", "j", "enter", "esc"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, st := withStore(t)
			st.Create("P")
			m = press(m, tt.keys...)
			if got := ids(st.Playlists()[0]); got != tt.want {
				t.Errorf("songs = %q, want %q", got, tt.want)
			}
			if !strings.Contains(m.notice, tt.wantNotice) {
				t.Errorf("notice = %q, want it to contain %q", m.notice, tt.wantNotice)
			}
			if m.mode != modeNormal {
				t.Errorf("mode = %v, want normal", m.mode)
			}
		})
	}
}

func TestPickerNotOnNonSong(t *testing.T) {
	m, _, _ := withStore(t)
	m.cur().rows[0].kind = rowAlbum
	if m = press(m, "A"); m.mode != modeNormal {
		t.Error("picker opened on an album row")
	}
}

func TestPlaylistViews(t *testing.T) {
	m, fp, st := withStore(t)
	st.Create("P")
	m = press(m, "A", "enter", "j", "A", "enter") // a, b into P

	m = press(m, "P")
	if v := m.cur(); v.src != srcPlaylists || len(v.rows) != 1 || v.rows[0].cells[1] != "2 tracks" {
		t.Fatalf("playlists view = %+v", v)
	}
	if len(m.stack) != 2 {
		t.Errorf("stack depth = %d, want 2", len(m.stack))
	}

	m = press(m, "enter") // open P
	if v := m.cur(); v.src != srcPlaylist || len(v.rows) != 2 {
		t.Fatalf("playlist view = %+v", v)
	}
	m = press(m, "j", "enter") // play "b"
	if len(fp.loaded) != 1 || !strings.HasSuffix(fp.loaded[0], "v=b") {
		t.Errorf("loaded = %v", fp.loaded)
	}

	m = press(m, "d") // remove "b" (cursor is on it)
	if got := ids(st.Playlists()[0]); got != "a" {
		t.Errorf("after remove: %q", got)
	}
	if v := m.cur(); len(v.rows) != 1 || v.cursor != 0 {
		t.Errorf("view not refreshed/clamped: rows=%d cursor=%d", len(v.rows), v.cursor)
	}

	m = press(m, "h") // back to the list, which shows the new count
	if got := m.cur().rows[0].cells[1]; got != "1 tracks" {
		t.Errorf("count = %q", got)
	}
}

func TestQueuePlaylist(t *testing.T) {
	m, _, st := withStore(t)
	st.Create("P")
	m = press(m, "A", "enter", "j", "A", "enter", "P", "a")
	if m.queue.Len() != 2 {
		t.Errorf("queue len = %d, want 2", m.queue.Len())
	}
}

func TestDeletePlaylistNeedsConfirm(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want int // playlists left
	}{
		{"once does nothing", []string{"d"}, 1},
		{"twice deletes", []string{"d", "d"}, 0},
		{"another key disarms", []string{"d", "j", "d"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, st := withStore(t)
			st.Create("P")
			m = press(m, "P")
			m = press(m, tt.keys...)
			if got := len(st.Playlists()); got != tt.want {
				t.Errorf("playlists = %d, want %d", got, tt.want)
			}
			if tt.want == 0 && len(m.cur().rows) != 0 {
				t.Error("view still lists the deleted playlist")
			}
		})
	}
}

func TestCreateFromPlaylistsView(t *testing.T) {
	m, _, st := withStore(t)
	m = press(m, "P", "c", "Road trip", "enter")
	if pls := st.Playlists(); len(pls) != 1 || pls[0].Name != "Road trip" || len(pls[0].Songs) != 0 {
		t.Errorf("playlists = %+v", pls)
	}
	if len(m.cur().rows) != 1 {
		t.Error("view not refreshed after create")
	}
	// Duplicate name surfaces as an error, not a crash.
	m = press(m, "c", "road trip", "enter")
	if m.err == nil {
		t.Error("expected duplicate error")
	}
}

func TestOverlayFitsTerminal(t *testing.T) {
	m, _, st := withStore(t)
	st.Create("P")
	for _, keys := range [][]string{{"A"}, {"A", "j", "enter"}} {
		v := press(m, keys...).View()
		lines := strings.Split(v, "\n")
		if len(lines) != m.height {
			t.Errorf("keys %v: %d lines, want %d", keys, len(lines), m.height)
		}
		for _, l := range lines {
			if w := ansi.StringWidth(l); w > m.width {
				t.Errorf("keys %v: line width %d > %d", keys, w, m.width)
			}
		}
		if !strings.Contains(v, "playlist") && !strings.Contains(v, "Playlist") {
			t.Errorf("keys %v: overlay text missing", keys)
		}
	}
}
