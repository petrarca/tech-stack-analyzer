package parsers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LockGraph is the result of parsing a lockfile that can express a
// package-to-package dependency graph: the dependency nodes plus the edges
// stated by the lockfile. Parsers that cannot produce edges return a nil
// Edges slice.
type LockGraph struct {
	Dependencies []types.Dependency
	Edges        []types.DependencyEdge
}

// GraphProducer is the contract a lockfile parser implements to expose the
// dependency graph in addition to the flat dependency list. The graph is read
// from the lockfile, not resolved.
//
// mode controls how much graph to emit:
//   - DependencyGraphOff:    no edges (Edges is nil)
//   - DependencyGraphDirect: only root -> direct dependency edges
//   - DependencyGraphFull:   the full transitive package-to-package graph
//
// Parsers are package-level functions, so this contract is expressed as a
// function shape rather than a Go interface; ParseGraphFunc documents it.
type GraphProducer interface {
	ParseGraph(content []byte, mode types.DependencyGraphMode) LockGraph
}

// ParseGraphFunc is the function form of GraphProducer used by package-level
// lockfile parsers.
type ParseGraphFunc func(content []byte, mode types.DependencyGraphMode) LockGraph
