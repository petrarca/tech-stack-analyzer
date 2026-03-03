package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MavenDependencyListParser handles parsing of Maven dependency list output
//
// To generate the dependency list file, run:
//
//	mvn dependency:list -DoutputFile=dependency-list.txt
//
// This command:
//   - Resolves all dependencies (direct and transitive) with their exact versions
//   - Resolves Maven special versions (LATEST, RELEASE) to actual version numbers
//   - Resolves property references (${spring.boot.version}) to concrete values
//   - Outputs all resolved dependencies to dependency-list.txt
//
// The output format is:
//
//	groupId:artifactId:type:version:scope [optional module info]
//
// Example:
//
//	org.springframework.boot:spring-boot-starter-web:jar:4.0.1:compile -- module spring.boot.starter.web [auto]
//
// The parser extracts resolved versions for dependencies declared in pom.xml.
// By default, only direct dependencies are used (includeTransitive=false).
// Set includeTransitive=true to include all transitive dependencies.
type MavenDependencyListParser struct{}

// NewMavenDependencyListParser creates a new Maven dependency list parser
func NewMavenDependencyListParser() *MavenDependencyListParser {
	return &MavenDependencyListParser{}
}

// ParseDependencyList parses Maven dependency:list output
// Format: groupId:artifactId:type:version:scope [optional module info]
// Example: org.springframework.boot:spring-boot-starter-web:jar:4.0.1:compile -- module spring.boot.starter.web [auto]
// If includeTransitive is false, returns all dependencies (filtering should be done by caller)
// If includeTransitive is true, returns all dependencies
func (p *MavenDependencyListParser) ParseDependencyList(content string, includeTransitive bool) []types.Dependency {
	var dependencies []types.Dependency

	// Pattern to match dependency lines
	// Format: groupId:artifactId:type:version:scope
	// May have ANSI color codes and module info after
	depPattern := regexp.MustCompile(`^\s+([^:]+):([^:]+):([^:]+):([^:]+):([^\s\[]+)`)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Skip empty lines and header lines
		if strings.TrimSpace(line) == "" || strings.Contains(line, "The following files have been resolved:") {
			continue
		}

		matches := depPattern.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}

		groupId := strings.TrimSpace(matches[1])
		artifactId := strings.TrimSpace(matches[2])
		depType := strings.TrimSpace(matches[3])
		version := strings.TrimSpace(matches[4])
		scope := strings.TrimSpace(matches[5])

		if groupId == "" || artifactId == "" {
			continue
		}

		dep := types.Dependency{
			Type:    DependencyTypeMaven,
			Name:    groupId + ":" + artifactId,
			Version: version,
			Scope:   mapMavenListScope(scope),
			Direct:  false, // All deps from list are considered resolved (we don't know which are direct)
		}

		// Build metadata
		metadata := make(map[string]interface{})

		if depType != "" && depType != "jar" {
			metadata["type"] = depType
		}

		// Mark as resolved from dependency list
		metadata["source"] = "dependency-list"

		if len(metadata) > 0 {
			dep.Metadata = metadata
		}

		dependencies = append(dependencies, dep)
	}

	return dependencies
}

// mapMavenListScope maps Maven scope from dependency list to our scope constants
func mapMavenListScope(scope string) string {
	switch scope {
	case "test":
		return types.ScopeDev
	case "provided", "runtime", "compile":
		return types.ScopeProd
	case "system":
		return types.ScopeSystem
	case "import":
		return types.ScopeImport
	default:
		return types.ScopeProd
	}
}
