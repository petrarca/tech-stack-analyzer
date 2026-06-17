package parsers

import (
	"encoding/xml"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// maxParentDepth bounds the parent-POM climb when collecting managed versions,
// matching the property-resolution bound (Maven typically allows ~10 levels).
// Nested BOM imports are bounded instead by a visited-coordinate set.
const maxParentDepth = 10

// BomResolver locates an imported BOM POM within the scanned tree by its
// coordinates and returns its raw content and directory (for parent/relative
// resolution). It returns ok=false when the BOM is not present in the repo
// (e.g. a third-party or private BOM published only to a registry), in which
// case its managed versions stay unresolved -- the offline equivalent of
// Trivy's repository fetch being unavailable.
//
// The parser stays free of any repository/index knowledge: the detector injects
// a resolver backed by the source index. A nil resolver disables BOM-import
// following (parent-chain resolution still works).
type BomResolver func(groupID, artifactID, version string) (content []byte, dir string, ok bool)

// CollectBomManagedVersions resolves a BOM by its coordinates and returns its
// "groupId:artifactId" -> resolved version table, following the BOM's own
// parent chain and nested imports. It is the Gradle-platform analogue of Maven
// dependencyManagement-import resolution: a Gradle platform()/enforcedPlatform()
// declaration references a pom-packaged BOM identical to a Maven imported BOM.
//
// Returns nil when the BOM cannot be fetched (resolver nil or ok=false) or
// declares no managed versions.
func CollectBomManagedVersions(groupID, artifactID, version string, provider types.Provider, bomResolver BomResolver) map[string]string {
	if bomResolver == nil || groupID == "" || artifactID == "" {
		return nil
	}
	content, dir, ok := bomResolver(groupID, artifactID, version)
	if !ok || len(content) == 0 {
		return nil
	}
	p := NewMavenParser()
	properties := p.extractProperties(string(content))
	managed := make(map[string]string)
	visited := map[string]bool{groupID + ":" + artifactID: true}
	p.collectManagedVersionsRecursive(string(content), dir, provider, properties, managed, bomResolver, visited, 0)
	if len(managed) == 0 {
		return nil
	}
	return managed
}

// collectManagedVersions builds a "groupId:artifactId" -> resolved version
// table from the current POM's <dependencyManagement>, those inherited from
// parent POMs, and those contributed by imported BOMs (scope=import,type=pom)
// resolved through bomResolver. Versions are resolved against the supplied
// properties so "${property}" references become concrete where possible.
//
// This is the offline analogue of Maven's dependencyManagement resolution. POMs
// not present in the repo cannot contribute and leave their entries unresolved.
//
// Nearest-wins: an entry already in the table (declared closer to the child) is
// not overwritten by a more distant ancestor or import. Per the Maven spec,
// the POM's own and inherited managed entries take precedence over imported
// BOMs, so imports are processed after the parent chain.
func (p *MavenParser) collectManagedVersions(content, pomDir string, provider types.Provider,
	properties map[string]string, bomResolver BomResolver) map[string]string {
	managed := make(map[string]string)
	visited := make(map[string]bool) // imported-BOM coordinates already followed (cycle guard)
	p.collectManagedVersionsRecursive(content, pomDir, provider, properties, managed, bomResolver, visited, 0)
	if len(managed) == 0 {
		return nil
	}
	return managed
}

// collectManagedVersionsRecursive walks the POM and its parent chain, filling
// the managed-version table. Closer declarations win, so the current POM is
// added before recursing into the parent. BOM imports found along the way are
// resolved after the direct/inherited entries. visited records imported-BOM
// coordinates already followed, preventing cycles across the mutual recursion
// with importBomManagedVersions.
func (p *MavenParser) collectManagedVersionsRecursive(content, pomDir string, provider types.Provider,
	properties map[string]string, managed map[string]string, bomResolver BomResolver, visited map[string]bool, depth int) {
	if depth > maxParentDepth {
		return
	}

	var project MavenProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return
	}

	// Build the property view for THIS POM: inherited properties from the
	// caller, plus this POM's own <properties>, parent properties reachable via
	// the provider, and project/parent coordinates (so ${project.version} and
	// ${some.version} in managed/import entries resolve). Without this, a
	// version reference in an ancestor's dependencyManagement (e.g. an imported
	// BOM's ${platform.version}) would stay unresolved and the fetch would fail.
	localProps := make(map[string]string)
	mergeProperties(localProps, properties)
	if provider != nil && pomDir != "" {
		mergeProperties(localProps, p.resolveParentProperties(content, pomDir, provider, 0))
	}
	mergeProperties(localProps, p.extractProperties(content))
	p.addProjectCoordinates(localProps, project.GroupId, project.ArtifactId, project.Version)
	p.addParentCoordinates(localProps, project.Parent)

	// Direct and profile dependencyManagement (excluding imports) win over
	// ancestors and imports.
	p.addManagedEntries(project.DependencyManagement.Dependencies, localProps, managed)
	for _, profile := range p.getActiveProfiles(project.Profiles) {
		p.addManagedEntries(profile.DependencyManagement.Dependencies, localProps, managed)
	}

	// Follow imported BOMs (scope=import, type=pom) when a resolver is wired.
	p.importBomManagedVersions(project.DependencyManagement.Dependencies, provider, localProps, managed, bomResolver, visited)

	if project.Parent.GroupId == "" {
		return
	}

	// Climb to the parent POM. Prefer a provider/relativePath read (in-repo
	// modules); fall back to fetching the parent by coordinate via the resolver
	// (published POMs, e.g. when crawling a fetched POM whose parent lives in a
	// repository, not on disk).
	if provider != nil && pomDir != "" {
		parentPath := p.resolveParentPath(pomDir, project.Parent.RelativePath)
		if parentContent, err := provider.ReadFile(parentPath); err == nil {
			p.collectManagedVersionsRecursive(string(parentContent), filepath.Dir(parentPath),
				provider, properties, managed, bomResolver, visited, depth+1)
			return
		}
	}
	if bomResolver != nil {
		parentVersion := p.resolvePropertyRefs(project.Parent.Version, localProps, make(map[string]bool))
		if parentContent, parentDir, ok := bomResolver(project.Parent.GroupId, project.Parent.ArtifactId, parentVersion); ok {
			p.collectManagedVersionsRecursive(string(parentContent), parentDir,
				provider, properties, managed, bomResolver, visited, depth+1)
		}
	}
}

