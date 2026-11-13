package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

var (
	songA = ytmusic.Song{VideoID: "a", Title: "A", Artists: "X"}
	songB = ytmusic.Song{VideoID: "b", Title: "B", Artists: "Y", Album: "Alb", Duration: "3:11"}
)

// open returns a store backed by a fresh temp dir; t.TempDir is deleted
// automatically when the test ends.
func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "playlists.json") // sub: save must create it
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name    string
		create  []string
		wantErr error
		want    []string
	}{
		{"one", []string{"Chill"}, nil, []string{"Chill"}},
		{"trimmed", []string{"  Chill  "}, nil, []string{"Chill"}},
		{"empty", []string{"   "}, ErrEmpty, nil},
		{"duplicate", []string{"Chill", "Chill"}, ErrExists, []string{"Chill"}},
		{"duplicate ignoring case", []string{"Chill", "chill"}, ErrExists, []string{"Chill"}},
		{"order kept", []string{"B", "A"}, nil, []string{"B", "A"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := open(t)
			var err error
			for _, n := range tt.create {
				err = s.Create(n) // the last error is the one we assert on
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			var got []string
			for _, p := range s.Playlists() {
				got = append(got, p.Name)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("names = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("names = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSongs(t *testing.T) {
	tests := []struct {
		name      string
		do        func(s *Store) error
		wantErr   error
		wantSongs []string // video IDs left in "P"
	}{
		{"add", func(s *Store) error { _, err := s.AddSong("P", songA); return err }, nil, []string{"a"}},
		{"add keeps order", func(s *Store) error {
			s.AddSong("P", songA)
			_, err := s.AddSong("P", songB)
			return err
		}, nil, []string{"a", "b"}},
		{"add duplicate is a no-op", func(s *Store) error {
			s.AddSong("P", songA)
			added, err := s.AddSong("P", songA)
			if added {
				t.Error("duplicate reported as added")
			}
			return err
		}, nil, []string{"a"}},
		{"add to missing", func(s *Store) error { _, err := s.AddSong("nope", songA); return err }, ErrNotFound, nil},
		{"add is case-insensitive", func(s *Store) error { _, err := s.AddSong("p", songA); return err }, nil, []string{"a"}},
		{"remove", func(s *Store) error {
			s.AddSong("P", songA)
			s.AddSong("P", songB)
			return s.RemoveSong("P", 0)
		}, nil, []string{"b"}},
		{"remove out of range", func(s *Store) error {
			s.AddSong("P", songA)
			return s.RemoveSong("P", 1)
		}, nil, []string{"a"}},
		{"remove from missing", func(s *Store) error { return s.RemoveSong("nope", 0) }, ErrNotFound, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := open(t)
			if err := s.Create("P"); err != nil {
				t.Fatal(err)
			}
			err := tt.do(s)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			var got []string
			for _, song := range s.Playlists()[0].Songs {
				got = append(got, song.VideoID)
			}
			if len(got) != len(tt.wantSongs) {
				t.Fatalf("songs = %v, want %v", got, tt.wantSongs)
			}
			for i := range got {
				if got[i] != tt.wantSongs[i] {
					t.Fatalf("songs = %v, want %v", got, tt.wantSongs)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	s, _ := open(t)
	s.Create("A")
	s.Create("B")
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if got := s.Playlists(); len(got) != 1 || got[0].Name != "B" {
		t.Errorf("after delete: %+v", got)
	}
	if err := s.Delete("A"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestPersistence(t *testing.T) {
	s, path := open(t)
	s.Create("P")
	s.AddSong("P", songB)

	again, err := Open(path) // a fresh process would do exactly this
	if err != nil {
		t.Fatal(err)
	}
	got := again.Playlists()
	if len(got) != 1 || got[0].Name != "P" || len(got[0].Songs) != 1 || got[0].Songs[0] != songB {
		t.Errorf("reloaded = %+v", got)
	}
	// Saving leaves only the real file behind, no temp files.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1", len(entries))
	}
}

func TestPlaylistsReturnsCopy(t *testing.T) {
	s, _ := open(t)
	s.Create("P")
	s.AddSong("P", songA)
	got := s.Playlists()
	got[0].Songs[0].Title = "changed"
	if s.Playlists()[0].Songs[0].Title != "A" {
		t.Error("editing the returned slice changed the store")
	}
}

func TestFailedSaveKeepsState(t *testing.T) {
	s, path := open(t)
	s.Create("P")
	// Make the directory read-only so the next save fails.
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := s.Create("Q"); err == nil {
		t.Fatal("expected a save error")
	}
	if got := s.Playlists(); len(got) != 1 {
		t.Errorf("failed save changed memory: %+v", got)
	}
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"missing file starts empty", filepath.Join(dir, "none.json"), false},
		{"corrupt file is an error", bad, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(s.Playlists()) != 0 {
				t.Error("expected empty store")
			}
		})
	}
}
