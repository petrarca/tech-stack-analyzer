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

	// The targets section keyed by the runtime target name carries the edges.
	// Normalize its keys (name/version) to the stripped "name@version" node id.
	targetLibs := deps.Targets[deps.RuntimeTarget.Name]
	normalized := make(map[string]dotnetTargetLib, len(targetLibs))
	for key, lib := range targetLibs {
		name, version := cutNameVersion(key)
		normalized[nodeID(name, version)] = lib
	}

	// Build the set of valid package nodes from the libraries section, and find
	// the root project.
	nodes := make(map[string]bool, len(deps.Libraries))
	rootID := ""
	for nameVer, lib := range deps.Libraries {
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
	if len(nodes) == 0 {
		return result
	}

	seen := make(map[string]bool)
	var unresolved []string

	// Direct dependencies: the root project's entry in the targets section.
	directOf := map[string]bool{}
	if rootID != "" {
		for depName, depVer := range normalized[rootID].Dependencies {
			to := nodeID(depName, depVer)
			if nodes[to] {
				directOf[to] = true
				addNugetEdge(&result.Edges, seen, ".", to, "")
			}
		}
	}

	if input.Mode == types.DependencyGraphDirect {
		result.Unresolved = unresolved
		return result
	}

	// Full transitive graph: every node's edges from the targets section.
	for id := range nodes {
		if id == rootID {
			continue // root edges already emitted from "."
		}
		for depName, depVer := range normalized[id].Dependencies {
			to := nodeID(depName, depVer)
			if nodes[to] {
				addNugetEdge(&result.Edges, seen, id, to, "")
			} else {
				unresolved = append(unresolved, id+" -> "+depName)
			}
		}
	}

	result.Unresolved = unresolved
	return result
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

// cutNameVersion splits a ".deps.json" "name/version" key and strips the
// synthetic runtimepack prefix from the name.
func cutNameVersion(key string) (name, version string) {
	name, version, _ = strings.Cut(key, "/")
	name = strings.TrimPrefix(name, runtimePackPrefix)
	return name, version
}

// nodeID builds the "name@version" node identifier used across all graph
// producers. A missing version yields just the name (kept detectable).
func nodeID(name, version string) string {
	name = strings.TrimPrefix(name, runtimePackPrefix)
	if version == "" {
		return name
	}
	return name + "@" + version
}
