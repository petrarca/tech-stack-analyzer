package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/require"
)

func TestParsePackageJSONEnhanced(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "basic package.json with semantic versions",
			content: `{
				"name": "test-app",
				"version": "1.0.0",
				"dependencies": {
					"express": "^4.18.0",
					"react": "~18.2.0",
					"lodash": ">=4.17.0"
				},
				"devDependencies": {
					"typescript": ">=4.5.0 <5.0.0",
					"jest": "^29.0.0"
				}
			}`,
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "express", Version: "^4.18.0", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "react", Version: "~18.2.0", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "lodash", Version: ">=4.17.0", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "typescript", Version: ">=4.5.0 <5.0.0", SourceFile: "package.json", Scope: "dev"},
				{Type: "npm", Name: "jest", Version: "^29.0.0", SourceFile: "package.json", Scope: "dev"},
			},
		},
		{
			name: "package.json with complex semantic versions",
			content: `{
				"name": "complex-app",
				"dependencies": {
					"package1": "1.0.0 || 2.0.0",
					"package2": "1.0.0 - 2.0.0",
					"package3": "~1.2.3",
					"package4": "latest"
				},
				"peerDependencies": {
					"react": ">=16.0.0"
				},
				"optionalDependencies": {
					"optional-pkg": "^1.0.0"
				}
			}`,
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "package1", Version: "1.0.0 || 2.0.0", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "package2", Version: "1.0.0 - 2.0.0", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "package3", Version: "~1.2.3", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "package4", Version: "latest", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "react", Version: ">=16.0.0", SourceFile: "package.json", Scope: "peer"},
				{Type: "npm", Name: "optional-pkg", Version: "^1.0.0", SourceFile: "package.json", Scope: "optional"},
			},
		},
		{
			name: "package.json with workspace and git dependencies",
			content: `{
				"name": "workspace-app",
				"dependencies": {
					"local-pkg": "workspace:*",
					"git-pkg": "github:user/repo#main",
					"file-pkg": "file:../local-package",
					"npm-pkg": "npm:package@^1.0.0"
				},
				"workspaces": [
					"packages/*"
				]
			}`,
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "local-pkg", Version: "workspace", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "git-pkg", Version: "github:user/repo#main", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "file-pkg", Version: "local", SourceFile: "package.json", Scope: "prod"},
				{Type: "npm", Name: "npm-pkg", Version: "package@^1.0.0", SourceFile: "package.json", Scope: "prod"},
			},
		},
		{
			name: "empty package.json",
			content: `{
				"name": "empty-app",
				"version": "1.0.0"
			}`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "package.json with only dev dependencies",
			content: `{
				"name": "dev-only-app",
				"devDependencies": {
					"nodemon": "^2.0.0",
					"ts-node": "^10.0.0"
				}
			}`,
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "nodemon", Version: "^2.0.0", SourceFile: "package.json", Scope: "dev"},
				{Type: "npm", Name: "ts-node", Version: "^10.0.0", SourceFile: "package.json", Scope: "dev"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePackageJSONEnhanced([]byte(tt.content))

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			// Create maps for easier comparison since order is not guaranteed
			expectedMap := make(map[string]types.Dependency)
			actualMap := make(map[string]types.Dependency)

			for _, dep := range tt.expectedDeps {
				expectedMap[dep.Name] = dep
			}
			for _, dep := range result {
				actualMap[dep.Name] = dep
			}

			// Compare each expected dependency
			for name, expected := range expectedMap {
				actual, exists := actualMap[name]
				require.True(t, exists, "Expected dependency %s should exist", name)
				require.Equal(t, expected.Type, actual.Type, "Dependency type should match for %s", name)
				require.Equal(t, expected.Name, actual.Name, "Dependency name should match for %s", name)
				require.Equal(t, expected.Version, actual.Version, "Dependency version should match for %s", name)
				require.Equal(t, expected.SourceFile, actual.SourceFile, "Source file should match for %s", name)
				require.Equal(t, expected.Scope, actual.Scope, "Dependency scope should match for %s", name)
			}
		})
	}
}

func TestParseSemanticVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^1.0.0", "^1.0.0"},
		{"~2.1.0", "~2.1.0"},
		{">=1.0.0", ">=1.0.0"},
		{"<=2.0.0", "<=2.0.0"},
		{">1.0.0 <2.0.0", ">1.0.0 <2.0.0"},
		{"1.0.0 || 2.0.0", "1.0.0 || 2.0.0"},
		{"1.0.0 - 2.0.0", "1.0.0 - 2.0.0"},
		{"1.0.0", "1.0.0"},
		{"latest", "latest"},
		{"*", "latest"},
		{"", "latest"},
		{"workspace:*", "workspace"},
		{"workspace:.", "workspace"},
		{"npm:package@^1.0.0", "package@^1.0.0"},
		{"file:../local-package", "local"},
		{"git:https://github.com/user/repo.git", "git:https://github.com/user/repo.git"},
		{"github:user/repo#main", "github:user/repo#main"},
		{"https://github.com/user/repo.git", "https://github.com/user/repo.git"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSemanticVersion(tt.input)
			require.Equal(t, tt.expected, result, "Semantic version parsing should match expected")
		})
	}
}

func TestIsWorkspaceProject(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "workspace with array",
			content: `{
				"name": "monorepo",
				"workspaces": ["packages/*", "apps/*"]
			}`,
			expected: true,
		},
		{
			name: "workspace with string",
			content: `{
				"name": "workspace-app",
				"workspace": "."
			}`,
			expected: true,
		},
		{
			name: "no workspace",
			content: `{
				"name": "regular-app",
				"dependencies": {"express": "^4.0.0"}
			}`,
			expected: false,
		},
		{
			name: "empty workspaces",
			content: `{
				"name": "empty-workspace",
				"workspaces": []
			}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWorkspaceProject([]byte(tt.content))
			require.Equal(t, tt.expected, result, "Workspace detection should match expected")
		})
	}
}

func TestGetWorkspacePackages(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "multiple workspace patterns",
			content: `{
				"name": "monorepo",
				"workspaces": ["packages/*", "apps/*", "tools/*"]
			}`,
			expected: []string{"packages/*", "apps/*", "tools/*"},
		},
		{
			name: "single workspace pattern",
			content: `{
				"name": "simple-monorepo",
				"workspaces": ["packages/*"]
			}`,
			expected: []string{"packages/*"},
		},
		{
			name: "no workspaces",
			content: `{
				"name": "regular-app",
				"dependencies": {"express": "^4.0.0"}
			}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWorkspacePackages([]byte(tt.content))
			require.Equal(t, tt.expected, result, "Workspace packages should match expected")
		})
	}
}
