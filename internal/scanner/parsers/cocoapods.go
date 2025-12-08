package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// CocoaPodsParser handles CocoaPods-specific file parsing (Podfile, Podfile.lock)
type CocoaPodsParser struct{}

// NewCocoaPodsParser creates a new CocoaPods parser
func NewCocoaPodsParser() *CocoaPodsParser {
	return &CocoaPodsParser{}
}

// ParsePodfile parses Podfile and extracts pod dependencies with versions
// Matches patterns like: pod 'PodName', 'version' or pod "PodName", "version"
func (p *CocoaPodsParser) ParsePodfile(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	// Pattern for: pod 'name' or pod "name" (no version)
	depRegexNoVersion := regexp.MustCompile(`pod ['"]([^'"]+)['"]`)
	// Pattern for: pod 'name', 'version' or pod "name", "version"
	depRegexWithVersion := regexp.MustCompile(`pod ['"]([^'"]+)['"],\s*['"]([^'"]+)['"]`)

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Skip comments and empty lines
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// First try to match with version
		if match := depRegexWithVersion.FindStringSubmatch(line); match != nil {
			podName := match[1]
			version := match[2]

			dependencies = append(dependencies, types.Dependency{
				Type:    "cocoapods",
				Name:    podName,
				Example: version,
			})
			continue
		}

		// Then try to match without version
		if match := depRegexNoVersion.FindStringSubmatch(line); match != nil {
			podName := match[1]

			dependencies = append(dependencies, types.Dependency{
				Type:    "cocoapods",
				Name:    podName,
				Example: "latest",
			})
		}
	}

	return dependencies
}

// ParsePodfileLock parses Podfile.lock and extracts pod dependencies with versions
// Matches patterns in the PODS section like: - PodName (version)
func (p *CocoaPodsParser) ParsePodfileLock(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	lines := strings.Split(content, "\n")
	inPodsSection := false

	// Pattern for: - PodName (version) - main pod entries only
	depRegex := regexp.MustCompile(`^\s*-\s*([^:\s(]+)\s*\(([^)]+)\)`)

	for _, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)

		// Track when we're in the PODS section
		if line == "PODS:" {
			inPodsSection = true
			continue
		}
		if line == "DEPENDENCIES:" || line == "SPEC REPOS:" || line == "CHECKSUMS:" || line == "COCOAPODS:" {
			inPodsSection = false
			continue
		}

		if !inPodsSection {
			continue
		}

		// Match pod entries, but skip dependency lines (those with 4+ spaces indentation)
		if strings.HasPrefix(originalLine, "    -") {
			continue
		}
		if match := depRegex.FindStringSubmatch(line); match != nil {
			podName := match[1]
			version := match[2]

			dependencies = append(dependencies, types.Dependency{
				Type:    "cocoapods",
				Name:    podName,
				Example: version,
			})
		}
	}

	return dependencies
}

// ExtractDependencies extracts dependencies from either Podfile or Podfile.lock content
func (p *CocoaPodsParser) ExtractDependencies(content, filename string) []types.Dependency {
	if strings.HasSuffix(filename, "Podfile") {
		return p.ParsePodfile(content)
	}
	if strings.HasSuffix(filename, "Podfile.lock") {
		return p.ParsePodfileLock(content)
	}
	return []types.Dependency{}
}
