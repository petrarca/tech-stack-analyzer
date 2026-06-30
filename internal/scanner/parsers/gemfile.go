package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Pre-compiled regexes for Ruby parsing performance
var (
	rubyDepRegexNoVersion   = regexp.MustCompile(`gem ['"]([^'"]+)['"]`)
	rubyDepRegexWithVersion = regexp.MustCompile(`gem ['"]([^'"]+)['"],\s*['"]([^'"]+)['"]`)
	rubyGroupRegex          = regexp.MustCompile(`group\s+:?(\w+)(?:\s*,\s*:?(\w+))*\s+do`)
	rubyGitRegex            = regexp.MustCompile(`git:\s*['"]([^'"]+)['"]`)
	rubyBranchRegex         = regexp.MustCompile(`branch:\s*['"]([^'"]+)['"]`)
	rubyPathRegex           = regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	rubyPlatformsRegex      = regexp.MustCompile(`platforms?:\s*\[([^\]]+)\]`)
)

// RubyParser handles Ruby-specific file parsing (Gemfile)
type RubyParser struct{}

// NewRubyParser creates a new Ruby parser
func NewRubyParser() *RubyParser {
	return &RubyParser{}
}

// ParseGemfile parses Gemfile and extracts gem dependencies with versions
// Handles groups (development, test), git sources, paths, platforms, and other options
func (p *RubyParser) ParseGemfile(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	lines := strings.Split(content, "\n")
	currentGroups := []string{} // Track current group context
	groupDepth := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Track group blocks (group :x do ... end), which set dependency scope.
		if groupMatch := rubyGroupRegex.FindStringSubmatch(trimmedLine); groupMatch != nil {
			currentGroups = extractRubyGroups(groupMatch)
			groupDepth++
			continue
		}
		if trimmedLine == "end" && groupDepth > 0 {
			groupDepth--
			if groupDepth == 0 {
				currentGroups = []string{}
			}
			continue
		}
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		if dep := p.parseGemLine(trimmedLine, currentGroups); dep != nil {
			dependencies = append(dependencies, *dep)
		}
	}

	return dependencies
}

// extractRubyGroups collects the non-empty capture groups from a `group` match.
func extractRubyGroups(groupMatch []string) []string {
	groups := []string{}
	for i := 1; i < len(groupMatch); i++ {
		if groupMatch[i] != "" {
			groups = append(groups, groupMatch[i])
		}
	}
	return groups
}

// parseGemLine parses a single `gem` line into a dependency, or returns nil
// when the line is not a gem declaration.
func (p *RubyParser) parseGemLine(trimmedLine string, currentGroups []string) *types.Dependency {
	var gemName, version string
	if match := rubyDepRegexWithVersion.FindStringSubmatch(trimmedLine); match != nil {
		gemName, version = match[1], match[2]
	} else if match := rubyDepRegexNoVersion.FindStringSubmatch(trimmedLine); match != nil {
		gemName, version = match[1], "latest"
	} else {
		return nil
	}
	if gemName == "" {
		return nil
	}
	return &types.Dependency{
		Type:     DependencyTypeRuby,
		Name:     gemName,
		Version:  version,
		Scope:    p.mapGemfileGroupToScope(currentGroups),
		Direct:   true,
		Metadata: p.buildRubyMetadata(trimmedLine, currentGroups),
	}
}

// mapGemfileGroupToScope maps Gemfile groups to dependency scopes
func (p *RubyParser) mapGemfileGroupToScope(groups []string) string {
	if len(groups) == 0 {
		return types.ScopeProd
	}

	// Check for test group
	for _, group := range groups {
		if group == "test" {
			return types.ScopeDev
		}
	}

	// Check for development group
	for _, group := range groups {
		if group == "development" {
			return types.ScopeDev
		}
	}

	return types.ScopeProd
}

// buildRubyMetadata creates metadata map for Ruby gem dependencies
func (p *RubyParser) buildRubyMetadata(line string, groups []string) map[string]interface{} {
	metadata := types.NewMetadata(MetadataSourceGemfile)

	// Add groups if present
	p.addGroupsToMetadata(metadata, groups)

	// Extract various metadata fields
	p.addGitSourceToMetadata(metadata, line)
	p.addBranchToMetadata(metadata, line)
	p.addPathToMetadata(metadata, line)
	p.addRequireFlagToMetadata(metadata, line)
	p.addPlatformsToMetadata(metadata, line)

	return metadata
}

// addGroupsToMetadata adds group information to metadata
func (p *RubyParser) addGroupsToMetadata(metadata map[string]interface{}, groups []string) {
	if len(groups) > 0 {
		metadata["groups"] = groups
	}
}

// addGitSourceToMetadata extracts and adds git source to metadata
func (p *RubyParser) addGitSourceToMetadata(metadata map[string]interface{}, line string) {
	if match := rubyGitRegex.FindStringSubmatch(line); match != nil {
		metadata["git"] = match[1]
	}
}

// addBranchToMetadata extracts and adds branch information to metadata
func (p *RubyParser) addBranchToMetadata(metadata map[string]interface{}, line string) {
	if match := rubyBranchRegex.FindStringSubmatch(line); match != nil {
		metadata["branch"] = match[1]
	}
}

// addPathToMetadata extracts and adds path information to metadata
func (p *RubyParser) addPathToMetadata(metadata map[string]interface{}, line string) {
	if match := rubyPathRegex.FindStringSubmatch(line); match != nil {
		metadata["path"] = match[1]
	}
}

// addRequireFlagToMetadata checks for require: false and adds to metadata
func (p *RubyParser) addRequireFlagToMetadata(metadata map[string]interface{}, line string) {
	if strings.Contains(line, "require: false") || strings.Contains(line, "require:false") {
		metadata["require"] = false
	}
}

// addPlatformsToMetadata extracts and adds platform information to metadata
func (p *RubyParser) addPlatformsToMetadata(metadata map[string]interface{}, line string) {
	if match := rubyPlatformsRegex.FindStringSubmatch(line); match != nil {
		platforms := strings.Split(match[1], ",")
		cleanPlatforms := make([]string, 0, len(platforms))
		for _, p := range platforms {
			platform := strings.TrimSpace(p)
			platform = strings.Trim(platform, ":")
			// Remove quotes from platform names
			platform = strings.Trim(platform, `"'`)
			if platform != "" {
				cleanPlatforms = append(cleanPlatforms, platform)
			}
		}
		if len(cleanPlatforms) > 0 {
			metadata["platforms"] = cleanPlatforms
		}
	}
}
