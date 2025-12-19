package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseYarnLock parses yarn.lock content and returns dependencies with resolved versions
// Note: yarn.lock doesn't distinguish direct vs transitive deps, so we need package.json
// to filter. This function returns only direct dependencies with scope information.
func ParseYarnLock(lockContent []byte, packageJSON *PackageJSON) []types.Dependency {
	if packageJSON == nil {
		return nil
	}

	// Build maps of direct dependency names with their scopes from package.json
	prodDeps := make(map[string]bool)
	devDeps := make(map[string]bool)
	for name := range packageJSON.Dependencies {
		prodDeps[name] = true
	}
	for name := range packageJSON.DevDependencies {
		devDeps[name] = true
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

				// Determine scope and include if it's a direct dependency
				var scope string
				var isDirect bool
				if prodDeps[currentPackage] {
					scope = types.ScopeProd
					isDirect = true
					delete(prodDeps, currentPackage)
				} else if devDeps[currentPackage] {
					scope = types.ScopeDev
					isDirect = true
					delete(devDeps, currentPackage)
				}

				if isDirect {
					dependencies = append(dependencies, types.Dependency{
						Type:       "npm",
						Name:       currentPackage,
						Version:    version,
						SourceFile: "yarn.lock",
						Scope:      scope,
					})
				}
				currentPackage = ""
			}
		}
	}

	return dependencies
}
