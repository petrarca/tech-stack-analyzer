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
	Name              string
	PackageId         string // NuGet package ID (defaults to Name/AssemblyName if not specified)
	Framework         string
	License           string // SPDX license expression from PackageLicenseExpression
	LicenseUrl        string // Deprecated PackageLicenseUrl (fallback if no expression)
	LicenseFile       string // PackageLicenseFile path
	Packages          []DotNetPackage
	ProjectReferences []string // Paths to referenced projects
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
	TargetFramework          string `xml:"TargetFramework"`
	TargetFrameworks         string `xml:"TargetFrameworks"` // multi-targeting: semicolon-separated list
	AssemblyName             string `xml:"AssemblyName"`
	PackageId                string `xml:"PackageId"`
	PackageLicenseExpression string `xml:"PackageLicenseExpression"`
	PackageLicenseUrl        string `xml:"PackageLicenseUrl"`
	PackageLicenseFile       string `xml:"PackageLicenseFile"`
	// Properties captures every child element of the PropertyGroup as a
	// name -> value map, so arbitrary MSBuild properties (e.g.
	// <NewtonsoftVersion>13.0.3</NewtonsoftVersion>) can resolve $(name)
	// references in package versions.
	Properties []msbuildProperty `xml:",any"`
}

// msbuildProperty is a single child element of a PropertyGroup captured by name.
type msbuildProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type ItemGroup struct {
	PackageReferences       []PackageReference `xml:"PackageReference"`
	PackageVersions         []PackageVersion   `xml:"PackageVersion"`         // Central Package Management (Directory.Packages.props)
	GlobalPackageReferences []PackageVersion   `xml:"GlobalPackageReference"` // CPM packages applied to every project
	ProjectReferences       []ProjectReference `xml:"ProjectReference"`
}

type PackageReference struct {
	Include         string `xml:"Include,attr"`
	Version         string `xml:"Version,attr"`
	VersionElement  string `xml:"Version"`              // Child <Version> element form (older PackageReference style)
	VersionOverride string `xml:"VersionOverride,attr"` // CPM per-reference override of the central version
	Condition       string `xml:"Condition,attr"`       // For conditional references (e.g., Debug/Release)
	PrivateAssets   string `xml:"PrivateAssets,attr"`   // For build-time only dependencies
	IncludeAssets   string `xml:"IncludeAssets,attr"`   // Asset inclusion control
	ExcludeAssets   string `xml:"ExcludeAssets,attr"`   // Asset exclusion control
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

	for _, pg := range project.PropertyGroups {
		applyProjectProperties(&result, pg)
		// License metadata is modern SDK-style only.
		if pg.PackageLicenseExpression != "" {
			result.License = pg.PackageLicenseExpression
		}
		if pg.PackageLicenseUrl != "" {
			result.LicenseUrl = pg.PackageLicenseUrl
		}
		if pg.PackageLicenseFile != "" {
			result.LicenseFile = pg.PackageLicenseFile
		}
	}

	p.extractItemGroups(&result, project.ItemGroups)
	p.finalizeProject(&result, project.PropertyGroups, content, filePath)
	return result
}

// parseLegacyProject parses legacy .NET Framework .csproj files
func (p *DotNetParser) parseLegacyProject(project LegacyProject, content, filePath string) DotNetProject {
	var result DotNetProject

	for _, pg := range project.PropertyGroups {
		applyProjectProperties(&result, pg)
	}

	p.extractItemGroups(&result, project.ItemGroups)
	p.finalizeProject(&result, project.PropertyGroups, content, filePath)
	return result
}

// applyProjectProperties copies the project identity/framework fields shared by
// modern and legacy projects from a PropertyGroup onto the result.
func applyProjectProperties(result *DotNetProject, pg PropertyGroup) {
	if pg.AssemblyName != "" {
		result.Name = pg.AssemblyName
	}
	if pg.PackageId != "" {
		result.PackageId = pg.PackageId
	}
	if pg.TargetFramework != "" {
		result.Framework = pg.TargetFramework
	} else if pg.TargetFrameworks != "" && result.Framework == "" {
		// Multi-targeting project: record the framework list as-is.
		result.Framework = pg.TargetFrameworks
	}
}

// extractItemGroups collects PackageReferences and ProjectReferences from the
// project's ItemGroups onto the result. Shared by modern and legacy parsing.
func (p *DotNetParser) extractItemGroups(result *DotNetProject, itemGroups []ItemGroup) {
	for _, ig := range itemGroups {
		for _, pr := range ig.PackageReferences {
			if pr.Include != "" {
				result.Packages = append(result.Packages, DotNetPackage{
					Name:     pr.Include,
					Version:  packageRefVersion(pr),
					Scope:    p.determineNuGetScope(pr),
					Metadata: p.buildNuGetMetadata(pr),
				})
			}
		}
		for _, projRef := range ig.ProjectReferences {
			if projRef.Include != "" {
				result.ProjectReferences = append(result.ProjectReferences, projRef.Include)
			}
		}
	}
}

