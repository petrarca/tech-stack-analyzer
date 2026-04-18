package cplusplus

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MockProvider satisfies types.Provider for unit tests.
type MockProvider struct {
	files map[string]string
}

func (m *MockProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockProvider) ListDir(path string) ([]types.File, error) { return nil, nil }
func (m *MockProvider) Open(path string) (string, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return "", os.ErrNotExist
}
func (m *MockProvider) Exists(path string) (bool, error) { _, ok := m.files[path]; return ok, nil }
func (m *MockProvider) IsDir(path string) (bool, error)  { return false, nil }
func (m *MockProvider) GetBasePath() string              { return "/test/base" }

// MockDependencyDetector is a minimal stub — it returns fixed tech matches.
type MockDependencyDetector struct{}

func (m *MockDependencyDetector) MatchDependencies(depNames []string, depType string) map[string][]string {
	return map[string][]string{
		"qt":      {"matched dependency: qt"},
		"openssl": {"matched dependency: openssl"},
		"cmake":   {"matched dependency: cmake"},
	}
}

func (m *MockDependencyDetector) AddPrimaryTechIfNeeded(_ *types.Payload, _ string) {}

func (m *MockDependencyDetector) ApplyMatchesToPayload(payload *types.Payload, matches map[string][]string) {
	for tech, reasons := range matches {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		m.AddPrimaryTechIfNeeded(payload, tech)
	}
}

// --- helpers ---

func hasTech(techs []string, target string) bool {
	for _, t := range techs {
		if t == target {
			return true
		}
	}
	return false
}

// --- Detect (conan + vcxproj combined table) ---

func TestDetector_Detect(t *testing.T) {
	depDetector := &MockDependencyDetector{}

	tests := []struct {
		name          string
		files         []types.File
		providerFiles map[string]string
		wantCount     int
		wantConan     bool // first result has conan tech
	}{
		{
			name: "conan project",
			files: []types.File{
				{Name: "conanfile.py"},
				{Name: "main.cpp"},
			},
			providerFiles: map[string]string{
				"/test/project/conanfile.py": `
class MyAppRecipe(ConanFile):
	def requirements(self):
		self.requires("openssl/3.2.6")
		self.requires("qt/6.5.0")
		self.tool_requires("cmake/3.25.0")
`,
			},
			wantCount: 1,
			wantConan: true,
		},
		{
			name: "conan project with packages files",
			files: []types.File{
				{Name: "conanfile.py"},
				{Name: "packagesCommon.txt"},
				{Name: "packagesVc17.txt"},
			},
			providerFiles: map[string]string{
				"/test/project/conanfile.py": `
class MyAppRecipe(ConanFile):
	def tool_requirements(self):
		self.tool_requires("buildtool/1.0.6")
`,
				"/test/project/packagesCommon.txt": `
acmecore_dev/25.4.1002.0
mylib_dev/2.0.0.26001
openssl/3.2.6
`,
				"/test/project/packagesVc17.txt": `
easyio/0.1.30.76402_2
dbconnect/21.15.0
`,
			},
			wantCount: 1,
			wantConan: true,
		},
		{
			name: "empty conanfile still detected",
			files: []types.File{
				{Name: "conanfile.py"},
			},
			providerFiles: map[string]string{
				"/test/project/conanfile.py": `# Empty conanfile`,
			},
			wantCount: 1,
			wantConan: true,
		},
		{
			name: "no manifest files - nothing detected",
			files: []types.File{
				{Name: "CMakeLists.txt"},
				{Name: "main.cpp"},
				{Name: "utils.h"},
			},
			providerFiles: map[string]string{},
			wantCount:     0,
		},
		{
			name: "vcxproj only",
			files: []types.File{
				{Name: "MyLib.vcxproj"},
			},
			providerFiles: map[string]string{
				"/test/project/MyLib.vcxproj": `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals"><RootNamespace>MyLib</RootNamespace></PropertyGroup>
  <PropertyGroup Label="Configuration">
    <ConfigurationType>DynamicLibrary</ConfigurationType>
    <PlatformToolset>v143</PlatformToolset>
  </PropertyGroup>
</Project>`,
			},
			wantCount: 1,
			wantConan: false,
		},
		{
			name: "csproj only - not detected",
			files: []types.File{
				{Name: "MyProject.csproj"},
			},
			providerFiles: map[string]string{},
			wantCount:     0,
		},
		{
			name: "conan and vcxproj together - both detected",
			files: []types.File{
				{Name: "conanfile.py"},
				{Name: "App.vcxproj"},
			},
			providerFiles: map[string]string{
				"/test/project/conanfile.py": `class AppRecipe(ConanFile): pass`,
				"/test/project/App.vcxproj": `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals"><RootNamespace>App</RootNamespace></PropertyGroup>
</Project>`,
			},
			wantCount: 2,
		},
		{
			name: "vcxproj with empty name - skipped",
			files: []types.File{
				{Name: "Broken.vcxproj"},
			},
			providerFiles: map[string]string{
				// No RootNamespace or ProjectName, and no fallback filename can be extracted
				// because the XML parses fine but yields no name. Actually nameFromPath will
				// still give "Broken", so this tests the read-error path instead.
			},
			// File not in provider → ReadFile returns error → skipped
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &Detector{}
			provider := &MockProvider{files: tt.providerFiles}
			results := detector.Detect(tt.files, "/test/project", "/test/base", provider, depDetector)

			assert.Len(t, results, tt.wantCount)

			if tt.wantConan && len(results) > 0 {
				assert.True(t, hasTech(results[0].Techs, "conan"), "expected 'conan' tech")
				assert.Equal(t, "cplusplus", results[0].Tech[0])
			}
		})
	}
}