// importBomManagedVersions resolves each BOM import (scope=import, type=pom) to
// its POM in the repo (via bomResolver) and merges its managed versions,
// following the BOM's own parent chain and nested BOM imports. The visited set
// guards against import cycles (a BOM that, directly or transitively, imports
// itself). Properties from the importing scope are carried in so the BOM's
// coordinate and version references resolve.
func (p *MavenParser) importBomManagedVersions(deps []MavenDependency, provider types.Provider,
	properties map[string]string, managed map[string]string, bomResolver BomResolver, visited map[string]bool) {
	if bomResolver == nil {
		return
	}
	for _, dep := range deps {
		if dep.Scope != types.ScopeImport || dep.GroupId == "" || dep.ArtifactId == "" {
			continue
		}
		coord := dep.GroupId + ":" + dep.ArtifactId
		if visited[coord] {
			continue
		}
		visited[coord] = true

		version := p.resolvePropertyRefs(dep.Version, properties, make(map[string]bool))
		bomContent, bomDir, ok := bomResolver(dep.GroupId, dep.ArtifactId, version)
		if !ok {
			continue
		}

		// Merge the imported BOM's own properties on top of the importing
		// scope so its managed versions and coordinates resolve.
		bomProps := make(map[string]string)
		mergeProperties(bomProps, properties)
		mergeProperties(bomProps, p.extractProperties(string(bomContent)))

		// Collect the BOM's managed versions, following its own parent chain
		// (via provider) and any nested BOM imports.
		p.collectManagedVersionsRecursive(string(bomContent), bomDir, provider, bomProps, managed, bomResolver, visited, 0)
	}
}

// addManagedEntries records resolved managed versions, keyed by
// "groupId:artifactId", without overwriting closer (already-present) entries.
// BOM imports (scope=import) are skipped here; they are followed separately.
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

// ApplyManagedVersions backfills any unresolved dependency version (empty,
// "latest", "${prop}", range) from a "groupId:artifactId" -> version table,
// leaving already-resolved versions untouched. Exported for the Gradle detector
// to apply versions managed by a platform()/enforcedPlatform() BOM.
func ApplyManagedVersions(deps []types.Dependency, managed map[string]string) {
	(&MavenParser{}).applyManagedVersions(deps, managed)
}
