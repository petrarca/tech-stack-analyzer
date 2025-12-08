package nodejs

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
	assert.Equal(t, "nodejs", detector.Name())
}

func TestDetector_Detect_BasicPackageJson(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content
	packageJsonContent := `{
  "name": "test-app",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "jest": "^27.0.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"express": {"matched dependency: express"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Node.js project")

	payload := results[0]
	assert.Equal(t, "test-app", payload.Name)
	assert.Equal(t, "/package.json", payload.Path[0])
	assert.Contains(t, payload.Tech, "nodejs", "Should have nodejs as primary tech")
	assert.Contains(t, payload.Techs, "express", "Should detect express from dependencies")

	// Check dependencies
	assert.Len(t, payload.Dependencies, 3, "Should have 3 dependencies (2 prod + 1 dev)")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "npm", dep.Type, "All dependencies should be npm type")
	}

	assert.True(t, depNames["express"], "Should have express dependency")
	assert.True(t, depNames["lodash"], "Should have lodash dependency")
	assert.True(t, depNames["jest"], "Should have jest dependency")
}

func TestDetector_Detect_PackageJsonWithLicense(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content with license
	packageJsonContent := `{
  "name": "licensed-app",
  "version": "1.0.0",
  "license": "MIT",
  "dependencies": {
    "express": "^4.18.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Node.js project")

	payload := results[0]
	assert.Equal(t, "licensed-app", payload.Name)
	assert.Contains(t, payload.Licenses, "MIT", "Should detect MIT license")
}

func TestDetector_Detect_PackageJsonWithoutName(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content without name
	packageJsonContent := `{
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to missing name
	assert.Empty(t, results, "Should not detect project without name")
}

func TestDetector_Detect_InvalidPackageJson(t *testing.T) {
	detector := &Detector{}

	// Create invalid JSON content
	invalidJsonContent := `{
  "name": "invalid-app",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0"
  // Missing closing brace
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": invalidJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to invalid JSON
	assert.Empty(t, results, "Should not detect project with invalid JSON")
}

func TestDetector_Detect_NoPackageJson(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no package.json
	files := []types.File{
		{Name: "app.js", Path: "/project/app.js"},
		{Name: "README.md", Path: "/project/README.md"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Node.js components without package.json")
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
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_PackageJsonWithOnlyDevDependencies(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content with only dev dependencies
	packageJsonContent := `{
  "name": "dev-only-app",
  "version": "1.0.0",
  "devDependencies": {
    "jest": "^27.0.0",
    "eslint": "^8.0.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"jest": {"matched dependency: jest"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Node.js project")

	payload := results[0]
	assert.Equal(t, "dev-only-app", payload.Name)
	assert.Contains(t, payload.Tech, "nodejs", "Should have nodejs as primary tech")
	assert.Contains(t, payload.Techs, "jest", "Should detect jest from dev dependencies")

	// Check dependencies
	assert.Len(t, payload.Dependencies, 2, "Should have 2 dev dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "npm", dep.Type, "All dependencies should be npm type")
	}

	assert.True(t, depNames["jest"], "Should have jest dependency")
	assert.True(t, depNames["eslint"], "Should have eslint dependency")
}

func TestDetector_Detect_PackageJsonWithEmptyDependencies(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content with empty dependencies
	packageJsonContent := `{
  "name": "no-deps-app",
  "version": "1.0.0",
  "dependencies": {},
  "devDependencies": {}
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Node.js project")

	payload := results[0]
	assert.Equal(t, "no-deps-app", payload.Name)
	assert.Contains(t, payload.Tech, "nodejs", "Should have nodejs as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content
	packageJsonContent := `{
  "name": "path-test-app",
  "version": "1.0.0"
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/package.json": packageJsonContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "package.json", Path: "/project/subdir/package.json"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Node.js project")

	payload := results[0]
	assert.Equal(t, "path-test-app", payload.Name)
	assert.Equal(t, "/subdir/package.json", payload.Path[0], "Should handle relative paths correctly")
}
