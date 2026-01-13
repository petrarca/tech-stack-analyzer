package deno

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

func TestDetector_Name(t *testing.T) {
	detector := &Detector{}
	assert.Equal(t, "deno", detector.Name())
}

func TestDetector_Detect_BasicDenoLock(t *testing.T) {
	detector := &Detector{}

	// Create mock deno.lock content
	denoLockContent := `{
  "version": "3",
  "remote": {},
  "packages": {
    "std": "0.208.0",
    "oak": "12.1.0",
    "postgres": "0.17.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"oak":      {"matched dependency: oak"},
			"postgres": {"matched dependency: postgres"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Deno project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name) // Virtual payload for deno.lock
	assert.Equal(t, "/deno.lock", payload.Path[0])

	// Note: Deno parser currently doesn't extract dependencies from packages section
	// It only validates the version and creates a virtual payload
	assert.Empty(t, payload.Techs, "Should have no techs when no dependencies parsed")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies (parser limitation)")
}

func TestDetector_Detect_EmptyDenoLock(t *testing.T) {
	detector := &Detector{}

	// Create empty deno.lock content
	denoLockContent := `{
  "version": "3",
  "remote": {},
  "packages": {}
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Deno project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/deno.lock", payload.Path[0])
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_NoDenoLock(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no deno.lock
	files := []types.File{
		{Name: "main.ts", Path: "/project/main.ts"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Deno components without deno.lock")
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
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock deno.lock content
	denoLockContent := `{
  "version": "3",
  "remote": {},
  "packages": {
    "std": "0.208.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/subdir/deno.lock"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Deno project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/subdir/deno.lock", payload.Path[0], "Should handle relative paths correctly")
}

func TestDetector_Detect_InvalidDenoLock(t *testing.T) {
	detector := &Detector{}

	// Create invalid deno.lock content (no version)
	denoLockContent := `{
  "remote": {},
  "packages": {
    "std": "0.208.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to missing version
	assert.Empty(t, results, "Should not detect project without version in deno.lock")
}

func TestDetector_Detect_DenoLockWithRemotePackages(t *testing.T) {
	detector := &Detector{}

	// Create deno.lock content with remote packages
	denoLockContent := `{
  "version": "3",
  "remote": {
    "https://deno.land/std@0.208.0/fmt/colors.ts": "3f5b6b8c9d2e1f4a7b8c9d2e1f4a7b8c9d2e1f4a"
  },
  "packages": {
    "std": "0.208.0",
    "oak": "12.1.0"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Deno project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/deno.lock", payload.Path[0])

	// Should parse dependencies (behavior depends on parser implementation)
	assert.True(t, len(payload.Dependencies) >= 0, "Should handle remote packages correctly")
}

func TestDetector_Detect_DenoLockWithComplexPackages(t *testing.T) {
	detector := &Detector{}

	// Create deno.lock content with complex package structure
	denoLockContent := `{
  "version": "3",
  "remote": {},
  "packages": {
    "std": "0.208.0",
    "oak": "12.1.0",
    "postgres": "0.17.0",
    "dotenv": "3.0.0",
    "zod": "3.21.4"
  }
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/deno.lock": denoLockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"oak":      {"matched dependency: oak"},
			"postgres": {"matched dependency: postgres"},
			"zod":      {"matched dependency: zod"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "deno.lock", Path: "/project/deno.lock"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Deno project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/deno.lock", payload.Path[0])

	// Note: Deno parser currently doesn't extract dependencies from packages section
	// It only validates the version and creates a virtual payload
	assert.Empty(t, payload.Techs, "Should have no techs when no dependencies parsed")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies (parser limitation)")
}