// --- Conan-specific tests ---

func TestDetector_Detect_ConanfileWithLicense(t *testing.T) {
	detector := &Detector{}
	depDetector := &MockDependencyDetector{}

	provider := &MockProvider{
		files: map[string]string{
			"/project/conanfile.py": `
class MyAppRecipe(ConanFile):
    license = "MIT"
    def requirements(self):
        self.requires("openssl/3.2.6")
`,
		},
	}

	results := detector.Detect(
		[]types.File{{Name: "conanfile.py", Path: "/project/conanfile.py"}},
		"/project", "/project", provider, depDetector,
	)

	require.Len(t, results, 1)
	require.Len(t, results[0].Licenses, 1)
	assert.Equal(t, "MIT", results[0].Licenses[0].LicenseName)
	assert.Equal(t, "direct", results[0].Licenses[0].DetectionType)
	assert.Equal(t, "conanfile.py", results[0].Licenses[0].SourceFile)
}

func TestDetector_extractConanProjectName(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "standard Recipe class",
			input: `
class MyAppRecipe(ConanFile):
	def requirements(self):
		self.requires("openssl/3.2.6")
`,
			expected: "myapp",
		},
		{
			name: "project name with multiple words",
			input: `
class MyProjectRecipe(ConanFile):
	def requirements(self):
		self.requires("qt/6.5.0")
`,
			expected: "myproject",
		},
		{
			name:     "simple one-word Recipe",
			input:    `class SimpleRecipe(ConanFile): pass`,
			expected: "simple",
		},
		{
			name: "no class definition",
			input: `
# No class definition
def requirements(self):
	pass
`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detector.extractConanProjectName(tt.input))
		})
	}
}

func TestDetector_extractConanLicense(t *testing.T) {
	detector := &Detector{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double-quoted license",
			input:    `    license = "MIT"`,
			expected: "MIT",
		},
		{
			name:     "SPDX expression",
			input:    `    license = "MIT OR Apache-2.0"`,
			expected: "MIT OR Apache-2.0",
		},
		{
			name:     "single-quoted license",
			input:    `    license = 'BSD-3-Clause'`,
			expected: "BSD-3-Clause",
		},
		{
			name:     "no license",
			input:    `class MyAppRecipe(ConanFile): pass`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detector.extractConanLicense(tt.input))
		})
	}
}

// --- vcxproj-specific tests ---

func TestDetector_Detect_Vcxproj_Basic(t *testing.T) {
	detector := &Detector{}
	depDetector := &MockDependencyDetector{}

	vcxprojContent := `<?xml version="1.0" encoding="utf-8"?>
<Project DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals">
    <RootNamespace>HDApi5</RootNamespace>
    <WindowsTargetPlatformVersion>10.0</WindowsTargetPlatformVersion>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Release|Win32'" Label="Configuration">
    <ConfigurationType>DynamicLibrary</ConfigurationType>
    <PlatformToolset>v143</PlatformToolset>
    <CharacterSet>MultiByte</CharacterSet>
    <UseOfMfc>Dynamic</UseOfMfc>
  </PropertyGroup>
  <ItemDefinitionGroup>
    <Link>
      <AdditionalDependencies>ws2_32.lib;%(AdditionalDependencies)</AdditionalDependencies>
    </Link>
  </ItemDefinitionGroup>
  <ItemGroup>
    <ProjectReference Include="..\HDCtrlEx\HDCtrlEx.vcxproj" />
  </ItemGroup>
</Project>`

	provider := &MockProvider{
		files: map[string]string{"/test/project/HDApi.vcxproj": vcxprojContent},
	}

	results := detector.Detect(
		[]types.File{{Name: "HDApi.vcxproj"}},
		"/test/project", "/test/base", provider, depDetector,
	)

	require.Len(t, results, 1)
	p := results[0]

	assert.Equal(t, "HDApi5", p.Name)
	assert.Equal(t, "msbuild-cpp", p.ComponentType)
	require.NotEmpty(t, p.Tech)
	assert.Equal(t, "cplusplus", p.Tech[0])
	assert.True(t, hasTech(p.Techs, "mfc"), "expected 'mfc' in techs")

	props := p.Properties["msbuild_cpp"].(map[string]interface{})
	assert.Equal(t, "v143", props["platform_toolset"])
	assert.Equal(t, "VS2022", props["vs_version"])
	assert.Equal(t, "DynamicLibrary", props["configuration_type"])
	assert.Equal(t, "10.0", props["windows_sdk"])

	var hasNativeLib, hasVcxprojRef bool
	for _, dep := range p.Dependencies {
		if dep.Type == "native-lib" && dep.Name == "ws2_32.lib" {
			hasNativeLib = true
		}
		if dep.Type == "vcxproj-ref" && dep.Name == "HDCtrlEx" {
			hasVcxprojRef = true
		}
	}
	assert.True(t, hasNativeLib, "expected native-lib dep ws2_32.lib")
	assert.True(t, hasVcxprojRef, "expected vcxproj-ref dep HDCtrlEx")
}
