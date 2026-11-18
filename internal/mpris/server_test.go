package mpris

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// TestServerRoundTrip talks to our own server over the real session bus, the
// way playerctl would. It is skipped when no session bus is available.
func TestServerRoundTrip(t *testing.T) {
	probe, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skipf("no session bus: %v", err)
	}
	defer probe.Close()

	got := make(chan Command, 8)
	srv, err := Start(func(c Command) { got <- c })
	if err != nil {
		t.Skipf("cannot claim %s (already running?): %v", ownName, err)
	}
	defer srv.Close()

	obj := probe.Object(ownName, objectPath)

	calls := []struct {
		method string
		args   []any
		want   Command
	}{
		{playIface + ".PlayPause", nil, Command{Kind: PlayPause}},
		{playIface + ".Next", nil, Command{Kind: Next}},
		{playIface + ".Previous", nil, Command{Kind: Previous}},
		{playIface + ".Seek", []any{int64(5_000_000)}, Command{Kind: SeekBy, Offset: 5 * time.Second}},
	}
	for _, c := range calls {
		if err := obj.Call(c.method, 0, c.args...).Err; err != nil {
			t.Fatalf("%s: %v", c.method, err)
		}
		select {
		case cmd := <-got:
			if cmd != c.want {
				t.Errorf("%s -> %+v, want %+v", c.method, cmd, c.want)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s: no command received", c.method)
		}
	}

	// Setting Volume over D-Bus becomes a command.
	if err := obj.Call(propsIface+".Set", 0, playIface, "Volume", dbus.MakeVariant(0.5)).Err; err != nil {
		t.Fatalf("set Volume: %v", err)
	}
	if cmd := <-got; cmd.Kind != SetVolume || cmd.Volume != 0.5 {
		t.Errorf("volume -> %+v", cmd)
	}

	// State pushed in is readable by clients.
	srv.SetState(State{HasTrack: true, Playing: true, VideoID: "abc", Title: "Song", Artists: "Band", Position: 2 * time.Second})
	var status string
	if err := obj.Call(propsIface+".Get", 0, playIface, "PlaybackStatus").Store(&status); err != nil || status != "Playing" {
		t.Errorf("PlaybackStatus = %q, %v", status, err)
	}
	var md map[string]dbus.Variant
	if err := obj.Call(propsIface+".Get", 0, playIface, "Metadata").Store(&md); err != nil || md["xesam:title"].Value() != "Song" {
		t.Errorf("Metadata = %v, %v", md, err)
	}
	var pos int64
	if err := obj.Call(propsIface+".Get", 0, playIface, "Position").Store(&pos); err != nil || pos != 2_000_000 {
		t.Errorf("Position = %d, %v", pos, err)
	}

	// A new track without an album must not inherit the old track's album.
	srv.SetState(State{HasTrack: true, Playing: true, VideoID: "abc", Title: "Song", Album: "Old"})
	srv.SetState(State{HasTrack: true, Playing: true, VideoID: "def", Title: "Next"})
	md = nil
	if err := obj.Call(propsIface+".Get", 0, playIface, "Metadata").Store(&md); err != nil {
		t.Fatal(err)
	}
	if _, stale := md["xesam:album"]; stale || md["xesam:title"].Value() != "Next" {
		t.Errorf("stale metadata after track change: %v", md)
	}
}
