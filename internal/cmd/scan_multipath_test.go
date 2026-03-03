package cmd

import (
	"testing"
)

func TestComputeCommonParent(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "single path",
			paths:    []string{"/home/user/projects/proj1"},
			expected: "/home/user/projects/proj1",
		},
		{
			name:     "two siblings",
			paths:    []string{"/home/user/projects/proj1", "/home/user/projects/proj2"},
			expected: "/home/user/projects",
		},
		{
			name:     "three siblings",
			paths:    []string{"/my/scan/folder1", "/my/scan/folder2", "/my/scan/folder3"},
			expected: "/my/scan",
		},
		{
			name:     "nested paths",
			paths:    []string{"/my/scan/folder1", "/my/scan/folder1/sub"},
			expected: "/my/scan/folder1",
		},
		{
			name:     "different depth",
			paths:    []string{"/my/scan/folder", "/my/other/scan"},
			expected: "/my",
		},
		{
			name:     "empty input",
			paths:    []string{},
			expected: ".",
		},
		{
			name:     "root only common",
			paths:    []string{"/home/alice/api", "/opt/services/worker"},
			expected: "/",
		},
		{
			name:     "order independent",
			paths:    []string{"/a/b/c", "/a/b/d"},
			expected: "/a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCommonParent(tt.paths)
			if got != tt.expected {
				t.Errorf("computeCommonParent(%v) = %q, want %q", tt.paths, got, tt.expected)
			}
		})
	}
}

func TestIsSystemRoot(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"root", "/", true},
		{"home", "/home", true},
		{"Users", "/Users", true},
		{"tmp", "/tmp", true},
		{"var", "/var", true},
		{"opt", "/opt", true},
		{"usr", "/usr", true},
		{"user home", "/home/alice", false},
		{"project dir", "/home/alice/projects", false},
		{"opt subdir", "/opt/myapp", false},
		{"deep path", "/var/lib/myapp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSystemRoot(tt.path)
			if got != tt.expected {
				t.Errorf("isSystemRoot(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}
