package parsers

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PnpmLockfile represents the structure of pnpm-lock.yaml
// Enhanced with deps.dev patterns for comprehensive dependency analysis
type PnpmLockfile struct {
	LockfileVersion string                  `yaml:"lockfileVersion"`
	Importers       map[string]PnpmImporter `yaml:"importers"`
	Packages        map[string]PnpmPackage  `yaml:"packages,omitempty"` // v9+ format
}

// PnpmImporter represents an importer in pnpm-lock.yaml
// Enhanced with deps.dev patterns for better dependency classification
type PnpmImporter struct {
	Dependencies         map[string]PnpmDependency `yaml:"dependencies"`
	DevDependencies      map[string]PnpmDependency `yaml:"devDependencies"`
	OptionalDependencies map[string]PnpmDependency `yaml:"optionalDependencies"`
}

// PnpmPackage represents a package in pnpm-lock.yaml (v9+ format)
// Enhanced with deps.dev patterns for workspace and resolution support
type PnpmPackage struct {
	Resolution PnpmResolution `yaml:"resolution"`
	Name       string         `yaml:"name,omitempty"`
	Version    string         `yaml:"version"`
	Dev        bool           `yaml:"dev,omitempty"`
	Optional   bool           `yaml:"optional,omitempty"`
}

// PnpmResolution represents package resolution information
type PnpmResolution struct {
	Directory string `yaml:"directory,omitempty"`
	Tarball   string `yaml:"tarball,omitempty"`
	Git       string `yaml:"git,omitempty"`
}

// PnpmDependency represents a dependency in pnpm-lock.yaml
// Enhanced with deps.dev patterns for semantic version preservation
type PnpmDependency struct {
	Specifier string `yaml:"specifier"`
	Version   string `yaml:"version"`
}

// ParsePnpmLock parses pnpm-lock.yaml content and returns direct dependencies only
// Enhanced with deps.dev patterns for workspace support and semantic version handling
func ParsePnpmLock(content []byte) []types.Dependency {
	return ParsePnpmLockWithOptions(content, NPMLockFileOptions{})
}

// ParsePnpmLockWithOptions parses pnpm-lock.yaml content with configurable options
func ParsePnpmLockWithOptions(content []byte, options NPMLockFileOptions) []types.Dependency {
	var lockfile PnpmLockfile
	if err := yaml.Unmarshal(content, &lockfile); err != nil {
		return nil
	}

	var dependencies []types.Dependency
	filter := NewDependencyFilter(options)

	// Handle both v6+ (importers) and v9+ (packages) lockfile formats
	if len(lockfile.Packages) > 0 {
		// v9+ format. Direct dependencies live under importers with both a
		// specifier (range) and a resolved version (e.g. "1.2.11(zod@4.3.6)").
		// The resolved version is read directly from the importer; the
		// version-suffixed keys in the top-level packages map are not used
		// for direct dependency resolution.
		rootImporter, exists := lockfile.Importers["."]
		if !exists {
			return nil
		}

		appendImporterDeps(rootImporter.Dependencies, "prod", filter, &dependencies)
		appendImporterDeps(rootImporter.DevDependencies, "dev", filter, &dependencies)
		appendImporterDeps(rootImporter.OptionalDependencies, "optional", filter, &dependencies)
	} else {
		// v6+ format with importers field - direct dependencies only
		rootImporter, exists := lockfile.Importers["."]
		if !exists {
			return nil
		}

		// Add direct dependencies to filter
		for name := range rootImporter.Dependencies {
			filter.AddDirectDependency(name, "prod")
		}
		for name := range rootImporter.DevDependencies {
			filter.AddDirectDependency(name, "dev")
		}
		for name := range rootImporter.OptionalDependencies {
			filter.AddDirectDependency(name, "optional")
		}

		// Parse production dependencies
		for name, dep := range rootImporter.Dependencies {
			version := parsePnpmVersion(dep.Version, PnpmResolution{})
			filter.CreateAndAppendDependency("npm", name, version, "pnpm-lock.yaml", &dependencies)
		}

		// Parse development dependencies
		for name, dep := range rootImporter.DevDependencies {
			version := parsePnpmVersion(dep.Version, PnpmResolution{})
			filter.CreateAndAppendDependency("npm", name, version, "pnpm-lock.yaml", &dependencies)
		}

		// Parse optional dependencies
		for name, dep := range rootImporter.OptionalDependencies {
			version := parsePnpmVersion(dep.Version, PnpmResolution{})
			filter.CreateAndAppendDependency("npm", name, version, "pnpm-lock.yaml", &dependencies)
		}
	}

	return dependencies
}

// appendImporterDeps adds importer dependencies (with resolved versions) to the
// dependency list via the shared filter. Used for the v9+ importer format.
func appendImporterDeps(deps map[string]PnpmDependency, scope string, filter *DependencyFilter, out *[]types.Dependency) {
	for name, dep := range deps {
		filter.AddDirectDependency(name, scope)
		version := resolvePnpmImporterVersion(dep.Version)
		before := len(*out)
		filter.CreateAndAppendDependency("npm", name, version, "pnpm-lock.yaml", out)
		// Record the declared specifier (range) when a dependency was added.
		if len(*out) > before {
			(*out)[before].SetDeclaredVersion(dep.Specifier)
		}
	}
}

// resolvePnpmImporterVersion extracts the resolved version from a pnpm v9
// importer version string, stripping the peer-dependency suffix in parentheses
// (e.g. "1.2.11(zod@4.3.6)" -> "1.2.11"). Workspace links ("link:...") and
// empty values fall back to "latest".
func resolvePnpmImporterVersion(version string) string {
	version = strings.TrimSpace(version)
	if i := strings.IndexByte(version, '('); i >= 0 {
		version = version[:i]
	}
	version = strings.TrimSpace(version)
	if version == "" || version == "*" {
		return "latest"
	}
	if strings.HasPrefix(version, "link:") || strings.HasPrefix(version, "file:") {
		return "workspace"
	}
	return version
}

// parsePnpmVersion parses pnpm version with semantic version preservation
// Enhanced with deps.dev patterns for workspace and git dependencies
func parsePnpmVersion(version string, resolution PnpmResolution) string {
	// Handle workspace dependencies
	if resolution.Directory != "" {
		return "workspace"
	}

	// Handle git dependencies
	if resolution.Git != "" {
		return "git:" + resolution.Git
	}

	// Handle tarball dependencies
	if resolution.Tarball != "" {
		if strings.HasPrefix(resolution.Tarball, "file:") {
			return "local"
		}
		return "tarball"
	}

	// Handle regular semantic versions with constraints
	version = strings.TrimSpace(version)
	if version == "" || version == "*" {
		return "latest"
	}

	// Preserve semantic version constraints (^, ~, >=, <=, etc.)
	return version
}

// GetPnpmLockfileVersion detects the pnpm-lock.yaml version format
func GetPnpmLockfileVersion(content []byte) string {
	var lockfile struct {
		LockfileVersion string `yaml:"lockfileVersion"`
	}

	if err := yaml.Unmarshal(content, &lockfile); err != nil {
		return "6" // Default to v6 if parsing fails
	}

	return lockfile.LockfileVersion
}
