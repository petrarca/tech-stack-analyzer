package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVcxprojParser_ParseVcxproj_FullProject(t *testing.T) {
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project DefaultTargets="Build" ToolsVersion="15.0" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals">
    <ProjectGuid>{12345678-ABCD-1234-ABCD-123456789ABC}</ProjectGuid>
    <RootNamespace>HelloApp</RootNamespace>
    <Keyword>MFCProj</Keyword>
    <WindowsTargetPlatformVersion>10.0.17763.0</WindowsTargetPlatformVersion>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Debug|Win32'" Label="Configuration">
    <ConfigurationType>Application</ConfigurationType>
    <PlatformToolset>v141</PlatformToolset>
    <CharacterSet>MultiByte</CharacterSet>
    <UseOfMfc>Dynamic</UseOfMfc>
    <CLRSupport>true</CLRSupport>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Release|Win32'" Label="Configuration">
    <ConfigurationType>Application</ConfigurationType>
    <PlatformToolset>v143</PlatformToolset>
    <CharacterSet>MultiByte</CharacterSet>
    <UseOfMfc>Dynamic</UseOfMfc>
  </PropertyGroup>
  <ItemDefinitionGroup Condition="'$(Configuration)|$(Platform)'=='Release|Win32'">
    <Link>
      <AdditionalDependencies>hdapi5.lib;hdctrlex.lib;%(AdditionalDependencies)</AdditionalDependencies>
    </Link>
  </ItemDefinitionGroup>
  <ItemGroup>
    <ProjectReference Include="..\HDApi\HDApi.vcxproj">
      <Project>{AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE}</Project>
    </ProjectReference>
  </ItemGroup>
</Project>`

	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(content, "/project/HelloApp/HelloApp.vcxproj")

	assert.Equal(t, "HelloApp", project.Name)
	assert.Equal(t, "v143", project.PlatformToolset, "should prefer Release PlatformToolset")
	assert.Equal(t, "Application", project.ConfigurationType)
	assert.Equal(t, "Dynamic", project.UseOfMfc)
	assert.Equal(t, "true", project.CLRSupport)
	assert.Equal(t, "MultiByte", project.CharacterSet)
	assert.Equal(t, "10.0.17763.0", project.WindowsTargetPlatformVersion)
	assert.Contains(t, project.AdditionalDependencies, "hdapi5.lib")
	assert.Contains(t, project.AdditionalDependencies, "hdctrlex.lib")
	assert.NotContains(t, project.AdditionalDependencies, "%(AdditionalDependencies)")
	assert.Len(t, project.ProjectReferences, 1)
	assert.Equal(t, "..\\HDApi\\HDApi.vcxproj", project.ProjectReferences[0])
}

func TestVcxprojParser_ParseVcxproj_MinimalProject(t *testing.T) {
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals">
    <RootNamespace>MyLib</RootNamespace>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Debug|Win32'" Label="Configuration">
    <ConfigurationType>StaticLibrary</ConfigurationType>
    <PlatformToolset>v100</PlatformToolset>
  </PropertyGroup>
</Project>`

	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(content, "/project/MyLib/MyLib 2010.vcxproj")

	assert.Equal(t, "MyLib", project.Name)
	assert.Equal(t, "v100", project.PlatformToolset)
	assert.Equal(t, "StaticLibrary", project.ConfigurationType)
	assert.Empty(t, project.UseOfMfc)
	assert.Empty(t, project.CLRSupport)
	assert.Empty(t, project.AdditionalDependencies)
	assert.Empty(t, project.ProjectReferences)
}

func TestVcxprojParser_ParseVcxproj_InvalidXML(t *testing.T) {
	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(`not valid xml at all`, "/project/Broken/Broken.vcxproj")
	assert.Equal(t, "Broken", project.Name, "should fallback to filename on parse error")
}

