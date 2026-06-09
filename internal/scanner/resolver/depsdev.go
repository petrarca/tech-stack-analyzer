package resolver

import (
	"errors"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// depsDevSystems maps our ecosystem names to deps.dev "system" identifiers.
// Ecosystems not listed are unsupported by this resolver.
var depsDevSystems = map[string]string{
	"nodejs":    "npm",
	"python":    "pypi",
	"rust":      "cargo",
	"java":      "maven", // kept for explicit "java" ecosystem
	"maven":     "maven", // payload.ComponentType for Maven projects
	"gradle":    "maven", // Gradle artifacts use Maven coordinates on deps.dev
	"go":        "go",
	"dotnet":    "nuget",
	"ruby":      "rubygems",
	"perl":      "cpan",
	"r":         "cran",
	"dart":      "pub",
	"elixir":    "hex",
	"swift":     "swift",
	"cplusplus": "conan",
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

// Resolve fans out over req.Dependencies, queries the online source for each
// declared dependency, and unions the results. This is the correct strategy for
// typical application projects: the project itself is usually not a published
// artifact, but its declared dependencies are. Each dep gets one call; 404s are
// skipped (the dep may be private/internal); the remaining results are merged.
//
// Falls through (Resolved=false) when disabled, no resolver is wired, the
// ecosystem is not supported, or the dependency list is empty.
func (r *DepsDevResolver) Resolve(req Request) (Result, error) {
	if !r.Enabled || r.Online == nil {
		return Result{Resolved: false}, nil
	}
	system, ok := depsDevSystems[req.Ecosystem]
	if !ok {
		return Result{Resolved: false}, nil
	}
	if len(req.Dependencies) == 0 {
		return Result{Resolved: false}, nil
	}

	seen := make(map[string]bool)
	var allEdges []types.DependencyEdge
	anyResolved := false

	for _, dep := range req.Dependencies {
		if dep.Name == "" || dep.Version == "" {
			continue
		}
		edges, err := r.Online.ResolveGraph(system, dep.Name, dep.Version, req.Mode)
		if err != nil {
			if errors.Is(err, ErrCoordinateNotFound) {
				// Private/internal dep: skip, continue with the rest.
				continue
			}
			return Result{}, err
		}
		anyResolved = true
		for _, e := range edges {
			if key := e.From + "|" + e.To; !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}
	}

	if !anyResolved {
		// All deps were either 404 or skipped; treat as not resolvable so the
		// chain can fall through rather than returning an empty authoritative result.
		return Result{Resolved: false}, nil
	}
	return Result{
		Edges:    allEdges,
		Source:   SourceDepsDev,
		Resolved: true,
	}, nil
}
