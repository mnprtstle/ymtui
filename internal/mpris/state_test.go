package mpris

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		name string
		s    State
		want string
	}{
		{"nothing loaded", State{}, "Stopped"},
		{"playing", State{HasTrack: true, Playing: true}, "Playing"},
		{"paused", State{HasTrack: true}, "Paused"},
		{"playing flag without track", State{Playing: true}, "Stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.status(); got != tt.want {
				t.Errorf("status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrackPath(t *testing.T) {
	tests := []struct{ id, want string }{
		{"dQw4w9WgXcQ", "/com/github/ymtui/track/dQw4w9WgXcQ"},
		{"a-b_c", "/com/github/ymtui/track/a_b_c"},
	}
	for _, tt := range tests {
		got := trackPath(tt.id)
		if string(got) != tt.want || !got.IsValid() {
			t.Errorf("trackPath(%q) = %q (valid=%v), want %q", tt.id, got, got.IsValid(), tt.want)
		}
	}
}

func TestMetadata(t *testing.T) {
	s := State{HasTrack: true, VideoID: "x-y", Title: "T", Artists: "A", Album: "Al", Length: 3*time.Minute + 11*time.Second}
	m := s.metadata()
	want := map[string]any{
		"mpris:trackid": dbus.ObjectPath("/com/github/ymtui/track/x_y"),
		"xesam:title":   "T",
		"xesam:artist":  []string{"A"},
		"xesam:album":   "Al",
		"mpris:length":  int64(191_000_000),
	}
	for k, w := range want {
		v, ok := m[k]
		if !ok {
			t.Errorf("missing %s", k)
			continue
		}
		if got := v.Value(); !equal(got, w) {
			t.Errorf("%s = %v, want %v", k, got, w)
		}
	}

	// Without a track only the mandatory trackid is present.
	if empty := (State{}).metadata(); len(empty) != 1 {
		t.Errorf("empty metadata has %d keys, want 1", len(empty))
	}
	// Optional fields are omitted when unknown.
	if m := (State{HasTrack: true, VideoID: "a"}).metadata(); m["xesam:album"].Value() != nil || m["mpris:length"].Value() != nil {
		t.Error("album/length should be omitted when unknown")
	}
}

func equal(a, b any) bool {
	if as, ok := a.([]string); ok {
		bs, _ := b.([]string)
		return len(as) == len(bs) && (len(as) == 0 || as[0] == bs[0])
	}
	return a == b
}
