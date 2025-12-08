package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRustParser(t *testing.T) {
	parser := NewRustParser()
	assert.NotNil(t, parser, "Should create a new RustParser")
	assert.IsType(t, &RustParser{}, parser, "Should return correct type")
}

func TestParseCargoToml(t *testing.T) {
	parser := NewRustParser()

	tests := []struct {
		name                string
		content             string
		expectedProjectName string
		expectedLicense     string
		expectedDeps        []types.Dependency
		expectedIsWorkspace bool
	}{
		{
			name: "basic Cargo.toml with dependencies",
			content: `[package]
name = "my-rust-app"
version = "0.1.0"
license = "MIT"

[dependencies]
serde = "1.0"
tokio = { version = "1.0", features = ["full"] }
serde_json = "1.0.91"

[dev-dependencies]
criterion = "0.4"

[build-dependencies]
cc = "1.0"
`,
			expectedProjectName: "my-rust-app",
			expectedLicense:     "MIT",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "serde", Example: "1.0"},
				{Type: "cargo", Name: "tokio", Example: "1.0"},
				{Type: "cargo", Name: "serde_json", Example: "1.0.91"},
				{Type: "cargo", Name: "criterion", Example: "0.4"},
				{Type: "cargo", Name: "cc", Example: "1.0"},
			},
			expectedIsWorkspace: false,
		},
		{
			name: "Cargo.toml with complex dependency formats",
			content: `[package]
name = "complex-app"
license = "Apache-2.0"

[dependencies]
# Simple version
local-crate = "0.1.0"

# Git dependency
git-dependency = { git = "https://github.com/user/repo.git", branch = "main" }

# Path dependency  
path-dependency = { path = "../local-crate" }

# Complex version with features
complex-dep = { version = "1.0", features = ["derive", "serde"], default-features = false }

# Workspace dependency
workspace-dep = { version = "1.0", workspace = true }

[dev-dependencies]
test-crate = { git = "https://github.com/test/repo.git", rev = "abc123" }
`,
			expectedProjectName: "complex-app",
			expectedLicense:     "Apache-2.0",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "local-crate", Example: "0.1.0"},
				{Type: "cargo", Name: "git-dependency", Example: "git:https://github.com/user/repo.git#main"},
				{Type: "cargo", Name: "path-dependency", Example: "path:../local-crate"},
				{Type: "cargo", Name: "complex-dep", Example: "1.0"},
				{Type: "cargo", Name: "workspace-dep", Example: "1.0"},
				{Type: "cargo", Name: "test-crate", Example: "git:https://github.com/test/repo.git#abc123"},
			},
			expectedIsWorkspace: false,
		},
		{
			name: "workspace Cargo.toml",
			content: `[workspace]
members = [
    "crates/core",
    "crates/utils",
]

[workspace.dependencies]
serde = "1.0"
tokio = "1.0"
thiserror = "1.0"

[workspace.metadata.crane]
version = "0.1.0"
`,
			expectedProjectName: "",
			expectedLicense:     "",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "serde", Example: "1.0"},
				{Type: "cargo", Name: "tokio", Example: "1.0"},
				{Type: "cargo", Name: "thiserror", Example: "1.0"},
			},
			expectedIsWorkspace: true,
		},
		{
			name: "minimal Cargo.toml",
			content: `[package]
name = "minimal-app"
version = "0.1.0"
`,
			expectedProjectName: "minimal-app",
			expectedLicense:     "",
			expectedDeps:        []types.Dependency{},
			expectedIsWorkspace: false,
		},
		{
			name: "Cargo.toml with only dev-dependencies",
			content: `[package]
name = "test-only-app"

[dev-dependencies]
criterion = "0.4"
proptest = "1.0"
`,
			expectedProjectName: "test-only-app",
			expectedLicense:     "",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "criterion", Example: "0.4"},
				{Type: "cargo", Name: "proptest", Example: "1.0"},
			},
			expectedIsWorkspace: false,
		},
		{
			name:                "empty Cargo.toml",
			content:             "",
			expectedProjectName: "",
			expectedLicense:     "",
			expectedDeps:        []types.Dependency{},
			expectedIsWorkspace: false,
		},
		{
			name: "Cargo.toml with comments and whitespace",
			content: `# This is a comment
[package]
name = "commented-app"  # App name
license = "MIT"        # License

# Dependencies section
[dependencies]
# JSON handling
serde = "1.0"
serde_json = "1.0.91"

# Async runtime  
tokio = { version = "1.0", features = ["full"] }

[dev-dependencies]
# Testing framework
criterion = "0.4"
`,
			expectedProjectName: "commented-app",
			expectedLicense:     "MIT",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "serde", Example: "1.0"},
				{Type: "cargo", Name: "serde_json", Example: "1.0.91"},
				{Type: "cargo", Name: "tokio", Example: "1.0"},
				{Type: "cargo", Name: "criterion", Example: "0.4"},
			},
			expectedIsWorkspace: false,
		},
		{
			name: "Cargo.toml with inline tables",
			content: `[package]
name = "inline-table-app"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1.0", features = ["full", "tracing"] }
tracing = "0.1"

[dev-dependencies]
criterion = { version = "0.4" }
`,
			expectedProjectName: "inline-table-app",
			expectedLicense:     "",
			expectedDeps: []types.Dependency{
				{Type: "cargo", Name: "serde", Example: "1.0"},
				{Type: "cargo", Name: "tokio", Example: "1.0"},
				{Type: "cargo", Name: "tracing", Example: "0.1"},
				{Type: "cargo", Name: "criterion", Example: "0.4"},
			},
			expectedIsWorkspace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectName, license, dependencies, isWorkspace := parser.ParseCargoToml(tt.content)

			assert.Equal(t, tt.expectedProjectName, projectName, "Should return correct project name")
			assert.Equal(t, tt.expectedLicense, license, "Should return correct license")
			assert.Equal(t, tt.expectedIsWorkspace, isWorkspace, "Should return correct workspace flag")

			require.Len(t, dependencies, len(tt.expectedDeps), "Should return correct number of dependencies")

			// Create dependency maps for order-independent comparison
			expectedDepMap := make(map[string]types.Dependency)
			actualDepMap := make(map[string]types.Dependency)

			for _, dep := range tt.expectedDeps {
				expectedDepMap[dep.Name] = dep
			}

			for _, dep := range dependencies {
				actualDepMap[dep.Name] = dep
			}

			// Verify all expected dependencies are present
			for name, expectedDep := range expectedDepMap {
				actualDep, exists := actualDepMap[name]
				require.True(t, exists, "Expected dependency %s not found", name)
				assert.Equal(t, expectedDep.Type, actualDep.Type, "Should have correct type for %s", name)
				assert.Equal(t, expectedDep.Example, actualDep.Example, "Should have correct version for %s", name)
			}
		})
	}
}

