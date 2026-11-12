package ytmusic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseAlbums_Fixture(t *testing.T) {
	got, err := parseAlbums(fixture(t, "search_albums.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d albums, want 4", len(got))
	}
	want := Album{BrowseID: "MPREb_yXhSI4FCUo6", Title: "OK Computer", Artists: "Radiohead", Kind: "Album", Year: "1997"}
	if got[0] != want {
		t.Errorf("albums[0] = %+v, want %+v", got[0], want)
	}
}

func TestParseArtists_Fixture(t *testing.T) {
	got, err := parseArtists(fixture(t, "search_artists.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no artists")
	}
	want := Artist{BrowseID: "UCr_iyUANcn9OX_yy9piYoLw", Name: "Radiohead", Info: "204M monthly audience"}
	if got[0] != want {
		t.Errorf("artists[0] = %+v, want %+v", got[0], want)
	}
}

func TestParseAlbumPage_Fixture(t *testing.T) {
	page, err := parseAlbumPage(fixture(t, "browse_album.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantAlbum := Album{Title: "OK Computer", Artists: "Radiohead", Kind: "Album", Year: "1997"}
	if page.Album != wantAlbum {
		t.Errorf("album = %+v, want %+v", page.Album, wantAlbum)
	}
	if len(page.Songs) != 4 {
		t.Fatalf("got %d songs, want 4", len(page.Songs))
	}
	want := Song{VideoID: "jNY_wLukVW0", Title: "Airbag", Artists: "Radiohead", Album: "OK Computer", Duration: "4:48"}
	if page.Songs[0] != want {
		t.Errorf("songs[0] = %+v, want %+v", page.Songs[0], want)
	}
}

func TestParseArtistPage_Fixture(t *testing.T) {
	page, err := parseArtistPage(fixture(t, "browse_artist.json"))
	if err != nil {
		t.Fatal(err)
	}
	if page.Name != "Radiohead" {
		t.Errorf("name = %q", page.Name)
	}
	if len(page.TopSongs) != 5 {
		t.Fatalf("got %d top songs, want 5", len(page.TopSongs))
	}
	wantSong := Song{VideoID: "zFYEYRcjK2g", Title: "Creep", Artists: "Radiohead", Album: "Pablo Honey"}
	if page.TopSongs[0] != wantSong {
		t.Errorf("top[0] = %+v, want %+v", page.TopSongs[0], wantSong)
	}
	// 3 albums + 2 singles; the videos and "fans might also like" carousels
	// have no album links and must be dropped.
	if len(page.Albums) != 5 {
		t.Fatalf("got %d albums, want 5: %+v", len(page.Albums), page.Albums)
	}
	wantAlbum := Album{BrowseID: "MPREb_E7NpKw54NIT", Title: "KID A MNESIA", Artists: "Radiohead", Year: "2021"}
	if page.Albums[0] != wantAlbum {
		t.Errorf("albums[0] = %+v, want %+v", page.Albums[0], wantAlbum)
	}
}

func TestParseBrowse_Edge(t *testing.T) {
	tests := []struct {
		name    string
		parse   func([]byte) (any, error)
		in      string
		want    any
		wantErr bool
	}{
		{"albums empty", func(b []byte) (any, error) { return parseAlbums(b) }, `{}`, []Album(nil), false},
		{"albums malformed", func(b []byte) (any, error) { return parseAlbums(b) }, `{"contents":`, nil, true},
		{"artists empty", func(b []byte) (any, error) { return parseArtists(b) }, `{}`, []Artist(nil), false},
		{"album page without title", func(b []byte) (any, error) { return parseAlbumPage(b) }, `{}`, AlbumPage{}, true},
		{"artist page without name", func(b []byte) (any, error) { return parseArtistPage(b) }, `{}`, ArtistPage{}, true},
		{"artist page malformed", func(b []byte) (any, error) { return parseArtistPage(b) }, `<html>`, ArtistPage{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parse([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBrowseRequests(t *testing.T) {
	album, artist := fixture(t, "browse_album.json"), fixture(t, "browse_artist.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got struct {
			BrowseID string `json:"browseId"`
		}
		json.NewDecoder(r.Body).Decode(&got)
		if r.URL.Path != "/youtubei/v1/browse" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got.BrowseID == "MPREb_x" {
			w.Write(album)
		} else {
			w.Write(artist)
		}
	}))
	defer srv.Close()
	c := newTestClient(srv.Client(), srv.URL)

	ap, err := c.Album(context.Background(), "MPREb_x")
	if err != nil || len(ap.Songs) != 4 {
		t.Errorf("Album: %d songs, err %v", len(ap.Songs), err)
	}
	rp, err := c.Artist(context.Background(), "UC_x")
	if err != nil || rp.Name != "Radiohead" {
		t.Errorf("Artist: %q, err %v", rp.Name, err)
	}
}
