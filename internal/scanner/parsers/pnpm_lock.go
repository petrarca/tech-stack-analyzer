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
		// v9+ format - extract direct dependencies from root importer for filtering
		rootImporter, exists := lockfile.Importers["."]
		if exists {
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
		}

		// Process all packages from v9+ format
		for path, pkg := range lockfile.Packages {
			name := extractPackageNameFromPnpmPath(path)
			if name == "" {
				continue
			}

			// Parse version with semantic version preservation
			version := parsePnpmVersion(pkg.Version, pkg.Resolution)

			// Use common filtering to create dependency
			filter.CreateAndAppendDependency("npm", name, version, "pnpm-lock.yaml", &dependencies)
		}
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

// extractPackageNameFromPnpmPath extracts package name from pnpm-lock.yaml path
// Enhanced with deps.dev patterns for workspace packages and scoped packages
func extractPackageNameFromPnpmPath(path string) string {
	// Handle workspace packages (local packages)
	if strings.HasPrefix(path, ".") {
		// Extract package name from workspace path
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "packages" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
		return ""
	}

	// Handle regular packages
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		// Handle scoped packages like @babel/core
		if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
			return parts[0] + "/" + parts[1]
		}
		return parts[0]
	}

	return ""
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
