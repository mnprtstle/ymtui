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

type shelfItem struct {
	Row struct {
		PlaylistItemData struct {
			VideoID string `json:"videoId"`
		} `json:"playlistItemData"`
		FlexColumns []struct {
			Column struct {
				Text struct {
					Runs []run `json:"runs"`
				} `json:"text"`
			} `json:"musicResponsiveListItemFlexColumnRenderer"`
		} `json:"flexColumns"`
	} `json:"musicResponsiveListItemRenderer"`
}

// separator is the run text YouTube puts between "artist • album • time".
const separator = " • "

// parseSongs extracts songs from a songs-filtered search response. Rows that
// lack a video ID or title are skipped rather than failing the whole search.
func parseSongs(data []byte) ([]Song, error) {
	var resp searchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	var songs []Song
	for _, tab := range resp.Contents.Tabbed.Tabs {
		for _, section := range tab.TabRenderer.Content.SectionList.Contents {
			for _, item := range section.MusicShelf.Contents {
				if s, ok := item.song(); ok {
					songs = append(songs, s)
				}
			}
		}
	}
	return songs, nil
}

func (it shelfItem) song() (Song, bool) {
	row := it.Row
	if row.PlaylistItemData.VideoID == "" || len(row.FlexColumns) < 2 {
		return Song{}, false
	}
	title := runsText(row.FlexColumns[0].Column.Text.Runs)
	if title == "" {
		return Song{}, false
	}

	// Second column joined: "Radiohead • Amnesiac • 3:11". Splitting on the
	// separator tolerates multiple artist runs ("A", " & ", "B") inside a part.
	parts := strings.Split(runsText(row.FlexColumns[1].Column.Text.Runs), separator)
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
