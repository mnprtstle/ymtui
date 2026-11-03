package ytmusic

import (
	"os"
	"reflect"
	"testing"
)

func TestParseSongs_Fixture(t *testing.T) {
	data, err := os.ReadFile("testdata/search_songs.json")
	if err != nil {
		t.Fatal(err)
	}
	songs, err := parseSongs(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 6 {
		t.Fatalf("got %d songs, want 6", len(songs))
	}
	want := Song{
		VideoID:  "QQnc-hM80UQ",
		Title:    "You And Whose Army?",
		Artists:  "Radiohead",
		Album:    "Amnesiac",
		Duration: "3:11",
	}
	if songs[0] != want {
		t.Errorf("songs[0] = %+v, want %+v", songs[0], want)
	}
	for i, s := range songs {
		if s.VideoID == "" || s.Title == "" || s.Duration == "" {
			t.Errorf("songs[%d] has empty field: %+v", i, s)
		}
	}
}

func TestParseSongs(t *testing.T) {
	row := func(id, title, sub string) string {
		return `{"musicResponsiveListItemRenderer":{"playlistItemData":{"videoId":"` + id + `"},"flexColumns":[` +
			`{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"` + title + `"}]}}},` +
			`{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":` + sub + `}}}]}}`
	}
	wrap := func(rows ...string) string {
		joined := ""
		for i, r := range rows {
			if i > 0 {
				joined += ","
			}
			joined += r
		}
		return `{"contents":{"tabbedSearchResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[` +
			`{"musicShelfRenderer":{"contents":[` + joined + `]}}]}}}}]}}}`
	}
	sep := `{"text":" • "}`

	tests := []struct {
		name    string
		in      string
		want    []Song
		wantErr bool
	}{
		{
			name: "artist album time",
			in:   wrap(row("a1", "T", `[{"text":"Art"},`+sep+`,{"text":"Alb"},`+sep+`,{"text":"1:23"}]`)),
			want: []Song{{VideoID: "a1", Title: "T", Artists: "Art", Album: "Alb", Duration: "1:23"}},
		},
		{
			name: "no album",
			in:   wrap(row("a2", "T", `[{"text":"Art"},`+sep+`,{"text":"4:56"}]`)),
			want: []Song{{VideoID: "a2", Title: "T", Artists: "Art", Duration: "4:56"}},
		},
		{
			name: "multiple artist runs",
			in:   wrap(row("a3", "T", `[{"text":"A"},{"text":" & "},{"text":"B"},`+sep+`,{"text":"Alb"},`+sep+`,{"text":"2:00"}]`)),
			want: []Song{{VideoID: "a3", Title: "T", Artists: "A & B", Album: "Alb", Duration: "2:00"}},
		},
		{
			name: "row without video id is skipped",
			in:   wrap(row("", "T", `[{"text":"Art"}]`), row("ok", "U", `[{"text":"Art"}]`)),
			want: []Song{{VideoID: "ok", Title: "U", Artists: "Art"}},
		},
		{name: "empty object", in: `{}`, want: nil},
		{name: "malformed json", in: `{"contents":`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSongs([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
