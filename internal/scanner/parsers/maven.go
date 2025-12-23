package parsers

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Pre-compiled regexes for performance
var (
	propertiesSectionRegex = regexp.MustCompile(`(?s)<properties>(.*?)</properties>`)
	propertyTagRegex       = regexp.MustCompile(`(?s)<([^>]+)>([^<]*)</([^>]+)>`)
	propertyRefRegex       = regexp.MustCompile(`\$\{([^}]+)\}`)
)

// MavenProject represents a parsed pom.xml structure
type MavenProject struct {
	XMLName              xml.Name                  `xml:"project"`
	GroupId              string                    `xml:"groupId"`
	ArtifactId           string                    `xml:"artifactId"`
	Version              string                    `xml:"version"`
	Parent               MavenParent               `xml:"parent"`
	Dependencies         MavenDependencies         `xml:"dependencies"`
	DependencyManagement MavenDependencyManagement `xml:"dependencyManagement"`
	Profiles             []MavenProfile            `xml:"profiles>profile"`
}

// MavenDependencies holds the list of dependencies
type MavenDependencies struct {
	Dependencies []MavenDependency `xml:"dependency"`
}

// MavenDependencyManagement holds the dependency management section
type MavenDependencyManagement struct {
	Dependencies []MavenDependency `xml:"dependencies>dependency"`
}

// MavenDependency represents a single Maven dependency
type MavenDependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope,omitempty"`
	Type       string `xml:"type,omitempty"`       // pom, jar, war, ear, etc.
	Classifier string `xml:"classifier,omitempty"` // sources, javadoc, etc.
	Optional   bool   `xml:"optional,omitempty"`
}

// MavenParent represents the parent POM reference
type MavenParent struct {
	GroupId      string `xml:"groupId"`
	ArtifactId   string `xml:"artifactId"`
	Version      string `xml:"version"`
	RelativePath string `xml:"relativePath"`
}

// MavenProfile represents a Maven build profile (aligned with deps.dev)
type MavenProfile struct {
	ID                   string                    `xml:"id"`
	Activation           MavenActivation           `xml:"activation"`
	Dependencies         MavenDependencies         `xml:"dependencies"`
	DependencyManagement MavenDependencyManagement `xml:"dependencyManagement"`
}

// MavenActivation represents profile activation conditions
type MavenActivation struct {
	ActiveByDefault string                  `xml:"activeByDefault"`
	JDK             string                  `xml:"jdk"`
	OS              MavenActivationOS       `xml:"os"`
	Property        MavenActivationProperty `xml:"property"`
	File            MavenActivationFile     `xml:"file"`
}

// MavenActivationOS represents OS-based activation
type MavenActivationOS struct {
	Name    string `xml:"name"`
	Family  string `xml:"family"`
	Arch    string `xml:"arch"`
	Version string `xml:"version"`
}

// MavenActivationProperty represents property-based activation
type MavenActivationProperty struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

// MavenActivationFile represents file-based activation
type MavenActivationFile struct {
	Exists  string `xml:"exists"`
	Missing string `xml:"missing"`
}

// MavenParser handles Maven-specific file parsing (pom.xml)
type MavenParser struct{}

// NewMavenParser creates a new Maven parser
func NewMavenParser() *MavenParser {
	return &MavenParser{}
}

// ExtractProjectInfo extracts groupId and artifactId from pom.xml
func (p *MavenParser) ExtractProjectInfo(content string) MavenProject {
	var project MavenProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return MavenProject{}
	}
	return project
}

// ParsePomXML parses pom.xml and extracts Maven dependencies with property resolution
// This is the simple version without parent POM resolution
func (p *MavenParser) ParsePomXML(content string) []types.Dependency {
	return p.ParsePomXMLWithProvider(content, "", nil)
}

