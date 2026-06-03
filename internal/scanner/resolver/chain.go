package resolver

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Chain runs resolvers in precedence order and returns the first one that
// resolves. Local (lockfile) resolvers come first; online (deps.dev) resolvers
// are the fallback, so they only fill gaps where no local resolution exists.
//
// The first resolver that returns Resolved=true wins -- its edges are tagged
// with its provenance and returned. This mirrors the lockfile-priority rule
// (first existing source wins) and extends it across resolution sources.
type Chain struct {
	resolvers []DependencyResolver
}

// NewChain builds a Chain from resolvers in precedence order (highest first).
func NewChain(resolvers ...DependencyResolver) *Chain {
	return &Chain{resolvers: resolvers}
}

// Resolve tries each resolver in order. Off mode short-circuits to no edges.
// The first resolver to resolve returns its (provenance-tagged) edges; if none
// resolve, the result is empty and Resolved=false.
func (c *Chain) Resolve(req Request) (Result, error) {
	if req.Mode == types.DependencyGraphOff {
		return Result{}, nil
	}
	for _, r := range c.resolvers {
		res, err := r.Resolve(req)
		if err != nil {
			return Result{}, err
		}
		if !res.Resolved {
			continue
		}
		res.Edges = tagSource(res.Edges, res.Source)
		return res, nil
	}
	return Result{}, nil
}

// tagSource stamps each edge with the resolver's provenance, unless already set.
func tagSource(edges []types.DependencyEdge, source Provenance) []types.DependencyEdge {
	if source == "" {
		return edges
	}
	for i := range edges {
		if edges[i].Source == "" {
			edges[i].Source = string(source)
		}
	}
	return edges
}
