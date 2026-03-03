package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Pre-compiled regexes for Gemfile.lock parsing
var (
	gemLockSpecRegex = regexp.MustCompile(`^\s{4}(\S+)\s+\(([^)]+)\)`)
)

// GemfileLockParser handles Gemfile.lock parsing
type GemfileLockParser struct{}

// NewGemfileLockParser creates a new Gemfile.lock parser
func NewGemfileLockParser() *GemfileLockParser {
	return &GemfileLockParser{}
}

// ParseGemfileLockOptions contains configuration options for ParseGemfileLock
type ParseGemfileLockOptions struct {
	IncludeTransitive bool // Include transitive dependencies (default: false for backward compatibility)
}

// ParseGemfileLock parses Gemfile.lock and extracts exact gem versions
// By default, only returns direct dependencies. Use ParseGemfileLockWithOptions to include transitive dependencies.
func (p *GemfileLockParser) ParseGemfileLock(content string) []types.Dependency {
	return p.ParseGemfileLockWithOptions(content, ParseGemfileLockOptions{IncludeTransitive: false})
}

// ParseGemfileLockWithOptions parses Gemfile.lock with configurable options
func (p *GemfileLockParser) ParseGemfileLockWithOptions(content string, options ParseGemfileLockOptions) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	lines := strings.Split(content, "\n")

	// Parse DEPENDENCIES section to identify direct dependencies
	directDeps := p.parseDirectDependencies(lines)

	// Parse GEM specs section to get all dependencies with exact versions
	inGemSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect GEM section
		if trimmedLine == "GEM" {
			inGemSection = true
			continue
		}

		// Exit GEM section when we hit PLATFORMS or DEPENDENCIES
		if trimmedLine == "PLATFORMS" || trimmedLine == "DEPENDENCIES" {
			inGemSection = false
			continue
		}

		if !inGemSection {
			continue
		}

		// Skip remote: line and empty lines
		if strings.HasPrefix(line, "  remote:") || trimmedLine == "" || trimmedLine == "specs:" {
			continue
		}

		// Parse gem spec line: "    rails (7.1.0)"
		if match := gemLockSpecRegex.FindStringSubmatch(line); match != nil {
			gemName := match[1]
			version := match[2]

			// Determine if this is a direct dependency
			isDirect := directDeps[gemName]

			// Skip transitive dependencies if not requested
			if !options.IncludeTransitive && !isDirect {
				continue
			}

			// Determine scope based on whether it's direct and from dev groups
			scope := types.ScopeProd
			if isDirect {
				// Check if it was in development/test groups from Gemfile
				// For now, we default to prod for lockfile deps
				// The Gemfile parser will handle dev/test classification
				scope = types.ScopeProd
			}

			metadata := types.NewMetadata(MetadataSourceGemfileLock)
			if isDirect {
				metadata["direct"] = true
			} else {
				metadata["direct"] = false
			}

			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypeRuby,
				Name:     gemName,
				Version:  version,
				Scope:    scope,
				Direct:   isDirect,
				Metadata: metadata,
			})
		}
	}

	return dependencies
}

// parseDirectDependencies extracts the list of direct dependencies from DEPENDENCIES section
func (p *GemfileLockParser) parseDirectDependencies(lines []string) map[string]bool {
	directDeps := make(map[string]bool)
	inDepsSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect DEPENDENCIES section
		if trimmedLine == "DEPENDENCIES" {
			inDepsSection = true
			continue
		}

		// Exit DEPENDENCIES section
		if inDepsSection && (trimmedLine == "BUNDLED WITH" || trimmedLine == "PLATFORMS" || trimmedLine == "") {
			if trimmedLine != "" {
				inDepsSection = false
			}
			continue
		}

		if !inDepsSection {
			continue
		}

		// Parse dependency line: "  rails (= 7.1.0)" or "  pg (~> 1.5)"
		// Extract just the gem name before any version constraint
		parts := strings.Fields(trimmedLine)
		if len(parts) > 0 {
			gemName := parts[0]
			directDeps[gemName] = true
		}
	}

	return directDeps
}

// ParseGemfileLockWithMetadata parses Gemfile.lock and extracts additional metadata
// By default, only returns direct dependencies. Use ParseGemfileLockWithMetadataAndOptions to include transitive dependencies.
func (p *GemfileLockParser) ParseGemfileLockWithMetadata(content string) ([]types.Dependency, map[string]interface{}) {
	return p.ParseGemfileLockWithMetadataAndOptions(content, ParseGemfileLockOptions{IncludeTransitive: false})
}

// ParseGemfileLockWithMetadataAndOptions parses Gemfile.lock with configurable options and extracts additional metadata
func (p *GemfileLockParser) ParseGemfileLockWithMetadataAndOptions(content string, options ParseGemfileLockOptions) ([]types.Dependency, map[string]interface{}) {
	dependencies := p.ParseGemfileLockWithOptions(content, options)

	metadata := make(map[string]interface{})

	lines := strings.Split(content, "\n")

	// Extract platforms
	platforms := p.parsePlatforms(lines)
	if len(platforms) > 0 {
		metadata["platforms"] = platforms
	}

	// Extract bundler version
	bundlerVersion := p.parseBundlerVersion(lines)
	if bundlerVersion != "" {
		metadata["bundler_version"] = bundlerVersion
	}

	return dependencies, metadata
}

// parsePlatforms extracts platform information from PLATFORMS section
func (p *GemfileLockParser) parsePlatforms(lines []string) []string {
	platforms := make([]string, 0)
	inPlatformsSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "PLATFORMS" {
			inPlatformsSection = true
			continue
		}

		if inPlatformsSection && (trimmedLine == "DEPENDENCIES" || trimmedLine == "BUNDLED WITH" || trimmedLine == "") {
			if trimmedLine != "" {
				inPlatformsSection = false
			}
			continue
		}

		if inPlatformsSection && trimmedLine != "" {
			platforms = append(platforms, trimmedLine)
		}
	}

	return platforms
}

// parseBundlerVersion extracts bundler version from BUNDLED WITH section
func (p *GemfileLockParser) parseBundlerVersion(lines []string) string {
	inBundledSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "BUNDLED WITH" {
			inBundledSection = true
			continue
		}

		if inBundledSection && trimmedLine != "" {
			return trimmedLine
		}
	}

	return ""
}
