package java

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
	// Simplified mock - not used in current tests
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
	// Simplified mock - not used in current tests
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
	assert.Equal(t, "java", detector.Name())
}

func TestDetector_Detect_MavenProject(t *testing.T) {
	detector := &Detector{}

	// Create mock pom.xml content (without namespaces for XML parsing)
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test-app</artifactId>
    <version>1.0.0</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-boot-starter</artifactId>
            <version>2.7.0</version>
        </dependency>
    </dependencies>
</project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pom.xml": pomContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"spring": {"matched dependency: spring-boot-starter"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "pom.xml", Path: "/project/pom.xml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Maven project")

	payload := results[0]
	assert.Equal(t, "com.example:test-app", payload.Name)
	assert.Equal(t, "/pom.xml", payload.Path[0])
	assert.Contains(t, payload.Tech, "java", "Should have java as primary tech")
	assert.Contains(t, payload.Techs, "maven", "Should detect maven")
	assert.Contains(t, payload.Techs, "spring", "Should detect spring from dependencies")
	assert.NotEmpty(t, payload.Dependencies, "Should have parsed dependencies")
	// Verify Maven properties
	assert.Contains(t, payload.Properties, "maven", "Should have maven properties")
	mavenProps := payload.Properties["maven"].(map[string]interface{})
	assert.Equal(t, "com.example", mavenProps["group_id"])
	assert.Equal(t, "test-app", mavenProps["artifact_id"])
}

func TestDetector_Detect_GradleProject(t *testing.T) {
	detector := &Detector{}

	// Create mock build.gradle content
	gradleContent := `plugins {
    id 'java'
    id 'org.springframework.boot' version '2.7.0'
}

group = 'com.example'
rootProject.name = 'test-gradle-app'

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter'
    testImplementation 'junit:junit:4.13.2'
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/build.gradle": gradleContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"spring": {"matched dependency: spring-boot-starter"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "build.gradle", Path: "/project/build.gradle"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Gradle project")

	payload := results[0]
	assert.Equal(t, "test-gradle-app", payload.Name)
	assert.Equal(t, "/build.gradle", payload.Path[0])
	assert.Contains(t, payload.Tech, "java", "Should have java as primary tech")
	assert.Contains(t, payload.Techs, "gradle", "Should detect gradle")
	assert.Contains(t, payload.Techs, "spring", "Should detect spring from dependencies")
	assert.NotEmpty(t, payload.Dependencies, "Should have parsed dependencies")
	// Verify Gradle properties
	assert.Contains(t, payload.Properties, "gradle", "Should have gradle properties")
	gradleProps := payload.Properties["gradle"].(map[string]string)
	assert.Equal(t, "com.example", gradleProps["group_id"])
	assert.Equal(t, "test-gradle-app", gradleProps["artifact_id"])
}

func TestDetector_Detect_GradleKtsProject(t *testing.T) {
	detector := &Detector{}

	// Create mock build.gradle.kts content
	gradleKtsContent := `plugins {
    java
    id("org.springframework.boot") version "2.7.0"
}

rootProject.name = "test-gradle-kts-app"

dependencies {
    implementation("org.springframework.boot:spring-boot-starter")
    testImplementation("junit:junit:4.13.2")
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/build.gradle.kts": gradleKtsContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"spring": {"matched dependency: spring-boot-starter"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "build.gradle.kts", Path: "/project/build.gradle.kts"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Gradle KTS project")

	payload := results[0]
	assert.Equal(t, "test-gradle-kts-app", payload.Name)
	assert.Equal(t, "/build.gradle.kts", payload.Path[0])
	assert.Contains(t, payload.Tech, "java", "Should have java as primary tech")
	assert.Contains(t, payload.Techs, "gradle", "Should detect gradle")
	assert.Contains(t, payload.Techs, "spring", "Should detect spring from dependencies")
}

func TestDetector_Detect_MavenAndGradleMixed(t *testing.T) {
	detector := &Detector{}

	// Create mock files
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>mixed-app</artifactId>
    <version>1.0.0</version>
</project>`

	gradleContent := `rootProject.name = 'mixed-app'
dependencies {
    implementation 'org.springframework.boot:spring-boot-starter'
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pom.xml":      pomContent,
			"/project/build.gradle": gradleContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"spring": {"matched dependency: spring-boot-starter"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "pom.xml", Path: "/project/pom.xml"},
		{Name: "build.gradle", Path: "/project/build.gradle"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - should prioritize Maven but include Gradle in paths
	require.Len(t, results, 1, "Should detect one project")

	payload := results[0]
	assert.Equal(t, "com.example:mixed-app", payload.Name)
	assert.Equal(t, "/pom.xml", payload.Path[0]) // Main file is pom.xml
	assert.Contains(t, payload.Tech, "java", "Should have java as primary tech")
	assert.Contains(t, payload.Techs, "maven", "Should detect maven")
	assert.Contains(t, payload.Techs, "gradle", "Should also detect gradle")
	// Verify Maven properties
	assert.Contains(t, payload.Properties, "maven", "Should have maven properties")
	mavenProps := payload.Properties["maven"].(map[string]interface{})
	assert.Equal(t, "com.example", mavenProps["group_id"])
	assert.Equal(t, "mixed-app", mavenProps["artifact_id"])
}

func TestDetector_Detect_NoJavaFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no Java files
	files := []types.File{
		{Name: "package.json", Path: "/project/package.json"},
		{Name: "requirements.txt", Path: "/project/requirements.txt"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Java components")
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

func TestDetector_Detect_InvalidPomXML(t *testing.T) {
	detector := &Detector{}

	// Create invalid XML content
	invalidPomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <groupId>com.example</groupId>
    <artifactId>test-app</artifactId>
    <!-- Missing closing tags -->
</project`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/pom.xml": invalidPomContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "pom.xml", Path: "/project/pom.xml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - should still detect but use directory name
	require.Len(t, results, 1, "Should detect project even with invalid XML")

	payload := results[0]
	assert.Equal(t, "project", payload.Name) // Should fallback to directory name
	assert.Contains(t, payload.Tech, "java", "Should have java as primary tech")
	assert.Contains(t, payload.Techs, "maven", "Should detect maven")
}

func TestDetector_formatProjectName(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name       string
		groupId    string
		artifactId string
		expected   string
	}{
		{
			name:       "both groupId and artifactId",
			groupId:    "com.example",
			artifactId: "test-app",
			expected:   "com.example:test-app",
		},
		{
			name:       "only artifactId",
			groupId:    "",
			artifactId: "test-app",
			expected:   "test-app",
		},
		{
			name:       "only groupId",
			groupId:    "com.example",
			artifactId: "",
			expected:   "",
		},
		{
			name:       "both empty",
			groupId:    "",
			artifactId: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.formatProjectName(tt.groupId, tt.artifactId)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetector_Detect_FileReadError(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider that returns error for pom.xml
	provider := &MockProvider{
		files: map[string]string{}, // Empty map will cause file not found error
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "pom.xml", Path: "/project/pom.xml"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}
