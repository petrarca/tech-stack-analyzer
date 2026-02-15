package docker

import (
	"os"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MockDebugProvider implements types.Provider for testing
type MockDebugProvider struct {
	files map[string]string
}

func (m *MockDebugProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockDebugProvider) ListDir(path string) ([]types.File, error) {
	return nil, nil
}

func (m *MockDebugProvider) Open(path string) (string, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return "", os.ErrNotExist
}

func (m *MockDebugProvider) Exists(path string) (bool, error) {
	_, exists := m.files[path]
	return exists, nil
}

func (m *MockDebugProvider) IsDir(path string) (bool, error) {
	return false, nil
}

func (m *MockDebugProvider) GetBasePath() string {
	return "/mock"
}

// MockDebugDependencyDetector implements components.DependencyDetector for testing
type MockDebugDependencyDetector struct {
	matchedTechs map[string][]string
}

func (m *MockDebugDependencyDetector) MatchDependencies(dependencies []string, depType string) map[string][]string {
	return m.matchedTechs
}

func (m *MockDebugDependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Mock implementation - do nothing
}

func TestDebug_Detection(t *testing.T) {
	detector := &Detector{}

	// Create simple docker-compose.yml content
	dockerComposeContent := `version: '3.8'
services:
  web:
    image: nginx:1.21
  db:
    image: postgres:13
`

	// Setup mock provider
	provider := &MockDebugProvider{
		files: map[string]string{
			"/project/docker-compose.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector with no matches
	depDetector := &MockDebugDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Debug output
	t.Logf("Number of results: %d", len(results))
	if len(results) > 0 {
		payload := results[0]
		t.Logf("Number of children: %d", len(payload.Children))
		for i, child := range payload.Children {
			t.Logf("Child %d: Name=%s, Tech=%v", i, child.Name, child.Tech)
		}
	}
}
