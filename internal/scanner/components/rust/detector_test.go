package rust

import (
	"os"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider implements types.Provider for testing
type MockProvider struct {
	files map[string]string
}

func (m *MockProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockProvider) ListDir(path string) ([]types.File, error) {
	return nil, nil
}

func (m *MockProvider) Open(path string) (string, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return "", os.ErrNotExist
}

func (m *MockProvider) Exists(path string) (bool, error) {
	_, exists := m.files[path]
	return exists, nil
}

func (m *MockProvider) IsDir(path string) (bool, error) {
	return false, nil
}

func (m *MockProvider) GetBasePath() string {
	return "/mock"
}

// MockDependencyDetector implements components.DependencyDetector for testing
type MockDependencyDetector struct {
	matchedTechs map[string][]string
}

func (m *MockDependencyDetector) MatchDependencies(dependencies []string, depType string) map[string][]string {
	return m.matchedTechs
}

func TestDetector_Name(t *testing.T) {
	detector := &Detector{}
	assert.Equal(t, "rust", detector.Name())
}

func TestDetector_Detect_BasicCargoToml(t *testing.T) {
	detector := &Detector{}

	// Create mock Cargo.toml content
	cargoContent := `[package]
name = "test-app"
version = "0.1.0"
edition = "2021"
license = "MIT"
description = "A test Rust application"

[dependencies]
tokio = { version = "1.0", features = ["full"] }
serde = { version = "1.0", features = ["derive"] }
reqwest = "0.11"

[dev-dependencies]
tokio-test = "0.4"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Cargo.toml": cargoContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"tokio": {"matched dependency: tokio"},
			"serde": {"matched dependency: serde"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Rust project")

	payload := results[0]
	assert.Equal(t, "test-app", payload.Name) // Uses package name from Cargo.toml
	assert.Equal(t, "/Cargo.toml", payload.Path[0])
	assert.Contains(t, payload.Tech, "rust", "Should have rust as primary tech")
	assert.Contains(t, payload.Techs, "cargo", "Should detect cargo from Cargo.toml")
	assert.Contains(t, payload.Techs, "tokio", "Should detect tokio from dependencies")
	assert.Contains(t, payload.Techs, "serde", "Should detect serde from dependencies")
	assert.Contains(t, payload.Licenses, "MIT", "Should detect MIT license")

	// Check dependencies - should include both dependencies and dev-dependencies
	assert.Len(t, payload.Dependencies, 4, "Should have 4 dependencies (3 deps + 1 dev-dep)")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "cargo", dep.Type, "All dependencies should be cargo type")
	}

	assert.True(t, depNames["tokio"], "Should have tokio dependency")
	assert.True(t, depNames["serde"], "Should have serde dependency")
	assert.True(t, depNames["reqwest"], "Should have reqwest dependency")
	assert.True(t, depNames["tokio-test"], "Should have tokio-test dev dependency")
}

func TestDetector_Detect_WorkspaceCargoToml(t *testing.T) {
	detector := &Detector{}

	// Create workspace Cargo.toml content (no [package] section)
	cargoContent := `[workspace]
members = [
    "crates/core",
    "crates/utils",
]

[workspace.dependencies]
tokio = { version = "1.0", features = ["full"] }
serde = "1.0"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Cargo.toml": cargoContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Rust workspace")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name) // Virtual payload for workspace files
	assert.Equal(t, "/Cargo.toml", payload.Path[0])
	assert.Contains(t, payload.Techs, "cargo", "Should detect cargo from Cargo.toml")
	assert.Len(t, payload.Dependencies, 2, "Should have 2 workspace dependencies")
}

