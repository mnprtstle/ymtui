package ytmusic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseAlbums extracts albums from an albums-filtered search response.
// Subtitle runs look like "Album • Radiohead • 1997".
func parseAlbums(data []byte) ([]Album, error) {
	rows, err := searchRows(data)
	if err != nil {
		return nil, err
	}
	var albums []Album
	for _, it := range rows {
		id := it.Row.NavigationEndpoint.BrowseEndpoint.BrowseID
		if !strings.HasPrefix(id, albumPrefix) || len(it.Row.FlexColumns) < 2 {
			continue
		}
		a := Album{BrowseID: id, Title: it.Row.FlexColumns[0].text()}
		parts := strings.Split(it.Row.FlexColumns[1].text(), separator)
		a.Kind = parts[0]
		if len(parts) > 1 {
			a.Artists = parts[1]
		}
		if len(parts) > 2 {
			a.Year = parts[2]
		}
		if a.Title != "" {
			albums = append(albums, a)
		}
	}
	return albums, nil
}

// parseArtists extracts artists from an artists-filtered search response.
// Subtitle runs look like "Artist • 204M monthly audience".
func parseArtists(data []byte) ([]Artist, error) {
	rows, err := searchRows(data)
	if err != nil {
		return nil, err
	}
	var artists []Artist
	for _, it := range rows {
		id := it.Row.NavigationEndpoint.BrowseEndpoint.BrowseID
		if !strings.HasPrefix(id, "UC") || len(it.Row.FlexColumns) < 1 {
			continue
		}
		a := Artist{BrowseID: id, Name: it.Row.FlexColumns[0].text()}
		if len(it.Row.FlexColumns) > 1 {
			if parts := strings.SplitN(it.Row.FlexColumns[1].text(), separator, 2); len(parts) == 2 {
				a.Info = parts[1]
			}
		}
		if a.Name != "" {
			artists = append(artists, a)
		}
	}
	return artists, nil
}

// albumPrefix starts every album/single/EP browse ID.
const albumPrefix = "MPRE"

// runs wraps the {"runs":[...]} object YouTube uses for every text value.
type runs struct {
	Runs []run `json:"runs"`
}

func (r runs) String() string { return runsText(r.Runs) }

type albumPageResponse struct {
	Contents struct {
		TwoColumn struct {
			Tabs []struct {
				TabRenderer struct {
					Content struct {
						SectionList struct {
							Contents []struct {
								Header struct {
									Title    runs `json:"title"`
									Subtitle runs `json:"subtitle"`
									Strap    runs `json:"straplineTextOne"`
								} `json:"musicResponsiveHeaderRenderer"`
							} `json:"contents"`
						} `json:"sectionListRenderer"`
					} `json:"content"`
				} `json:"tabRenderer"`
			} `json:"tabs"`
			Secondary struct {
				SectionList struct {
					Contents []struct {
						MusicShelf struct {
							Contents []shelfItem `json:"contents"`
						} `json:"musicShelfRenderer"`
					} `json:"contents"`
				} `json:"sectionListRenderer"`
			} `json:"secondaryContents"`
		} `json:"twoColumnBrowseResultsRenderer"`
	} `json:"contents"`
}

// parseAlbumPage reads an album's header and tracklist. A tracklist row has
// the title and a fixed duration column; artist and album come from the header.
func parseAlbumPage(data []byte) (AlbumPage, error) {
	var resp albumPageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return AlbumPage{}, fmt.Errorf("decode album page: %w", err)
	}
	var page AlbumPage
	tc := resp.Contents.TwoColumn
	for _, tab := range tc.Tabs {
		for _, sec := range tab.TabRenderer.Content.SectionList.Contents {
			h := sec.Header
			if h.Title.String() == "" {
				continue
			}
			page.Title, page.Artists = h.Title.String(), h.Strap.String()
			parts := strings.Split(h.Subtitle.String(), separator)
			page.Kind = parts[0]
			if len(parts) > 1 {
				page.Year = parts[len(parts)-1]
			}
		}
	}
	if page.Title == "" {
		return AlbumPage{}, fmt.Errorf("album page has no title")
	}
	for _, sec := range tc.Secondary.SectionList.Contents {
		for _, it := range sec.MusicShelf.Contents {
			row := it.Row
			if row.PlaylistItemData.VideoID == "" || len(row.FlexColumns) == 0 {
				continue
			}
			s := Song{
				VideoID: row.PlaylistItemData.VideoID,
				Title:   row.FlexColumns[0].text(),
				Artists: page.Artists,
				Album:   page.Title,
			}
			if len(row.FixedColumns) > 0 {
				s.Duration = row.FixedColumns[0].text()
			}
			if s.Title != "" {
				page.Songs = append(page.Songs, s)
			}
		}
	}
	return page, nil
}

