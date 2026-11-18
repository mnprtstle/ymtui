// Package session saves what was playing when the app quit, so the next start
// can pick up where it left off.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mnprtstle/ymtui/internal/atomicfile"
	"github.com/mnprtstle/ymtui/internal/store"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// State is the resumable part of the player. Songs are in play order, so a
// shuffled queue comes back in the same order (with shuffle itself off).
type State struct {
	Songs    []ytmusic.Song `json:"songs"`
	Index    int            `json:"index"`    // current song in Songs
	Position float64        `json:"position"` // seconds into it
	Duration float64        `json:"duration"` // seconds, for the progress bar
	Repeat   int            `json:"repeat"`
}

// Valid reports whether there is something worth restoring.
func (s State) Valid() bool { return s.Index >= 0 && s.Index < len(s.Songs) }

// DefaultPath is session.json next to the playlists.
func DefaultPath() (string, error) {
	dir, err := store.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// Load reads path. A missing file yields an empty State and no error. A
// corrupt file returns an error; the caller should just start fresh.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read session: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// Save writes s atomically.
func Save(path string, s State) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	return atomicfile.Write(path, data)
}
