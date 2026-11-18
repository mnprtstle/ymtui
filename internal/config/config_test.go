package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		file    *string // nil = no file
		want    Config
		wantErr bool
	}{
		{"missing file gives defaults", nil, Default(), false},
		{"partial file keeps other defaults", ptr(`{"theme":{"accent":"#112233"}}`),
			Config{Volume: 100, Theme: Theme{Accent: "#112233"}}, false},
		{"volume zero is respected", ptr(`{"volume":0}`), Config{Volume: 0}, false},
		{"bad json", ptr(`{`), Default(), true},
		{"volume out of range", ptr(`{"volume":150}`), Default(), true},
		{"bad color", ptr(`{"theme":{"amber":"red"}}`), Default(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if tt.file != nil {
				if err := os.WriteFile(path, []byte(*tt.file), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Load(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