// ParsePomXMLWithProvider parses pom.xml with parent POM resolution support
// If provider and pomDir are given, it will look up parent POMs to inherit properties
func (p *MavenParser) ParsePomXMLWithProvider(content string, pomDir string, provider types.Provider) []types.Dependency {
	var dependencies []types.Dependency

	// Parse the POM structure
	var project MavenProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return dependencies
	}

	// Build properties map: parent properties -> local properties -> project coordinates
	properties := make(map[string]string)

	// 1. Resolve parent properties (if provider available)
	if provider != nil && pomDir != "" {
		parentProps := p.resolveParentProperties(content, pomDir, provider, 0)
		mergeProperties(properties, parentProps)
	}

	// 2. Extract local properties (override parent)
	localProps := p.extractProperties(content)
	mergeProperties(properties, localProps)

	// 3. Add project coordinates (override all)
	p.addProjectCoordinates(properties, project.GroupId, project.ArtifactId, project.Version)

	// 4. Process profiles and merge active profiles (following deps.dev pattern)
	activeProfiles := p.getActiveProfiles(project.Profiles)
	for _, profile := range activeProfiles {
		// Merge profile dependencies
		for _, dep := range profile.Dependencies.Dependencies {
			if dep.GroupId != "" && dep.ArtifactId != "" {
				dependencies = append(dependencies, types.Dependency{
					Type:    "maven",
					Name:    dep.GroupId + ":" + dep.ArtifactId,
					Version: p.resolveVersion(dep.Version, properties),
					Scope:   mapMavenScope(dep.Scope),
				})
			}
		}
	}

	// Build dependencies list from <dependencies> section
	for _, dep := range project.Dependencies.Dependencies {
		if dep.GroupId != "" && dep.ArtifactId != "" {
			dependencies = append(dependencies, types.Dependency{
				Type:    "maven",
				Name:    dep.GroupId + ":" + dep.ArtifactId,
				Version: p.resolveVersion(dep.Version, properties),
				Scope:   mapMavenScope(dep.Scope),
			})
		}
	}

	// Process dependency management section (for BOM imports and version management)
	depMgmtDeps := p.parseDependencyManagement(project.DependencyManagement.Dependencies, properties)
	dependencies = append(dependencies, depMgmtDeps...)

	// Process profile dependency management
	for _, profile := range activeProfiles {
		profileDepMgmt := p.parseDependencyManagement(profile.DependencyManagement.Dependencies, properties)
		dependencies = append(dependencies, profileDepMgmt...)
	}

	return dependencies
}

// parseDependencyManagement processes dependency management section
// Following Maven semantics: only BOM imports (scope=import, type=pom) are actual dependencies
// Regular dependencyManagement entries are just for version management, not dependencies
func (p *MavenParser) parseDependencyManagement(deps []MavenDependency, properties map[string]string) []types.Dependency {
	var dependencies []types.Dependency

	for _, dep := range deps {
		if dep.GroupId != "" && dep.ArtifactId != "" {
			// Only include BOM imports (scope=import and type=pom)
			// Per Maven spec, BOM imports require both scope=import AND type=pom
			// If type is not specified, it defaults to "jar", not "pom"
			if dep.Scope == "import" && dep.Type == "pom" {
				dependencies = append(dependencies, types.Dependency{
					Type:    "maven",
					Name:    dep.GroupId + ":" + dep.ArtifactId,
					Version: p.resolveVersion(dep.Version, properties),
					Scope:   types.ScopeImport,
				})
			}
		}
	}

	return dependencies
}

// mapMavenScope maps Maven scope to our scope constants
func mapMavenScope(mavenScope string) string {
	switch mavenScope {
	case "test":
		return types.ScopeDev
	case "provided", "runtime":
		return types.ScopeProd
	case "system":
		return types.ScopeSystem
	case "import":
		return types.ScopeImport // BOM imports
	case "compile", "":
		return types.ScopeProd
	default:
		return types.ScopeProd
	}
}

// addProjectCoordinates adds project.* and pom.* properties for the given coordinates
func (p *MavenParser) addProjectCoordinates(properties map[string]string, groupId, artifactId, version string) {
	if groupId != "" {
		properties["project.groupId"] = groupId
		properties["pom.groupId"] = groupId
	}
	if artifactId != "" {
		properties["project.artifactId"] = artifactId
		properties["pom.artifactId"] = artifactId
	}
	if version != "" {
		properties["project.version"] = version
		properties["pom.version"] = version
	}
}