func TestDetector_Detect_CargoTomlWithoutPackage(t *testing.T) {
	detector := &Detector{}

	// Create Cargo.toml without [package] section
	cargoContent := `[dependencies]
tokio = "1.0"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Cargo.toml": cargoContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Rust project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name) // Virtual payload when no package section
	assert.Equal(t, "/Cargo.toml", payload.Path[0])
	assert.Contains(t, payload.Techs, "cargo", "Should detect cargo")
	assert.Len(t, payload.Dependencies, 1, "Should have 1 dependency")
}

func TestDetector_Detect_EmptyCargoToml(t *testing.T) {
	detector := &Detector{}

	// Create empty Cargo.toml content
	cargoContent := `[package]
name = "empty-app"
version = "0.1.0"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Cargo.toml": cargoContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Rust project")

	payload := results[0]
	assert.Equal(t, "empty-app", payload.Name)
	assert.Contains(t, payload.Techs, "cargo", "Should detect cargo")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
	assert.Empty(t, payload.Licenses, "Should have no license")
}

func TestDetector_Detect_NoCargoToml(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no Cargo.toml
	files := []types.File{
		{Name: "main.rs", Path: "/project/main.rs"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Rust components without Cargo.toml")
}

func TestDetector_Detect_EmptyFilesList(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Test with empty files list
	results := detector.Detect([]types.File{}, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any components from empty file list")
}

func TestDetector_Detect_FileReadError(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider that returns error (empty files map)
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock Cargo.toml content
	cargoContent := `[package]
name = "path-test-app"
version = "0.1.0"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/Cargo.toml": cargoContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Cargo.toml", Path: "/project/subdir/Cargo.toml"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Rust project")

	payload := results[0]
	assert.Equal(t, "path-test-app", payload.Name)
	assert.Equal(t, "/subdir/Cargo.toml", payload.Path[0], "Should handle relative paths correctly")
}

func TestDetectLicense(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name     string
		license  string
		expected string
	}{
		{
			name:     "MIT license",
			license:  "MIT",
			expected: "MIT",
		},
		{
			name:     "Apache license",
			license:  "Apache-2.0",
			expected: "Apache-2.0",
		},
		{
			name:     "GPL license",
			license:  "GPL-3.0",
			expected: "GPL-3.0",
		},
		{
			name:     "BSD license",
			license:  "BSD-3-Clause",
			expected: "BSD-3-Clause",
		},
		{
			name:     "ISC license",
			license:  "ISC",
			expected: "ISC",
		},
		{
			name:     "Unknown license",
			license:  "Custom-License-1.0",
			expected: "Custom-License-1.0",
		},
		{
			name:     "Empty license",
			license:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.detectLicense(tt.license)
			assert.Equal(t, tt.expected, result, "Should normalize license correctly")
		})
	}
}

func TestDetector_Detect_DifferentLicenseFormats(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name            string
		cargoToml       string
		expectedLicense string
	}{
		{
			name: "MIT license",
			cargoToml: `[package]
name = "test-app"
version = "0.1.0"
license = "MIT"
`,
			expectedLicense: "MIT",
		},
		{
			name: "Apache license",
			cargoToml: `[package]
name = "test-app"
version = "0.1.0"
license = "Apache-2.0"
`,
			expectedLicense: "Apache-2.0",
		},
		{
			name: "No license",
			cargoToml: `[package]
name = "test-app"
version = "0.1.0"
`,
			expectedLicense: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock provider
			provider := &MockProvider{
				files: map[string]string{
					"/project/Cargo.toml": tt.cargoToml,
				},
			}

			// Setup mock dependency detector
			depDetector := &MockDependencyDetector{
				matchedTechs: map[string][]string{},
			}

			// Create file list
			files := []types.File{
				{Name: "Cargo.toml", Path: "/project/Cargo.toml"},
			}

			// Test detection
			results := detector.Detect(files, "/project", "/project", provider, depDetector)

			// Verify results
			require.Len(t, results, 1, "Should detect one Rust project")

			payload := results[0]
			if tt.expectedLicense != "" {
				assert.Contains(t, payload.Licenses, tt.expectedLicense, "Should detect normalized license")
			} else {
				assert.Empty(t, payload.Licenses, "Should have no license")
			}
		})
	}
}
