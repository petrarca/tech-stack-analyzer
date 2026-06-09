package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseRenvLockGraph parses renv.lock and returns the package-to-package edges.
// It implements the GraphProducer contract.
//
// renv.lock states a real graph: each package's Requirements array lists the
// package names it depends on, resolved to the locked version in Packages.
// Direct deps are not marked (renv tracks them in DESCRIPTION/.R sources), so
// direct mode uses the not-referenced heuristic. Base/recommended R packages
// (R, stats, utils, ...) are skipped -- they are not CRAN nodes.
func ParseRenvLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock renvLock
	if err := json.Unmarshal(input.Lockfile, &lock); err != nil {
		return result
	}

	versionByName := make(map[string]string, len(lock.Packages))
	for key, pkg := range lock.Packages {
		name := pkg.Package
		if name == "" {
			name = key
		}
		if pkg.Version != "" {
			versionByName[name] = pkg.Version
		}
	}
	node := func(name string) string {
		if rBasePackage(name) {
			return ""
		}
		if v, ok := versionByName[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = renvDirectEdges(lock, node)
	case types.DependencyGraphFull:
		result.Edges = renvFullEdges(lock, node)
	}
	return result
}

// renvFullEdges builds every package -> requirement edge, deduped.
func renvFullEdges(lock renvLock, node func(string) string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	seen := make(map[string]bool)
	for key, pkg := range lock.Packages {
		name := pkg.Package
		if name == "" {
			name = key
		}
		from := node(name)
		if from == "" {
			continue
		}
		for _, req := range pkg.Requirements {
			to := node(req)
			if to == "" || to == from {
				continue
			}
			if k := from + "|" + to; !seen[k] {
				seen[k] = true
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges
}

// renvDirectEdges builds root -> direct edges. A root is any package not listed
// in another package's Requirements.
func renvDirectEdges(lock renvLock, node func(string) string) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, pkg := range lock.Packages {
		for _, req := range pkg.Requirements {
			if to := node(req); to != "" {
				referenced[to] = true
			}
		}
	}
	var edges []types.DependencyEdge
	for key, pkg := range lock.Packages {
		name := pkg.Package
		if name == "" {
			name = key
		}
		n := node(name)
		if n == "" || referenced[n] {
			continue
		}
		edges = append(edges, types.DependencyEdge{From: ".", To: n})
	}
	return edges
}
