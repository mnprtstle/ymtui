package ytmusic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The structs below model only the parts of YouTube's search response we use.
// encoding/json ignores every field not declared here, and a missing field
// just leaves the zero value, so no nil checks are needed while walking.

type searchResponse struct {
	Contents struct {
		Tabbed struct {
			Tabs []struct {
				TabRenderer struct {
					Content struct {
						SectionList struct {
							Contents []struct {
								MusicShelf struct {
									Contents []shelfItem `json:"contents"`
								} `json:"musicShelfRenderer"`
							} `json:"contents"`
						} `json:"sectionListRenderer"`
					} `json:"content"`
				} `json:"tabRenderer"`
			} `json:"tabs"`
		} `json:"tabbedSearchResultsRenderer"`
	} `json:"contents"`
}

// run is one fragment of styled text; a displayed string is a list of runs.
type run struct {
	Text string `json:"text"`
}

// shelfItem is one row (musicResponsiveListItemRenderer). Search results,
// album tracklists and an artist's top songs all use this one shape; only
// which fields are filled differs.
type shelfItem struct {
	Row struct {
		PlaylistItemData struct {
			VideoID string `json:"videoId"`
		} `json:"playlistItemData"`
		// NavigationEndpoint is where the row itself leads: set on album and
		// artist rows, absent on songs.
		NavigationEndpoint struct {
			BrowseEndpoint struct {
				BrowseID string `json:"browseId"`
			} `json:"browseEndpoint"`
		} `json:"navigationEndpoint"`
		FlexColumns  []columnText `json:"flexColumns"`
		FixedColumns []columnText `json:"fixedColumns"`
	} `json:"musicResponsiveListItemRenderer"`
}

// columnText covers both column kinds. They differ only in the JSON key that
// wraps the text, so each gets a field and `runs` picks whichever is set.
type columnText struct {
	Flex struct {
		Text struct {
			Runs []run `json:"runs"`
		} `json:"text"`
	} `json:"musicResponsiveListItemFlexColumnRenderer"`
	Fixed struct {
		Text struct {
			Runs []run `json:"runs"`
		} `json:"text"`
	} `json:"musicResponsiveListItemFixedColumnRenderer"`
}

func (c columnText) text() string {
	return runsText(c.Flex.Text.Runs) + runsText(c.Fixed.Text.Runs)
}

// separator is the run text YouTube puts between "artist • album • time".
const separator = " • "

// searchRows decodes a search response and returns every row in it.
func searchRows(data []byte) ([]shelfItem, error) {
	var resp searchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	var rows []shelfItem
	for _, tab := range resp.Contents.Tabbed.Tabs {
		for _, section := range tab.TabRenderer.Content.SectionList.Contents {
			rows = append(rows, section.MusicShelf.Contents...)
		}
	}
	return rows, nil
}

// parseSongs extracts songs from a songs-filtered search response. Rows that
// lack a video ID or title are skipped rather than failing the whole search.
func parseSongs(data []byte) ([]Song, error) {
	rows, err := searchRows(data)
	if err != nil {
		return nil, err
	}
	var songs []Song
	for _, item := range rows {
		if s, ok := item.song(); ok {
			songs = append(songs, s)
		}
	}
	return songs, nil
}

func (it shelfItem) song() (Song, bool) {
	row := it.Row
	if row.PlaylistItemData.VideoID == "" || len(row.FlexColumns) < 2 {
		return Song{}, false
	}
	title := row.FlexColumns[0].text()
	if title == "" {
		return Song{}, false
	}

	// Second column joined: "Radiohead • Amnesiac • 3:11". Splitting on the
	// separator tolerates multiple artist runs ("A", " & ", "B") inside a part.
	parts := strings.Split(row.FlexColumns[1].text(), separator)
	s := Song{VideoID: row.PlaylistItemData.VideoID, Title: title, Artists: parts[0]}
	switch len(parts) {
	case 2: // artist • time
		s.Duration = parts[1]
	case 3: // artist • album • time
		s.Album, s.Duration = parts[1], parts[2]
	}
	return s, true
}

func runsText(runs []run) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
}
