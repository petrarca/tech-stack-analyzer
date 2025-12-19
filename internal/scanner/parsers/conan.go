package parsers

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ConanParser handles Conan dependency parsing from conanfile.py and packages*.txt files
type ConanParser struct {
	requireRegex     *regexp.Regexp
	toolRequireRegex *regexp.Regexp
}

// NewConanParser creates a new Conan parser
func NewConanParser() *ConanParser {
	return &ConanParser{
		requireRegex:     regexp.MustCompile(`self\.requires\(["']([^"']+)["']`),
		toolRequireRegex: regexp.MustCompile(`self\.tool_requires\(["']([^"']+)["']`),
	}
}

// ExtractDependencies extracts Conan dependencies from conanfile.py content
func (p *ConanParser) ExtractDependencies(content string) []types.Dependency {
	var dependencies []types.Dependency

	// Parse self.requires() calls
	matches := p.requireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			dep := p.ParseConanDependency(match[1])
			dependencies = append(dependencies, dep)
		}
	}

	// Parse self.tool_requires() calls
	toolMatches := p.toolRequireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range toolMatches {
		if len(match) > 1 {
			dep := p.ParseConanDependency(match[1])
			dependencies = append(dependencies, dep)
		}
	}

	return dependencies
}

// ParseConanDependency parses a Conan dependency string in format "name/version" or "name/version/user/channel#build"
func (p *ConanParser) ParseConanDependency(depString string) types.Dependency {
	parts := strings.Split(depString, "/")
	if len(parts) >= 2 {
		name := parts[0]
		version := strings.Join(parts[1:], "/")
		return types.Dependency{
			Name:    name,
			Version: version,
			Type:    "conan",
		}
	}

	// Fallback if no version found
	return types.Dependency{
		Name:    depString,
		Version: "",
		Type:    "conan",
	}
}

// ExtractDependenciesFromFiles extracts Conan dependencies from conanfile.py and packages*.txt files
func (p *ConanParser) ExtractDependenciesFromFiles(conanContent string, packagesFiles []types.File, currentPath string, provider types.Provider) []types.Dependency {
	var dependencies []types.Dependency

	// Parse conanfile.py dependencies
	conanDeps := p.ExtractDependencies(conanContent)
	dependencies = append(dependencies, conanDeps...)

	// Parse packages*.txt files if present
	for _, file := range packagesFiles {
		if strings.HasPrefix(file.Name, "packages") && strings.HasSuffix(file.Name, ".txt") {
			content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
			if err != nil {
				continue
			}

			// Parse packages file content line by line
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				// Skip empty lines and comments
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				// Parse package/version format
				if strings.Contains(line, "/") {
					dep := p.ParseConanDependency(line)
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	return dependencies
}

func init() {
	// Auto-register this parser
}