func TestVcxprojParser_ParseVcxproj_ProjectNameOverridesRootNamespace(t *testing.T) {
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals">
    <RootNamespace>SomeNamespace</RootNamespace>
    <ProjectName>ActualProjectName</ProjectName>
  </PropertyGroup>
</Project>`

	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(content, "/project/test.vcxproj")
	assert.Equal(t, "ActualProjectName", project.Name, "ProjectName should override RootNamespace")
}

func TestVcxprojParser_ParseVcxproj_FirstNonEmptyWins(t *testing.T) {
	// When multiple PropertyGroups each have a ProjectName, the first one wins.
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup Label="Globals">
    <ProjectName>FirstName</ProjectName>
  </PropertyGroup>
  <PropertyGroup Label="Configuration">
    <ProjectName>SecondName</ProjectName>
  </PropertyGroup>
</Project>`

	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(content, "/project/test.vcxproj")
	assert.Equal(t, "FirstName", project.Name, "first non-empty ProjectName should win")
}

func TestVcxprojParser_CollectAdditionalDependencies_Dedup(t *testing.T) {
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <ItemDefinitionGroup Condition="Debug">
    <Link>
      <AdditionalDependencies>debug_lib.lib;common.lib;%(AdditionalDependencies)</AdditionalDependencies>
    </Link>
  </ItemDefinitionGroup>
  <ItemDefinitionGroup Condition="Release">
    <Link>
      <AdditionalDependencies>release_lib.lib;common.lib;%(AdditionalDependencies)</AdditionalDependencies>
    </Link>
  </ItemDefinitionGroup>
</Project>`

	parser := NewVcxprojParser()
	project := parser.ParseVcxproj(content, "/project/test.vcxproj")

	assert.Contains(t, project.AdditionalDependencies, "debug_lib.lib")
	assert.Contains(t, project.AdditionalDependencies, "release_lib.lib")
	assert.Contains(t, project.AdditionalDependencies, "common.lib")
	assert.NotContains(t, project.AdditionalDependencies, "%(AdditionalDependencies)")

	// common.lib must appear exactly once
	count := 0
	for _, lib := range project.AdditionalDependencies {
		if lib == "common.lib" {
			count++
		}
	}
	assert.Equal(t, 1, count, "common.lib should be deduplicated")
}

func TestVcxprojParser_nameFromPath_VSYearSuffixes(t *testing.T) {
	parser := NewVcxprojParser()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"no suffix", "/project/HDApi.vcxproj", "HDApi"},
		{"VS2010 suffix", "/project/HDApi 2010.vcxproj", "HDApi"},
		{"VS2012 suffix", "/project/HDApi 2012.vcxproj", "HDApi"},
		{"VS2013 suffix", "/project/HDApi 2013.vcxproj", "HDApi"},
		{"VS2015 suffix", "/project/HDApi 2015.vcxproj", "HDApi"},
		{"VS2017 suffix", "/project/HDApi 2017.vcxproj", "HDApi"},
		{"VS2019 suffix", "/project/HDApi 2019.vcxproj", "HDApi"},
		{"VS2022 suffix", "/project/HDApi 2022.vcxproj", "HDApi"},
		{"no year but number", "/project/HDCtrlEx100.vcxproj", "HDCtrlEx100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parser.nameFromPath(tt.path))
		})
	}
}

func TestPlatformToolsetToVSVersion(t *testing.T) {
	tests := []struct {
		name     string
		toolset  string
		expected string
	}{
		{"v100 = VS2010", "v100", "VS2010"},
		{"v110 = VS2012", "v110", "VS2012"},
		{"v120 = VS2013", "v120", "VS2013"},
		{"v140 = VS2015", "v140", "VS2015"},
		{"v141 = VS2017", "v141", "VS2017"},
		{"v142 = VS2019", "v142", "VS2019"},
		{"v143 = VS2022", "v143", "VS2022"},
		{"unknown toolset returns empty", "v999", ""},
		{"empty string returns empty", "", ""},
		{"uppercase not recognized", "V143", ""},
		{"trailing space not recognized", "v143 ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PlatformToolsetToVSVersion(tt.toolset))
		})
	}
}
