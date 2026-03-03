package githubactions

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
}

func TestDetector_Name(t *testing.T) {
	detector := &Detector{}
	assert.Equal(t, "githubactions", detector.Name())
}

func TestDetector_Detect_BasicWorkflow(t *testing.T) {
	detector := &Detector{}

	workflowContent := `name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: npm test
`

	provider := &MockProvider{
		files: map[string]string{
			"/project/.github/workflows/ci.yml": workflowContent,
		},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"github-actions": {"github action matched"},
		},
	}

	files := []types.File{
		{Name: ".github/workflows/ci.yml", Path: "/project/.github/workflows/ci.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)

	require.Len(t, results, 1, "Should detect one GitHub Actions component")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name, "Should be a virtual component")
	assert.NotEmpty(t, payload.Dependencies, "Should have action dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
	}
	assert.True(t, depNames["actions/checkout"], "Should have actions/checkout dependency")
	assert.True(t, depNames["actions/setup-node"], "Should have actions/setup-node dependency")
}

func TestDetector_Detect_WorkflowWithServices(t *testing.T) {
	detector := &Detector{}

	workflowContent := `name: Test
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:14
      redis:
        image: redis:7
    steps:
      - uses: actions/checkout@v4
      - run: npm test
`

	provider := &MockProvider{
		files: map[string]string{
			"/project/.github/workflows/test.yml": workflowContent,
		},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: ".github/workflows/test.yml", Path: "/project/.github/workflows/test.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)

	require.Len(t, results, 1, "Should detect one GitHub Actions component")

	payload := results[0]
	depNames := make(map[string]bool)
	depTypes := make(map[string]string)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		depTypes[dep.Name] = dep.Type
	}
	assert.True(t, depNames["actions/checkout"], "Should have actions/checkout")
	assert.True(t, depNames["postgres"], "Should have postgres service")
	assert.True(t, depNames["redis"], "Should have redis service")
	assert.Equal(t, "docker", depTypes["postgres"], "Service deps should be docker type")
}

func TestDetector_Detect_NoWorkflowFiles(t *testing.T) {
	detector := &Detector{}

	provider := &MockProvider{
		files: map[string]string{},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
		{Name: "README.md", Path: "/project/README.md"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)
	assert.Empty(t, results, "Should not detect anything without workflow files")
}

func TestDetector_Detect_EmptyWorkflow(t *testing.T) {
	detector := &Detector{}

	// Workflow with no jobs
	workflowContent := `name: Empty
on: push
`

	provider := &MockProvider{
		files: map[string]string{
			"/project/.github/workflows/empty.yml": workflowContent,
		},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: ".github/workflows/empty.yml", Path: "/project/.github/workflows/empty.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)
	assert.Empty(t, results, "Should not detect component from workflow with no jobs")
}

func TestDetector_Detect_WorkflowWithContainer(t *testing.T) {
	detector := &Detector{}

	workflowContent := `name: Container
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    container: node:18-alpine
    steps:
      - uses: actions/checkout@v4
`

	provider := &MockProvider{
		files: map[string]string{
			"/project/.github/workflows/container.yml": workflowContent,
		},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: ".github/workflows/container.yml", Path: "/project/.github/workflows/container.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)

	require.Len(t, results, 1, "Should detect one GitHub Actions component")

	payload := results[0]
	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
	}
	assert.True(t, depNames["node"], "Should have container image dependency")
	assert.True(t, depNames["actions/checkout"], "Should have action dependency")
}

func TestDetector_Detect_FileReadError(t *testing.T) {
	detector := &Detector{}

	provider := &MockProvider{
		files: map[string]string{}, // File doesn't exist
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: ".github/workflows/ci.yml", Path: "/project/.github/workflows/ci.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)
	assert.Empty(t, results, "Should not detect anything when file read fails")
}

func TestDetector_Detect_WorkflowWithOnlyRunSteps(t *testing.T) {
	detector := &Detector{}

	// Workflow with only run steps (no uses: actions)
	workflowContent := `name: Simple
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "hello"
      - run: make build
`

	provider := &MockProvider{
		files: map[string]string{
			"/project/.github/workflows/simple.yml": workflowContent,
		},
	}

	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	files := []types.File{
		{Name: ".github/workflows/simple.yml", Path: "/project/.github/workflows/simple.yml"},
	}

	results := detector.Detect(files, "/project", "/mock", provider, depDetector)
	assert.Empty(t, results, "Should not detect component from workflow with no action dependencies")
}
