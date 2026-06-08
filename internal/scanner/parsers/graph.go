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
	// Unresolved lists dependency references the lockfile stated but whose
	// target could not be resolved to a known "name@version" node (e.g.
	// lockfile drift, an unparseable reference). These are reported rather than
	// silently dropped so consumers can detect gaps. Each entry is the raw
	// reference (e.g. "from -> depName").
	Unresolved []string
}

// GraphInput is the input to a graph producer. It carries the lockfile (or
// pre-generated tree file) content, the optional manifest content for the same
// component (e.g. package.json, Cargo.toml, pyproject.toml), and the requested
// emission mode.
//
// The manifest is used to derive direct dependencies accurately for the direct
// mode and to classify edge scope (dev/test/prod/optional/peer). It may be nil
// when no manifest is present or applicable (e.g. a pre-generated tree file is
// self-describing); producers must tolerate a nil Manifest.
type GraphInput struct {
	Lockfile []byte
	Manifest []byte
	Mode     types.DependencyGraphMode
}

// GraphProducer is the contract a lockfile parser implements to expose the
// dependency graph in addition to the flat dependency list. The graph is read
// from the lockfile, not resolved.
//
// Mode controls how much graph to emit:
//   - DependencyGraphOff:    no edges (Edges is nil)
//   - DependencyGraphDirect: only root -> direct dependency edges
//   - DependencyGraphFull:   the full transitive package-to-package graph
//
// Parsers are package-level functions, so this contract is expressed as a
// function shape rather than a Go interface; ParseGraphFunc documents it.
type GraphProducer interface {
	ParseGraph(input GraphInput) LockGraph
}

// ParseGraphFunc is the function form of GraphProducer used by package-level
// lockfile parsers.
type ParseGraphFunc func(input GraphInput) LockGraph
