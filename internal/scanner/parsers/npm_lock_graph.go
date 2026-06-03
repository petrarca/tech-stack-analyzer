package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// packageLockGraphFile is a minimal view of package-lock.json for graph
// extraction. The v3 packages section keys are node_modules paths; each entry
// names its dependency ranges, which form the edges of the resolved graph. The
// target version is resolved by walking node_modules (npm's nearest-wins rule).
type packageLockGraphFile struct {
	Packages map[string]packageLockGraphEntry `json:"packages"`
}

type packageLockGraphEntry struct {
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

// ParsePackageLockGraph parses package-lock.json and returns the dependencies
// plus the package-to-package edges, honoring the requested graph mode. It
// implements the GraphProducer contract (ParseGraphFunc). Only the v3 packages
// format carries per-package dependency maps; v1/v2-only lockfiles yield no
// edges.
func ParsePackageLockGraph(content []byte, mode types.DependencyGraphMode) LockGraph {
	// Flat parser is best-effort without package.json (no scope/declared info).
	result := LockGraph{Dependencies: ParsePackageLock(content, nil)}

	if mode == types.DependencyGraphOff {
		return result
	}

	var lock packageLockGraphFile
	if err := json.Unmarshal(content, &lock); err != nil || len(lock.Packages) == 0 {
		return result
	}

	switch mode {
	case types.DependencyGraphDirect:
		result.Edges = npmDirectEdges(lock)
	case types.DependencyGraphFull:
		result.Edges = npmFullEdges(lock)
	}
	return result
}

// npmResolveDep resolves a dependency name referenced from fromPath to its
// locked "name@version" node, applying npm's nearest-wins node_modules lookup:
// check fromPath/node_modules/name, then walk up to the root.
func npmResolveDep(packages map[string]packageLockGraphEntry, fromPath, name string) string {
	prefix := fromPath
	for {
		var candidate string
		if prefix == "" {
			candidate = "node_modules/" + name
		} else {
			candidate = prefix + "/node_modules/" + name
		}
		if entry, ok := packages[candidate]; ok && entry.Version != "" {
			return name + "@" + entry.Version
		}
		if prefix == "" {
			return "" // reached root, unresolved
		}
		// Walk up one node_modules level.
		if i := strings.LastIndex(prefix, "/node_modules/"); i >= 0 {
			prefix = prefix[:i]
		} else {
			prefix = ""
		}
	}
}

// npmFullEdges builds every package -> dependency edge stated by the v3
// packages section.
func npmFullEdges(lock packageLockGraphFile) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for path, entry := range lock.Packages {
		from := npmNodeFromPath(path, entry)
		if from == "" {
			continue
		}
		emit := func(deps map[string]string) {
			for name := range deps {
				if to := npmResolveDep(lock.Packages, path, name); to != "" {
					edges = append(edges, types.DependencyEdge{From: from, To: to})
				}
			}
		}
		emit(entry.Dependencies)
		emit(entry.OptionalDependencies)
		emit(entry.PeerDependencies)
	}
	return edges
}

// npmDirectEdges builds root -> direct-dependency edges from the root package
// entry (the "" path key). The synthetic "." marker is the from node.
func npmDirectEdges(lock packageLockGraphFile) []types.DependencyEdge {
	root, ok := lock.Packages[""]
	if !ok {
		return nil
	}
	var edges []types.DependencyEdge
	emit := func(deps map[string]string) {
		for name := range deps {
			if to := npmResolveDep(lock.Packages, "", name); to != "" {
				edges = append(edges, types.DependencyEdge{From: ".", To: to})
			}
		}
	}
	emit(root.Dependencies)
	emit(root.OptionalDependencies)
	emit(root.PeerDependencies)
	return edges
}

// npmNodeFromPath builds the "name@version" node id for a packages entry. The
// root entry ("" path) has no node id (it is the synthetic root).
func npmNodeFromPath(path string, entry packageLockGraphEntry) string {
	if path == "" || entry.Version == "" {
		return ""
	}
	name := extractNameFromNodeModulesPath(path)
	if name == "" {
		return ""
	}
	return name + "@" + entry.Version
}
