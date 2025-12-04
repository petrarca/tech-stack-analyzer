package parsers

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MavenParser handles Maven-specific file parsing (pom.xml)
type MavenParser struct{}

// NewMavenParser creates a new Maven parser
func NewMavenParser() *MavenParser {
	return &MavenParser{}
}

// ParsePomXML parses pom.xml and extracts Maven dependencies with property resolution
func (p *MavenParser) ParsePomXML(content string) []types.Dependency {
	var dependencies []types.Dependency

	// First extract properties using regex since XML unmarshaling of dynamic tags is complex
	properties := p.extractProperties(content)

	type MavenDependency struct {
		GroupId    string `xml:"groupId"`
		ArtifactId string `xml:"artifactId"`
		Version    string `xml:"version"`
	}

	type MavenProject struct {
		XMLName      xml.Name `xml:"project"`
		Dependencies struct {
			Dependencies []MavenDependency `xml:"dependency"`
		} `xml:"dependencies"`
	}

	var mavenProject MavenProject
	if err := xml.Unmarshal([]byte(content), &mavenProject); err != nil {
		return dependencies
	}

	for _, dep := range mavenProject.Dependencies.Dependencies {
		if dep.GroupId != "" && dep.ArtifactId != "" {
			dependencyName := dep.GroupId + ":" + dep.ArtifactId
			version := p.resolveVersion(dep.Version, properties)

			dependencies = append(dependencies, types.Dependency{
				Type:    "maven",
				Name:    dependencyName,
				Example: version,
			})
		}
	}

	return dependencies
}

// extractProperties extracts Maven properties from pom.xml content using regex
func (p *MavenParser) extractProperties(content string) map[string]string {
	properties := make(map[string]string)

	// Find properties section content with DOTALL flag to handle multiline
	propertiesRegex := regexp.MustCompile(`(?s)<properties>(.*?)</properties>`)
	propertiesMatch := propertiesRegex.FindStringSubmatch(content)

	if len(propertiesMatch) < 2 {
		return properties
	}

	propertiesContent := propertiesMatch[1]

	// Extract individual properties using regex that captures opening tag, content, and closing tag
	// This matches: <property.name>value</property.name>
	propertyRegex := regexp.MustCompile(`(?s)<([^>]+)>([^<]*)</([^>]+)>`)
	propertyMatches := propertyRegex.FindAllStringSubmatch(propertiesContent, -1)

	for _, match := range propertyMatches {
		if len(match) >= 4 && match[1] == match[3] { // Ensure opening and closing tags match
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

	// Check if version contains a property reference like ${property.name}
	if strings.HasPrefix(version, "${") && strings.HasSuffix(version, "}") {
		// Extract property name: ${property.name} -> property.name
		propName := version[2 : len(version)-1]
		if resolved, exists := properties[propName]; exists {
			return resolved
		}
		// Property not found, return original as fallback
		return version
	}

	// No property reference, return as-is
	return version
}
