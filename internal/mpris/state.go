// Package mpris exposes the player on the D-Bus session bus using the MPRIS
// standard, so media keys, playerctl and desktop widgets can control it.
//
// The package knows nothing about the UI: callers push a State in with
// SetState and receive Commands through a callback.
package mpris

import (
	"time"

	"github.com/godbus/dbus/v5"
)

// CommandKind says what a remote control asked for.
type CommandKind int

const (
	Play CommandKind = iota
	Pause
	PlayPause
	Stop
	Next
	Previous
	SeekBy    // Command.Offset, relative; may be negative
	SetVolume // Command.Volume, 0..1
)

// Command is one request from a D-Bus client.
type Command struct {
	Kind   CommandKind
	Offset time.Duration
	Volume float64
}

// State is everything MPRIS clients can see.
type State struct {
	HasTrack bool // false = nothing loaded (status "Stopped")
	Playing  bool
	VideoID  string
	Title    string
	Artists  string
	Album    string
	Length   time.Duration
	Position time.Duration
	Volume   float64 // 0..1
	CanNext  bool
	CanPrev  bool
}

// Playback status strings defined by the MPRIS spec.
func (s State) status() string {
	switch {
	case !s.HasTrack:
		return "Stopped"
	case s.Playing:
		return "Playing"
	}
	return "Paused"
}

// trackPath is the D-Bus object path that identifies the current track.
// Object paths only allow [A-Za-z0-9_], but video IDs also contain '-'.
func trackPath(videoID string) dbus.ObjectPath {
	b := []byte(videoID)
	for i, c := range b {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			b[i] = '_'
		}
	}
	return dbus.ObjectPath("/com/github/ymtui/track/" + string(b))
}

const noTrack = dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")

// metadata builds the xesam metadata map. The Variant wrapper is how D-Bus
// carries "any type" values: a{sv} is a map of string to variant.
func (s State) metadata() map[string]dbus.Variant {
	if !s.HasTrack {
		return map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(noTrack)}
	}
	m := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(trackPath(s.VideoID)),
		"xesam:title":   dbus.MakeVariant(s.Title),
		"xesam:url":     dbus.MakeVariant("https://music.youtube.com/watch?v=" + s.VideoID),
	}
	if s.Artists != "" {
		m["xesam:artist"] = dbus.MakeVariant([]string{s.Artists})
	}
	if s.Album != "" {
		m["xesam:album"] = dbus.MakeVariant(s.Album)
	}
	if s.Length > 0 {
		m["mpris:length"] = dbus.MakeVariant(s.Length.Microseconds()) // int64 µs
	}
	return m
}
