package ui

import "github.com/mnprtstle/ymtui/internal/ytmusic"

// tab is a search result type; Tab cycles through them.
type tab int

const (
	tabSongs tab = iota
	tabAlbums
	tabArtists
	numTabs
)

var tabNames = [numTabs]string{"Songs", "Albums", "Artists"}

// rowKind says what Enter does on a row.
type rowKind int

const (
	rowSong   rowKind = iota // play it
	rowAlbum                 // push an album view
	rowArtist                // push an artist view
)

// row is one line of a list. Every list in the app (songs, albums, artists,
// an album's tracks, an artist's page) is a []row, so the table rendering and
// cursor code exist once. cells are the four display columns after the "#".
type row struct {
	kind  rowKind
	cells [4]string
	song  ytmusic.Song // valid when kind == rowSong
	id    string       // browse ID, when kind is rowAlbum or rowArtist
}

// Column headers per list type.
var (
	songsHeader   = [4]string{"Title", "Artist", "Album", "Time"}
	albumsHeader  = [4]string{"Title", "Artist", "Type", "Year"}
	artistsHeader = [4]string{"Name", "Info", "", ""}
	pageHeader    = [4]string{"Title", "Type", "Album", "Year"} // artist page: songs and releases mixed
)

// listView is one entry in the view stack. The stack works like directories:
// Enter/l pushes, h/Esc pops, and each view keeps its own cursor so going
// back lands where you left.
type listView struct {
	title   string // breadcrumb segment
	id      string // browse ID of the page this view is loading or showing
	header  [4]string
	rows    []row
	loading bool

	cursor, offset int
}

func songRows(songs []ytmusic.Song) []row {
	rows := make([]row, len(songs))
	for i, s := range songs {
		rows[i] = row{kind: rowSong, song: s, cells: [4]string{s.Title, s.Artists, s.Album, s.Duration}}
	}
	return rows
}

func albumRows(albums []ytmusic.Album) []row {
	rows := make([]row, len(albums))
	for i, a := range albums {
		rows[i] = row{kind: rowAlbum, id: a.BrowseID, cells: [4]string{a.Title, a.Artists, a.Kind, a.Year}}
	}
	return rows
}

func artistRows(artists []ytmusic.Artist) []row {
	rows := make([]row, len(artists))
	for i, a := range artists {
		rows[i] = row{kind: rowArtist, id: a.BrowseID, cells: [4]string{a.Name, a.Info}}
	}
	return rows
}

// artistPageRows lists top songs, then releases. Releases reuse the columns:
// Type goes under "Type", the year under "Year".
func artistPageRows(p ytmusic.ArtistPage) []row {
	var rows []row
	for _, s := range p.TopSongs {
		rows = append(rows, row{kind: rowSong, song: s, cells: [4]string{s.Title, "Song", s.Album, ""}})
	}
	for _, a := range p.Albums {
		kind := a.Kind
		if kind == "" {
			kind = "Album"
		}
		rows = append(rows, row{kind: rowAlbum, id: a.BrowseID, cells: [4]string{a.Title, kind, "", a.Year}})
	}
	return rows
}

func headerFor(t tab) [4]string {
	switch t {
	case tabAlbums:
		return albumsHeader
	case tabArtists:
		return artistsHeader
	}
	return songsHeader
}

// songs returns the playable songs in the view and the index of the cursor
// row among them (-1 if the cursor is on a non-song row). Enter on a song
// queues all songs in the view starting there.
func (v *listView) songs() (songs []ytmusic.Song, at int) {
	at = -1
	for i, r := range v.rows {
		if r.kind != rowSong {
			continue
		}
		if i == v.cursor {
			at = len(songs)
		}
		songs = append(songs, r.song)
	}
	return songs, at
}
