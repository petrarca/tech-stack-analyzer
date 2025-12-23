package parsers

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DotNetParser handles .NET project file parsing (.csproj)
type DotNetParser struct{}

// DotNetProject represents a parsed .NET project
type DotNetProject struct {
	Name      string
	Framework string
	Packages  []DotNetPackage
}

// DotNetPackage represents a NuGet package reference
type DotNetPackage struct {
	Name     string
	Version  string
	Scope    string                 // prod, dev, build
	Metadata map[string]interface{} // Additional package metadata
}

// XML structures for parsing .csproj files
type Project struct {
	XMLName        xml.Name        `xml:"Project"`
	Sdk            string          `xml:"Sdk,attr"`
	PropertyGroups []PropertyGroup `xml:"PropertyGroup"`
	ItemGroups     []ItemGroup     `xml:"ItemGroup"`
}

type PropertyGroup struct {
	TargetFramework string `xml:"TargetFramework"`
	AssemblyName    string `xml:"AssemblyName"`
}

type ItemGroup struct {
	PackageReferences []PackageReference `xml:"PackageReference"`
	ProjectReferences []ProjectReference `xml:"ProjectReference"`
}

type PackageReference struct {
	Include       string `xml:"Include,attr"`
	Version       string `xml:"Version,attr"`
	Condition     string `xml:"Condition,attr"`     // For conditional references (e.g., Debug/Release)
	PrivateAssets string `xml:"PrivateAssets,attr"` // For build-time only dependencies
	IncludeAssets string `xml:"IncludeAssets,attr"` // Asset inclusion control
	ExcludeAssets string `xml:"ExcludeAssets,attr"` // Asset exclusion control
}

type ProjectReference struct {
	Include string `xml:"Include,attr"`
}

// PackagesConfig represents packages.config file structure (legacy .NET Framework)
type PackagesConfig struct {
	XMLName  xml.Name  `xml:"packages"`
	Packages []Package `xml:"package"`
}

type Package struct {
	ID                    string `xml:"id,attr"`
	Version               string `xml:"version,attr"`
	TargetFramework       string `xml:"targetFramework,attr"`
	DevelopmentDependency string `xml:"developmentDependency,attr"`
}

// DirectoryPackagesProps represents Directory.Packages.props file structure (central package management)
type DirectoryPackagesProps struct {
	XMLName    xml.Name    `xml:"Project"`
	ItemGroups []ItemGroup `xml:"ItemGroup"`
}

type PackageVersion struct {
	Include string `xml:"Include,attr"`
	Version string `xml:"Version,attr"`
}

// Legacy .NET Framework project structures
type LegacyProject struct {
	XMLName        xml.Name        `xml:"Project"`
	ToolsVersion   string          `xml:"ToolsVersion,attr"`
	DefaultTargets string          `xml:"DefaultTargets,attr"`
	PropertyGroups []PropertyGroup `xml:"PropertyGroup"`
	ItemGroups     []ItemGroup     `xml:"ItemGroup"`
}

// NewDotNetParser creates a new DotNetParser instance
func NewDotNetParser() *DotNetParser {
	return &DotNetParser{}
}

// ParseCsproj parses a .csproj file and extracts project information
func (p *DotNetParser) ParseCsproj(content, filePath string) DotNetProject {
	var project DotNetProject

	// Try to parse as modern SDK-style project first
	var modernProject Project
	if err := xml.Unmarshal([]byte(content), &modernProject); err == nil {
		project = p.parseModernProject(modernProject, content, filePath)
	} else {
		// Try to parse as legacy .NET Framework project
		var legacyProject LegacyProject
		if err := xml.Unmarshal([]byte(content), &legacyProject); err == nil {
			project = p.parseLegacyProject(legacyProject, content, filePath)
		} else {
			// Both parsing attempts failed, return fallback project
			project = DotNetProject{
				Name:      p.extractProjectNameFromContent(content, filePath),
				Framework: "",
				Packages:  []DotNetPackage{},
			}
		}
	}

	return project
}

// parseModernProject parses modern SDK-style .csproj files
func (p *DotNetParser) parseModernProject(project Project, content, filePath string) DotNetProject {
	var result DotNetProject

	// Extract project name and framework from PropertyGroups
	for _, pg := range project.PropertyGroups {
		if pg.AssemblyName != "" {
			result.Name = pg.AssemblyName
		}
		if pg.TargetFramework != "" {
			result.Framework = pg.TargetFramework
		}
	}

	// Extract packages from ItemGroups
	for _, ig := range project.ItemGroups {
		for _, pr := range ig.PackageReferences {
			if pr.Include != "" {
				result.Packages = append(result.Packages, DotNetPackage{
					Name:     pr.Include,
					Version:  pr.Version,
					Scope:    p.determineNuGetScope(pr),
					Metadata: p.buildNuGetMetadata(pr),
				})
			}
		}
	}

	// If no AssemblyName found, try to extract from filename using regex
	if result.Name == "" {
		result.Name = p.extractProjectNameFromContent(content, filePath)
	}

	return result
}

