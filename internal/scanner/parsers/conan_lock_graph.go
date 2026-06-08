package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// conanLock is the JSON view of conan.lock. v1 expresses a real graph under
// graph_lock.nodes (node-id -> {ref, requires:[node-ids]}); node "0" is the
// root. v2 carries only a flat requires list (no edges).
type conanLock struct {
	GraphLock struct {
		Nodes map[string]conanLockNode `json:"nodes"`
	} `json:"graph_lock"`
}

type conanLockNode struct {
	Ref      string   `json:"ref"`
	Requires []string `json:"requires"`
}

// ParseConanLockGraph parses conan.lock and returns the package-to-package
// edges, honoring the requested graph mode. It implements the GraphProducer
// contract.
//
// Only conan.lock v1 (graph_lock.nodes) states a graph. Refs look like
// "name/version@user/channel#rrev"; the node id is "name@version". Node "0" is
// the root, so its requires are the direct dependencies. v2 lockfiles have no
// edge section and yield nothing.
func ParseConanLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock conanLock
	if err := json.Unmarshal(input.Lockfile, &lock); err != nil || len(lock.GraphLock.Nodes) == 0 {
		return result
	}

	// node-id -> "name@version"
	nodeID := make(map[string]string, len(lock.GraphLock.Nodes))
	for id, node := range lock.GraphLock.Nodes {
		if n := conanRefNode(node.Ref); n != "" {
			nodeID[id] = n
		}
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = conanDirectEdges(lock.GraphLock.Nodes, nodeID)
	case types.DependencyGraphFull:
		result.Edges = conanFullEdges(lock.GraphLock.Nodes, nodeID)
	}
	return result
}

// conanDirectEdges builds root -> direct edges from node "0"'s requires.
func conanDirectEdges(nodes map[string]conanLockNode, nodeID map[string]string) []types.DependencyEdge {
	root, ok := nodes["0"]
	if !ok {
		return nil
	}
	var edges []types.DependencyEdge
	for _, req := range root.Requires {
		if to := nodeID[req]; to != "" {
			edges = append(edges, types.DependencyEdge{From: ".", To: to})
		}
	}
	return edges
}

// conanFullEdges builds every node -> requires edge; node "0" is the root ".".
func conanFullEdges(nodes map[string]conanLockNode, nodeID map[string]string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for id, node := range nodes {
		from := "."
		if id != "0" {
			from = nodeID[id]
		}
		if from == "" {
			continue
		}
		for _, req := range node.Requires {
			if to := nodeID[req]; to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges
}

// conanRefNode converts a conan ref ("name/version@user/channel#rrev") into a
// "name@version" node id. Returns "" when the ref lacks a name/version.
func conanRefNode(ref string) string {
	if ref == "" {
		return ""
	}
	// Strip everything after '@' (user/channel) and '#' (revision).
	ref = strings.SplitN(ref, "@", 2)[0]
	ref = strings.SplitN(ref, "#", 2)[0]
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "@" + parts[1]
}
