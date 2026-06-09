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
	Packages        map[string]PnpmPackage  `yaml:"packages,omitempty"`  // v9+ format
	Snapshots       map[string]PnpmSnapshot `yaml:"snapshots,omitempty"` // v9+ resolved dependency graph
}

// PnpmSnapshot is a v9 snapshot entry: a resolved package instance with the
// exact versions of its dependencies. The keys are "name@version(peers)".
type PnpmSnapshot struct {
	Dependencies         map[string]string `yaml:"dependencies"`
	OptionalDependencies map[string]string `yaml:"optionalDependencies"`
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

// ParsePnpmLockGraph parses pnpm-lock.yaml and returns the dependencies plus the
// package-to-package edges, honoring the requested graph mode. It implements the
// GraphProducer contract (ParseGraphFunc).
func ParsePnpmLockGraph(input GraphInput) LockGraph {
	content := input.Lockfile
	deps := ParsePnpmLockWithOptions(content, NPMLockFileOptions{})
	result := LockGraph{Dependencies: deps}

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lockfile PnpmLockfile
	if err := yaml.Unmarshal(content, &lockfile); err != nil {
		return result
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = pnpmDirectEdges(lockfile)
	case types.DependencyGraphFull:
		result.Edges = pnpmSnapshotEdges(lockfile.Snapshots)
	}
	return result
}

// pnpmDirectEdges builds root -> direct-dependency edges from the importers
// section. The "from" node is the synthetic root marker ".".
func pnpmDirectEdges(lockfile PnpmLockfile) []types.DependencyEdge {
	root, ok := lockfile.Importers["."]
	if !ok {
		return nil
	}
	var edges []types.DependencyEdge
	add := func(deps map[string]PnpmDependency, scope string) {
		for name, dep := range deps {
			ver := resolvePnpmImporterVersion(dep.Version)
			edges = append(edges, types.DependencyEdge{From: ".", To: pnpmNodeID(name + "@" + ver), Scope: scope})
		}
	}
	add(root.Dependencies, types.ScopeProd)
	add(root.DevDependencies, types.ScopeDev)
	add(root.OptionalDependencies, types.ScopeOptional)
	return edges
}

// pnpmSnapshotEdges builds the full package-to-package graph from v9 snapshot
// entries. Snapshot keys and dependency versions carry peer-dependency suffixes
// ("name@1.2.3(peer@4.5.6)") which are trimmed to a clean "name@version" node.
func pnpmSnapshotEdges(snapshots map[string]PnpmSnapshot) []types.DependencyEdge {
	if len(snapshots) == 0 {
		return nil
	}
	var edges []types.DependencyEdge
	for key, snap := range snapshots {
		from := pnpmNodeID(key)
		if from == "" {
			continue
		}
		for depName, depVer := range snap.Dependencies {
			edges = append(edges, types.DependencyEdge{From: from, To: pnpmNodeID(depName + "@" + depVer)})
		}
		for depName, depVer := range snap.OptionalDependencies {
			// Scope is set on optional edges in full mode, matching direct mode (F-07).
			edges = append(edges, types.DependencyEdge{From: from, To: pnpmNodeID(depName + "@" + depVer), Scope: types.ScopeOptional})
		}
	}
	return edges
}

// pnpmNodeID normalizes a pnpm snapshot key / dependency reference into a
// stable "name@version" node identity by stripping the peer-dependency suffix.
func pnpmNodeID(ref string) string {
	if i := strings.IndexByte(ref, '('); i >= 0 {
		ref = ref[:i]
	}
	return strings.TrimSpace(ref)
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
