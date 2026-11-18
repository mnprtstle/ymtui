package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "session.json")
	want := State{
		Songs:    []ytmusic.Song{{VideoID: "a", Title: "A"}, {VideoID: "b", Title: "B"}},
		Index:    1,
		Position: 42.5,
		Duration: 200,
		Repeat:   2,
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		wantErr bool
	}{
		{"missing file", nil, false},
		{"corrupt file", ptr("{nope"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.json")
			if tt.content != nil {
				os.WriteFile(path, []byte(*tt.content), 0o644)
			}
			s, err := Load(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if s.Valid() {
				t.Error("expected an invalid (empty) state")
			}
		})
	}
}

func TestValid(t *testing.T) {
	songs := []ytmusic.Song{{VideoID: "a"}}
	tests := []struct {
		s    State
		want bool
	}{
		{State{}, false},
		{State{Songs: songs, Index: 0}, true},
		{State{Songs: songs, Index: 1}, false},
		{State{Songs: songs, Index: -1}, false},
	}
	for _, tt := range tests {
		if got := tt.s.Valid(); got != tt.want {
			t.Errorf("%+v.Valid() = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func ptr(s string) *string { return &s }
