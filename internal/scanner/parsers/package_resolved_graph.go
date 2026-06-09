package parsers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParsePackageResolvedGraph parses Package.resolved into root-rooted edges. It
// implements the GraphProducer contract.
//
// Package.resolved is a flat pin list: it records each resolved package and its
// version but NOT package-to-package edges (Swift Package Manager keeps the
// dependency relationships in Package.swift, which is Swift code, not the
// lockfile). The graph is therefore rooted at the synthetic "." node, with one
// edge per resolved pin. This captures the full reachable set; finer edges are
// not stated by the lockfile (a known limitation shared with other flat-pin
// lockfiles such as pubspec.lock).
//
// direct and full modes produce the same edge set here, since the lockfile does
// not distinguish direct from transitive pins.
func ParsePackageResolvedGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	for _, dep := range (&SwiftParser{}).ParsePackageResolved(string(input.Lockfile)) {
		result.Edges = append(result.Edges, types.DependencyEdge{
			From: ".",
			To:   dep.Name + "@" + dep.Version,
		})
	}
	return result
}
