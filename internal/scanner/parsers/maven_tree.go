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

// coordinate returns the "groupId:artifactId" coordinate (without version),
// matching the flat dependency Name used elsewhere for Maven.
func (n mavenTreeNode) coordinate() string {
	if n.GroupID == "" || n.ArtifactID == "" {
		return ""
	}
	return n.GroupID + ":" + n.ArtifactID
}

// ParseMavenTreeVersions parses a pre-generated `mvn dependency:tree
// -DoutputType=json` file and returns a map of "groupId:artifactId" ->
// resolved version. Maven's dependency:tree output carries fully resolved
// versions (conflict mediation, dependencyManagement/BOM overrides, and
// property interpolation already applied), so it is an authoritative source
// for backfilling versionless dependencies parsed from pom.xml.
//
// When the same coordinate appears at multiple versions in the tree (rare for
// a mediated tree, but possible across modules), the first occurrence in a
// depth-first walk wins; this matches Maven's nearest-wins mediation for the
// flat view. The root project itself is excluded.
func ParseMavenTreeVersions(content []byte) map[string]string {
	var root mavenTreeNode
	if err := json.Unmarshal(content, &root); err != nil {
		return nil
	}

	versions := make(map[string]string)
	var walk func(nodes []mavenTreeNode)
	walk = func(nodes []mavenTreeNode) {
		for _, n := range nodes {
			if coord := n.coordinate(); coord != "" && n.Version != "" {
				if _, seen := versions[coord]; !seen {
					versions[coord] = n.Version
				}
			}
			walk(n.Children)
		}
	}
	walk(root.Children)

	if len(versions) == 0 {
		return nil
	}
	return versions
}

// ParseMavenTreeGraph parses a pre-generated `mvn dependency:tree
// -DoutputType=json` file and returns the package-to-package edges, honoring
// the requested graph mode. It implements the GraphProducer contract
// (ParseGraphFunc).
//
// The flat dependency list is left empty: the java detector already populates
// payload.Dependencies from pom.xml / dependency:list. This producer only
// contributes edges.
func ParseMavenTreeGraph(input GraphInput) LockGraph {
	var result LockGraph

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var root mavenTreeNode
	if err := json.Unmarshal(input.Lockfile, &root); err != nil {
		return result
	}

	switch input.Mode {
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
