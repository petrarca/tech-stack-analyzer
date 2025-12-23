package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewDelphiParser(t *testing.T) {
	parser := NewDelphiParser()
	assert.NotNil(t, parser, "Should create a new DelphiParser")
	assert.IsType(t, &DelphiParser{}, parser, "Should return correct type")
}

func TestParseDproj(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name            string
		content         string
		filename        string
		expectedProject DelphiProject
		expectedDeps    []types.Dependency
	}{
		{
			name: "VCL project with packages",
			content: `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<FrameworkType>VCL</FrameworkType>
		<ProjectVersion>18.8</ProjectVersion>
		<Config Condition="'$(Config)'==''">Debug</Config>
		<DCC_UsePackage>vcl;rtl;vclimg;vclx;$(DCC_UsePackage)</DCC_UsePackage>
	</PropertyGroup>
</Project>`,
			filename: "MyProject.dproj",
			expectedProject: DelphiProject{
				Name:      "MyProject",
				Framework: "VCL",
				Packages:  []string{"vcl", "rtl", "vclimg", "vclx"},
			},
			expectedDeps: []types.Dependency{
				{Type: "delphi", Name: "vcl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "vclimg", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "vclx", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
		{
			name: "FMX project with packages",
			content: `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<FrameworkType>FMX</FrameworkType>
		<ProjectVersion>18.8</ProjectVersion>
		<Config Condition="'$(Config)'==''">Debug</Config>
		<DCC_UsePackage>fmx;rtl;fmxase;$(DCC_UsePackage)</DCC_UsePackage>
	</PropertyGroup>
</Project>`,
			filename: "FMXApp.dproj",
			expectedProject: DelphiProject{
				Name:      "FMXApp",
				Framework: "FMX",
				Packages:  []string{"fmx", "rtl", "fmxase"},
			},
			expectedDeps: []types.Dependency{
				{Type: "delphi", Name: "fmx", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "fmxase", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
		{
			name: "Empty project file",
			content: `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<ProjectVersion>18.8</ProjectVersion>
	</PropertyGroup>
</Project>`,
			filename: "EmptyProject.dproj",
			expectedProject: DelphiProject{
				Name:      "EmptyProject",
				Framework: "",
				Packages:  []string{},
			},
			expectedDeps: []types.Dependency{},
		},
		{
			name: "Project with no framework but with packages",
			content: `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<ProjectVersion>18.8</ProjectVersion>
		<DCC_UsePackage>rtl;vcl;$(DCC_UsePackage)</DCC_UsePackage>
	</PropertyGroup>
</Project>`,
			filename: "NoFramework.dproj",
			expectedProject: DelphiProject{
				Name:      "NoFramework",
				Framework: "",
				Packages:  []string{"rtl", "vcl"},
			},
			expectedDeps: []types.Dependency{
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "vcl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
		{
			name: "Project with duplicate packages",
			content: `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<FrameworkType>VCL</FrameworkType>
		<DCC_UsePackage>rtl;vcl;rtl;vcl;$(DCC_UsePackage)</DCC_UsePackage>
	</PropertyGroup>
</Project>`,
			filename: "Duplicates.dproj",
			expectedProject: DelphiProject{
				Name:      "Duplicates",
				Framework: "VCL",
				Packages:  []string{"rtl", "vcl"},
			},
			expectedDeps: []types.Dependency{
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "vcl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := parser.ParseDproj(tt.content, tt.filename)

			assert.Equal(t, tt.expectedProject.Name, project.Name, "Project name should match")
			assert.Equal(t, tt.expectedProject.Framework, project.Framework, "Framework should match")
			assert.Equal(t, tt.expectedProject.Packages, project.Packages, "Packages should match")

			// Test CreateDependencies method
			dependencies := parser.CreateDependencies(project)
			assert.Equal(t, len(tt.expectedDeps), len(dependencies), "Number of dependencies should match")

			for i, expectedDep := range tt.expectedDeps {
				if i < len(dependencies) {
					assert.Equal(t, expectedDep.Type, dependencies[i].Type, "Dependency type should match")
					assert.Equal(t, expectedDep.Name, dependencies[i].Name, "Dependency name should match")
					assert.Equal(t, expectedDep.Version, dependencies[i].Version, "Dependency version should match")
					assert.Equal(t, expectedDep.Scope, dependencies[i].Scope, "Dependency scope should match")
					assert.Equal(t, expectedDep.Direct, dependencies[i].Direct, "Dependency direct flag should match")
					assert.Equal(t, expectedDep.Metadata, dependencies[i].Metadata, "Dependency metadata should match")
				}
			}
		})
	}
}

func TestDelphiExtractProjectName(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"simple name", "MyProject.dproj", "MyProject"},
		{"name with path", "C:\\Projects\\MyApp.dproj", "C:\\Projects\\MyApp"},
		{"name with spaces", "My Project.dproj", "My Project"},
		{"already without extension", "MyProject", "MyProject"},
		{"multiple dots", "My.Project.v1.dproj", "My.Project.v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractProjectName(tt.filename)
			assert.Equal(t, tt.expected, result, "Project name extraction should match")
		})
	}
}

func TestExtractFrameworkType(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"VCL framework", `<FrameworkType>VCL</FrameworkType>`, "VCL"},
		{"FMX framework", `<FrameworkType>FMX</FrameworkType>`, "FMX"},
		{"framework with whitespace", `<FrameworkType>  VCL  </FrameworkType>`, "VCL"},
		{"no framework", `<ProjectVersion>18.8</ProjectVersion>`, ""},
		{"empty framework", `<FrameworkType></FrameworkType>`, ""},
		{"malformed XML", `<FrameworkType>VCL</FrameworkType><Other>Value</Other>`, "VCL"},
		{"case sensitive", `<FrameworkType>vcl</FrameworkType>`, "vcl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractFrameworkType(tt.content)
			assert.Equal(t, tt.expected, result, "Framework extraction should match")
		})
	}
}

func TestExtractPackages(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "basic packages",
			content:  `<DCC_UsePackage>rtl;vcl;vclimg;$(DCC_UsePackage)</DCC_UsePackage>`,
			expected: []string{"rtl", "vcl", "vclimg"},
		},
		{
			name:     "packages with spaces",
			content:  `<DCC_UsePackage> rtl ; vcl ; vclimg ; $(DCC_UsePackage) </DCC_UsePackage>`,
			expected: []string{"rtl", "vcl", "vclimg"},
		},
		{
			name:     "empty packages",
			content:  `<DCC_UsePackage></DCC_UsePackage>`,
			expected: []string{},
		},
		{
			name:     "only variables",
			content:  `<DCC_UsePackage>$(DCC_UsePackage)</DCC_UsePackage>`,
			expected: []string{},
		},
		{
			name:     "mixed packages and variables",
			content:  `<DCC_UsePackage>rtl;$(BDS);vcl;$(DCC_UsePackage)</DCC_UsePackage>`,
			expected: []string{"rtl", "vcl"},
		},
		{
			name: "multiple DCC_UsePackage entries",
			content: `<DCC_UsePackage>rtl;vcl</DCC_UsePackage>
<DCC_UsePackage>vclimg;fmx</DCC_UsePackage>`,
			expected: []string{"rtl", "vcl", "vclimg", "fmx"},
		},
		{
			name: "duplicates across entries",
			content: `<DCC_UsePackage>rtl;vcl</DCC_UsePackage>
<DCC_UsePackage>vcl;rtl;fmx</DCC_UsePackage>`,
			expected: []string{"rtl", "vcl", "fmx"},
		},
		{
			name:     "empty entries",
			content:  `<DCC_UsePackage>rtl;;vcl;;$(DCC_UsePackage)</DCC_UsePackage>`,
			expected: []string{"rtl", "vcl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractPackages(tt.content)
			assert.Equal(t, tt.expected, result, "Package extraction should match")
		})
	}
}

