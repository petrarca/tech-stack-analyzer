package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PackageLockJSON represents the structure of package-lock.json
type PackageLockJSON struct {
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]PackageInfo `json:"packages"`
}

// PackageInfo represents a package in package-lock.json
type PackageInfo struct {
	Version  string `json:"version"`
	Resolved string `json:"resolved,omitempty"`
}

// ParsePackageLock parses package-lock.json content and returns direct dependencies with resolved versions
// Note: package-lock.json doesn't contain scope information, so we need package.json to determine scope
func ParsePackageLock(content []byte, packageJSON *PackageJSON) []types.Dependency {
	var lockfile PackageLockJSON
	if err := json.Unmarshal(content, &lockfile); err != nil {
		return nil
	}

	// Build maps of direct dependency names with their scopes from package.json
	prodDeps := make(map[string]bool)
	devDeps := make(map[string]bool)
	if packageJSON != nil {
		for name := range packageJSON.Dependencies {
			prodDeps[name] = true
		}
		for name := range packageJSON.DevDependencies {
			devDeps[name] = true
		}
	}

	var dependencies []types.Dependency

	// Extract direct dependencies from node_modules/
	// Direct deps: "node_modules/express" (one level)
	// Transitive deps: "node_modules/express/node_modules/accepts" (nested node_modules)
	for path, pkg := range lockfile.Packages {
		// Skip root package (empty path)
		if path == "" {
			continue
		}

		// Skip transitive dependencies (nested node_modules/)
		// Count occurrences of "node_modules/" - direct deps have exactly 1
		if strings.Count(path, "node_modules/") != 1 {
			continue
		}

		// Extract package name from path (e.g., "express" from "node_modules/express")
		name := extractNameFromNodeModulesPath(path)
		if name != "" {
			// Determine scope based on package.json
			var scope string
			if prodDeps[name] {
				scope = types.ScopeProd
			} else if devDeps[name] {
				scope = types.ScopeDev
			} else {
				// If not found in package.json, default to prod (likely a transitive dep that became direct)
				scope = types.ScopeProd
			}

			dependencies = append(dependencies, types.Dependency{
				Type:       "npm",
				Name:       name,
				Version:    pkg.Version,
				SourceFile: "package-lock.json",
				Scope:      scope,
			})
		}
	}

	return dependencies
}

// extractNameFromNodeModulesPath extracts package name from package-lock.json path
// e.g., "node_modules/express" -> "express"
// e.g., "node_modules/@babel/core" -> "@babel/core"
func extractNameFromNodeModulesPath(path string) string {
	// Remove "node_modules/" prefix
	name := strings.TrimPrefix(path, "node_modules/")

	// Handle scoped packages like @babel/core
	if strings.HasPrefix(name, "@") {
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			return strings.Join(parts[:2], "/")
		}
	}

	// Handle regular packages - just return the first part
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[0]
	}

	return name
}
