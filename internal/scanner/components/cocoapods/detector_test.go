package cocoapods

import (
	"os"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
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

// MockDependencyDetector implements the DependencyDetector interface for testing
type MockDependencyDetector struct{}

func (m *MockDependencyDetector) MatchDependencies(dependencies []string, depType string) map[string][]string {
	result := make(map[string][]string)

	// Mock some common CocoaPods dependencies to tech mapping
	for _, dep := range dependencies {
		switch dep {
		case "AFNetworking":
			result["afnetworking"] = []string{"cocoapods dependency matched"}
		case "Alamofire":
			result["alamofire"] = []string{"cocoapods dependency matched"}
		case "SDWebImage":
			result["sdwebimage"] = []string{"cocoapods dependency matched"}
		}
	}

	return result
}

func (m *MockDependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Mock implementation - do nothing
}

func TestDetector_Name(t *testing.T) {
	detector := &Detector{}
	expected := "cocoapods"
	if got := detector.Name(); got != expected {
		t.Errorf("Detector.Name() = %v, want %v", got, expected)
	}
}

func TestDetector_Detect_Podfile(t *testing.T) {
	detector := &Detector{}

	files := []types.File{
		{Name: "Podfile", Path: "Podfile"},
	}

	provider := &MockProvider{
		files: map[string]string{
			"/mock/Podfile": `# Test Podfile
target 'TestApp' do
  use_frameworks!
  pod 'AFNetworking', '~> 4.0'
  pod 'Alamofire', '~> 5.0'
  pod 'SDWebImage'
end`,
		},
	}

	depDetector := &MockDependencyDetector{}

	payloads := detector.Detect(files, "/mock", "/mock", provider, depDetector)

	assert.Len(t, payloads, 1, "Expected 1 payload")

	payload := payloads[0]
	assert.Equal(t, "CocoaPods", payload.Name, "Expected payload name 'CocoaPods'")
	assert.Contains(t, payload.Tech, "cocoapods", "Expected payload to have 'cocoapods' as primary tech")
	assert.Len(t, payload.Dependencies, 3, "Expected 3 dependencies")
}

func TestDetector_Detect_PodfileLock(t *testing.T) {
	detector := &Detector{}

	files := []types.File{
		{Name: "Podfile.lock", Path: "Podfile.lock"},
	}

	provider := &MockProvider{
		files: map[string]string{
			"/mock/Podfile.lock": `PODS:
  - AFNetworking (4.0.1):
    - AFNetworking/NSURLSession (= 4.0.1)
  - Alamofire (5.4.0)
  - SDWebImage (5.10.0)

DEPENDENCIES:
  - AFNetworking (~> 4.0)
  - Alamofire (~> 5.0)
  - SDWebImage

SPEC REPOS:
  https://github.com/CocoaPods/Specs.git

SPEC CHECKSUMS:
  AFNetworking: 7864df3814a4aec3aca1793b919c499a85a9dbbb
  Alamofire: 3b6a5a8506356337e5a9cb663e0c48a85222d773
  SDWebImage: 91fba697a2d23dcbebe3531eea86c5d3f7944678

COCOAPODS: 1.10.0`,
		},
	}

	depDetector := &MockDependencyDetector{}

	payloads := detector.Detect(files, "/mock", "/mock", provider, depDetector)

	assert.Len(t, payloads, 1, "Expected 1 payload")

	payload := payloads[0]
	assert.Equal(t, "CocoaPods", payload.Name, "Expected payload name 'CocoaPods'")
	assert.Contains(t, payload.Tech, "cocoapods", "Expected payload to have 'cocoapods' as primary tech")
	assert.Len(t, payload.Dependencies, 3, "Expected 3 dependencies")
}

func TestDetector_Detect_NoPodfile(t *testing.T) {
	detector := &Detector{}

	files := []types.File{
		{Name: "README.md", Path: "README.md"},
	}

	provider := &MockProvider{}
	depDetector := &MockDependencyDetector{}

	payloads := detector.Detect(files, "/mock", "/mock", provider, depDetector)

	assert.Len(t, payloads, 0, "Expected 0 payloads")
}

func TestDetector_Detect_EmptyPodfile(t *testing.T) {
	detector := &Detector{}

	files := []types.File{
		{Name: "Podfile", Path: "Podfile"},
	}

	provider := &MockProvider{
		files: map[string]string{
			"/mock/Podfile": `# Empty Podfile
# No pods defined`,
		},
	}

	depDetector := &MockDependencyDetector{}

	payloads := detector.Detect(files, "/mock", "/mock", provider, depDetector)

	assert.Len(t, payloads, 0, "Expected 0 payloads for empty Podfile")
}
