package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// RubyParser handles Ruby-specific file parsing (Gemfile)
type RubyParser struct{}

// NewRubyParser creates a new Ruby parser
func NewRubyParser() *RubyParser {
	return &RubyParser{}
}

// ParseGemfile parses Gemfile and extracts gem dependencies with versions
// Matches TypeScript logic: gem "<gem-name>", "<version>"
// Updated to handle both single and double quotes, and gems without versions
func (p *RubyParser) ParseGemfile(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	// Pattern for: gem 'name' or gem "name" (no version)
	depRegexNoVersion := regexp.MustCompile(`gem ['"]([^'"]+)['"]`)
	// Pattern for: gem 'name', 'version' or gem "name", "version"
	depRegexWithVersion := regexp.MustCompile(`gem ['"]([^'"]+)['"],\s*['"]([^'"]+)['"]`)

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// First try to match with version
		if match := depRegexWithVersion.FindStringSubmatch(line); match != nil {
			gemName := match[1]
			version := match[2]

			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypeRuby,
				Name:     gemName,
				Version:  version,
				Scope:    types.ScopeProd,
				Direct:   true,
				Metadata: types.NewMetadata(MetadataSourceGemfile),
			})
			continue
		}

		// Then try to match without version
		if match := depRegexNoVersion.FindStringSubmatch(line); match != nil {
			gemName := match[1]

			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypeRuby,
				Name:     gemName,
				Version:  "latest",
				Scope:    types.ScopeProd,
				Direct:   true,
				Metadata: types.NewMetadata(MetadataSourceGemfile),
			})
		}
	}

	return dependencies
}