// mergeProperties copies all properties from src to dst
func mergeProperties(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

// extractProperties extracts Maven properties from pom.xml content using regex
func (p *MavenParser) extractProperties(content string) map[string]string {
	properties := make(map[string]string)

	propertiesMatch := propertiesSectionRegex.FindStringSubmatch(content)
	if len(propertiesMatch) < 2 {
		return properties
	}

	propertyMatches := propertyTagRegex.FindAllStringSubmatch(propertiesMatch[1], -1)
	for _, match := range propertyMatches {
		if len(match) >= 4 && match[1] == match[3] {
			propName := strings.TrimSpace(match[1])
			propValue := strings.TrimSpace(match[2])
			if propName != "" && propValue != "" {
				properties[propName] = propValue
			}
		}
	}

	return properties
}

// resolveVersion resolves Maven property references in version strings
func (p *MavenParser) resolveVersion(version string, properties map[string]string) string {
	if version == "" {
		return "latest"
	}
	return p.resolvePropertyRefs(version, properties, make(map[string]bool))
}

// resolvePropertyRefs resolves all ${...} references in a string, recursively with cycle detection
func (p *MavenParser) resolvePropertyRefs(value string, properties map[string]string, seen map[string]bool) string {
	if !strings.Contains(value, "${") {
		return value
	}

	return propertyRefRegex.ReplaceAllStringFunc(value, func(match string) string {
		propName := match[2 : len(match)-1]
		if seen[propName] {
			return match // Cycle detected, return unresolved
		}
		if resolved, ok := properties[propName]; ok {
			seen[propName] = true
			result := p.resolvePropertyRefs(resolved, properties, seen)
			delete(seen, propName)
			return result
		}
		return match // Property not found
	})
}

// resolveParentProperties recursively resolves properties from parent POMs
// depth prevents infinite recursion (Maven typically allows ~10 levels)
func (p *MavenParser) resolveParentProperties(content string, pomDir string, provider types.Provider, depth int) map[string]string {
	properties := make(map[string]string)

	if depth > 10 {
		return properties
	}

	// Parse to get parent reference
	var project MavenProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return properties
	}

	if project.Parent.GroupId == "" {
		return properties
	}

	// Resolve parent POM path
	parentPomPath := p.resolveParentPath(pomDir, project.Parent.RelativePath)

	// Read parent POM
	parentContent, err := provider.ReadFile(parentPomPath)
	if err != nil {
		return properties
	}

	// Recursively get grandparent properties first
	parentDir := filepath.Dir(parentPomPath)
	grandparentProps := p.resolveParentProperties(string(parentContent), parentDir, provider, depth+1)
	mergeProperties(properties, grandparentProps)

	// Extract parent's own properties (override grandparent)
	parentProps := p.extractProperties(string(parentContent))
	mergeProperties(properties, parentProps)

	// Parse parent project for coordinates
	var parentProject MavenProject
	if err := xml.Unmarshal(parentContent, &parentProject); err == nil {
		// Add parent.* properties
		if parentProject.GroupId != "" {
			properties["parent.groupId"] = parentProject.GroupId
		}
		if parentProject.ArtifactId != "" {
			properties["parent.artifactId"] = parentProject.ArtifactId
		}
		if parentProject.Version != "" {
			properties["parent.version"] = parentProject.Version
			// Set project.version from parent if not already set
			if _, exists := properties["project.version"]; !exists {
				properties["project.version"] = parentProject.Version
				properties["pom.version"] = parentProject.Version
			}
		}
	}

	return properties
}

// resolveParentPath determines the parent POM file path
func (p *MavenParser) resolveParentPath(pomDir, relativePath string) string {
	if relativePath == "" {
		relativePath = "../pom.xml" // Maven default
	} else if !strings.HasSuffix(relativePath, ".xml") {
		relativePath = filepath.Join(relativePath, "pom.xml")
	}
	return filepath.Clean(filepath.Join(pomDir, relativePath))
}

// getActiveProfiles returns profiles that should be activated
// Following deps.dev pattern: merge default profiles if no other profile is active
func (p *MavenParser) getActiveProfiles(profiles []MavenProfile) []MavenProfile {
	var activeProfiles []MavenProfile
	var defaultProfiles []MavenProfile

	for _, profile := range profiles {
		// Check if profile is active by default
		if strings.ToLower(strings.TrimSpace(profile.Activation.ActiveByDefault)) == "true" {
			defaultProfiles = append(defaultProfiles, profile)
		}

		// Check other activation conditions
		if p.isProfileActive(profile.Activation) {
			activeProfiles = append(activeProfiles, profile)
		}
	}

	// If no profiles are explicitly active, use default profiles
	if len(activeProfiles) == 0 {
		return defaultProfiles
	}

	return activeProfiles
}

// isProfileActive checks if a profile should be activated based on its activation conditions
// Following deps.dev pattern: simplified activation logic for static analysis
func (p *MavenParser) isProfileActive(activation MavenActivation) bool {
	// For static analysis, we use simplified activation logic
	// In deps.dev, they check JDK, OS, property, and file conditions
	// For our use case, we'll activate profiles with activeByDefault=true
	// and skip complex runtime conditions (JDK version, OS detection, etc.)

	// Property-based activation: check if property name is set (without value check)
	if activation.Property.Name != "" {
		// Simplified: treat any property-based activation as potentially active
		// In a full implementation, this would check against system properties
		return false // Conservative: don't activate property-based profiles without runtime info
	}

	// File-based activation: skip for static analysis
	if activation.File.Exists != "" || activation.File.Missing != "" {
		return false // Conservative: don't activate file-based profiles without filesystem check
	}

	// JDK and OS-based activation: skip for static analysis
	if activation.JDK != "" || activation.OS.Name != "" || activation.OS.Family != "" {
		return false // Conservative: don't activate environment-based profiles
	}

	return false
}
