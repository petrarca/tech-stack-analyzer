package dotnet

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
	assert.Equal(t, "dotnet", detector.Name())
}

func TestDetector_Detect_BasicCsproj(t *testing.T) {
	detector := &Detector{}

	// Create mock .csproj content
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>TestApp</AssemblyName>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
    <PackageReference Include="EntityFrameworkCore" Version="6.0.0" />
    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
  </ItemGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/TestApp.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"EntityFrameworkCore": {"matched dependency: EntityFrameworkCore"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "TestApp.csproj", Path: "/project/TestApp.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "TestApp", payload.Name) // Uses AssemblyName from .csproj
	assert.Equal(t, "TestApp.csproj", payload.Path[0])
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
	assert.Contains(t, payload.Techs, "dotnet", "Should detect dotnet with framework info")

	// Check dependencies
	assert.Len(t, payload.Dependencies, 3, "Should have 3 NuGet dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "nuget", dep.Type, "All dependencies should be nuget type")
		assert.True(t, dep.Direct, "All NuGet dependencies should be direct")
		assert.NotEmpty(t, dep.Scope, "All NuGet dependencies should have a scope")
		assert.NotNil(t, dep.Metadata, "All NuGet dependencies should have metadata")
	}

	assert.True(t, depNames["Microsoft.AspNetCore.App"], "Should have Microsoft.AspNetCore.App dependency")
	assert.True(t, depNames["EntityFrameworkCore"], "Should have EntityFrameworkCore dependency")
	assert.True(t, depNames["Newtonsoft.Json"], "Should have Newtonsoft.Json dependency")

	// Verify no child components exist (unified with Java behavior)
	assert.Empty(t, payload.Childs, "Should have no child components (unified with Java)")

	// Verify techs are added to parent component
	assert.Contains(t, payload.Techs, "dotnet", "Parent should have dotnet tech")
	assert.Contains(t, payload.Techs, "EntityFrameworkCore", "Parent should have EntityFrameworkCore tech")
}

func TestDetector_Detect_MultipleCsprojFiles(t *testing.T) {
	detector := &Detector{}

	// Create mock .csproj content for multiple projects
	webAppContent := `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>WebApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>
</Project>`

	classLibContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>ClassLib</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
  </ItemGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/WebApp.csproj":   webAppContent,
			"/project/ClassLib.csproj": classLibContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "WebApp.csproj", Path: "/project/WebApp.csproj"},
		{Name: "ClassLib.csproj", Path: "/project/ClassLib.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 2, "Should detect two .NET projects")

	// First project (WebApp)
	webApp := results[0]
	assert.Equal(t, "WebApp", webApp.Name)
	assert.Equal(t, "WebApp.csproj", webApp.Path[0])
	assert.Contains(t, webApp.Tech, "dotnet")
	assert.Len(t, webApp.Dependencies, 1, "WebApp should have 1 dependency")

	// Second project (ClassLib)
	classLib := results[1]
	assert.Equal(t, "ClassLib", classLib.Name)
	assert.Equal(t, "ClassLib.csproj", classLib.Path[0])
	assert.Contains(t, classLib.Tech, "dotnet")
	assert.Len(t, classLib.Dependencies, 1, "ClassLib should have 1 dependency")
}

func TestDetector_Detect_CsprojWithoutAssemblyName(t *testing.T) {
	detector := &Detector{}

	// Create .csproj without AssemblyName
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.Extensions.Logging" Version="6.0.0" />
  </ItemGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/NoName.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "NoName.csproj", Path: "/project/NoName.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - note: parser now extracts name from filename
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "NoName", payload.Name) // Extracted from filename
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
}

func TestDetector_Detect_EmptyCsproj(t *testing.T) {
	detector := &Detector{}

	// Create empty .csproj content
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/Empty.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "Empty.csproj", Path: "/project/Empty.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - note: parser now extracts name from filename even for empty projects
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "Empty", payload.Name) // Extracted from filename
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
}

func TestDetector_Detect_NoCsprojFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no .csproj files
	files := []types.File{
		{Name: "Program.cs", Path: "/project/Program.cs"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any .NET components without .csproj files")
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
		{Name: "TestApp.csproj", Path: "/project/TestApp.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock .csproj content
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>PathTestApp</AssemblyName>
  </PropertyGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/PathTestApp.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "PathTestApp.csproj", Path: "/project/subdir/PathTestApp.csproj"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "PathTestApp", payload.Name)
	assert.Equal(t, "subdir/PathTestApp.csproj", payload.Path[0], "Should handle relative paths correctly")
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
}

func TestDetector_Detect_NoDependencies(t *testing.T) {
	detector := &Detector{}

	// Create .csproj with no dependencies
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>NoDepsApp</AssemblyName>
  </PropertyGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/NoDepsApp.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "NoDepsApp.csproj", Path: "/project/NoDepsApp.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "NoDepsApp", payload.Name)
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
	assert.Empty(t, payload.Dependencies, "Should have no dependencies")
	assert.Empty(t, payload.Childs, "Should have no child components")
}

func TestDetector_Detect_NoMatchingDependencies(t *testing.T) {
	detector := &Detector{}

	// Create .csproj with dependencies that don't match any tech
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>NoMatchApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="UnknownPackage1" Version="1.0.0" />
    <PackageReference Include="UnknownPackage2" Version="2.0.0" />
  </ItemGroup>
</Project>`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/NoMatchApp.csproj": csprojContent,
		},
	}

	// Setup mock dependency detector with no matches
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "NoMatchApp.csproj", Path: "/project/NoMatchApp.csproj"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one .NET project")

	payload := results[0]
	assert.Equal(t, "NoMatchApp", payload.Name)
	assert.Contains(t, payload.Tech, "dotnet", "Should have dotnet as primary tech")
	assert.Len(t, payload.Dependencies, 2, "Should have 2 dependencies")
	assert.Empty(t, payload.Childs, "Should have no child components when no matches")
}