func TestRustParser_EdgeCases(t *testing.T) {
	parser := NewRustParser()

	// Test workspace detection
	t.Run("workspace detection", func(t *testing.T) {
		content := `[workspace]
members = ["crates/*"]

[workspace.dependencies]
serde = "1.0"
`

		projectName, license, dependencies, isWorkspace := parser.ParseCargoToml(content)

		assert.Equal(t, "", projectName)
		assert.Equal(t, "", license)
		assert.True(t, isWorkspace)
		assert.Len(t, dependencies, 1)
		assert.Equal(t, "serde", dependencies[0].Name)
	})

	// Test malformed dependency entries
	t.Run("malformed dependencies", func(t *testing.T) {
		content := `[package]
name = "malformed-app"

[dependencies]
# Valid dependency
valid-dep = "1.0"

# Empty dependency (should be skipped)
empty-dep = ""

# Malformed inline table (should be skipped)
malformed-dep = { }

# Another valid dependency
another-dep = "2.0"
`

		projectName, _, dependencies, _ := parser.ParseCargoToml(content)

		assert.Equal(t, "malformed-app", projectName)
		assert.Len(t, dependencies, 2) // Only valid dependencies

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Contains(t, depMap, "valid-dep")
		assert.Contains(t, depMap, "another-dep")
		assert.NotContains(t, depMap, "empty-dep")
		assert.NotContains(t, depMap, "malformed-dep")
	})

	// Test dependency with complex git reference
	t.Run("complex git dependencies", func(t *testing.T) {
		content := `[package]
name = "git-app"

[dependencies]
git-dep-branch = { git = "https://github.com/user/repo.git", branch = "develop" }
git-dep-tag = { git = "https://github.com/user/repo2.git", tag = "v1.0.0" }
git-dep-rev = { git = "https://github.com/user/repo3.git", rev = "a1b2c3d" }
local-dep = { path = "./local-crate" }
simple-dep = "1.0"
`

		_, _, dependencies, _ := parser.ParseCargoToml(content)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Equal(t, "git:https://github.com/user/repo.git#develop", depMap["git-dep-branch"].Example)
		assert.Equal(t, "git:https://github.com/user/repo2.git#v1.0.0", depMap["git-dep-tag"].Example)
		assert.Equal(t, "git:https://github.com/user/repo3.git#a1b2c3d", depMap["git-dep-rev"].Example)
		assert.Equal(t, "path:./local-crate", depMap["local-dep"].Example)
		assert.Equal(t, "1.0", depMap["simple-dep"].Example)
	})
}

