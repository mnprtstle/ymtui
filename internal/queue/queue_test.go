package queue

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

func songs(ids ...string) []ytmusic.Song {
	out := make([]ytmusic.Song, len(ids))
	for i, id := range ids {
		out[i] = ytmusic.Song{VideoID: id}
	}
	return out
}

func ids(ss []ytmusic.Song) string {
	var b []byte
	for _, s := range ss {
		b = append(b, s.VideoID...)
	}
	return string(b)
}

func newQ() *Queue { return New(rand.New(rand.NewPCG(1, 2))) }

func TestNextPrev(t *testing.T) {
	tests := []struct {
		name   string
		repeat int // number of CycleRepeat calls
		start  int
		do     func(q *Queue) (ytmusic.Song, bool)
		want   string // "" = expect ok == false
	}{
		{"next", 0, 0, func(q *Queue) (ytmusic.Song, bool) { return q.Next(false) }, "b"},
		{"next at end, repeat off", 0, 2, func(q *Queue) (ytmusic.Song, bool) { return q.Next(false) }, ""},
		{"next at end, repeat all wraps", 1, 2, func(q *Queue) (ytmusic.Song, bool) { return q.Next(false) }, "a"},
		{"auto next, repeat one replays", 2, 1, func(q *Queue) (ytmusic.Song, bool) { return q.Next(true) }, "b"},
		{"manual next, repeat one moves on", 2, 1, func(q *Queue) (ytmusic.Song, bool) { return q.Next(false) }, "c"},
		{"prev", 0, 2, func(q *Queue) (ytmusic.Song, bool) { return q.Prev() }, "b"},
		{"prev at start, repeat off stays", 0, 0, func(q *Queue) (ytmusic.Song, bool) { return q.Prev() }, "a"},
		{"prev at start, repeat all wraps", 1, 0, func(q *Queue) (ytmusic.Song, bool) { return q.Prev() }, "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := newQ()
			q.Replace(songs("a", "b", "c"), tt.start)
			for range tt.repeat {
				q.CycleRepeat()
			}
			got, ok := tt.do(q)
			if tt.want == "" {
				if ok {
					t.Fatalf("got %q, want no song", got.VideoID)
				}
				return
			}
			if !ok || got.VideoID != tt.want {
				t.Fatalf("got %q (ok=%v), want %q", got.VideoID, ok, tt.want)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	q := newQ()
	if _, ok := q.Next(false); ok {
		t.Error("Next on empty queue returned a song")
	}
	if _, ok := q.Prev(); ok {
		t.Error("Prev on empty queue returned a song")
	}
	if _, ok := q.Current(); ok {
		t.Error("Current on empty queue returned a song")
	}
}

func TestAddStartsAtFirst(t *testing.T) {
	q := newQ()
	q.Add(songs("a", "b")...)
	got, ok := q.Next(false) // nothing played yet: Next plays the first
	if !ok || got.VideoID != "a" {
		t.Fatalf("got %q ok=%v, want a", got.VideoID, ok)
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		remove   int
		wantList string
		wantCur  string // "" = no current
		wantNext string
		wantPrev string
	}{
		{"before current", 2, 0, "bcd", "c", "d", "b"},
		{"after current", 1, 3, "abc", "b", "c", "a"},
		{"current", 2, 2, "abd", "", "d", "b"},
		{"current at start", 0, 0, "bcd", "", "b", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := func() *Queue {
				q := newQ()
				q.Replace(songs("a", "b", "c", "d"), tt.start)
				q.Remove(tt.remove)
				return q
			}
			q := build()
			if got := ids(q.Songs()); got != tt.wantList {
				t.Errorf("list = %q, want %q", got, tt.wantList)
			}
			cur, ok := q.Current()
			if (tt.wantCur == "") == ok || (ok && cur.VideoID != tt.wantCur) {
				t.Errorf("current = %q ok=%v, want %q", cur.VideoID, ok, tt.wantCur)
			}
			if s, ok := build().Next(false); !ok || s.VideoID != tt.wantNext {
				t.Errorf("next = %q ok=%v, want %q", s.VideoID, ok, tt.wantNext)
			}
			s, ok := build().Prev()
			if (tt.wantPrev == "") == ok || (ok && s.VideoID != tt.wantPrev) {
				t.Errorf("prev = %q ok=%v, want %q", s.VideoID, ok, tt.wantPrev)
			}
		})
	}
}

func TestShuffle(t *testing.T) {
	q := newQ()
	q.Replace(songs("a", "b", "c", "d", "e", "f"), 2)
	q.ToggleShuffle()

	got := q.Songs()
	if got[0].VideoID != "c" || q.Position() != 0 {
		t.Errorf("current song should lead: list %q pos %d", ids(got), q.Position())
	}
	sorted := ids(got)
	b := []byte(sorted)
	slices.Sort(b)
	if string(b) != "abcdef" {
		t.Errorf("shuffle lost or duplicated songs: %q", sorted)
	}

	// Walking forward visits every song exactly once.
	seen := map[string]bool{"c": true}
	for range 5 {
		s, ok := q.Next(false)
		if !ok || seen[s.VideoID] {
			t.Fatalf("repeat or early end at %q ok=%v", s.VideoID, ok)
		}
		seen[s.VideoID] = true
	}
	if _, ok := q.Next(false); ok {
		t.Error("expected end of queue")
	}

	// Un-shuffling restores the original order, with the current song kept.
	q.Jump(3)
	cur, _ := q.Current()
	q.ToggleShuffle()
	if ids(q.Songs()) != "abcdef" {
		t.Errorf("order not restored: %q", ids(q.Songs()))
	}
	if now, _ := q.Current(); now.VideoID != cur.VideoID {
		t.Errorf("current changed on un-shuffle: %q -> %q", cur.VideoID, now.VideoID)
	}
}

func TestAddWhileShuffled(t *testing.T) {
	q := newQ()
	q.Replace(songs("a", "b", "c"), 0)
	q.ToggleShuffle()
	q.Add(songs("x", "y")...)
	got := q.Songs()
	if len(got) != 5 || got[0].VideoID != "a" {
		t.Fatalf("unexpected list %q", ids(got))
	}
}

func TestCycleRepeat(t *testing.T) {
	q := newQ()
	want := []Repeat{RepeatAll, RepeatOne, RepeatOff}
	for i, w := range want {
		if got := q.CycleRepeat(); got != w {
			t.Errorf("step %d: got %v, want %v", i, got, w)
		}
	}
}
