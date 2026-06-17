package mavenresolve

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// HybridResolver resolves the Maven transitive graph using deps.dev for the
// public set and the repository crawl only for what deps.dev cannot resolve
// (private artifacts deps.dev returns 404 for). This is fast -- deps.dev
// returns each public coordinate's whole subtree in one request, avoiding the
// per-POM crawl of the large public tree -- while still covering private
// artifacts via the repo chain.
//
// It is used as the effective Maven graph source when deps.dev is selected and
// a repository chain is available. When only deps.dev is selected (no repo) it
// degrades to deps.dev alone; the privacy-strict repo-only mode never uses this
// resolver (it must not contact deps.dev).
type HybridResolver struct {
	depsDev resolver.DependencyResolver // public graph (deps.dev)
	repo    *GraphResolver              // private fallback (repo crawl)
}

// NewHybridResolver composes deps.dev with a repo-crawl fallback. Both must be
// non-nil; otherwise callers should use whichever single resolver they have.
func NewHybridResolver(depsDev resolver.DependencyResolver, repo *GraphResolver) *HybridResolver {
	if depsDev == nil || repo == nil {
		return nil
	}
	return &HybridResolver{depsDev: depsDev, repo: repo}
}

// Name implements resolver.DependencyResolver.
func (h *HybridResolver) Name() string { return "maven-hybrid" }

// Resolve runs deps.dev first, then crawls only the coordinates deps.dev could
// not resolve (its Unresolved list), and unions the edges.
func (h *HybridResolver) Resolve(req resolver.Request) (resolver.Result, error) {
	ddRes, err := h.depsDev.Resolve(req)
	if err != nil {
		return resolver.Result{}, err
	}

	// Crawl only the coordinates deps.dev did not resolve (private artifacts).
	fallbackDeps := coordinatesFromUnresolved(ddRes.Unresolved)
	if len(fallbackDeps) == 0 {
		return ddRes, nil
	}

	repoReq := req
	repoReq.Dependencies = fallbackDeps
	repoRes, err := h.repo.Resolve(repoReq)
	if err != nil {
		return resolver.Result{}, err
	}

	edges := unionEdges(ddRes.Edges, repoRes.Edges)
	if len(edges) == 0 {
		return resolver.Result{Resolved: false}, nil
	}
	return resolver.Result{
		Edges:    edges,
		Source:   resolver.SourceMavenRepo, // mixed; repo tag marks the crawled part
		Resolved: true,
	}, nil
}

// coordinatesFromUnresolved parses "name@version" entries into Coordinates,
// skipping any without a concrete version.
func coordinatesFromUnresolved(unresolved []string) []resolver.Coordinates {
	var out []resolver.Coordinates
	for _, u := range unresolved {
		i := strings.LastIndex(u, "@")
		if i <= 0 || i == len(u)-1 {
			continue
		}
		out = append(out, resolver.Coordinates{Name: u[:i], Version: u[i+1:]})
	}
	return out
}

// unionEdges concatenates two edge sets, deduplicating on from|to.
func unionEdges(a, b []types.DependencyEdge) []types.DependencyEdge {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]types.DependencyEdge, 0, len(a)+len(b))
	for _, set := range [][]types.DependencyEdge{a, b} {
		for _, e := range set {
			key := e.From + "|" + e.To
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, e)
		}
	}
	return out
}
