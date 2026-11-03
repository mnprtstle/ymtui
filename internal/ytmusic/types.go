// Package ytmusic talks to the YouTube Music InnerTube API. All knowledge of
// YouTube's response shapes lives here; callers only see the types below.
package ytmusic

import "fmt"

// Song is one track from a search result.
type Song struct {
	VideoID  string
	Title    string
	Artists  string // as displayed, e.g. "Radiohead"
	Album    string // empty for videos and singles without an album
	Duration string // as displayed, e.g. "3:11"
}

// String formats the song as one line for the CLI.
func (s Song) String() string {
	album := s.Album
	if album == "" {
		album = "-"
	}
	return fmt.Sprintf("%s — %s — %s — %s", s.Title, s.Artists, album, s.Duration)
}
