package git

import "testing"

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
