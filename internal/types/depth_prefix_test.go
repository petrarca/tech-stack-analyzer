package types

import "testing"

func TestDepthPrefix(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		depth int
		want  string
	}{
		// Basic depth-1 cases
		{"depth 1 single segment", "/server", 1, "/server"},
		{"depth 1 deep path", "/server/api/auth/handler.go", 1, "/server"},
		{"depth 1 two segments", "/server/api", 1, "/server"},

		// Depth-2 cases
		{"depth 2 exact", "/server/api", 2, "/server/api"},
		{"depth 2 deep path", "/server/api/auth/handler.go", 2, "/server/api"},

		// Path without leading slash (normalisation)
		{"no leading slash depth 1", "server/api/auth", 1, "/server"},
		{"no leading slash depth 2", "server/api/auth", 2, "/server/api"},

		// Edge cases that must return empty string
		{"empty path", "", 1, ""},
		{"root slash only", "/", 1, ""},
		{"depth 0", "/server/api", 0, ""},
		{"negative depth", "/server/api", -1, ""},
		{"depth exceeds segments", "/server", 2, ""},
		{"depth exactly at segments", "/server/api", 3, ""},

		// Single-character segments
		{"single char segments", "/a/b/c", 1, "/a"},
		{"single char segments depth 2", "/a/b/c", 2, "/a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DepthPrefix(tt.path, tt.depth)
			if got != tt.want {
				t.Errorf("DepthPrefix(%q, %d) = %q, want %q", tt.path, tt.depth, got, tt.want)
			}
		})
	}
}
