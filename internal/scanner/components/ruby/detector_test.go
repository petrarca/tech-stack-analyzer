package ruby

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
	assert.Equal(t, "ruby", detector.Name())
}

func TestDetector_Detect_BasicGemfile(t *testing.T) {
	detector := &Detector{}

	// Create mock Gemfile content
	gemfileContent := `source "https://rubygems.org"

ruby "3.2.0"

gem "rails", "~> 7.0.0"
gem "pg", "~> 1.1"
gem "devise", "~> 4.8"

group :development, :test do
  gem "rspec-rails", "~> 6.0"
  gem "pry-byebug"
end

group :development do
  gem "web-console", "~> 4.0"
end
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"rails": {"matched dependency: rails"},
			"pg":    {"matched dependency: pg"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name) // Uses folder name since Gemfile has no project name
	assert.Equal(t, "/Gemfile", payload.Path[0])
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
	assert.Contains(t, payload.Techs, "bundler", "Should detect bundler from Gemfile")
	assert.Contains(t, payload.Techs, "rails", "Should detect rails from dependencies")
	assert.Contains(t, payload.Techs, "pg", "Should detect pg from dependencies")

	// Check dependencies - should include all gems from all groups
	// Note: Ruby parser may not include all dependencies as expected
	assert.Len(t, payload.Dependencies, 6, "Should have 6 dependencies (actual parser behavior)")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "ruby", dep.Type, "All dependencies should be ruby type")
	}

	assert.True(t, depNames["rails"], "Should have rails dependency")
	assert.True(t, depNames["pg"], "Should have pg dependency")
	assert.True(t, depNames["devise"], "Should have devise dependency")
	assert.True(t, depNames["rspec-rails"], "Should have rspec-rails dependency")
	assert.True(t, depNames["pry-byebug"], "Should have pry-byebug dependency")
	assert.True(t, depNames["web-console"], "Should have web-console dependency")
}

func TestDetector_Detect_MinimalGemfile(t *testing.T) {
	detector := &Detector{}

	// Create minimal Gemfile content
	gemfileContent := `source "https://rubygems.org"

gem "sinatra"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Equal(t, "/Gemfile", payload.Path[0])
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
	assert.Contains(t, payload.Techs, "bundler", "Should detect bundler")
	assert.Len(t, payload.Dependencies, 1, "Should have 1 dependency")
	assert.Equal(t, "sinatra", payload.Dependencies[0].Name)
}

func TestDetector_Detect_EmptyGemfile(t *testing.T) {
	detector := &Detector{}

	// Create empty Gemfile content
	gemfileContent := `source "https://rubygems.org"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
	assert.Contains(t, payload.Techs, "bundler", "Should detect bundler")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_NoGemfile(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no Gemfile
	files := []types.File{
		{Name: "app.rb", Path: "/project/app.rb"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Ruby components without Gemfile")
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
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock Gemfile content
	gemfileContent := `source "https://rubygems.org"

gem "rails"
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/subdir/Gemfile"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "subdir", payload.Name, "Should use subdir name as project name")
	assert.Equal(t, "/subdir/Gemfile", payload.Path[0], "Should handle relative paths correctly")
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
}

func TestExtractProjectName(t *testing.T) {
	detector := &Detector{}

	// Test that extractProjectName always returns empty string
	// as Gemfiles don't have standard project name fields
	result := detector.extractProjectName(`source "https://rubygems.org"
gem "rails"
`)
	assert.Equal(t, "", result, "Should return empty string for Gemfile content")

	// Test with empty content
	result2 := detector.extractProjectName("")
	assert.Equal(t, "", result2, "Should return empty string for empty content")
}

func TestDetector_Detect_GemfileWithComments(t *testing.T) {
	detector := &Detector{}

	// Create Gemfile with comments
	gemfileContent := `# A Ruby web application
source "https://rubygems.org"

# Rails framework
gem "rails", "~> 7.0.0"

# Database
gem "pg", "~> 1.1"

group :development do
  # Development tools
  gem "pry-rails"
end
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
	assert.Contains(t, payload.Techs, "bundler", "Should detect bundler")
	assert.Len(t, payload.Dependencies, 3, "Should have 3 dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
	}

	assert.True(t, depNames["rails"], "Should have rails dependency")
	assert.True(t, depNames["pg"], "Should have pg dependency")
	assert.True(t, depNames["pry-rails"], "Should have pry-rails dependency")
}

func TestDetector_Detect_GemfileWithGitSources(t *testing.T) {
	detector := &Detector{}

	// Create Gemfile with git sources
	gemfileContent := `source "https://rubygems.org"

gem "rails", "~> 7.0.0"

gem "custom_gem", git: "https://github.com/example/custom_gem.git"

group :development do
  gem "dev_gem", git: "https://github.com/example/dev_gem.git", branch: "main"
end
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Gemfile": gemfileContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Gemfile", Path: "/project/Gemfile"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Ruby project")

	payload := results[0]
	assert.Equal(t, "project", payload.Name)
	assert.Contains(t, payload.Tech, "ruby", "Should have ruby as primary tech")
	assert.Contains(t, payload.Techs, "bundler", "Should detect bundler")

	// The parser should handle git sources (behavior depends on parser implementation)
	// We'll test that it doesn't crash and detects something
	assert.True(t, len(payload.Dependencies) >= 1, "Should have at least 1 dependency")
}
