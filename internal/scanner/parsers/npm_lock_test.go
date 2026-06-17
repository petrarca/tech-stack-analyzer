package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestParsePackageLock(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		wantDeps map[string]string
	}{
		{
			name: "basic dependencies",
			content: `{
				"name": "test-project",
				"version": "1.0.0",
				"lockfileVersion": 2,
				"packages": {
					"": {"name": "test-project", "version": "1.0.0"},
					"node_modules/express": {"version": "4.18.2"},
					"node_modules/lodash": {"version": "4.17.21"}
				}
			}`,
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"lodash":  "4.17.21",
			},
		},
		{
			name: "scoped packages",
			content: `{
				"name": "test-project",
				"version": "1.0.0",
				"lockfileVersion": 2,
				"packages": {
					"": {"name": "test-project", "version": "1.0.0"},
					"node_modules/@babel/core": {"version": "7.23.0"},
					"node_modules/@types/node": {"version": "20.10.0"}
				}
			}`,
			expected: 2,
			wantDeps: map[string]string{
				"@babel/core": "7.23.0",
				"@types/node": "20.10.0",
			},
		},
		{
			name: "filters transitive dependencies",
			content: `{
				"name": "test-project",
				"version": "1.0.0",
				"lockfileVersion": 2,
				"packages": {
					"": {"name": "test-project", "version": "1.0.0"},
					"node_modules/express": {"version": "4.18.2"},
					"node_modules/express/node_modules/accepts": {"version": "1.3.8"},
					"node_modules/express/node_modules/body-parser": {"version": "1.20.2"}
				}
			}`,
			expected: 1,
			wantDeps: map[string]string{
				"express": "4.18.2",
			},
		},
		{
			name:     "empty packages",
			content:  `{"name": "test", "packages": {}}`,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name:     "invalid json",
			content:  `{invalid}`,
			expected: 0,
			wantDeps: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParsePackageLock([]byte(tt.content), nil)

			if len(deps) != tt.expected {
				t.Errorf("ParsePackageLock() got %d dependencies, want %d", len(deps), tt.expected)
			}

			for _, dep := range deps {
				if dep.Type != "npm" {
					t.Errorf("ParsePackageLock() dep.Type = %s, want npm", dep.Type)
				}
				if dep.Scope != types.ScopeProd {
					t.Errorf("ParsePackageLock() dep.Scope = %s, want %s", dep.Scope, types.ScopeProd)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParsePackageLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}

// TestParsePackageLock_StringDependenciesMap is a regression test for a
// lockfile-wide unmarshal failure. In package-lock.json v2/v3, the root ""
// entry (and each package entry) carries a "dependencies" map of
// name -> version-range STRINGS. The struct previously typed this field as a
// map of nested package OBJECTS, so any such string value made the entire
// json.Unmarshal fail and ParsePackageLock return zero dependencies (every
// component emitted versionless). Fixture uses fictional package names.
func TestParsePackageLock_StringDependenciesMap(t *testing.T) {
	content := `{
		"name": "myapp",
		"version": "1.0.0",
		"lockfileVersion": 2,
		"packages": {
			"": {
				"name": "myapp",
				"version": "1.0.0",
				"dependencies": {
					"widget": "^1.2.0",
					"@scope/helper": "^3.0.0"
				}
			},
			"node_modules/widget": {
				"version": "1.2.3",
				"dependencies": {
					"leftpad": "^1.0.0"
				}
			},
			"node_modules/@scope/helper": {"version": "3.4.5"},
			"node_modules/leftpad": {"version": "1.3.0"}
		}
	}`
	deps := ParsePackageLock([]byte(content), nil)

	got := make(map[string]string, len(deps))
	for _, d := range deps {
		got[d.Name] = d.Version
	}
	for name, version := range map[string]string{
		"widget":        "1.2.3",
		"@scope/helper": "3.4.5",
	} {
		if got[name] != version {
			t.Errorf("dep %s: got version %q, want %q (all: %v)", name, got[name], version, got)
		}
	}
	if len(deps) == 0 {
		t.Fatal("expected dependencies, got none (lockfile unmarshal regression)")
	}
}

func TestExtractNameFromNodeModulesPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"node_modules/express", "express"},
		{"node_modules/lodash", "lodash"},
		{"node_modules/@babel/core", "@babel/core"},
		{"node_modules/@types/node", "@types/node"},
		{"node_modules/@scope/package/subpath", "@scope/package"},
		{"express", "express"},
		{"@babel/core", "@babel/core"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractNameFromNodeModulesPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractNameFromNodeModulesPath(%s) = %s, want %s", tt.path, result, tt.expected)
			}
		})
	}
}
