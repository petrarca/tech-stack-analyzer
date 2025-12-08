package python

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
	assert.Equal(t, "python", detector.Name())
}

func TestDetector_Detect_BasicPyProject(t *testing.T) {
	detector := &Detector{}

	// Create mock pyproject.toml content
	pyprojectContent := `[build-system]
requires = ["setuptools>=45", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "test-app"
version = "1.0.0"
description = "A test Python application"
license = {text = "MIT"}
dependencies = [
    "flask>=2.0.0",
    "requests==2.26.0",
    "numpy",
]

[project.optional-dependencies]
dev = [
    "pytest>=6.0.0",
    "black",
]`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pyproject.toml": pyprojectContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"flask": {"matched dependency: flask"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "pyproject.toml", Path: "/project/pyproject.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Python project")

	payload := results[0]
	assert.Equal(t, "test-app", payload.Name)
	assert.Equal(t, "/pyproject.toml", payload.Path[0])
	assert.Contains(t, payload.Tech, "python", "Should have python as primary tech")
	assert.Contains(t, payload.Techs, "flask", "Should detect flask from dependencies")
	// License parsing returns raw value, not just the text
	assert.Contains(t, payload.Licenses, "{text = \"MIT\"}", "Should detect MIT license")

	// Check dependencies
	assert.Len(t, payload.Dependencies, 3, "Should have 3 dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "python", dep.Type, "All dependencies should be python type")
	}

	assert.True(t, depNames["flask"], "Should have flask dependency")
	assert.True(t, depNames["requests"], "Should have requests dependency")
	assert.True(t, depNames["numpy"], "Should have numpy dependency")
}

func TestDetector_Detect_PoetryFormat(t *testing.T) {
	detector := &Detector{}

	// Create mock pyproject.toml content with Poetry format
	poetryContent := `[tool.poetry]
name = "poetry-app"
version = "1.0.0"
description = "A Poetry-based Python application"
license = "Apache-2.0"

[tool.poetry.dependencies]
python = "^3.8"
django = "^4.0.0"
celery = ">=5.0.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.0.0"
black = "^22.0.0"
]`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pyproject.toml": poetryContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"django": {"matched dependency: django"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "pyproject.toml", Path: "/project/pyproject.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	assert.NotEmpty(t, results, "Should detect Poetry project with name extraction from [tool.poetry]")
	assert.Len(t, results, 1, "Should detect exactly one Poetry project")

	payload := results[0]
	assert.Equal(t, "poetry-app", payload.Name, "Should extract project name from [tool.poetry]")
	assert.Contains(t, payload.Tech, "python", "Should detect Python technology as primary tech")
	assert.Contains(t, payload.Techs, "django", "Should detect Django from dependencies")
}

func TestDetector_Detect_PyProjectWithoutName(t *testing.T) {
	detector := &Detector{}

	// Create mock pyproject.toml content without name
	pyprojectContent := `[build-system]
requires = ["setuptools>=45", "wheel"]

[project]
version = "1.0.0"
dependencies = [
    "flask>=2.0.0",
]`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pyproject.toml": pyprojectContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "pyproject.toml", Path: "/project/pyproject.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to missing name
	assert.Empty(t, results, "Should not detect project without name")
}

