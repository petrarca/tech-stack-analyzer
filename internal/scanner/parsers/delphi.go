package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

var (
	delphiFrameworkRegex = regexp.MustCompile(`<FrameworkType>([^<]+)</FrameworkType>`)
	delphiPackageRegex   = regexp.MustCompile(`<DCC_UsePackage>([^<]+)</DCC_UsePackage>`)
)

// DelphiParser handles Delphi project file parsing (.dproj)
type DelphiParser struct{}

// DelphiProject represents a parsed Delphi project
type DelphiProject struct {
	Name      string
	Framework string   // VCL or FMX
	Packages  []string // DCC_UsePackage entries
}

// NewDelphiParser creates a new DelphiParser instance
func NewDelphiParser() *DelphiParser {
	return &DelphiParser{}
}

// ParseDproj parses a .dproj file and extracts project information
func (p *DelphiParser) ParseDproj(content, filename string) DelphiProject {
	project := DelphiProject{
		Name:     p.extractProjectName(filename),
		Packages: []string{},
	}

	// Extract FrameworkType (VCL or FMX)
	project.Framework = p.extractFrameworkType(content)

	// Extract packages from DCC_UsePackage
	project.Packages = p.extractPackages(content)

	return project
}

// extractProjectName extracts project name from filename
func (p *DelphiParser) extractProjectName(filename string) string {
	// Remove .dproj extension
	name := strings.TrimSuffix(filename, ".dproj")
	return name
}

// extractFrameworkType extracts VCL or FMX from FrameworkType element
func (p *DelphiParser) extractFrameworkType(content string) string {
	// Match <FrameworkType>VCL</FrameworkType> or <FrameworkType>FMX</FrameworkType>
	if matches := delphiFrameworkRegex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractPackages extracts package names from DCC_UsePackage element
func (p *DelphiParser) extractPackages(content string) []string {
	packages := make([]string, 0)
	seen := make(map[string]bool)

	// Match <DCC_UsePackage>pkg1;pkg2;pkg3;$(DCC_UsePackage)</DCC_UsePackage>
	matches := delphiPackageRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			// Split by semicolon
			pkgList := strings.Split(match[1], ";")
			for _, pkg := range pkgList {
				pkg = strings.TrimSpace(pkg)
				// Skip empty, variables like $(DCC_UsePackage), and duplicates
				if pkg == "" || strings.HasPrefix(pkg, "$") || seen[pkg] {
					continue
				}
				seen[pkg] = true
				packages = append(packages, pkg)
			}
		}
	}

	return packages
}

// IsVCL checks if the project uses VCL framework
func (p *DelphiParser) IsVCL(framework string) bool {
	return strings.EqualFold(framework, "VCL")
}

// IsFMX checks if the project uses FireMonkey (FMX) framework
func (p *DelphiParser) IsFMX(framework string) bool {
	return strings.EqualFold(framework, "FMX")
}

// CreateDependencies creates dependency objects from Delphi project packages
func (p *DelphiParser) CreateDependencies(project DelphiProject) []types.Dependency {
	dependencies := make([]types.Dependency, 0, len(project.Packages))

	for _, pkg := range project.Packages {
		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeDelphi,
			Name:     pkg,
			Version:  "",
			Scope:    types.ScopeProd,
			Direct:   true,
			Metadata: types.NewMetadata(MetadataSourceDproj),
		})
	}

	return dependencies
}