// finalizeProject resolves $(property) references in package versions and
// applies the name/PackageId fallbacks shared by modern and legacy parsing.
func (p *DotNetParser) finalizeProject(result *DotNetProject, propertyGroups []PropertyGroup, content, filePath string) {
	// Resolve $(property) references in package versions using the project's
	// MSBuild properties (e.g. <PackageReference Version="$(JsonVersion)" />).
	applyPropertyResolution(result.Packages, collectMSBuildProperties(propertyGroups))

	if result.Name == "" {
		result.Name = p.extractProjectNameFromContent(content, filePath)
	}
	if result.PackageId == "" {
		result.PackageId = result.Name
	}
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
			Type:     DependencyTypeNuget,
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
		// Central Package Management uses <PackageVersion> elements.
		for _, pv := range ig.PackageVersions {
			if pv.Include != "" && pv.Version != "" {
				packageVersions[pv.Include] = pv.Version
			}
		}
		// GlobalPackageReference declares a CPM package applied to every project.
		for _, gpr := range ig.GlobalPackageReferences {
			if gpr.Include != "" && gpr.Version != "" {
				packageVersions[gpr.Include] = gpr.Version
			}
		}
		// Tolerate files that (incorrectly) use <PackageReference Version="...">.
		for _, pr := range ig.PackageReferences {
			if pr.Include != "" && pr.Version != "" {
				packageVersions[pr.Include] = pr.Version
			}
		}
	}

	return packageVersions
}

// packageRefVersion returns the effective version of a PackageReference, in
// precedence order: the CPM per-reference VersionOverride, the Version
// attribute, then the child <Version> element (an older but valid
// PackageReference style). An empty result means the version is centrally
// managed (Directory.Packages.props) and must be backfilled by the detector.
func packageRefVersion(pr PackageReference) string {
	switch {
	case pr.VersionOverride != "":
		return pr.VersionOverride
	case pr.Version != "":
		return pr.Version
	default:
		return strings.TrimSpace(pr.VersionElement)
	}
}

// msbuildPropertyRef matches an MSBuild property reference such as $(MyVersion).
var msbuildPropertyRef = regexp.MustCompile(`\$\(([^)]+)\)`)

// collectMSBuildProperties builds a property name -> value map from all
// PropertyGroups. MSBuild property names are case-insensitive, so keys are
// lowercased. Later definitions win (MSBuild evaluates top to bottom).
//
// Note: Go's encoding/xml routes elements that match a named struct field to
// that field, not to the catch-all ",any" slice. The named fields on
// PropertyGroup (AssemblyName, TargetFramework, etc.) are therefore absent
// from pg.Properties. To make them available for $(name) substitution they
// are added explicitly below, using the same lowercased key convention.
// Explicit user-defined properties in pg.Properties always win over the named
// fields (the loop over pg.Properties runs first), so there is no risk of a
// built-in field silently overriding a developer-defined property of the same
// name.
func collectMSBuildProperties(groups []PropertyGroup) map[string]string {
	props := make(map[string]string)
	for _, pg := range groups {
		// Catch-all properties (arbitrary developer-defined elements).
		for _, p := range pg.Properties {
			name := strings.ToLower(strings.TrimSpace(p.XMLName.Local))
			if name == "" {
				continue
			}
			props[name] = strings.TrimSpace(p.Value)
		}
		// Named struct fields that the XML decoder consumed before the catch-all
		// had a chance to see them. Only non-empty values are added, and they
		// must not overwrite an already-set property (developer definitions win).
		for key, val := range namedPropertyGroupFields(pg) {
			if val != "" {
				if _, exists := props[key]; !exists {
					props[key] = val
				}
			}
		}
	}
	return props
}

// namedPropertyGroupFields returns the lowercased MSBuild property name ->
// value pairs for the fields that are explicitly modelled on PropertyGroup.
// These are not reachable via the ",any" catch-all in encoding/xml.
func namedPropertyGroupFields(pg PropertyGroup) map[string]string {
	return map[string]string{
		"targetframework":          pg.TargetFramework,
		"targetframeworks":         pg.TargetFrameworks,
		"assemblyname":             pg.AssemblyName,
		"packageid":                pg.PackageId, //nolint:misspell // "packageid" is the lowercased MSBuild property name PackageId, not a misspelling
		"packagelicenseexpression": pg.PackageLicenseExpression,
		"packagelicenseurl":        pg.PackageLicenseUrl,
		"packagelicensefile":       pg.PackageLicenseFile,
	}
}

// resolveMSBuildProperties substitutes $(Name) references in value using the
// property map (built by collectMSBuildProperties). It resolves transitively
// (a property whose value references another property) with a bounded depth to
// avoid cycles. An unresolved reference is left as-is so the version stays
// detectably unresolved.
func resolveMSBuildProperties(value string, props map[string]string) string {
	if !strings.Contains(value, "$(") {
		return value
	}
	const maxDepth = 10
	for i := 0; i < maxDepth && strings.Contains(value, "$("); i++ {
		replaced := msbuildPropertyRef.ReplaceAllStringFunc(value, func(ref string) string {
			m := msbuildPropertyRef.FindStringSubmatch(ref)
			if len(m) < 2 {
				return ref
			}
			if v, ok := props[strings.ToLower(strings.TrimSpace(m[1]))]; ok {
				return v
			}
			return ref // unknown property: leave the reference intact
		})
		if replaced == value {
			break // no further substitution possible
		}
		value = replaced
	}
	return value
}

// applyPropertyResolution resolves $(property) references in every package
// version using the project's MSBuild properties. Called for both modern and
// legacy projects after packages are collected.
func applyPropertyResolution(packages []DotNetPackage, props map[string]string) {
	if len(props) == 0 {
		return
	}
	for i := range packages {
		if packages[i].Version == "" {
			continue
		}
		packages[i].Version = resolveMSBuildProperties(packages[i].Version, props)
	}
}
