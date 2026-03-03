package parsers

import (
	"testing"
)

func TestParsePoetryLock(t *testing.T) {
	tests := []struct {
		name             string
		lockContent      string
		pyprojectContent string
		expected         int
		wantDeps         map[string]string
	}{
		{
			name: "basic poetry.lock with direct dependencies",
			lockContent: `[[package]]
name = "requests"
version = "2.31.0"
description = "Python HTTP library"

[[package]]
name = "urllib3"
version = "2.0.0"
description = "HTTP library"
`,
			pyprojectContent: `[tool.poetry.dependencies]
python = "^3.8"
requests = "^2.31.0"
`,
			expected: 1,
			wantDeps: map[string]string{
				"requests": "2.31.0",
			},
		},
		{
			name: "poetry.lock with dev dependencies",
			lockContent: `[[package]]
name = "requests"
version = "2.31.0"

[[package]]
name = "pytest"
version = "8.0.0"
`,
			pyprojectContent: `[tool.poetry.dependencies]
requests = "^2.31.0"

[tool.poetry.dev-dependencies]
pytest = "^8.0.0"
`,
			expected: 2,
			wantDeps: map[string]string{
				"requests": "2.31.0",
				"pytest":   "8.0.0",
			},
		},
		{
			name: "poetry.lock filters transitive dependencies",
			lockContent: `[[package]]
name = "requests"
version = "2.31.0"

[[package]]
name = "urllib3"
version = "2.0.0"

[[package]]
name = "certifi"
version = "2023.0.0"
`,
			pyprojectContent: `[tool.poetry.dependencies]
requests = "^2.31.0"
`,
			expected: 1,
			wantDeps: map[string]string{
				"requests": "2.31.0",
			},
		},
		{
			name: "PEP 621 project.dependencies format",
			lockContent: `[[package]]
name = "fastapi"
version = "0.100.0"

[[package]]
name = "uvicorn"
version = "0.23.0"
`,
			pyprojectContent: `[project.dependencies]
"fastapi>=0.100.0",
"uvicorn>=0.23.0",
`,
			expected: 2,
			wantDeps: map[string]string{
				"fastapi": "0.100.0",
				"uvicorn": "0.23.0",
			},
		},
		{
			name:        "empty lock content",
			lockContent: ``,
			pyprojectContent: `[tool.poetry.dependencies]
requests = "^2.31.0"
`,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name: "empty pyproject content",
			lockContent: `[[package]]
name = "requests"
version = "2.31.0"
`,
			pyprojectContent: ``,
			expected:         0,
			wantDeps:         map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParsePoetryLock([]byte(tt.lockContent), tt.pyprojectContent)

			if len(deps) != tt.expected {
				t.Errorf("ParsePoetryLock() got %d dependencies, want %d", len(deps), tt.expected)
				for _, d := range deps {
					t.Logf("  got: %s = %s", d.Name, d.Version)
				}
			}

			for _, dep := range deps {
				if dep.Type != "python" {
					t.Errorf("ParsePoetryLock() dep.Type = %s, want python", dep.Type)
				}
				if dep.SourceFile != "poetry.lock" {
					t.Errorf("ParsePoetryLock() dep.SourceFile = %s, want poetry.lock", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParsePoetryLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}

func TestNormalizePackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"requests", "requests"},
		{"Requests", "requests"},
		{"my_package", "my-package"},
		{"My_Package", "my-package"},
		{"some-package", "some-package"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePackageName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePackageName(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