func TestIsVCL(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name      string
		framework string
		expected  bool
	}{
		{"VCL uppercase", "VCL", true},
		{"vcl lowercase", "vcl", true},
		{"VCL mixed case", "Vcl", true},
		{"FMX framework", "FMX", false},
		{"empty string", "", false},
		{"random text", "SomethingElse", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsVCL(tt.framework)
			assert.Equal(t, tt.expected, result, "VCL detection should match")
		})
	}
}

func TestIsFMX(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name      string
		framework string
		expected  bool
	}{
		{"FMX uppercase", "FMX", true},
		{"fmx lowercase", "fmx", true},
		{"FMX mixed case", "Fmx", true},
		{"VCL framework", "VCL", false},
		{"empty string", "", false},
		{"random text", "SomethingElse", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsFMX(tt.framework)
			assert.Equal(t, tt.expected, result, "FMX detection should match")
		})
	}
}

func TestDelphiCreateDependencies(t *testing.T) {
	parser := NewDelphiParser()

	tests := []struct {
		name     string
		project  DelphiProject
		expected []types.Dependency
	}{
		{
			name: "empty packages",
			project: DelphiProject{
				Name:      "Test",
				Framework: "VCL",
				Packages:  []string{},
			},
			expected: []types.Dependency{},
		},
		{
			name: "single package",
			project: DelphiProject{
				Name:      "Test",
				Framework: "VCL",
				Packages:  []string{"rtl"},
			},
			expected: []types.Dependency{
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
		{
			name: "multiple packages",
			project: DelphiProject{
				Name:      "Test",
				Framework: "FMX",
				Packages:  []string{"rtl", "vcl", "fmx"},
			},
			expected: []types.Dependency{
				{Type: "delphi", Name: "rtl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "vcl", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
				{Type: "delphi", Name: "fmx", Version: "", Scope: types.ScopeProd, Direct: true, Metadata: types.NewMetadata(".dproj")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CreateDependencies(tt.project)
			assert.Equal(t, len(tt.expected), len(result), "Number of dependencies should match")

			for i, expectedDep := range tt.expected {
				if i < len(result) {
					assert.Equal(t, expectedDep.Type, result[i].Type, "Dependency type should match")
					assert.Equal(t, expectedDep.Name, result[i].Name, "Dependency name should match")
					assert.Equal(t, expectedDep.Version, result[i].Version, "Dependency version should match")
					assert.Equal(t, expectedDep.Scope, result[i].Scope, "Dependency scope should match")
					assert.Equal(t, expectedDep.Direct, result[i].Direct, "Dependency direct flag should match")
					assert.Equal(t, expectedDep.Metadata, result[i].Metadata, "Dependency metadata should match")
				}
			}
		})
	}
}

func TestDelphiParserIntegration(t *testing.T) {
	// Integration test with a realistic .dproj file content
	parser := NewDelphiParser()

	realContent := `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
	<PropertyGroup>
		<ProjectGuid>{12345678-1234-1234-1234-123456789ABC}</ProjectGuid>
		<ProjectVersion>18.8</ProjectVersion>
		<FrameworkType>VCL</FrameworkType>
		<MainSource>MyProject.dpr</MainSource>
		<Base>True</Base>
		<Config Condition="'$(Config)'==''">Debug</Config>
		<DCC_UsePackage>rtl;vcl;vclimg;vclx;dxCore;dxGDIPlus;cxLibrary;dxTheme;$(DCC_UsePackage)</DCC_UsePackage>
		<DCC_DcuOutput>.\$(Platform)\$(Config)</DCC_DcuOutput>
		<DCC_ExeOutput>.\$(Platform)\$(Config)</DCC_ExeOutput>
		<DCC_E>false</DCC_E>
		<DCC_N>false</DCC_N>
		<DCC_S>false</DCC_S>
		<DCC_F>false</DCC_F>
		<DCC_K>false</DCC_K>
	</PropertyGroup>
</Project>`

	project := parser.ParseDproj(realContent, "MyRealProject.dproj")

	// Verify project parsing
	assert.Equal(t, "MyRealProject", project.Name, "Project name should be extracted correctly")
	assert.Equal(t, "VCL", project.Framework, "Framework should be VCL")
	assert.Equal(t, []string{"rtl", "vcl", "vclimg", "vclx", "dxCore", "dxGDIPlus", "cxLibrary", "dxTheme"}, project.Packages, "All packages should be extracted")

	// Verify dependency creation
	dependencies := parser.CreateDependencies(project)
	assert.Equal(t, 8, len(dependencies), "Should create 8 dependencies")

	// Check specific dependency
	rtlDep := dependencies[0]
	assert.Equal(t, "delphi", rtlDep.Type)
	assert.Equal(t, "rtl", rtlDep.Name)
	assert.Equal(t, "", rtlDep.Version)
	assert.Equal(t, types.ScopeProd, rtlDep.Scope)
	assert.Equal(t, true, rtlDep.Direct)
	assert.Equal(t, types.NewMetadata(".dproj"), rtlDep.Metadata)

	// Verify framework detection
	assert.True(t, parser.IsVCL(project.Framework), "Should detect VCL framework")
	assert.False(t, parser.IsFMX(project.Framework), "Should not detect FMX framework")
}
