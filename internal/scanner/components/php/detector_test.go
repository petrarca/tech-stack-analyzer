package php

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
	assert.Equal(t, "php", detector.Name())
}

func TestDetector_Detect_BasicComposer(t *testing.T) {
	detector := &Detector{}

	// Create mock composer.json content
	composerContent := `{
    "name": "example/test-app",
    "description": "A test PHP application",
    "type": "project",
    "license": "MIT",
    "require": {
        "php": "^8.1",
        "laravel/framework": "^10.0",
        "guzzlehttp/guzzle": "^7.0"
    },
    "require-dev": {
        "phpunit/phpunit": "^10.0",
        "symfony/console": "^6.0"
    }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/composer.json": composerContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"laravel/framework": {"matched dependency: laravel/framework"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "composer.json", Path: "/project/composer.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one PHP project")

	payload := results[0]
	assert.Equal(t, "example/test-app", payload.Name)
	assert.Equal(t, "/composer.json", payload.Path[0])
	assert.Contains(t, payload.Tech, "php", "Should have php as primary tech")
	assert.Contains(t, payload.Techs, "phpcomposer", "Should detect phpcomposer from composer.json")
	assert.Contains(t, payload.Techs, "laravel/framework", "Should detect laravel from dependencies")
	assert.Contains(t, payload.Licenses, "MIT", "Should detect MIT license")

	// Check dependencies - should include both require and require-dev
	assert.Len(t, payload.Dependencies, 5, "Should have 5 dependencies (3 require + 2 require-dev)")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "php", dep.Type, "All dependencies should be php type")
	}

	assert.True(t, depNames["php"], "Should have php dependency")
	assert.True(t, depNames["laravel/framework"], "Should have laravel dependency")
	assert.True(t, depNames["guzzlehttp/guzzle"], "Should have guzzle dependency")
	assert.True(t, depNames["phpunit/phpunit"], "Should have phpunit dependency")
	assert.True(t, depNames["symfony/console"], "Should have symfony dependency")
}

func TestDetector_Detect_ComposerWithoutName(t *testing.T) {
	detector := &Detector{}

	// Create composer.json without name
	composerContent := `{
    "description": "A test PHP application",
    "require": {
        "php": "^8.1"
    }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/composer.json": composerContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "composer.json", Path: "/project/composer.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to missing name
	assert.Empty(t, results, "Should not detect project without name")
}

func TestDetector_Detect_NoComposerFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no composer.json
	files := []types.File{
		{Name: "index.php", Path: "/project/index.php"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any PHP components without composer.json")
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
		{Name: "composer.json", Path: "/project/composer.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_EmptyComposer(t *testing.T) {
	detector := &Detector{}

	// Create empty composer.json content
	emptyComposerContent := `{
    "name": "example/empty-app"
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/composer.json": emptyComposerContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "composer.json", Path: "/project/composer.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one PHP project")

	payload := results[0]
	assert.Equal(t, "example/empty-app", payload.Name)
	assert.Contains(t, payload.Tech, "php", "Should have php as primary tech")
	assert.Contains(t, payload.Techs, "phpcomposer", "Should detect phpcomposer")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
	assert.Empty(t, payload.Licenses, "Should have no license")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock composer.json content
	composerContent := `{
    "name": "example/path-test-app"
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/composer.json": composerContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "composer.json", Path: "/project/subdir/composer.json"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one PHP project")

	payload := results[0]
	assert.Equal(t, "example/path-test-app", payload.Name)
	assert.Equal(t, "/subdir/composer.json", payload.Path[0], "Should handle relative paths correctly")
}

func TestDetectLicense(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name     string
		license  string
		expected string
	}{
		{
			name:     "MIT lowercase",
			license:  "mit",
			expected: "MIT",
		},
		{
			name:     "Apache variants",
			license:  "apache-2.0",
			expected: "Apache-2.0",
		},
		{
			name:     "Apache simple",
			license:  "apache",
			expected: "Apache-2.0",
		},
		{
			name:     "Apache with space",
			license:  "apache 2.0",
			expected: "Apache-2.0",
		},
		{
			name:     "GPL variants",
			license:  "gpl-3.0",
			expected: "GPL-3.0",
		},
		{
			name:     "GPL simple",
			license:  "gpl",
			expected: "GPL-3.0",
		},
		{
			name:     "BSD",
			license:  "bsd",
			expected: "BSD",
		},
		{
			name:     "ISC",
			license:  "isc",
			expected: "ISC",
		},
		{
			name:     "LGPL",
			license:  "lgpl-3.0",
			expected: "LGPL-3.0",
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
		composerJSON    string
		expectedLicense string
	}{
		{
			name: "MIT license",
			composerJSON: `{
    "name": "example/test-app",
    "license": "mit"
}`,
			expectedLicense: "MIT",
		},
		{
			name: "Apache license",
			composerJSON: `{
    "name": "example/test-app",
    "license": "apache-2.0"
}`,
			expectedLicense: "Apache-2.0",
		},
		{
			name: "GPL license",
			composerJSON: `{
    "name": "example/test-app",
    "license": "gpl"
}`,
			expectedLicense: "GPL-3.0",
		},
		{
			name: "No license",
			composerJSON: `{
    "name": "example/test-app"
}`,
			expectedLicense: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock provider
			provider := &MockProvider{
				files: map[string]string{
					"/project/composer.json": tt.composerJSON,
				},
			}

			// Setup mock dependency detector
			depDetector := &MockDependencyDetector{
				matchedTechs: map[string][]string{},
			}

			// Create file list
			files := []types.File{
				{Name: "composer.json", Path: "/project/composer.json"},
			}

			// Test detection
			results := detector.Detect(files, "/project", "/project", provider, depDetector)

			// Verify results
			require.Len(t, results, 1, "Should detect one PHP project")

			payload := results[0]
			if tt.expectedLicense != "" {
				assert.Contains(t, payload.Licenses, tt.expectedLicense, "Should detect normalized license")
			} else {
				assert.Empty(t, payload.Licenses, "Should have no license")
			}
		})
	}
}
