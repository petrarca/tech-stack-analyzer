package git

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSanitizeRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS with oauth2 token",
			input:    "https://oauth2:cgmglpat-1234KTnz6is1WZ4pve8jM@git.cgm.ag/example/repo.git",
			expected: "https://git.cgm.ag/example/repo.git",
		},
		{
			name:     "HTTPS with user and password",
			input:    "https://user:password123@github.com/org/repo.git",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "HTTPS with token as username only",
			input:    "https://ghp_abc123def456@github.com/org/repo.git",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "HTTPS without credentials",
			input:    "https://github.com/org/repo.git",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "SSH URL unchanged",
			input:    "git@github.com:org/repo.git",
			expected: "git@github.com:org/repo.git",
		},
		{
			name:     "HTTP with credentials",
			input:    "http://user:pass@gitlab.example.com/project.git",
			expected: "http://gitlab.example.com/project.git",
		},
		{
			name:     "HTTPS with gitlab-ci-token",
			input:    "https://gitlab-ci-token:glcbt-64_abc123@gitlab.com/group/project.git",
			expected: "https://gitlab.com/group/project.git",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "HTTPS with port and credentials",
			input:    "https://user:token@git.example.com:8443/repo.git",
			expected: "https://git.example.com:8443/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRemoteURL(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS URL",
			input:    "https://github.com/org/repo.git",
			expected: "github.com/org/repo",
		},
		{
			name:     "SSH URL",
			input:    "git@github.com:org/repo.git",
			expected: "github.com:org/repo",
		},
		{
			name:     "HTTP URL",
			input:    "http://gitlab.example.com/project.git",
			expected: "gitlab.example.com/project",
		},
		{
			name:     "git protocol URL",
			input:    "git://github.com/org/repo.git",
			expected: "github.com/org/repo",
		},
		{
			name:     "trailing slash removed",
			input:    "https://github.com/org/repo/",
			expected: "github.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRemoteURL(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateRootIDFromMultiPaths_OrderIndependence(t *testing.T) {
	// Verify that argument order does not affect the result (sort is applied internally)
	cases := []struct {
		name     string
		pathsA   []string
		pathsB   []string
		wantSame bool
	}{
		{"reverse order same result", []string{"proj2", "proj1"}, []string{"proj1", "proj2"}, true},
		{"three elements shuffled", []string{"c", "a", "b"}, []string{"a", "b", "c"}, true},
		{"different sets differ", []string{"proj1", "proj2"}, []string{"proj1", "proj3"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id1 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", tc.pathsA)
			id2 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", tc.pathsB)
			if tc.wantSame && id1 != id2 {
				t.Errorf("expected same ID for %v vs %v, got %q vs %q", tc.pathsA, tc.pathsB, id1, id2)
			}
			if !tc.wantSame && id1 == id2 {
				t.Errorf("expected different IDs for %v vs %v, got same %q", tc.pathsA, tc.pathsB, id1)
			}
		})
	}
}

func TestGenerateRootIDFromMultiPaths(t *testing.T) {
	// Helper to compute expected hash
	hashOf := func(content string) string {
		h := sha256.Sum256([]byte(content))
		return hex.EncodeToString(h[:])[:20]
	}

	t.Run("deterministic for same inputs", func(t *testing.T) {
		id1 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj1", "proj2"})
		id2 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj1", "proj2"})
		if id1 != id2 {
			t.Errorf("same inputs produced different IDs: %q vs %q", id1, id2)
		}
	})

	t.Run("order independent", func(t *testing.T) {
		id1 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj2", "proj1"})
		id2 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj1", "proj2"})
		if id1 != id2 {
			t.Errorf("different order produced different IDs: %q vs %q", id1, id2)
		}
	})

	t.Run("different subsets produce different IDs", func(t *testing.T) {
		id1 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj1", "proj2"})
		id2 := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj1", "proj3"})
		if id1 == id2 {
			t.Errorf("different subsets produced same ID: %q", id1)
		}
	})

	t.Run("different roots produce different IDs", func(t *testing.T) {
		id1 := GenerateRootIDFromMultiPaths("/tmp/rootA", []string{"proj1", "proj2"})
		id2 := GenerateRootIDFromMultiPaths("/tmp/rootB", []string{"proj1", "proj2"})
		if id1 == id2 {
			t.Errorf("different roots produced same ID: %q", id1)
		}
	})

	t.Run("path-based hash content matches expected format", func(t *testing.T) {
		// For a non-git directory, the hash should be based on the path + sorted subfolder names
		// /tmp/nonexistent-root:proj1:proj2
		id := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"proj2", "proj1"})
		expected := hashOf("/tmp/nonexistent-root:proj1:proj2")
		if id != expected {
			t.Errorf("got %q, want %q (hash of %q)", id, expected, "/tmp/nonexistent-root:proj1:proj2")
		}
	})

	t.Run("returns 20 character hex string", func(t *testing.T) {
		id := GenerateRootIDFromMultiPaths("/tmp/nonexistent-root", []string{"a", "b"})
		if len(id) != 20 {
			t.Errorf("ID length = %d, want 20", len(id))
		}
		// Verify it's valid hex
		_, err := hex.DecodeString(id)
		if err != nil {
			t.Errorf("ID %q is not valid hex: %v", id, err)
		}
	})
}