type artistPageResponse struct {
	Header struct {
		Immersive struct {
			Title runs `json:"title"`
		} `json:"musicImmersiveHeaderRenderer"`
	} `json:"header"`
	Contents struct {
		SingleColumn struct {
			Tabs []struct {
				TabRenderer struct {
					Content struct {
						SectionList struct {
							Contents []struct {
								// The first shelf is "Top songs"; the rest are
								// carousels of releases, videos, playlists...
								MusicShelf struct {
									Contents []shelfItem `json:"contents"`
								} `json:"musicShelfRenderer"`
								Carousel struct {
									Contents []struct {
										Item struct {
											Title      runs `json:"title"`
											Subtitle   runs `json:"subtitle"`
											Navigation struct {
												BrowseEndpoint struct {
													BrowseID string `json:"browseId"`
												} `json:"browseEndpoint"`
											} `json:"navigationEndpoint"`
										} `json:"musicTwoRowItemRenderer"`
									} `json:"contents"`
								} `json:"musicCarouselShelfRenderer"`
							} `json:"contents"`
						} `json:"sectionListRenderer"`
					} `json:"content"`
				} `json:"tabRenderer"`
			} `json:"tabs"`
		} `json:"singleColumnBrowseResultsRenderer"`
	} `json:"contents"`
}

// parseArtistPage collects top songs plus every carousel item that links to
// an album page. That one filter (browse ID prefix) drops videos, playlists
// and "fans might also like" artists without matching on shelf titles, which
// YouTube localizes and renames.
func parseArtistPage(data []byte) (ArtistPage, error) {
	var resp artistPageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ArtistPage{}, fmt.Errorf("decode artist page: %w", err)
	}
	page := ArtistPage{Name: resp.Header.Immersive.Title.String()}
	if page.Name == "" {
		return ArtistPage{}, fmt.Errorf("artist page has no name")
	}
	for _, tab := range resp.Contents.SingleColumn.Tabs {
		for _, sec := range tab.TabRenderer.Content.SectionList.Contents {
			for _, it := range sec.MusicShelf.Contents {
				// Top songs columns: title, artist, plays, album.
				row := it.Row
				if row.PlaylistItemData.VideoID == "" || len(row.FlexColumns) < 2 {
					continue
				}
				s := Song{
					VideoID: row.PlaylistItemData.VideoID,
					Title:   row.FlexColumns[0].text(),
					Artists: row.FlexColumns[1].text(),
				}
				if len(row.FlexColumns) > 3 {
					s.Album = row.FlexColumns[3].text()
				}
				if s.Title != "" {
					page.TopSongs = append(page.TopSongs, s)
				}
			}
			for _, c := range sec.Carousel.Contents {
				id := c.Item.Navigation.BrowseEndpoint.BrowseID
				if !strings.HasPrefix(id, albumPrefix) {
					continue
				}
				// Subtitle is "2021" or "Single • 2021".
				a := Album{BrowseID: id, Title: c.Item.Title.String(), Artists: page.Name}
				parts := strings.Split(c.Item.Subtitle.String(), separator)
				a.Year = parts[len(parts)-1]
				if len(parts) > 1 {
					a.Kind = parts[0]
				}
				page.Albums = append(page.Albums, a)
			}
		}
	}
	return page, nil
}
