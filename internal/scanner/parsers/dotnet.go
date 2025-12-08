package parsers

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"
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
	Name    string
	Version string
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
	Include string `xml:"Include,attr"`
	Version string `xml:"Version,attr"`
}

type ProjectReference struct {
	Include string `xml:"Include,attr"`
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
					Name:    pr.Include,
					Version: pr.Version,
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
					Name:    pr.Include,
					Version: pr.Version,
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