func TestDetector_Detect_NoPyProjectToml(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no pyproject.toml
	files := []types.File{
		{Name: "app.py", Path: "/project/app.py"},
		{Name: "requirements.txt", Path: "/project/requirements.txt"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Python components without pyproject.toml")
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
		{Name: "pyproject.toml", Path: "/project/pyproject.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_PyProjectWithEmptyDependencies(t *testing.T) {
	detector := &Detector{}

	// Create mock pyproject.toml content with empty dependencies
	pyprojectContent := `[build-system]
requires = ["setuptools>=45", "wheel"]

[project]
name = "no-deps-app"
version = "1.0.0"
dependencies = []

[project.optional-dependencies]
dev = []
]`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pyproject.toml": pyprojectContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "pyproject.toml", Path: "/project/pyproject.toml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Python project")

	payload := results[0]
	assert.Equal(t, "no-deps-app", payload.Name)
	assert.Contains(t, payload.Tech, "python", "Should have python as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock pyproject.toml content
	pyprojectContent := `[project]
name = "path-test-app"
version = "1.0.0"
]`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/pyproject.toml": pyprojectContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "pyproject.toml", Path: "/project/subdir/pyproject.toml"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Python project")

	payload := results[0]
	assert.Equal(t, "path-test-app", payload.Name)
	assert.Equal(t, "/subdir/pyproject.toml", payload.Path[0], "Should handle relative paths correctly")
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "basic project name",
			content: `[project]
name = "test-app"
version = "1.0.0"
]`,
			expected: "test-app",
		},
		{
			name: "project name with single quotes",
			content: `[project]
name = 'test-app'
version = "1.0.0"
]`,
			expected: "test-app",
		},
		{
			name: "project name with whitespace",
			content: `[project]
name =    "test-app"    
version = "1.0.0"
]`,
			expected: "test-app",
		},
		{
			name: "no project section",
			content: `[build-system]
requires = ["setuptools"]
]`,
			expected: "",
		},
		{
			name: "empty project section",
			content: `[project]
version = "1.0.0"
]`,
			expected: "",
		},
		{
			name: "project name in different section",
			content: `[tool.poetry]
name = "poetry-app"
version = "1.0.0"
]`,
			expected: "poetry-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProjectName(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []types.Dependency
	}{
		{
			name: "basic dependencies",
			content: `[project]
dependencies = [
    "flask>=2.0.0",
    "requests==2.26.0",
    "numpy",
]`,
			expected: []types.Dependency{
				{Type: "python", Name: "flask", Example: "2.0.0"},
				{Type: "python", Name: "requests", Example: "2.26.0"},
				{Type: "python", Name: "numpy", Example: "latest"},
			},
		},
		{
			name: "poetry dependencies",
			content: `[tool.poetry.dependencies]
python = "^3.8"
django = "^4.0.0"
celery = ">=5.0.0"
]`,
			expected: []types.Dependency{
				{Type: "python", Name: "python", Example: "3.8"},
				{Type: "python", Name: "django", Example: "4.0.0"},
				{Type: "python", Name: "celery", Example: "5.0.0"},
			},
		},
		{
			name: "no dependencies",
			content: `[project]
dependencies = []
]`,
			expected: []types.Dependency{},
		},
		{
			name: "no dependencies section",
			content: `[project]
name = "test-app"
]`,
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDependencies(tt.content)
			assert.Equal(t, len(tt.expected), len(result), "Should have correct number of dependencies")

			for i, expectedDep := range tt.expected {
				if i < len(result) {
					assert.Equal(t, expectedDep.Type, result[i].Type)
					assert.Equal(t, expectedDep.Name, result[i].Name)
					assert.Equal(t, expectedDep.Example, result[i].Example)
				}
			}
		})
	}
}

func TestDetectLicense(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "MIT license",
			content: `[project]
name = "test-app"
license = {text = "MIT"}
]`,
			expected: "{text = \"MIT\"}", // Returns raw value
		},
		{
			name: "Apache license",
			content: `[project]
name = "test-app"
license = "Apache-2.0"
]`,
			expected: "Apache-2.0",
		},
		{
			name: "no license",
			content: `[project]
name = "test-app"
version = "1.0.0"
]`,
			expected: "",
		},
		{
			name: "license in different section",
			content: `[tool.poetry]
name = "test-app"
license = "MIT"
]`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &types.Payload{}
			detectLicense(tt.content, payload)

			if tt.expected != "" {
				assert.Contains(t, payload.Licenses, tt.expected, "Should detect license")
			} else {
				assert.Empty(t, payload.Licenses, "Should not detect license")
			}
		})
	}
}