// parseLegacyProject parses legacy .NET Framework .csproj files
func (p *DotNetParser) parseLegacyProject(project LegacyProject, content, filePath string) DotNetProject {
	var result DotNetProject

	// Extract project name and framework from PropertyGroups
	for _, pg := range project.PropertyGroups {
		if pg.AssemblyName != "" {
			result.Name = pg.AssemblyName
		}
		if pg.TargetFramework != "" {
			result.Framework = pg.TargetFramework
		}
	}

	// Extract packages from ItemGroups
	for _, ig := range project.ItemGroups {
		for _, pr := range ig.PackageReferences {
			if pr.Include != "" {
				result.Packages = append(result.Packages, DotNetPackage{
					Name:     pr.Include,
					Version:  pr.Version,
					Scope:    p.determineNuGetScope(pr),
					Metadata: p.buildNuGetMetadata(pr),
				})
			}
		}
	}

	// If no AssemblyName found, try to extract from filename using regex
	if result.Name == "" {
		result.Name = p.extractProjectNameFromContent(content, filePath)
	}

	return result
}

// extractProjectNameFromContent attempts to extract project name from XML content
func (p *DotNetParser) extractProjectNameFromContent(content, filePath string) string {
	// Try to match AssemblyName in content (might be in different format)
	assemblyNameRegex := regexp.MustCompile(`<AssemblyName>([^<]+)</AssemblyName>`)
	if matches := assemblyNameRegex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Extract from filename as fallback
	filename := filepath.Base(filePath)
	projectName := strings.TrimSuffix(filename, ".csproj")
	return projectName
}

// GetFrameworkType determines if the framework is modern .NET or legacy .NET Framework
func (p *DotNetParser) GetFrameworkType(framework string) string {
	if strings.HasPrefix(framework, "net4") || strings.HasPrefix(framework, "net3") || strings.HasPrefix(framework, "net2") || strings.HasPrefix(framework, "net1") {
		return ".NET Framework"
	}
	return ".NET"
}

// IsModernFramework checks if the target framework is modern .NET (5+)
func (p *DotNetParser) IsModernFramework(framework string) bool {
	return strings.HasPrefix(framework, "net5") ||
		strings.HasPrefix(framework, "net6") ||
		strings.HasPrefix(framework, "net7") ||
		strings.HasPrefix(framework, "net8") ||
		strings.HasPrefix(framework, "net9")
}

// IsLegacyFramework checks if the target framework is legacy .NET Framework
func (p *DotNetParser) IsLegacyFramework(framework string) bool {
	return strings.HasPrefix(framework, "net4") ||
		strings.HasPrefix(framework, "net3") ||
		strings.HasPrefix(framework, "net2") ||
		strings.HasPrefix(framework, "net1")
}

// determineNuGetScope determines the scope of a NuGet package based on its attributes
// Aligned with deps.dev patterns: regular (prod), dev, test, build
func (p *DotNetParser) determineNuGetScope(pr PackageReference) string {
	// Check for build-time only dependencies (PrivateAssets="All")
	if pr.PrivateAssets == "All" || pr.PrivateAssets == "all" {
		return types.ScopeBuild
	}

	// Check condition for Debug/Test configurations
	condition := strings.ToLower(pr.Condition)
	if strings.Contains(condition, "debug") || strings.Contains(condition, "test") {
		return types.ScopeDev
	}

	// Default to prod (regular dependency)
	return types.ScopeProd
}

// buildNuGetMetadata creates metadata map for NuGet packages aligned with deps.dev
func (p *DotNetParser) buildNuGetMetadata(pr PackageReference) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add source file
	metadata["source"] = ".csproj"

	// Add PrivateAssets if set (build-time only)
	if pr.PrivateAssets != "" {
		metadata["private_assets"] = pr.PrivateAssets
	}

	// Add condition if set (conditional reference)
	if pr.Condition != "" {
		metadata["condition"] = pr.Condition
	}

	return metadata
}

// ParsePackagesConfig parses packages.config file and returns dependencies
func (p *DotNetParser) ParsePackagesConfig(content string) []types.Dependency {
	var dependencies []types.Dependency
	var packagesConfig PackagesConfig

	if err := xml.Unmarshal([]byte(content), &packagesConfig); err != nil {
		return dependencies
	}

	for _, pkg := range packagesConfig.Packages {
		if pkg.ID == "" {
			continue
		}

		// Determine scope based on developmentDependency attribute
		scope := types.ScopeProd
		if pkg.DevelopmentDependency == "true" {
			scope = types.ScopeDev
		}

		metadata := make(map[string]interface{})
		metadata["source"] = "packages.config"
		if pkg.TargetFramework != "" {
			metadata["target_framework"] = pkg.TargetFramework
		}

		dependencies = append(dependencies, types.Dependency{
			Type:     "nuget",
			Name:     pkg.ID,
			Version:  pkg.Version,
			Scope:    scope,
			Direct:   true,
			Metadata: metadata,
		})
	}

	return dependencies
}

// ParseDirectoryPackagesProps parses Directory.Packages.props file and returns package versions
func (p *DotNetParser) ParseDirectoryPackagesProps(content string) map[string]string {
	packageVersions := make(map[string]string)
	var dirPackages DirectoryPackagesProps

	if err := xml.Unmarshal([]byte(content), &dirPackages); err != nil {
		return packageVersions
	}

	for _, ig := range dirPackages.ItemGroups {
		for _, pv := range ig.PackageReferences {
			if pv.Include != "" && pv.Version != "" {
				packageVersions[pv.Include] = pv.Version
			}
		}
	}

	return packageVersions
}
