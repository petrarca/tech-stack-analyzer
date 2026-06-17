package parsers

import (
	"encoding/xml"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// maxParentDepth bounds the parent-POM climb when collecting managed versions,
// matching the property-resolution bound (Maven typically allows ~10 levels).
const maxParentDepth = 10

// collectManagedVersions builds a "groupId:artifactId" -> resolved version
// table from the current POM's <dependencyManagement> plus those inherited from
// parent POMs reachable through the provider. Versions are resolved against the
// supplied properties (already merged across the parent/property scope), so
// "${property}" references become concrete where possible.
//
// This is the offline analogue of Maven's dependencyManagement resolution: a
// versionless <dependency> in a child POM takes its version from a managed
// entry declared in this POM or an ancestor/BOM POM. POMs not present on disk
// (published to a registry) cannot contribute and leave the entry unresolved.
//
// Nearest-wins: an entry already in the table (declared closer to the child) is
// not overwritten by a more distant ancestor.
func (p *MavenParser) collectManagedVersions(content, pomDir string, provider types.Provider, properties map[string]string) map[string]string {
	managed := make(map[string]string)
	p.collectManagedVersionsRecursive(content, pomDir, provider, properties, managed, 0)
	if len(managed) == 0 {
		return nil
	}
	return managed
}

// collectManagedVersionsRecursive walks the POM and its parent chain, filling
// the managed-version table. Closer declarations win, so the current POM is
// added before recursing into the parent.
func (p *MavenParser) collectManagedVersionsRecursive(content, pomDir string, provider types.Provider,
	properties map[string]string, managed map[string]string, depth int) {
	if depth > maxParentDepth {
		return
	}

	var project MavenProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return
	}

	// Current POM's dependencyManagement (and active profiles') win over ancestors.
	p.addManagedEntries(project.DependencyManagement.Dependencies, properties, managed)
	for _, profile := range p.getActiveProfiles(project.Profiles) {
		p.addManagedEntries(profile.DependencyManagement.Dependencies, properties, managed)
	}

	// Climb to the parent POM when reachable through the provider.
	if provider == nil || pomDir == "" || project.Parent.GroupId == "" {
		return
	}
	parentPath := p.resolveParentPath(pomDir, project.Parent.RelativePath)
	parentContent, err := provider.ReadFile(parentPath)
	if err != nil {
		return
	}
	p.collectManagedVersionsRecursive(string(parentContent), filepath.Dir(parentPath),
		provider, properties, managed, depth+1)
}

// addManagedEntries records resolved managed versions, keyed by
// "groupId:artifactId", without overwriting closer (already-present) entries.
// Only concrete versions are recorded; BOM imports (scope=import) manage
// versions transitively and are not direct version sources here.
func (p *MavenParser) addManagedEntries(deps []MavenDependency, properties map[string]string, managed map[string]string) {
	for _, dep := range deps {
		if dep.GroupId == "" || dep.ArtifactId == "" || dep.Version == "" {
			continue
		}
		if dep.Scope == types.ScopeImport {
			continue
		}
		key := dep.GroupId + ":" + dep.ArtifactId
		if _, exists := managed[key]; exists {
			continue
		}
		resolved := p.resolvePropertyRefs(dep.Version, properties, make(map[string]bool))
		if semver.IsResolved(resolved) {
			managed[key] = resolved
		}
	}
}

// applyManagedVersions backfills versionless (or otherwise unresolved)
// dependencies from the managed-version table. A dependency that already has a
// concrete version is never overwritten. The declared form is preserved in
// metadata.declared, and metadata.source records the resolution origin.
func (p *MavenParser) applyManagedVersions(deps []types.Dependency, managed map[string]string) {
	if len(managed) == 0 {
		return
	}
	for i := range deps {
		dep := &deps[i]
		if semver.IsResolved(dep.Version) {
			continue
		}
		resolved, ok := managed[dep.Name]
		if !ok {
			continue
		}
		declared := dep.Version
		dep.Version = resolved
		dep.SetDeclaredVersion(declared)
		if dep.Metadata == nil {
			dep.Metadata = make(map[string]interface{})
		}
		dep.Metadata["source"] = "dependency-management"
	}
}
