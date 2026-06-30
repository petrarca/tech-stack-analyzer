package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// runtimePackPrefix is the synthetic name prefix the .NET SDK gives the bundled
// runtime in a self-contained app's .deps.json (e.g.
// "runtimepack.Microsoft.NETCore.App.Runtime.linux-x64"). It is stripped so the
// runtime resolves to the same node a framework-dependent app would use.
const runtimePackPrefix = "runtimepack."

// dotnetDepsFile is the JSON view of a .NET Core <App>.deps.json file. The
// libraries section lists every resolved package (with its type); the targets
// section, keyed by the runtime target, states the dependency edges and direct
// set. See https://github.com/dotnet/sdk for the format.
type dotnetDepsFile struct {
	RuntimeTarget dotnetRuntimeTarget                   `json:"runtimeTarget"`
	Libraries     map[string]dotnetDepsLibrary          `json:"libraries"`
	Targets       map[string]map[string]dotnetTargetLib `json:"targets"`
}

type dotnetRuntimeTarget struct {
	Name string `json:"name"`
}

type dotnetDepsLibrary struct {
	Type string `json:"type"` // package | project | runtimepack | reference ...
}

type dotnetTargetLib struct {
	Dependencies map[string]string `json:"dependencies"`
}

// ParseDotNetDepsGraph parses a .NET Core <App>.deps.json file and returns the
// fully-resolved transitive dependency graph. A .deps.json is a build artifact
// the .NET SDK emits with the complete resolved closure (direct, transitive,
// and the bundled runtime), so it is the highest-fidelity NuGet graph source
// when present -- no lockfile opt-in and no network required.
//
// It implements the ParseGraphFunc contract. Edges are emitted as
// "name@version"; direct dependencies are linked from the synthetic "." root.
// Names carry the synthetic "runtimepack." prefix stripped (Trivy parity), so
// runtime packs and the targets references that point at them share one node.
func ParseDotNetDepsGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var deps dotnetDepsFile
	if err := json.Unmarshal(input.Lockfile, &deps); err != nil || len(deps.Libraries) == 0 {
		return result
	}

	normalized := normalizeTargets(deps.Targets[deps.RuntimeTarget.Name])
	nodes, rootID := collectDepsNodes(deps.Libraries)
	if len(nodes) == 0 {
		return result
	}

	seen := make(map[string]bool)

	// Direct dependencies: the root project's entry in the targets section.
	if rootID != "" {
		for depName, depVer := range normalized[rootID].Dependencies {
			if to := nodeID(depName, depVer); nodes[to] {
				addNugetEdge(&result.Edges, seen, ".", to, "")
			}
		}
	}
	if input.Mode == types.DependencyGraphDirect {
		return result
	}

	result.Unresolved = appendTransitiveEdges(&result.Edges, seen, nodes, rootID, normalized)
	return result
}

// normalizeTargets maps a .deps.json targets section (keyed by "name/version")
// to "name@version" node ids, so its dependency entries align with the node set.
func normalizeTargets(targetLibs map[string]dotnetTargetLib) map[string]dotnetTargetLib {
	normalized := make(map[string]dotnetTargetLib, len(targetLibs))
	for key, lib := range targetLibs {
		name, version := cutNameVersion(key)
		normalized[nodeID(name, version)] = lib
	}
	return normalized
}

// collectDepsNodes builds the set of valid graph nodes from the libraries
// section and returns the root project's node id (empty if none).
func collectDepsNodes(libraries map[string]dotnetDepsLibrary) (nodes map[string]bool, rootID string) {
	nodes = make(map[string]bool, len(libraries))
	for nameVer, lib := range libraries {
		if !isGraphableLibrary(lib.Type) {
			continue
		}
		name, version := cutNameVersion(nameVer)
		if name == "" {
			continue
		}
		id := nodeID(name, version)
		nodes[id] = true
		if strings.EqualFold(lib.Type, "project") {
			rootID = id
		}
	}
	return nodes, rootID
}

// appendTransitiveEdges emits every non-root node's edges from the targets
// section and returns the list of references that could not be resolved to a
// known node.
func appendTransitiveEdges(edges *[]types.DependencyEdge, seen map[string]bool, nodes map[string]bool, rootID string, normalized map[string]dotnetTargetLib) []string {
	var unresolved []string
	for id := range nodes {
		if id == rootID {
			continue // root edges already emitted from "."
		}
		for depName, depVer := range normalized[id].Dependencies {
			if to := nodeID(depName, depVer); nodes[to] {
				addNugetEdge(edges, seen, id, to, "")
			} else {
				unresolved = append(unresolved, id+" -> "+depName)
			}
		}
	}
	return unresolved
}

// isGraphableLibrary reports whether a .deps.json library type contributes a
// graph node. package/project/runtimepack are real components; reference and
// other synthetic types are skipped.
func isGraphableLibrary(libType string) bool {
	switch strings.ToLower(libType) {
	case "package", "project", "runtimepack":
		return true
	default:
		return false
	}
}

// cutNameVersion splits a ".deps.json" "name/version" key into its components.
// It is a pure splitter; stripping of the runtimepack prefix is the caller's
// responsibility (nodeID does it).
func cutNameVersion(key string) (name, version string) {
	name, version, _ = strings.Cut(key, "/")
	return name, version
}

// nodeID builds the "name@version" node identifier used across all graph
// producers. It is the single place that strips the synthetic "runtimepack."
// prefix the .NET SDK adds to bundled runtime entries, so all callers -- both
// those coming via cutNameVersion (libraries/targets keys) and those using raw
// dependency-map names -- normalise to the same node identity.
func nodeID(name, version string) string {
	name = strings.TrimPrefix(name, runtimePackPrefix)
	if version == "" {
		return name
	}
	return name + "@" + version
}
