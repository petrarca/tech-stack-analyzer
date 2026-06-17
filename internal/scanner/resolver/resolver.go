// Package resolver defines the seam for producing package-to-package
// dependency edges from different sources behind a single interface.
//
// Two resolution sources are interchangeable at the edge level (both emit the
// same name@version DAG; see docs/design/dependency-graph.md):
//
//   - lockfile: offline, reflects the repo's own resolved state (authoritative)
//   - deps.dev: online, reflects published package versions (approximation)
//
// A Chain runs resolvers in precedence order (local-first), so the online
// fallback only fills gaps where no lockfile/tree-file is present. Resolution
// is gated by the dependency-graph mode and, for online sources, an explicit
// opt-in -- the offline guarantee is preserved by default.
package resolver

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Provenance identifies where a set of edges came from, recorded on each edge
// for downstream trust decisions.
type Provenance string

const (
	// SourceLockfile marks edges resolved locally from a lockfile or
	// pre-generated tree file. Authoritative for the scanned repo.
	SourceLockfile Provenance = "lockfile"
	// SourceDepsDev marks edges resolved online via deps.dev. An approximation
	// keyed by published version, not the repo's own resolution.
	SourceDepsDev Provenance = "deps.dev"
	// SourceMavenRepo marks edges resolved by crawling Maven POMs from the
	// repository chain (in-repo, ~/.m2, configured remote). Covers private
	// artifacts and reflects each POM's own declared dependencies.
	SourceMavenRepo Provenance = "maven-repo"
)

// Request carries everything a resolver needs to produce edges for one
// component, independent of how it resolves them.
type Request struct {
	// Ecosystem is the package ecosystem ("nodejs", "python", "rust",
	// "java", ...). Resolvers map this to their own system identifiers.
	Ecosystem string
	// Dir is the absolute path of the component directory to read files from.
	Dir string
	// Provider reads files within Dir (lockfiles, tree files, manifests).
	Provider types.Provider
	// Mode is the requested emission depth (off/direct/full). A resolver must
	// honor direct vs full and return nil edges for off.
	Mode types.DependencyGraphMode
	// Dependencies is the ordered list of declared dependency coordinates for
	// resolvers that work by coordinate rather than by reading files (e.g.
	// deps.dev). Each entry is a (name, version) pair for a declared dep.
	// Online resolvers fan-out over this list and union the results, so a
	// typical application project with N declared deps produces N calls and the
	// correct full transitive graph without requiring the project itself to be
	// a published artifact. Empty for file-based resolvers.
	Dependencies []Coordinates
}

// Coordinates identifies a published package version for coordinate-based
// resolution.
type Coordinates struct {
	Name    string // ecosystem package name (Maven: groupId:artifactId)
	Version string
}

// Result is a resolver's output: the edges it produced and their provenance.
// Resolved is false when the resolver could not handle the request (e.g. no
// lockfile present), signalling the Chain to try the next resolver.
type Result struct {
	Edges    []types.DependencyEdge
	Source   Provenance
	Resolved bool
	// Unresolved lists dependency references that could not be resolved to a
	// known node (lockfile drift, unparseable refs). Reported, not dropped.
	Unresolved []string
}

// DependencyResolver produces package-to-package edges for a component from a
// single source. Implementations must be pure with respect to the request and
// must not emit edges when Request.Mode is off.
type DependencyResolver interface {
	// Name is a short identifier for logging/diagnostics.
	Name() string
	// Resolve returns edges for the request. It returns Resolved=false (and no
	// error) when this resolver does not apply, so the Chain can fall through
	// to the next one. An error is reserved for genuine failures.
	Resolve(req Request) (Result, error)
}
