package resolver

import (
	"errors"

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

// OnlineGraphResolver is the pluggable contract for resolving a package's
// dependency graph from an online, coordinate-keyed source. deps.dev is the
// reference implementation; a mirror or alternative service exposing the same
// data can provide another implementation without touching the chain or
// detectors.
//
// system is the source's ecosystem identifier (already mapped from our
// ecosystem vocabulary, e.g. "maven", "npm"). The returned edges use the
// "name@version" node identity and the "." synthetic root, matching the local
// resolvers. An unknown coordinate returns (nil, nil), not an error.
type OnlineGraphResolver interface {
	ResolveGraph(system, name, version string, mode types.DependencyGraphMode) ([]types.DependencyEdge, error)
}

// DepsDevFetcher is the function form of OnlineGraphResolver, for lightweight
// implementations and tests.
type DepsDevFetcher func(system, name, version string, mode types.DependencyGraphMode) ([]types.DependencyEdge, error)

// ResolveGraph lets a DepsDevFetcher satisfy OnlineGraphResolver.
func (f DepsDevFetcher) ResolveGraph(system, name, version string, mode types.DependencyGraphMode) ([]types.DependencyEdge, error) {
	return f(system, name, version, mode)
}

// DepsDevResolver resolves edges online as a fallback for manifest-only
// ecosystems (Maven, Gradle) where no local lockfile/tree-file is present. It
// crosses the offline boundary and is therefore opt-in: with no Online resolver
// wired it is a no-op that always falls through.
//
// Online edges are an approximation keyed by published version (not the repo's
// own resolution) and are runtime-scoped (no test/provided). They are tagged
// with SourceDepsDev so downstream can distinguish them from authoritative
// lockfile edges. See docs/design/dependency-graph.md.
type DepsDevResolver struct {
	// Enabled gates online resolution. False (default) makes Resolve a no-op,
	// preserving the offline guarantee.
	Enabled bool
	// Online performs the resolution. Nil makes Resolve a no-op even when
	// Enabled, so the resolver is safe to construct unconditionally. Pluggable:
	// deps.dev today, an alternative source or mirror in the future.
	Online OnlineGraphResolver
}

// Name implements DependencyResolver.
func (r *DepsDevResolver) Name() string { return "deps.dev" }

// Resolve queries the online source when enabled, a resolver is wired, the
// ecosystem is supported, and the root coordinates are known. Otherwise it
// falls through (Resolved=false) so the Chain leaves edge production to local
// resolvers or emits nothing.
func (r *DepsDevResolver) Resolve(req Request) (Result, error) {
	if !r.Enabled || r.Online == nil {
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

	edges, err := r.Online.ResolveGraph(system, req.Coordinates.Name, req.Coordinates.Version, req.Mode)
	if err != nil {
		// ErrCoordinateNotFound means the service does not know this coordinate;
		// treat as "not applicable" so the chain can fall through (F-09).
		if errors.Is(err, ErrCoordinateNotFound) {
			return Result{Resolved: false}, nil
		}
		return Result{}, err
	}
	return Result{
		Edges:    edges,
		Source:   SourceDepsDev,
		Resolved: true,
	}, nil
}
