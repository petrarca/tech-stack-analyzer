package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// MavenTreeFile is the pre-generated, machine-readable Maven dependency tree.
// A resolved Maven dependency graph cannot be derived statically from pom.xml
// (it requires Maven's conflict mediation, dependencyManagement/BOM overrides,
// version ranges and scope rules). The graph producer therefore reads a file
// the user/CI generated with:
//
//	mvn dependency:tree -DoutputType=json -DoutputFile=dependency-tree.json
//
// This mirrors maven_dependency_list.go (which ingests mvn dependency:list).
// The analyzer never runs Maven; it only reads the file when present, exactly
// like every other lockfile.
const MavenTreeFileName = "dependency-tree.json"

// mavenTreeNode is one node of the dependency:tree JSON output. The root node
// is the project itself; children are its resolved dependencies, recursively.
type mavenTreeNode struct {
	GroupID    string          `json:"groupId"`
	ArtifactID string          `json:"artifactId"`
	Version    string          `json:"version"`
	Scope      string          `json:"scope"`
	Children   []mavenTreeNode `json:"children"`
}

// node returns the "groupId:artifactId@version" node identity, matching the
// "groupId:artifactId" naming used elsewhere for Maven dependencies.
func (n mavenTreeNode) node() string {
	if n.GroupID == "" || n.ArtifactID == "" || n.Version == "" {
		return ""
	}
	return n.GroupID + ":" + n.ArtifactID + "@" + n.Version
}

// ParseMavenTreeGraph parses a pre-generated `mvn dependency:tree
// -DoutputType=json` file and returns the package-to-package edges, honoring
// the requested graph mode. It implements the GraphProducer contract
// (ParseGraphFunc).
//
// The flat dependency list is left empty: the java detector already populates
// payload.Dependencies from pom.xml / dependency:list. This producer only
// contributes edges.
func ParseMavenTreeGraph(content []byte, mode types.DependencyGraphMode) LockGraph {
	var result LockGraph

	if mode == types.DependencyGraphOff {
		return result
	}

	var root mavenTreeNode
	if err := json.Unmarshal(content, &root); err != nil {
		return result
	}

	switch mode {
	case types.DependencyGraphDirect:
		// Root -> its direct children only. The synthetic "." marker is the
		// from node, consistent with the other ecosystems.
		for _, child := range root.Children {
			if to := child.node(); to != "" {
				result.Edges = append(result.Edges, types.DependencyEdge{From: ".", To: to})
			}
		}
	case types.DependencyGraphFull:
		result.Edges = mavenTreeEdges(root)
	}
	return result
}

// mavenTreeEdges walks the tree and emits a parent -> child edge for every
// resolved dependency relationship.
func mavenTreeEdges(root mavenTreeNode) []types.DependencyEdge {
	var edges []types.DependencyEdge
	var walk func(parentNode string, children []mavenTreeNode)
	walk = func(parentNode string, children []mavenTreeNode) {
		for _, child := range children {
			to := child.node()
			if to != "" && parentNode != "" {
				edges = append(edges, types.DependencyEdge{From: parentNode, To: to})
			}
			if to != "" {
				walk(to, child.Children)
			}
		}
	}
	// The root project itself is not emitted as a node; its children are the
	// direct dependencies, walked with their own subtrees.
	walk(root.node(), root.Children)
	return edges
}
