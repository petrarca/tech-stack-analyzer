package resolver

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// depsDevSystems maps our ecosystem names to deps.dev "system" identifiers.
// Ecosystems not listed are unsupported by this resolver.
var depsDevSystems = map[string]string{
	"nodejs": "npm",
	"python": "pypi",
	"rust":   "cargo",
	"java":   "maven",
	"go":     "go",
	"dotnet": "nuget",
	"ruby":   "rubygems",
}

// DepsDevFetcher fetches the resolved dependency graph for a published package
// version from deps.dev. It is injected so the resolver stays offline and
// testable by default; the real implementation performs the HTTPS call
//
//	GET /v3/systems/{system}/packages/{name}/versions/{version}:dependencies
//
// and is wired only when online resolution is explicitly enabled.
type DepsDevFetcher func(system, name, version string, mode types.DependencyGraphMode) ([]types.DependencyEdge, error)

// DepsDevResolver resolves edges online via deps.dev as a fallback for
// manifest-only ecosystems (Maven, Gradle) where no local lockfile/tree-file is
// present. It crosses the offline boundary and is therefore opt-in: with no
// fetcher wired it is a no-op that always falls through.
//
// deps.dev edges are an approximation keyed by published version (not the
// repo's own resolution) and are runtime-scoped (no test/provided). They are
// tagged with SourceDepsDev so downstream can distinguish them from
// authoritative lockfile edges. See docs/design/dependency-graph.md.
type DepsDevResolver struct {
	// Enabled gates online resolution. False (default) makes Resolve a no-op,
	// preserving the offline guarantee.
	Enabled bool
	// Fetch performs the network call. Nil makes Resolve a no-op even when
	// Enabled, so the resolver is safe to construct unconditionally.
	Fetch DepsDevFetcher
}

// Name implements DependencyResolver.
func (r *DepsDevResolver) Name() string { return "deps.dev" }

// Resolve queries deps.dev when enabled, a fetcher is wired, the ecosystem is
// supported, and the root coordinates are known. Otherwise it falls through
// (Resolved=false) so the Chain leaves edge production to local resolvers or
// emits nothing.
func (r *DepsDevResolver) Resolve(req Request) (Result, error) {
	if !r.Enabled || r.Fetch == nil {
		return Result{Resolved: false}, nil
	}
	system, ok := depsDevSystems[req.Ecosystem]
	if !ok {
		return Result{Resolved: false}, nil
	}
	if req.Coordinates == nil || req.Coordinates.Name == "" || req.Coordinates.Version == "" {
		// Coordinate-based resolution needs the root package version, which the
		// detector must supply. Without it, fall through.
		return Result{Resolved: false}, nil
	}

	edges, err := r.Fetch(system, req.Coordinates.Name, req.Coordinates.Version, req.Mode)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Edges:    edges,
		Source:   SourceDepsDev,
		Resolved: true,
	}, nil
}
