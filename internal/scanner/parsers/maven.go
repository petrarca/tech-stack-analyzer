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
	XMLName      xml.Name          `xml:"project"`
	GroupId      string            `xml:"groupId"`
	ArtifactId   string            `xml:"artifactId"`
	Version      string            `xml:"version"`
	Parent       MavenParent       `xml:"parent"`
	Dependencies MavenDependencies `xml:"dependencies"`
}

// MavenDependencies holds the list of dependencies
type MavenDependencies struct {
	Dependencies []MavenDependency `xml:"dependency"`
}

// MavenDependency represents a single Maven dependency
type MavenDependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

// MavenParent represents the parent POM reference
type MavenParent struct {
	GroupId      string `xml:"groupId"`
	ArtifactId   string `xml:"artifactId"`
	Version      string `xml:"version"`
	RelativePath string `xml:"relativePath"`
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

	// Build dependencies list
	for _, dep := range project.Dependencies.Dependencies {
		if dep.GroupId != "" && dep.ArtifactId != "" {
			dependencies = append(dependencies, types.Dependency{
				Type:    "maven",
				Name:    dep.GroupId + ":" + dep.ArtifactId,
				Example: p.resolveVersion(dep.Version, properties),
			})
		}
	}

	return dependencies
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
