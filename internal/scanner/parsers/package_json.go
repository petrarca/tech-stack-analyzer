package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PackageJSON represents the structure of package.json
// Enhanced version with additional fields for comprehensive dependency analysis
type PackageJSONEnhanced struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	Workspaces           []string          `json:"workspaces"`
	Workspace            string            `json:"workspace"`
}

// ParsePackageJSONEnhanced parses package.json content and returns direct dependencies with semantic version constraints
// Enhanced with deps.dev patterns for semantic version preservation and workspace support
func ParsePackageJSONEnhanced(content []byte) []types.Dependency {
	var packageJSON PackageJSONEnhanced
	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return nil
	}

	dependencies := make([]types.Dependency, 0)

	// Add production dependencies with semantic version constraints
	for name, version := range packageJSON.Dependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       DependencyTypeNpm,
			Name:       name,
			Version:    parseSemanticVersion(version),
			SourceFile: "package.json",
			Scope:      "prod",
		})
	}

	// Add development dependencies with semantic version constraints
	for name, version := range packageJSON.DevDependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       DependencyTypeNpm,
			Name:       name,
			Version:    parseSemanticVersion(version),
			SourceFile: "package.json",
			Scope:      "dev",
		})
	}

	// Add peer dependencies with semantic version constraints
	for name, version := range packageJSON.PeerDependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       DependencyTypeNpm,
			Name:       name,
			Version:    parseSemanticVersion(version),
			SourceFile: "package.json",
			Scope:      "peer",
		})
	}

	// Add optional dependencies with semantic version constraints
	for name, version := range packageJSON.OptionalDependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       DependencyTypeNpm,
			Name:       name,
			Version:    parseSemanticVersion(version),
			SourceFile: "package.json",
			Scope:      "optional",
		})
	}

	return dependencies
}

// parseSemanticVersion parses and normalizes semantic version strings
// Enhanced with deps.dev patterns using npm semver normalization
func parseSemanticVersion(version string) string {
	// Use semver package for npm version normalization
	return semver.NormalizeNPMVersion(version)
}

// IsWorkspaceProject checks if package.json indicates a workspace project
// Based on deps.dev patterns for npm/yarn workspace detection
func IsWorkspaceProject(content []byte) bool {
	var packageJSON PackageJSONEnhanced
	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return false
	}

	// Check for workspaces array
	if len(packageJSON.Workspaces) > 0 {
		return true
	}

	// Check for workspace field
	if packageJSON.Workspace != "" {
		return true
	}

	return false
}

// GetWorkspacePackages extracts workspace package patterns from package.json
func GetWorkspacePackages(content []byte) []string {
	var packageJSON PackageJSONEnhanced
	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return nil
	}

	return packageJSON.Workspaces
}
