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

// Album is an album, single or EP, from search or an artist page.
type Album struct {
	BrowseID string // MPREb_..., pass to Client.Album
	Title    string
	Artists  string
	Kind     string // "Album", "Single" or "EP"; may be empty
	Year     string
}

// Artist is an artist search result.
type Artist struct {
	BrowseID string // UC..., pass to Client.Artist
	Name     string
	Info     string // e.g. "204M monthly audience"
}

// AlbumPage is an album with its tracklist. Each song's Artists and Album are
// filled from the page header, since rows on an album page don't repeat them.
type AlbumPage struct {
	Album
	Songs []Song
}

// ArtistPage is what we show for an artist: top songs and releases.
type ArtistPage struct {
	Name     string
	TopSongs []Song
	Albums   []Album // albums, singles and EPs
}
