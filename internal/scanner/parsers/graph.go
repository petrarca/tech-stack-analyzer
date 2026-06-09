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

// ParseGraphFunc is the function type for lockfile graph producers.
type ParseGraphFunc func(input GraphInput) LockGraph

// LockfileProducer pairs a lockfile (or pre-generated tree file) name with its
// graph parser, plus an optional manifest filename for the same component
// (e.g. Cargo.toml for Cargo.lock, package.json for package-lock.json). The
// list supplied to a resolver is ordered: the first file that exists wins,
// matching each ecosystem's flat-extraction priority.
//
// Defined here (in parsers) so both the resolver and components packages can
// use the single canonical type without a duplicate or an adapter.
type LockfileProducer struct {
	Lockfile string
	Manifest string // optional; read and passed to the producer for direct/scope
	Parse    ParseGraphFunc
}
