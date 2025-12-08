package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDotNetParser(t *testing.T) {
	parser := NewDotNetParser()
	assert.NotNil(t, parser, "Should create a new DotNetParser")
	assert.IsType(t, &DotNetParser{}, parser, "Should return correct type")
}

func TestParseCsproj(t *testing.T) {
	parser := NewDotNetParser()

	tests := []struct {
		name     string
		content  string
		expected DotNetProject
	}{
		{
			name: "modern SDK-style project with packages",
			content: `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>MyWebApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
    <PackageReference Include="Microsoft.EntityFrameworkCore" Version="6.0.0" />
    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
  </ItemGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "MyWebApp",
				Framework: "net6.0",
				Packages: []DotNetPackage{
					{Name: "Microsoft.AspNetCore.App", Version: ""},
					{Name: "Microsoft.EntityFrameworkCore", Version: "6.0.0"},
					{Name: "Newtonsoft.Json", Version: "13.0.1"},
				},
			},
		},
		{
			name: "modern project without AssemblyName",
			content: `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.Extensions.Logging" Version="7.0.0" />
  </ItemGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "test", // Extracted from filename
				Framework: "net7.0",
				Packages: []DotNetPackage{
					{Name: "Microsoft.Extensions.Logging", Version: "7.0.0"},
				},
			},
		},
		{
			name: "legacy .NET Framework project",
			content: `<Project ToolsVersion="15.0" DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup>
    <TargetFramework>net48</TargetFramework>
    <AssemblyName>LegacyApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="System.Net.Http" Version="4.3.4" />
    <PackageReference Include="Newtonsoft.Json" Version="12.0.3" />
  </ItemGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "LegacyApp",
				Framework: "net48",
				Packages: []DotNetPackage{
					{Name: "System.Net.Http", Version: "4.3.4"},
					{Name: "Newtonsoft.Json", Version: "12.0.3"},
				},
			},
		},
		{
			name: "project with no packages",
			content: `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>SimpleApp</AssemblyName>
  </PropertyGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "SimpleApp",
				Framework: "net6.0",
				Packages:  []DotNetPackage{},
			},
		},
		{
			name: "project with multiple PropertyGroups",
			content: `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
  </PropertyGroup>
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <AssemblyName>MultiGroupApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.Extensions.Hosting" Version="8.0.0" />
  </ItemGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "MultiGroupApp",
				Framework: "net8.0",
				Packages: []DotNetPackage{
					{Name: "Microsoft.Extensions.Hosting", Version: "8.0.0"},
				},
			},
		},
		{
			name: "project with empty PackageReference",
			content: `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>TestApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="" Version="1.0.0" />
    <PackageReference Include="ValidPackage" Version="2.0.0" />
  </ItemGroup>
</Project>`,
			expected: DotNetProject{
				Name:      "TestApp",
				Framework: "net6.0",
				Packages: []DotNetPackage{
					{Name: "ValidPackage", Version: "2.0.0"}, // Empty Include should be skipped
				},
			},
		},
		{
			name:     "empty project file",
			content:  "",
			expected: DotNetProject{Name: "test", Framework: "", Packages: []DotNetPackage{}},
		},
		{
			name: "invalid XML",
			content: `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <AssemblyName>InvalidApp</AssemblyName>
  </PropertyGroup>
  <!-- Missing closing tag -->
</Project`,
			expected: DotNetProject{Name: "InvalidApp", Framework: "", Packages: []DotNetPackage{}}, // Can extract name from malformed XML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseCsproj(tt.content, "test.csproj")

			assert.Equal(t, tt.expected.Name, result.Name, "Should have correct project name")
			assert.Equal(t, tt.expected.Framework, result.Framework, "Should have correct framework")
			require.Len(t, result.Packages, len(tt.expected.Packages), "Should have correct number of packages")

			// Create maps for order-independent comparison
			expectedPackageMap := make(map[string]DotNetPackage)
			actualPackageMap := make(map[string]DotNetPackage)

			for _, pkg := range tt.expected.Packages {
				expectedPackageMap[pkg.Name] = pkg
			}

			for _, pkg := range result.Packages {
				actualPackageMap[pkg.Name] = pkg
			}

			// Verify all expected packages are present
			for name, expectedPkg := range expectedPackageMap {
				actualPkg, exists := actualPackageMap[name]
				require.True(t, exists, "Expected package %s not found", name)
				assert.Equal(t, expectedPkg.Version, actualPkg.Version, "Should have correct version for %s", name)
			}
		})
	}
}

func TestGetFrameworkType(t *testing.T) {
	parser := NewDotNetParser()

	tests := []struct {
		framework string
		expected  string
	}{
		{"net6.0", ".NET"},
		{"net7.0", ".NET"},
		{"net8.0", ".NET"},
		{"netstandard2.0", ".NET"},
		{"netcoreapp3.1", ".NET"},
		{"net48", ".NET Framework"},
		{"net472", ".NET Framework"},
		{"net4.8", ".NET Framework"},
		{"net4.7.1", ".NET Framework"},
		{"net40", ".NET Framework"},
		{"net35", ".NET Framework"},
		{"net20", ".NET Framework"},
		{"net11", ".NET Framework"},
		{"", ".NET"}, // Empty defaults to modern
	}

	for _, tt := range tests {
		t.Run(tt.framework, func(t *testing.T) {
			result := parser.GetFrameworkType(tt.framework)
			assert.Equal(t, tt.expected, result, "Should correctly identify framework type")
		})
	}
}

func TestIsModernFramework(t *testing.T) {
	parser := NewDotNetParser()

	modernFrameworks := []string{
		"net5.0", "net5.0-windows", "net6.0", "net6.0-android",
		"net7.0", "net7.0-ios", "net8.0", "net8.0-macos",
		"net9.0", "net9.0-tvos",
	}

	legacyFrameworks := []string{
		"net48", "net472", "net4.8", "net4.7.1", "net40",
		"net35", "net20", "net11", "netcoreapp3.1", "netstandard2.0",
	}

	for _, framework := range modernFrameworks {
		t.Run("modern_"+framework, func(t *testing.T) {
			assert.True(t, parser.IsModernFramework(framework), "Should identify %s as modern", framework)
		})
	}

	for _, framework := range legacyFrameworks {
		t.Run("legacy_"+framework, func(t *testing.T) {
			assert.False(t, parser.IsModernFramework(framework), "Should identify %s as not modern", framework)
		})
	}
}

func TestIsLegacyFramework(t *testing.T) {
	parser := NewDotNetParser()

	legacyFrameworks := []string{
		"net48", "net472", "net4.8", "net4.7.1", "net40",
		"net35", "net20", "net11",
	}

	modernFrameworks := []string{
		"net5.0", "net6.0", "net7.0", "net8.0", "net9.0",
		"netcoreapp3.1", "netstandard2.0",
	}

	for _, framework := range legacyFrameworks {
		t.Run("legacy_"+framework, func(t *testing.T) {
			assert.True(t, parser.IsLegacyFramework(framework), "Should identify %s as legacy", framework)
		})
	}

	for _, framework := range modernFrameworks {
		t.Run("modern_"+framework, func(t *testing.T) {
			assert.False(t, parser.IsLegacyFramework(framework), "Should identify %s as not legacy", framework)
		})
	}
}

func TestExtractProjectNameFromContent(t *testing.T) {
	parser := NewDotNetParser()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "extract AssemblyName from content",
			content:  `<AssemblyName>MyProject</AssemblyName>`,
			expected: "MyProject",
		},
		{
			name:     "extract AssemblyName with whitespace",
			content:  `<AssemblyName>  MyProject  </AssemblyName>`,
			expected: "MyProject",
		},
		{
			name:     "fallback to filename for .csproj content",
			content:  `<Project Reference="..\OtherProject.csproj" />`,
			expected: "test",
		},
		{
			name:     "fallback to filename for non-.csproj content",
			content:  `<SomeOtherXml></SomeOtherXml>`,
			expected: "test",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractProjectNameFromContent(tt.content, "test.csproj")
			assert.Equal(t, tt.expected, result, "Should extract correct project name")
		})
	}
}

func TestDotNetParser_Integration(t *testing.T) {
	parser := NewDotNetParser()

	// Test realistic modern ASP.NET Core project
	modernProject := `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <AssemblyName>AwesomeWebApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.OpenApi" Version="7.0.0" />
    <PackageReference Include="Swashbuckle.AspNetCore" Version="6.4.0" />
    <PackageReference Include="Microsoft.EntityFrameworkCore.SqlServer" Version="7.0.0" />
    <PackageReference Include="Microsoft.AspNetCore.Identity.EntityFrameworkCore" Version="7.0.0" />
  </ItemGroup>
  <ItemGroup>
    <ProjectReference Include="..\AwesomeWebApp.Core\AwesomeWebApp.Core.csproj" />
  </ItemGroup>
</Project>`

	result := parser.ParseCsproj(modernProject, "AwesomeWebApp.csproj")

	assert.Equal(t, "AwesomeWebApp", result.Name)
	assert.Equal(t, "net7.0", result.Framework)
	assert.Len(t, result.Packages, 4)

	// Verify framework type detection
	assert.Equal(t, ".NET", parser.GetFrameworkType(result.Framework))
	assert.True(t, parser.IsModernFramework(result.Framework))
	assert.False(t, parser.IsLegacyFramework(result.Framework))

	// Test realistic legacy .NET Framework project
	legacyProject := `<Project ToolsVersion="15.0" DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <PropertyGroup>
    <Configuration Condition=" '$(Configuration)' == '' ">Debug</Configuration>
    <Platform Condition=" '$(Platform)' == '' ">AnyCPU</Platform>
    <TargetFramework>net48</TargetFramework>
    <AssemblyName>LegacyEnterpriseApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="System.Web.Http" Version="5.2.7" />
    <PackageReference Include="EntityFramework" Version="6.4.4" />
    <PackageReference Include="Newtonsoft.Json" Version="12.0.3" />
  </ItemGroup>
</Project>`

	result = parser.ParseCsproj(legacyProject, "LegacyEnterpriseApp.csproj")

	assert.Equal(t, "LegacyEnterpriseApp", result.Name)
	assert.Equal(t, "net48", result.Framework)
	assert.Len(t, result.Packages, 3)

	// Verify framework type detection
	assert.Equal(t, ".NET Framework", parser.GetFrameworkType(result.Framework))
	assert.False(t, parser.IsModernFramework(result.Framework))
	assert.True(t, parser.IsLegacyFramework(result.Framework))
}

func TestDotNetParser_EdgeCases(t *testing.T) {
	parser := NewDotNetParser()

	// Test project with only ItemGroup and no PropertyGroup
	t.Run("no property group", func(t *testing.T) {
		content := `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <PackageReference Include="TestPackage" Version="1.0.0" />
  </ItemGroup>
</Project>`

		result := parser.ParseCsproj(content, "MinimalProject.csproj")
		assert.Equal(t, "MinimalProject", result.Name) // Extracted from filename
		assert.Equal(t, "", result.Framework)
		assert.Len(t, result.Packages, 1)
		assert.Equal(t, "TestPackage", result.Packages[0].Name)
	})

	// Test project with malformed XML but parseable parts
	t.Run("partial malformed XML", func(t *testing.T) {
		content := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="ValidPackage" Version="1.0.0" />
  </ItemGroup>
  <InvalidTag>
  </InvalidTag>`

		result := parser.ParseCsproj(content, "MalformedProject.csproj")
		// Malformed XML should cause parsing to fail and return empty values
		assert.Equal(t, "MalformedProject", result.Name) // Extracted from filename
		assert.Equal(t, "", result.Framework)            // No framework parsed
		assert.Len(t, result.Packages, 0)                // No packages parsed
	})

	// Test project with comments and whitespace
	t.Run("comments and whitespace", func(t *testing.T) {
		content := `<Project Sdk="Microsoft.NET.Sdk">
  <!-- This is a comment -->
  <PropertyGroup>
    <!-- Target framework comment -->
    <TargetFramework>net8.0</TargetFramework>
    <AssemblyName>CommentedApp</AssemblyName>
  </PropertyGroup>
  
  <!-- Packages section -->
  <ItemGroup>
    <PackageReference Include="Microsoft.Extensions.Hosting" Version="8.0.0" />
    <!-- Another package reference -->
    <PackageReference Include="Serilog" Version="3.0.1" />
  </ItemGroup>
</Project>`

		result := parser.ParseCsproj(content, "CommentedApp.csproj")
		assert.Equal(t, "CommentedApp", result.Name)
		assert.Equal(t, "net8.0", result.Framework)
		assert.Len(t, result.Packages, 2)
	})
}
