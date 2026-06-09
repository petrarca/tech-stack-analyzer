package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// nugetPackagesLock is the JSON view of packages.lock.json. The top-level
// dependencies map is keyed by target framework; each framework maps package
// names to an entry with a resolved version, a type (Direct/Transitive/
// Project/CentralTransitive), and a nested dependencies map (name -> version
// range) that states the edges.
type nugetPackagesLock struct {
	Version      int                                  `json:"version"`
	Dependencies map[string]map[string]nugetLockEntry `json:"dependencies"`
}

type nugetLockEntry struct {
	Type         string            `json:"type"`
	Resolved     string            `json:"resolved"`
	Dependencies map[string]string `json:"dependencies"`
}

// ParsePackagesLockGraph parses packages.lock.json and returns the
// package-to-package edges, honoring the requested graph mode. It implements
// the GraphProducer contract (ParseGraphFunc).
//
// The lockfile is self-describing: type == "Direct" marks direct dependencies,
// and each entry's dependencies map states its edges. Versions are resolved per
// (framework, name); edges resolve to "name@version". Frameworks are merged
// (an edge is emitted once per from|to across all target frameworks).
func ParsePackagesLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock nugetPackagesLock
	if err := json.Unmarshal(input.Lockfile, &lock); err != nil || len(lock.Dependencies) == 0 {
		return result
	}

	seen := make(map[string]bool)
	var unresolved []string

	for _, framework := range lock.Dependencies {
		// Resolve names to versions within this framework.
		versionOf := func(name string) string {
			if e, ok := framework[name]; ok && e.Resolved != "" {
				return name + "@" + e.Resolved
			}
			return ""
		}
		for name, entry := range framework {
			if entry.Resolved == "" {
				continue // Project entries have no resolved version
			}
			switch input.Mode {
			case types.DependencyGraphDirect:
				if entry.Type == "Direct" {
					addNugetEdge(&result.Edges, seen, ".", name+"@"+entry.Resolved, "")
				}
			case types.DependencyGraphFull:
				from := name + "@" + entry.Resolved
				for depName := range entry.Dependencies {
					if to := versionOf(depName); to != "" {
						addNugetEdge(&result.Edges, seen, from, to, "")
					} else {
						unresolved = append(unresolved, from+" -> "+depName)
					}
				}
			}
		}
	}
	result.Unresolved = unresolved
	return result
}

// addNugetEdge appends an edge once (deduped across target frameworks).
func addNugetEdge(edges *[]types.DependencyEdge, seen map[string]bool, from, to, scope string) {
	key := from + "|" + to
	if seen[key] {
		return
	}
	seen[key] = true
	*edges = append(*edges, types.DependencyEdge{From: from, To: to, Scope: scope})
}
