// Package store persists local playlists as JSON in the XDG data directory.
//
// A Store keeps every playlist in memory and rewrites the whole file on each
// change. That is fine at this scale (a few hundred songs is a few tens of KB)
// and keeps the code small. It is not safe for concurrent use; the UI calls it
// from Update, which is single-threaded.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// Sentinel errors. Callers compare with errors.Is, so wrapping with extra
// context (fmt.Errorf("...: %w", err)) never breaks the check.
var (
	ErrExists   = errors.New("playlist already exists")
	ErrNotFound = errors.New("playlist not found")
	ErrEmpty    = errors.New("playlist name is empty")
)

// Playlist is a named, ordered list of songs.
type Playlist struct {
	Name  string         `json:"name"`
	Songs []ytmusic.Song `json:"songs"`
}

// fileFormat is the on-disk shape. The version lets a future release migrate
// old files instead of guessing.
type fileFormat struct {
	Version   int        `json:"version"`
	Playlists []Playlist `json:"playlists"`
}

const currentVersion = 1

// Store is the set of playlists plus the file they live in.
type Store struct {
	path      string
	playlists []Playlist
}

// DefaultPath is $XDG_DATA_HOME/ymtui/playlists.json, falling back to
// ~/.local/share/ymtui/playlists.json. (os.UserConfigDir exists in the
// standard library but there is no data-dir equivalent, hence this.)
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find data dir: %w", err)
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "ymtui", "playlists.json"), nil
}

// Open loads the store at path. A missing file is not an error: it is the
// first run, and the store starts empty. The file is created on first save.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read playlists %s: %w", path, err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse playlists %s: %w", path, err)
	}
	s.playlists = f.Playlists
	return s, nil
}

// Playlists returns a copy of all playlists, so callers can't change the
// store's data by accident.
func (s *Store) Playlists() []Playlist { return clone(s.playlists) }

// Create adds an empty playlist. Names are unique ignoring case.
func (s *Store) Create(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmpty
	}
	if find(s.playlists, name) >= 0 {
		return fmt.Errorf("create %q: %w", name, ErrExists)
	}
	next := clone(s.playlists)
	next = append(next, Playlist{Name: name})
	return s.commit(next)
}

// Delete removes a playlist.
func (s *Store) Delete(name string) error {
	i := find(s.playlists, name)
	if i < 0 {
		return fmt.Errorf("delete %q: %w", name, ErrNotFound)
	}
	next := clone(s.playlists)
	next = slices.Delete(next, i, i+1)
	return s.commit(next)
}

// AddSong appends a song to a playlist. Adding a song that is already there
// is not an error; added reports whether anything changed.
func (s *Store) AddSong(name string, song ytmusic.Song) (added bool, err error) {
	i := find(s.playlists, name)
	if i < 0 {
		return false, fmt.Errorf("add to %q: %w", name, ErrNotFound)
	}
	dup := slices.ContainsFunc(s.playlists[i].Songs, func(x ytmusic.Song) bool { return x.VideoID == song.VideoID })
	if dup {
		return false, nil
	}
	next := clone(s.playlists)
	next[i].Songs = append(next[i].Songs, song)
	if err := s.commit(next); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveSong deletes the song at index idx from a playlist.
func (s *Store) RemoveSong(name string, idx int) error {
	i := find(s.playlists, name)
	if i < 0 {
		return fmt.Errorf("remove from %q: %w", name, ErrNotFound)
	}
	if idx < 0 || idx >= len(s.playlists[i].Songs) {
		return fmt.Errorf("remove from %q: index %d out of range", name, idx)
	}
	next := clone(s.playlists)
	next[i].Songs = slices.Delete(next[i].Songs, idx, idx+1)
	return s.commit(next)
}

// commit saves next and only then makes it the live state. Every mutation
// builds a modified copy and commits it, so a failed write (disk full,
// permissions) leaves memory and disk agreeing with each other.
func (s *Store) commit(next []Playlist) error {
	if err := s.save(next); err != nil {
		return err
	}
	s.playlists = next
	return nil
}

// save writes atomically: write a temp file in the same directory, then
// rename it over the target. Rename within one filesystem is atomic, so a
// crash leaves either the old file or the new one, never half of one.
func (s *Store) save(pls []Playlist) error {
	data, err := json.MarshalIndent(fileFormat{Version: currentVersion, Playlists: pls}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode playlists: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "playlists-*.tmp")
	if err != nil {
		return fmt.Errorf("save playlists: %w", err)
	}
	// On any failure below, don't leave the temp file behind. After a
	// successful rename it no longer exists, so this Remove is a no-op.
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("save playlists: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save playlists: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("save playlists: %w", err)
	}
	return nil
}

// find returns the index of the playlist called name (ignoring case), or -1.
func find(pls []Playlist, name string) int {
	return slices.IndexFunc(pls, func(p Playlist) bool { return strings.EqualFold(p.Name, name) })
}

// clone deep-copies playlists. A plain slice copy would share each
// playlist's Songs backing array, and edits to the copy would leak through.
func clone(pls []Playlist) []Playlist {
	out := make([]Playlist, len(pls))
	for i, p := range pls {
		out[i] = Playlist{Name: p.Name, Songs: slices.Clone(p.Songs)}
	}
	return out
}
