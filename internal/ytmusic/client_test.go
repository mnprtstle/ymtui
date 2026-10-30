package ytmusic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSearch(t *testing.T) {
	fixture, err := os.ReadFile("testdata/search_songs.json")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		status  int
		body    []byte
		wantLen int
		wantErr bool
	}{
		{name: "ok", status: http.StatusOK, body: fixture, wantLen: 6},
		{name: "server error", status: http.StatusInternalServerError, body: []byte("boom"), wantErr: true},
		{name: "bad json", status: http.StatusOK, body: []byte("<html>"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/youtubei/v1/search" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				var got struct {
					Query   string `json:"query"`
					Params  string `json:"params"`
					Context struct {
						Client struct {
							ClientName string `json:"clientName"`
						} `json:"client"`
					} `json:"context"`
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode request body: %v", err)
				}
				if got.Query != "radiohead" || got.Params != songsParams || got.Context.Client.ClientName != "WEB_REMIX" {
					t.Errorf("unexpected request body: %+v", got)
				}
				w.WriteHeader(tt.status)
				w.Write(tt.body)
			}))
			defer srv.Close()

			songs, err := newTestClient(srv.Client(), srv.URL).Search(context.Background(), "radiohead")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if len(songs) != tt.wantLen {
				t.Errorf("got %d songs, want %d", len(songs), tt.wantLen)
			}
		})
	}
}
