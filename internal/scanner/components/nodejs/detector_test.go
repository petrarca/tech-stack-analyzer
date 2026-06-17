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

func (m *MockDependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Mock implementation - do nothing
}

func (m *MockDependencyDetector) ApplyMatchesToPayload(payload *types.Payload, matches map[string][]string) {
	for tech, reasons := range matches {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		m.AddPrimaryTechIfNeeded(payload, tech)
	}
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

func TestDetector_Detect_WorkspaceAncestorLock(t *testing.T) {
	detector := &Detector{}

	// A nested workspace member with ranges and NO adjacent lock; the resolved
	// versions live in the workspace-root yarn.lock one level up.
	memberPkg := `{
  "name": "@example/ui",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "classnames": "~2.3.2"
  }
}`
	rootLock := `# yarn lockfile v1

"classnames@npm:~2.3.2":
  version: 2.3.2
  resolution: "classnames@npm:2.3.2"

"react@npm:^18.2.0":
  version: 18.2.0
  resolution: "react@npm:18.2.0"
`

	provider := &MockProvider{
		files: map[string]string{
			"/ws/packages/ui/package.json": memberPkg,
			"/ws/yarn.lock":                rootLock,
		},
	}
	depDetector := &MockDependencyDetector{matchedTechs: map[string][]string{}}
	files := []types.File{{Name: "package.json", Path: "/ws/packages/ui/package.json"}}

	results := detector.Detect(files, "/ws/packages/ui", "/ws", provider, depDetector)
	require.Len(t, results, 1)

	versions := make(map[string]string)
	source := make(map[string]interface{})
	for _, dep := range results[0].Dependencies {
		versions[dep.Name] = dep.Version
		if dep.Metadata != nil {
			source[dep.Name] = dep.Metadata["source"]
		}
	}
	assert.Equal(t, "18.2.0", versions["react"], "react should resolve from ancestor workspace lock")
	assert.Equal(t, "2.3.2", versions["classnames"], "classnames should resolve from ancestor workspace lock")
	assert.Equal(t, "workspace-lock", source["react"], "resolution origin should be recorded")
}

func TestDetector_Detect_NoAncestorLockKeepsRanges(t *testing.T) {
	detector := &Detector{}

	// No lock anywhere: ranges from package.json are retained (fallback).
	memberPkg := `{
  "name": "@example/api",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0"
  }
}`
	provider := &MockProvider{
		files: map[string]string{
			"/ws/packages/api/package.json": memberPkg,
		},
	}
	depDetector := &MockDependencyDetector{matchedTechs: map[string][]string{}}
	files := []types.File{{Name: "package.json", Path: "/ws/packages/api/package.json"}}

	results := detector.Detect(files, "/ws/packages/api", "/ws", provider, depDetector)
	require.Len(t, results, 1)
	require.Len(t, results[0].Dependencies, 1)
	assert.Equal(t, "^4.18.0", results[0].Dependencies[0].Version, "range retained when no lock found")
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
	// Check if any license object has the expected license name
	found := false
	for _, license := range payload.Licenses {
		if license.LicenseName == "MIT" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should detect MIT license")
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

	// A package.json with only devDependencies (no runtime dependencies) is
	// build tooling (e.g., grunt/webpack for a Java or Perl project), not a
	// Node.js application or library. It should NOT create a component.
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

	// Should skip: zero runtime dependencies means this is build tooling
	assert.Empty(t, results, "Should not detect a nodejs component for devDependencies-only package")
}

func TestDetector_Detect_PackageJsonWithEmptyDependencies(t *testing.T) {
	detector := &Detector{}

	// A package.json with empty dependencies (both runtime and dev) should
	// NOT create a component -- there is no evidence of a real Node.js project.
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

	// Should skip: zero runtime dependencies
	assert.Empty(t, results, "Should not detect a nodejs component for empty-dependencies package")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock package.json content (must have runtime dependencies to be detected)
	packageJsonContent := `{
  "name": "path-test-app",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0"
  }
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