func TestRustParser_Integration(t *testing.T) {
	parser := NewRustParser()

	// Test realistic Rust web application
	webAppCargo := `[package]
name = "rust-web-server"
version = "0.1.0"
edition = "2021"
license = "MIT"
authors = ["John Doe <john@example.com>"]
description = "A high-performance web server"

[dependencies]
# Web framework
axum = "0.6"
tokio = { version = "1.0", features = ["full"] }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

# Database
sqlx = { version = "0.7", features = ["postgres", "runtime-tokio-rustls"] }

# Logging
tracing = "0.1"
tracing-subscriber = "0.3"

# Configuration
config = "0.13"
dotenvy = "0.15"

[dev-dependencies]
criterion = { version = "0.4", features = ["html_reports"] }
tempfile = "3.0"

[build-dependencies]
tonic-build = "0.8"
`

	projectName, license, dependencies, isWorkspace := parser.ParseCargoToml(webAppCargo)

	assert.Equal(t, "rust-web-server", projectName)
	assert.Equal(t, "MIT", license)
	assert.False(t, isWorkspace)
	assert.Len(t, dependencies, 12) // 8 prod + 2 dev + 1 build + 1 extra

	// Verify key dependencies
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Check production dependencies
	assert.Equal(t, "cargo", depMap["axum"].Type)
	assert.Equal(t, "0.6", depMap["axum"].Example)
	assert.Equal(t, "cargo", depMap["tokio"].Type)
	assert.Equal(t, "1.0", depMap["tokio"].Example)
	assert.Equal(t, "cargo", depMap["sqlx"].Type)
	assert.Equal(t, "0.7", depMap["sqlx"].Example)

	// Check dev dependencies
	assert.Equal(t, "cargo", depMap["criterion"].Type)
	assert.Contains(t, depMap["criterion"].Example, "0.4")

	// Check build dependencies
	assert.Equal(t, "cargo", depMap["tonic-build"].Type)
	assert.Equal(t, "0.8", depMap["tonic-build"].Example)

	// Test realistic workspace setup
	workspaceCargo := `[workspace]
resolver = "2"
members = [
    "crates/core",
    "crates/cli", 
    "crates/server",
]

[workspace.dependencies]
# Internal crates
core-lib = { path = "crates/core", version = "0.1.0" }
cli-lib = { path = "crates/cli", version = "0.1.0" }

# External dependencies
tokio = { version = "1.0", features = ["full"] }
serde = { version = "1.0", features = ["derive"] }
anyhow = "1.0"
thiserror = "1.0"

[workspace.metadata.crane]
# Common workspace configuration
publish-regex = "^crate-.*"
`

	projectName, license, dependencies, isWorkspace = parser.ParseCargoToml(workspaceCargo)

	assert.Equal(t, "", projectName) // Workspaces don't have package names
	assert.Equal(t, "", license)
	assert.True(t, isWorkspace)
	assert.Len(t, dependencies, 6) // Only workspace dependencies

	// Verify workspace dependencies
	depMap = make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "cargo", depMap["tokio"].Type)
	assert.Equal(t, "1.0", depMap["tokio"].Example)
	assert.Equal(t, "cargo", depMap["serde"].Type)
	assert.Equal(t, "1.0", depMap["serde"].Example)
	assert.Equal(t, "cargo", depMap["anyhow"].Type)
	assert.Equal(t, "1.0", depMap["anyhow"].Example)
}

func TestRustParser_ErrorHandling(t *testing.T) {
	parser := NewRustParser()

	// Test with completely invalid TOML-like content
	t.Run("invalid content", func(t *testing.T) {
		content := `This is not valid TOML at all!
[unclosed section
dependencies = "no closing quote`

		projectName, license, dependencies, isWorkspace := parser.ParseCargoToml(content)

		// Should return empty/default values
		assert.Equal(t, "", projectName)
		assert.Equal(t, "", license)
		assert.False(t, isWorkspace)
		assert.Empty(t, dependencies)
	})

	// Test with only comments
	t.Run("only comments", func(t *testing.T) {
		content := `# This is just a comment
# Another comment
# No actual content here`

		projectName, license, dependencies, isWorkspace := parser.ParseCargoToml(content)

		assert.Equal(t, "", projectName)
		assert.Equal(t, "", license)
		assert.False(t, isWorkspace)
		assert.Empty(t, dependencies)
	})
}
