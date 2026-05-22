package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Pre-compiled regexes for CocoaPods file parsing
var (
	// Podfile: pod 'Name', 'version' or pod "Name", "version"
	podfileDepWithVersion = regexp.MustCompile(`pod ['"]([^'"]+)['"],\s*['"]([^'"]+)['"]`)
	// Podfile: pod 'Name' or pod "Name" (no version)
	podfileDepNoVersion = regexp.MustCompile(`pod ['"]([^'"]+)['"]`)

	// Podfile.lock: - PodName (version)
	podfileLockDep = regexp.MustCompile(`^\s*-\s*([^:\s(]+)\s*\(([^)]+)\)`)

	// .podspec: s.dependency "Name", ">= version" or spec.dependency 'Name', '~> version'
	podspecDepWithVersion = regexp.MustCompile(`\.dependency\s+['"]([^'"]+)['"],\s*['"]([^'"]+)['"]`)
	// .podspec: s.dependency "Name" (no version)
	podspecDepNoVersion = regexp.MustCompile(`\.dependency\s+['"]([^'"]+)['"]`)
)

// CocoaPodsParser handles CocoaPods-specific file parsing (Podfile, Podfile.lock, .podspec)
type CocoaPodsParser struct{}

// NewCocoaPodsParser creates a new CocoaPods parser
func NewCocoaPodsParser() *CocoaPodsParser {
	return &CocoaPodsParser{}
}

// ParsePodfile parses Podfile and extracts pod dependencies with versions
// Matches patterns like: pod 'PodName', 'version' or pod "PodName", "version"
func (p *CocoaPodsParser) ParsePodfile(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		if match := podfileDepWithVersion.FindStringSubmatch(line); match != nil {
			dependencies = append(dependencies, types.Dependency{
				Type:    DependencyTypeCocoapods,
				Name:    match[1],
				Version: match[2],
			})
			continue
		}

		if match := podfileDepNoVersion.FindStringSubmatch(line); match != nil {
			dependencies = append(dependencies, types.Dependency{
				Type:    DependencyTypeCocoapods,
				Name:    match[1],
				Version: "latest",
			})
		}
	}

	return dependencies
}

// ParsePodfileLock parses Podfile.lock and extracts pod dependencies with versions
// Matches patterns in the PODS section like: - PodName (version)
func (p *CocoaPodsParser) ParsePodfileLock(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)
	inPodsSection := false

	for _, line := range strings.Split(content, "\n") {
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
		if match := podfileLockDep.FindStringSubmatch(line); match != nil {
			podName := match[1]
			version := match[2]

			dependencies = append(dependencies, types.Dependency{
				Type:    DependencyTypeCocoapods,
				Name:    podName,
				Version: version,
			})
		}
	}

	return dependencies
}

// ParsePodspec parses .podspec files and extracts dependency declarations.
// Matches patterns like: s.dependency "Name", ">= version" or spec.dependency 'Name'
func (p *CocoaPodsParser) ParsePodspec(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		if match := podspecDepWithVersion.FindStringSubmatch(line); match != nil {
			dependencies = append(dependencies, types.Dependency{
				Type:    DependencyTypeCocoapods,
				Name:    match[1],
				Version: match[2],
			})
			continue
		}

		if match := podspecDepNoVersion.FindStringSubmatch(line); match != nil {
			dependencies = append(dependencies, types.Dependency{
				Type:    DependencyTypeCocoapods,
				Name:    match[1],
				Version: "latest",
			})
		}
	}

	return dependencies
}

// ExtractDependencies extracts dependencies from Podfile, Podfile.lock, or .podspec content
func (p *CocoaPodsParser) ExtractDependencies(content, filename string) []types.Dependency {
	if strings.HasSuffix(filename, "Podfile") {
		return p.ParsePodfile(content)
	}
	if strings.HasSuffix(filename, "Podfile.lock") {
		return p.ParsePodfileLock(content)
	}
	if strings.HasSuffix(filename, ".podspec") {
		return p.ParsePodspec(content)
	}
	return []types.Dependency{}
}
