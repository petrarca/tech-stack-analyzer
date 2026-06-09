package parsers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseMixLockGraph parses mix.lock and returns the package-to-package edges,
// honoring the requested graph mode. It implements the GraphProducer contract.
//
// mix.lock states a real graph: each entry's 6th tuple element lists the
// dependency tuples ({:dep, "constraint", [...]}). Every package is locked at a
// single version, so dep names resolve cleanly to "name@version". Direct deps
// are not marked in mix.lock (they live in mix.exs deps/0, which is Elixir
// code), so direct mode uses the not-referenced heuristic: a package not pulled
// by any other package is a root-level dependency.
func ParseMixLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	entries := parseMixLockEntries(string(input.Lockfile))
	versionByName := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Version != "" {
			versionByName[e.Name] = e.Version
		}
	}
	node := func(name string) string {
		if v, ok := versionByName[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = mixDirectEdges(entries, node)
	case types.DependencyGraphFull:
		result.Edges, result.Unresolved = mixFullEdges(entries, node)
	}
	return result
}

// mixFullEdges builds every package -> dependency edge stated by mix.lock.
func mixFullEdges(entries []mixLockEntry, node func(string) string) (edges []types.DependencyEdge, unresolved []string) {
	for _, e := range entries {
		from := node(e.Name)
		if from == "" {
			continue
		}
		for _, dep := range e.Dependencies {
			if to := node(dep); to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			} else {
				unresolved = append(unresolved, from+" -> "+dep)
			}
		}
	}
	return edges, unresolved
}

// mixDirectEdges builds root -> direct edges. A root is any locked package not
// referenced as a dependency by another package. The synthetic "." marker is
// the from node.
func mixDirectEdges(entries []mixLockEntry, node func(string) string) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, e := range entries {
		for _, dep := range e.Dependencies {
			referenced[dep] = true
		}
	}
	var edges []types.DependencyEdge
	for _, e := range entries {
		if referenced[e.Name] {
			continue
		}
		if to := node(e.Name); to != "" {
			edges = append(edges, types.DependencyEdge{From: ".", To: to})
		}
	}
	return edges
}
