package parsers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// bunLock is the JSONC view of bun.lock (Bun's text lockfile, v1+).
//
//	packages: { "name": ["name@version", registry, info{dependencies,...}, hash] }
//	workspaces: { "": {name, dependencies, devDependencies, ...} }
//
// bun.lock is JSONC (trailing commas), so it is normalized before decoding.
type bunLock struct {
	Packages   map[string]json.RawMessage `json:"packages"`
	Workspaces map[string]bunWorkspace    `json:"workspaces"`
}

type bunWorkspace struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

// bunPackageInfo is the 3rd element of a package entry array.
type bunPackageInfo struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

var bunTrailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// ParseBunLockGraph parses bun.lock and returns the package-to-package edges,
// honoring the requested graph mode. It implements the GraphProducer contract.
//
// Each package entry carries its resolved version (in the identifier) and its
// dependency maps (3rd array element); dependency names resolve to the locked
// version of that package. The "" workspace gives the root's direct deps.
func ParseBunLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	normalized := bunTrailingComma.ReplaceAll(input.Lockfile, []byte("$1"))
	var lock bunLock
	if err := json.Unmarshal(normalized, &lock); err != nil {
		return result
	}

	// name -> locked version (from each package entry's identifier).
	versionByName := make(map[string]string, len(lock.Packages))
	infoByName := make(map[string]bunPackageInfo, len(lock.Packages))
	for name, raw := range lock.Packages {
		ident, info := bunParseEntry(raw)
		if v := bunVersionFromIdent(ident); v != "" {
			versionByName[name] = v
		}
		infoByName[name] = info
	}
	node := func(name string) string {
		if v, ok := versionByName[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = bunDirectEdges(lock.Workspaces, node)
	case types.DependencyGraphFull:
		result.Edges = bunFullEdges(infoByName, node)
	}
	return result
}

// bunParseEntry decodes a package entry array: [ident, registry, info, hash].
func bunParseEntry(raw json.RawMessage) (ident string, info bunPackageInfo) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return "", info
	}
	_ = json.Unmarshal(arr[0], &ident)
	if len(arr) >= 3 {
		_ = json.Unmarshal(arr[2], &info)
	}
	return ident, info
}

// bunVersionFromIdent extracts the version from "name@version" /
// "@scope/name@version". Returns "" for non-version specifiers (file:, etc.).
func bunVersionFromIdent(ident string) string {
	at := strings.LastIndexByte(ident, '@')
	if at <= 0 {
		return ""
	}
	ver := ident[at+1:]
	if ver == "" || strings.ContainsAny(ver, "/:") {
		return "" // workspace/file/link specifier, not a concrete version
	}
	return ver
}

// bunFullEdges builds every package -> dependency edge from each entry's
// dependency maps, deduped.
func bunFullEdges(infoByName map[string]bunPackageInfo, node func(string) string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	seen := make(map[string]bool)
	for name, info := range infoByName {
		from := node(name)
		if from == "" {
			continue
		}
		emit := func(deps map[string]string) {
			for dep := range deps {
				to := node(dep)
				if to == "" || from == to {
					continue
				}
				if key := from + "|" + to; !seen[key] {
					seen[key] = true
					edges = append(edges, types.DependencyEdge{From: from, To: to})
				}
			}
		}
		emit(info.Dependencies)
		emit(info.OptionalDependencies)
		emit(info.PeerDependencies)
	}
	return edges
}

// bunDirectEdges builds root -> direct edges from the "" workspace's dependency
// maps, scoped.
func bunDirectEdges(workspaces map[string]bunWorkspace, node func(string) string) []types.DependencyEdge {
	root, ok := workspaces[""]
	if !ok {
		return nil
	}
	var edges []types.DependencyEdge
	emit := func(deps map[string]string, scope string) {
		for dep := range deps {
			if to := node(dep); to != "" {
				edges = append(edges, types.DependencyEdge{From: ".", To: to, Scope: scope})
			}
		}
	}
	emit(root.Dependencies, types.ScopeProd)
	emit(root.DevDependencies, types.ScopeDev)
	emit(root.OptionalDependencies, types.ScopeOptional)
	emit(root.PeerDependencies, types.ScopePeer)
	return edges
}
