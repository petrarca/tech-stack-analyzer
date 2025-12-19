package parsers

import (
	"testing"
)

func TestParseUvLock(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		projectName string
		expected    int
		wantDeps    map[string]string
	}{
		{
			name: "basic project with dependencies",
			content: `version = 1
revision = 3
requires-python = ">=3.8"

[[package]]
name = "requests"
version = "2.31.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "urllib3"
version = "2.0.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "my-project"
source = { editable = "." }
dependencies = [
    { name = "requests" },
]
`,
			projectName: "my-project",
			expected:    1,
			wantDeps: map[string]string{
				"requests": "2.31.0",
			},
		},
		{
			name: "project with optional dependencies",
			content: `version = 1

[[package]]
name = "requests"
version = "2.31.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "pytest"
version = "8.0.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "my-project"
source = { editable = "." }
dependencies = [
    { name = "requests" },
]

[package.optional-dependencies]
dev = [
    { name = "pytest" },
]
`,
			projectName: "my-project",
			expected:    2,
			wantDeps: map[string]string{
				"requests": "2.31.0",
				"pytest":   "8.0.0",
			},
		},
		{
			name:        "empty content",
			content:     ``,
			projectName: "test",
			expected:    0,
			wantDeps:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParseUvLock([]byte(tt.content), tt.projectName)

			if len(deps) != tt.expected {
				t.Errorf("ParseUvLock() got %d dependencies, want %d", len(deps), tt.expected)
			}

			for _, dep := range deps {
				if dep.Type != "python" {
					t.Errorf("ParseUvLock() dep.Type = %s, want python", dep.Type)
				}
				if dep.SourceFile != "uv.lock" {
					t.Errorf("ParseUvLock() dep.SourceFile = %s, want uv.lock", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParseUvLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}
