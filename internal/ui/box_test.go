package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBoxDimensions(t *testing.T) {
	tests := []struct {
		name         string
		title, right string
		body         []string
		w, h         int
	}{
		{"title and right", "Songs", "1/9", []string{"a", "b"}, 40, 6},
		{"no right label", "Queue", "", nil, 30, 4},
		{"body taller than box is cut", "T", "", []string{"1", "2", "3", "4"}, 20, 4},
		{"too narrow for right label", "A long title", "12/345", []string{"x"}, 18, 3},
		{"long body line is truncated", "T", "", []string{strings.Repeat("x", 100)}, 20, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(box(tt.title, tt.right, tt.body, tt.w, tt.h, accent), "\n")
			if len(lines) != tt.h {
				t.Errorf("height = %d, want %d", len(lines), tt.h)
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w != tt.w {
					t.Errorf("line %d width = %d, want %d: %q", i, w, tt.w, l)
				}
			}
		})
	}
}

func TestGradientEndpoints(t *testing.T) {
	// Out-of-range input clamps instead of producing garbage colors.
	if gradient(-1) != gradient(0) || gradient(2) != gradient(1) {
		t.Error("gradient does not clamp")
	}
	if gradient(0) == gradient(0.5) || gradient(0.5) == gradient(1) {
		t.Error("gradient stops are not distinct")
	}
}
