package cplusplus

import (
	"os"
	"strings"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MockProvider for testing
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
	return "/test/base"
}

// MockDependencyDetector for testing
type MockDependencyDetector struct{}

func (m *MockDependencyDetector) MatchDependencies(depNames []string, depType string) map[string][]string {
	return map[string][]string{
		"qt":       {"matched dependency: qt"},
		"openssl":  {"matched dependency: openssl"},
		"cmake":    {"matched dependency: cmake"},
		"security": {"matched dependency: openssl"},
	}
}

func (m *MockDependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Mock implementation - do nothing
}

func TestDetector_Detect(t *testing.T) {
	depDetector := &MockDependencyDetector{}

	tests := []struct {
		name     string
		files    []types.File
		provider *MockProvider
		expected int // Number of payloads expected
	}{
		{
			name: "detect conan project",
			files: []types.File{
				{Name: "conanfile.py"},
				{Name: "main.cpp"},
			},
			provider: &MockProvider{
				files: map[string]string{
					"/test/project/conanfile.py": `
class MedistarRecipe(ConanFile):
	def requirements(self):
		self.requires("openssl/3.2.6")
		self.requires("qt/6.5.0")
		self.tool_requires("cmake/3.25.0")
					`,
				},
			},
			expected: 1,
		},
		{
			name: "detect conan project with packages files",
			files: []types.File{
				{Name: "conanfile.py"},
				{Name: "packagesCommon.txt"},
				{Name: "packagesVc17.txt"},
			},
			provider: &MockProvider{
				files: map[string]string{
					"/test/project/conanfile.py": `
class MedistarRecipe(ConanFile):
	def tool_requirements(self):
		self.tool_requires("msuic/1.0.6")
						`,
					"/test/project/packagesCommon.txt": `
cbox_dev/25.4.1002.0
cgmassist_dev/2.0.0.26001
openssl/3.2.6
					`,
					"/test/project/packagesVc17.txt": `
iqeasy/0.1.30.76402_2
occi/21.15.0
					`,
				},
			},
			expected: 1,
		},
		{
			name: "no conanfile - should not detect",
			files: []types.File{
				{Name: "CMakeLists.txt"},
				{Name: "main.cpp"},
				{Name: "utils.h"},
			},
			provider: &MockProvider{},
			expected: 0,
		},
		{
			name: "empty conanfile - should still detect",
			files: []types.File{
				{Name: "conanfile.py"},
			},
			provider: &MockProvider{
				files: map[string]string{
					"/test/project/conanfile.py": `# Empty conanfile`,
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &Detector{}
			results := detector.Detect(tt.files, "/test/project", "/test/base", tt.provider, depDetector)

			if len(results) != tt.expected {
				t.Errorf("Expected %d payloads, got %d", tt.expected, len(results))
				return
			}

			if tt.expected > 0 {
				depDetector.validatePayload(t, results[0], tt.files, tt.provider)
			}
		})
	}
}

func (d *MockDependencyDetector) validatePayload(t *testing.T, payload *types.Payload, files []types.File, provider *MockProvider) {
	// Check that primary tech is set to cplusplus
	if len(payload.Tech) == 0 {
		t.Error("Expected primary tech to be set")
	}
	if payload.Tech[0] != "cplusplus" {
		t.Errorf("Expected primary tech 'cplusplus', got '%s'", payload.Tech[0])
	}

	// Check that Conan tech is added
	if !d.hasTech(payload.Techs, "conan") {
		t.Error("Expected 'conan' tech to be added")
	}

	// Check dependencies are parsed when content is available
	d.validateDependencies(t, payload, files, provider)
}

func (d *MockDependencyDetector) hasTech(techs []string, target string) bool {
	for _, tech := range techs {
		if tech == target {
			return true
		}
	}
	return false
}

func (d *MockDependencyDetector) validateDependencies(t *testing.T, payload *types.Payload, files []types.File, provider *MockProvider) {
	hasConanfile := false
	for _, file := range files {
		if file.Name == "conanfile.py" {
			hasConanfile = true
			break
		}
	}

	if hasConanfile {
		_, hasContent := provider.files["/test/project/conanfile.py"]
		if hasContent && len(payload.Dependencies) == 0 {
			conanContent := provider.files["/test/project/conanfile.py"]
			if strings.Contains(conanContent, "requires") || strings.Contains(conanContent, "self.requires") {
				t.Error("Expected dependencies to be parsed from conanfile.py when content is available")
			}
		}
	}
}

func TestDetector_extractProjectName(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input: `
class MedistarRecipe(ConanFile):
	def requirements(self):
		self.requires("openssl/3.2.6")
			`,
			expected: "medistar",
		},
		{
			input: `
class MyProjectRecipe(ConanFile):
	def requirements(self):
		self.requires("qt/6.5.0")
			`,
			expected: "myproject",
		},
		{
			input: `
class SimpleRecipe(ConanFile):
	pass
			`,
			expected: "simple",
		},
		{
			input: `
# No class definition
def requirements(self):
	pass
			`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := detector.extractProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected project name '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
