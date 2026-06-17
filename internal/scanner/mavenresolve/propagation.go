package mavenresolve

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PropagateVersions fills versionless Maven/Gradle dependencies from the same
// coordinate resolved elsewhere in the scan. In a multi-module build the
// identical artifact is often declared in several modules: versionless in one
// (its version comes from a BOM only reachable in another module's parent
// chain) yet concretely resolved in another. Because a coordinate identifies
// the same artifact regardless of which module declares it, a version resolved
// anywhere in the scan is authoritative for its versionless siblings.
//
// This is a deterministic, offline pass over the assembled payload tree, run
// after all components are detected so the cross-module view is complete.
// Resolved versions are never overwritten; only versionless entries are filled,
// and the origin is recorded in metadata.source = "cross-module".
func PropagateVersions(root *types.Payload) {
	if root == nil {
		return
	}
	resolved := collectResolvedVersions(root)
	if len(resolved) == 0 {
		return
	}
	applyResolvedVersions(root, resolved)
}

// isMavenDepType reports whether a dependency type uses Maven coordinates.
func isMavenDepType(depType string) bool {
	return depType == "maven" || depType == "gradle"
}

// collectResolvedVersions builds a "name -> resolved version" map from all
// Maven/Gradle dependencies across the tree that already carry a concrete
// version. The first concrete version seen for a coordinate wins; the goal is
// only to fill entries that have no version at all.
func collectResolvedVersions(root *types.Payload) map[string]string {
	resolved := make(map[string]string)
	var walk func(p *types.Payload)
	walk = func(p *types.Payload) {
		for _, dep := range p.Dependencies {
			if !isMavenDepType(dep.Type) {
				continue
			}
			if _, seen := resolved[dep.Name]; seen {
				continue
			}
			if semver.IsResolved(dep.Version) {
				resolved[dep.Name] = dep.Version
			}
		}
		for _, child := range p.Children {
			walk(child)
		}
	}
	walk(root)
	return resolved
}

// applyResolvedVersions fills versionless Maven/Gradle dependencies from the
// resolved map, without overwriting concrete versions.
func applyResolvedVersions(root *types.Payload, resolved map[string]string) {
	var walk func(p *types.Payload)
	walk = func(p *types.Payload) {
		for i := range p.Dependencies {
			dep := &p.Dependencies[i]
			if !isMavenDepType(dep.Type) || semver.IsResolved(dep.Version) {
				continue
			}
			version, ok := resolved[dep.Name]
			if !ok {
				continue
			}
			declared := dep.Version
			dep.Version = version
			dep.SetDeclaredVersion(declared)
			if dep.Metadata == nil {
				dep.Metadata = make(map[string]interface{})
			}
			dep.Metadata["source"] = "cross-module"
		}
		for _, child := range p.Children {
			walk(child)
		}
	}
	walk(root)
}
