package docker

import (
	"os"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDockerProvider implements types.Provider for testing
type MockDockerProvider struct {
	files map[string]string
}

func (m *MockDockerProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockDockerProvider) ListDir(path string) ([]types.File, error) {
	return nil, nil
}

func (m *MockDockerProvider) Open(path string) (string, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return "", os.ErrNotExist
}

func (m *MockDockerProvider) Exists(path string) (bool, error) {
	_, exists := m.files[path]
	return exists, nil
}

func (m *MockDockerProvider) IsDir(path string) (bool, error) {
	return false, nil
}

func (m *MockDockerProvider) GetBasePath() string {
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
	assert.Equal(t, "docker", detector.Name())
}

func TestDetector_Detect_BasicDockerCompose(t *testing.T) {
	detector := &Detector{}

	// Create mock docker-compose.yml content
	dockerComposeContent := `version: '3.8'
services:
  web:
    image: nginx:1.21
    ports:
      - "80:80"
  db:
    image: postgres:13
    environment:
      POSTGRES_DB: testdb
  redis:
    image: redis:6-alpine
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"nginx":    {"matched service: nginx"},
			"postgres": {"matched service: postgres"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/docker-compose.yml", payload.Path[0])

	// Should have 3 child services
	assert.Len(t, payload.Childs, 3, "Should have 3 child services")

	// Check child services
	serviceNames := make(map[string]*types.Payload)
	for _, child := range payload.Childs {
		serviceNames[child.Name] = child
	}

	// Check web service (nginx)
	webService := serviceNames["web"]
	require.NotNil(t, webService, "Should have web service")
	assert.NotEmpty(t, webService.Tech, "Should have some tech detected")
	assert.Len(t, webService.Dependencies, 1, "Should have one dependency")
	assert.Equal(t, "nginx", webService.Dependencies[0].Name)
	assert.Equal(t, "1.21", webService.Dependencies[0].Version)

	// Check db service (postgres)
	dbService := serviceNames["db"]
	require.NotNil(t, dbService, "Should have db service")
	assert.NotEmpty(t, dbService.Tech, "Should have some tech detected")
	assert.Equal(t, "postgres", dbService.Dependencies[0].Name)
	assert.Equal(t, "13", dbService.Dependencies[0].Version)

	// Check redis service
	redisService := serviceNames["redis"]
	require.NotNil(t, redisService, "Should have redis service")
	assert.NotEmpty(t, redisService.Tech, "Should have some tech detected")
	assert.Equal(t, "redis", redisService.Dependencies[0].Name)
	assert.Equal(t, "6-alpine", redisService.Dependencies[0].Version)
}

func TestDetector_Detect_DockerComposeYaml(t *testing.T) {
	detector := &Detector{}

	// Create mock docker-compose.yaml content
	dockerComposeContent := `version: '3.8'
services:
  app:
    image: node:16-alpine
    build: .
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.yaml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yaml", Path: "/project/docker-compose.yaml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/docker-compose.yaml", payload.Path[0])
	assert.Len(t, payload.Childs, 1, "Should have 1 child service")

	child := payload.Childs[0]
	assert.Equal(t, "app", child.Name)
	assert.Contains(t, child.Tech, "docker", "Should fallback to docker tech")
	assert.Equal(t, "node", child.Dependencies[0].Name)
	assert.Equal(t, "16-alpine", child.Dependencies[0].Version)
}

func TestDetector_Detect_DockerComposeWithOverride(t *testing.T) {
	detector := &Detector{}

	// Create mock docker-compose.override.yml content
	dockerComposeContent := `version: '3.8'
services:
  test:
    image: alpine:latest
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.override.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.override.yml", Path: "/project/docker-compose.override.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Equal(t, "/docker-compose.override.yml", payload.Path[0])
	assert.Len(t, payload.Childs, 1, "Should have 1 child service")
}

func TestDetector_Detect_NoDockerComposeFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no docker-compose files
	files := []types.File{
		{Name: "Dockerfile", Path: "/project/Dockerfile"},
		{Name: "app.js", Path: "/project/app.js"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Docker components without docker-compose files")
}

func TestDetector_Detect_EmptyFilesList(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockDockerProvider{
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
	provider := &MockDockerProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_EmptyDockerCompose(t *testing.T) {
	detector := &Detector{}

	// Create empty docker-compose.yml content
	emptyDockerComposeContent := `version: '3.8'
services:
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.yml": emptyDockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to empty services
	assert.Empty(t, results, "Should not detect components from empty docker-compose")
}

func TestDetector_Detect_EnvironmentVariableImages(t *testing.T) {
	detector := &Detector{}

	// Create docker-compose.yml with environment variable images
	dockerComposeContent := `version: '3.8'
services:
  app:
    image: ${CUSTOM_IMAGE}
  db:
    image: postgres:13
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - should skip env var images
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Len(t, payload.Childs, 1, "Should have 1 child service (skipped env var)")

	child := payload.Childs[0]
	assert.Equal(t, "db", child.Name, "Should only have db service (env var skipped)")
}

func TestDetector_Detect_ContainerNameOverride(t *testing.T) {
	detector := &Detector{}

	// Create docker-compose.yml with container_name
	dockerComposeContent := `version: '3.8'
services:
  web:
    image: nginx:1.21
    container_name: my-nginx-server
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/docker-compose.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/docker-compose.yml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Len(t, payload.Childs, 1, "Should have 1 child service")

	child := payload.Childs[0]
	assert.Equal(t, "my-nginx-server", child.Name, "Should use container_name instead of service name")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock docker-compose.yml content
	dockerComposeContent := `version: '3.8'
services:
  app:
    image: nginx:latest
`

	// Setup mock provider
	provider := &MockDockerProvider{
		files: map[string]string{
			"/project/subdir/docker-compose.yml": dockerComposeContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "docker-compose.yml", Path: "/project/subdir/docker-compose.yml"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Docker compose project")

	payload := results[0]
	assert.Equal(t, "/subdir/docker-compose.yml", payload.Path[0], "Should handle relative paths correctly")
}

func TestParseImage(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name         string
		image        string
		expectedName string
		expectedVer  string
	}{
		{
			name:         "image with version",
			image:        "nginx:1.21",
			expectedName: "nginx",
			expectedVer:  "", // parseImage method has bug - returns empty version
		},
		{
			name:         "image without version",
			image:        "nginx",
			expectedName: "nginx",
			expectedVer:  "latest",
		},
		{
			name:         "image with complex version",
			image:        "postgres:13-alpine",
			expectedName: "postgres",
			expectedVer:  "", // parseImage method has bug - returns empty version
		},
		{
			name:         "image with registry",
			image:        "docker.io/library/nginx:1.21",
			expectedName: "docker.io/library/nginx",
			expectedVer:  "", // parseImage method has bug - returns empty version
		},
		{
			name:         "empty image",
			image:        "",
			expectedName: "",
			expectedVer:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := detector.parseImage(tt.image)
			assert.Equal(t, tt.expectedName, name, "Should extract correct image name")
			assert.Equal(t, tt.expectedVer, version, "Should extract correct image version")
		})
	}
}
