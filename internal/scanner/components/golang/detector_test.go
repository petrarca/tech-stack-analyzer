package golang

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
	assert.Equal(t, "golang", detector.Name())
}

func TestDetector_Detect_GoMod(t *testing.T) {
	detector := &Detector{}

	// Create mock go.mod content
	goModContent := `module github.com/example/test-app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
	gorm.io/gorm v1.25.4
)

require (
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
)`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/go.mod": goModContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"gin":  {"matched dependency: github.com/gin-gonic/gin"},
			"gorm": {"matched dependency: gorm.io/gorm"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "go.mod", Path: "/project/go.mod"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name) // Uses folder name
	assert.Equal(t, "/go.mod", payload.Path[0])
	assert.Contains(t, payload.Tech, "golang", "Should have golang as primary tech")
	assert.Contains(t, payload.Techs, "gin", "Should detect gin from dependencies")
	assert.Contains(t, payload.Techs, "gorm", "Should detect gorm from dependencies")

	// Check dependencies - should only include direct dependencies (not indirect)
	assert.Len(t, payload.Dependencies, 3, "Should have 3 direct dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "golang", dep.Type, "All dependencies should be golang type")
	}

	assert.True(t, depNames["github.com/gin-gonic/gin@v1.9.1"], "Should have gin dependency")
	assert.True(t, depNames["github.com/stretchr/testify@v1.8.4"], "Should have testify dependency")
	assert.True(t, depNames["gorm.io/gorm@v1.25.4"], "Should have gorm dependency")
}

func TestDetector_Detect_MainGo(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with main.go
	files := []types.File{
		{Name: "main.go", Path: "/project/main.go"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name) // Uses folder name
	assert.Equal(t, "/main.go", payload.Path[0])
	assert.Contains(t, payload.Tech, "golang", "Should have golang as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies for main.go detection")
}

func TestDetector_Detect_BothGoModAndMainGo(t *testing.T) {
	detector := &Detector{}

	// Create mock go.mod content
	goModContent := `module github.com/example/test-app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
)`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/go.mod": goModContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with both go.mod and main.go
	files := []types.File{
		{Name: "go.mod", Path: "/project/go.mod"},
		{Name: "main.go", Path: "/project/main.go"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - should detect both as separate components
	require.Len(t, results, 2, "Should detect two Go components")

	// First should be go.mod component
	goModPayload := results[0]
	assert.Equal(t, "project", goModPayload.Name)
	assert.Equal(t, "/go.mod", goModPayload.Path[0])
	assert.Contains(t, goModPayload.Tech, "golang")
	assert.Len(t, goModPayload.Dependencies, 1, "Should have 1 dependency")

	// Second should be main.go component
	mainGoPayload := results[1]
	assert.Equal(t, "project", mainGoPayload.Name)
	assert.Equal(t, "/main.go", mainGoPayload.Path[0])
	assert.Contains(t, mainGoPayload.Tech, "golang")
	assert.Empty(t, mainGoPayload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_NoGoFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no Go files
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
		{Name: "app.py", Path: "/project/app.py"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Go components without go.mod or main.go")
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
		{Name: "go.mod", Path: "/project/go.mod"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_EmptyGoMod(t *testing.T) {
	detector := &Detector{}

	// Create empty go.mod content
	emptyGoModContent := `module github.com/example/test-app

go 1.21
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/go.mod": emptyGoModContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "go.mod", Path: "/project/go.mod"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Contains(t, payload.Tech, "golang", "Should have golang as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_GoModWithIndirectOnly(t *testing.T) {
	detector := &Detector{}

	// Create go.mod content with only indirect dependencies
	goModContent := `module github.com/example/test-app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
)`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/go.mod": goModContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "go.mod", Path: "/project/go.mod"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Contains(t, payload.Tech, "golang", "Should have golang as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies (indirect are skipped)")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock go.mod content
	goModContent := `module github.com/example/test-app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
)`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/go.mod": goModContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "go.mod", Path: "/project/subdir/go.mod"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "subdir", payload.Name, "Should use subdir name as project name")
	assert.Equal(t, "/subdir/go.mod", payload.Path[0], "Should handle relative paths correctly")
}

func TestDetector_Detect_MainGoInSubdirectory(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with main.go in subdirectory
	files := []types.File{
		{Name: "main.go", Path: "/project/subdir/main.go"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Go project")

	payload := results[0]
	assert.Equal(t, "subdir", payload.Name, "Should use subdir name as project name")
	assert.Equal(t, "/subdir/main.go", payload.Path[0], "Should handle relative paths correctly")
	assert.Contains(t, payload.Tech, "golang", "Should have golang as primary tech")
}

func TestDetectGoMod_DependencyParsing(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name         string
		content      string
		expectedDeps []string
	}{
		{
			name: "basic dependencies",
			content: `module test

require (
	github.com/gin-gonic/gin v1.9.1
	gorm.io/gorm v1.25.4
)`,
			expectedDeps: []string{
				"github.com/gin-gonic/gin@v1.9.1",
				"gorm.io/gorm@v1.25.4",
			},
		},
		{
			name: "with indirect dependencies",
			content: `module test

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4 // indirect
)`,
			expectedDeps: []string{
				"github.com/gin-gonic/gin@v1.9.1",
			},
		},
		{
			name: "with comments",
			content: `module test

require (
	github.com/gin-gonic/gin v1.9.1 // web framework
	gorm.io/gorm v1.25.4 // orm
)`,
			expectedDeps: []string{}, // Comments are skipped
		},
		{
			name: "no dependencies",
			content: `module test

go 1.21`,
			expectedDeps: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock provider
			provider := &MockProvider{
				files: map[string]string{
					"/project/go.mod": tt.content,
				},
			}

			// Setup mock dependency detector
			depDetector := &MockDependencyDetector{
				matchedTechs: map[string][]string{},
			}

			// Create file list
			files := []types.File{
				{Name: "go.mod", Path: "/project/go.mod"},
			}

			// Test detection
			results := detector.Detect(files, "/project", "/project", provider, depDetector)

			// Verify results
			require.Len(t, results, 1, "Should detect one Go project")

			payload := results[0]
			assert.Len(t, payload.Dependencies, len(tt.expectedDeps), "Should have correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				if i < len(payload.Dependencies) {
					assert.Equal(t, expectedDep, payload.Dependencies[i].Name, "Should have correct dependency name")
					assert.Equal(t, "golang", payload.Dependencies[i].Type, "Should be golang type")
				}
			}
		})
	}
}
