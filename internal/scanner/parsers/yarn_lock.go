package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseYarnLock parses yarn.lock content and returns dependencies with resolved versions
// Note: yarn.lock doesn't distinguish direct vs transitive deps, so we need package.json
// to filter. This function returns only direct dependencies.
func ParseYarnLock(lockContent []byte, packageJSON *PackageJSON) []types.Dependency {
	if packageJSON == nil {
		return nil
	}

	// Build set of direct dependency names from package.json
	directDeps := make(map[string]bool)
	for name := range packageJSON.Dependencies {
		directDeps[name] = true
	}
	for name := range packageJSON.DevDependencies {
		directDeps[name] = true
	}

	var dependencies []types.Dependency
	content := string(lockContent)

	// Parse yarn.lock format (v4+)
	// Format: "package@npm:^version":
	//           version: "x.y.z"
	// Package name can be scoped like @babel/core
	packagePattern := regexp.MustCompile(`^"((?:@[^/]+/)?[^@]+)@npm:[^"]+":`)
	versionPattern := regexp.MustCompile(`^\s+version:\s+"?([^"\s]+)"?`)

	lines := strings.Split(content, "\n")
	var currentPackage string

	for _, line := range lines {
		// Check for package declaration
		if matches := packagePattern.FindStringSubmatch(line); len(matches) > 1 {
			currentPackage = matches[1]
			continue
		}

		// Check for version line
		if currentPackage != "" {
			if matches := versionPattern.FindStringSubmatch(line); len(matches) > 1 {
				version := matches[1]

				// Only include if it's a direct dependency
				if directDeps[currentPackage] {
					dependencies = append(dependencies, types.Dependency{
						Type:       "npm",
						Name:       currentPackage,
						Version:    version,
						SourceFile: "yarn.lock",
					})
					// Remove from map to avoid duplicates (same package, different version ranges)
					delete(directDeps, currentPackage)
				}
				currentPackage = ""
			}
		}
	}

	return dependencies
}
