package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// CycloneDXFileName is a CycloneDX SBOM committed in the repo (e.g. produced by
// cyclonedx-maven-plugin's makeAggregateBom with dependencyGraph). It carries a
// resolved dependency-graph edge section the analyzer can ingest directly --
// another read-only, standards-based graph source alongside the per-ecosystem
// lockfiles.
const CycloneDXFileName = "bom.json"

// cyclonedxBOM is the minimal CycloneDX view for graph ingest: components (to
// map bom-refs to name@version) and the dependencies edge section.
type cyclonedxBOM struct {
	Components   []cyclonedxComponent  `json:"components"`
	Dependencies []cyclonedxDependency `json:"dependencies"`
	Metadata     struct {
		Component struct {
			BOMRef string `json:"bom-ref"`
		} `json:"component"`
	} `json:"metadata"`
}

type cyclonedxComponent struct {
	BOMRef  string `json:"bom-ref"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl"`
}

type cyclonedxDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

// ParseCycloneDXGraph parses a CycloneDX SBOM's dependencies section into
// package-to-package edges. It implements the GraphProducer contract.
//
// CycloneDX states edges by bom-ref; each ref is mapped to its component's
// "name@version" node. The metadata.component (the SBOM subject) is treated as
// the root: its dependencies become "." edges in direct mode, and all
// ref -> dependsOn pairs are edges in full mode.
func ParseCycloneDXGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var bom cyclonedxBOM
	if err := json.Unmarshal(input.Lockfile, &bom); err != nil {
		return result
	}

	nodeByRef := cyclonedxNodesByRef(bom.Components)
	rootRef := bom.Metadata.Component.BOMRef

	var unresolved []string
	for _, d := range bom.Dependencies {
		from := nodeByRef[d.Ref]
		isRoot := d.Ref == rootRef || from == ""
		unresolved = appendCycloneDXEdges(&result.Edges, unresolved, d, nodeByRef, from, isRoot, input.Mode)
	}
	result.Unresolved = unresolved
	return result
}

// cyclonedxNodesByRef maps each component's bom-ref to its "name@version" node,
// falling back to a PURL-derived identity.
func cyclonedxNodesByRef(components []cyclonedxComponent) map[string]string {
	nodeByRef := make(map[string]string, len(components))
	for _, c := range components {
		node := cyclonedxNode(c.Name, c.Version, c.PURL)
		if node != "" && c.BOMRef != "" {
			nodeByRef[c.BOMRef] = node
		}
	}
	return nodeByRef
}

// appendCycloneDXEdges emits the edges declared by a single dependency entry,
// honoring the graph mode, and returns the (possibly extended) unresolved list.
func appendCycloneDXEdges(edges *[]types.DependencyEdge, unresolved []string, d cyclonedxDependency, nodeByRef map[string]string, from string, isRoot bool, mode types.DependencyGraphMode) []string {
	edgeFrom := from
	if isRoot {
		edgeFrom = "."
	}
	for _, depRef := range d.DependsOn {
		to := nodeByRef[depRef]
		if to == "" {
			unresolved = append(unresolved, d.Ref+" -> "+depRef)
			continue
		}
		switch mode {
		case types.DependencyGraphDirect:
			if isRoot {
				*edges = append(*edges, types.DependencyEdge{From: ".", To: to})
			}
		case types.DependencyGraphFull:
			*edges = append(*edges, types.DependencyEdge{From: edgeFrom, To: to})
		}
	}
	return unresolved
}

// cyclonedxNode builds a "name@version" node from a component, preferring the
// explicit name+version and falling back to parsing the PURL.
func cyclonedxNode(name, version, purl string) string {
	if name != "" && version != "" {
		return name + "@" + version
	}
	// PURL form: pkg:type/namespace/name@version?qualifiers
	if purl != "" {
		p := purl
		if i := strings.IndexByte(p, '?'); i >= 0 {
			p = p[:i]
		}
		if at := strings.LastIndexByte(p, '@'); at >= 0 {
			ver := p[at+1:]
			rest := p[:at]
			nm := rest
			if slash := strings.LastIndexByte(rest, '/'); slash >= 0 {
				nm = rest[slash+1:]
			}
			if nm != "" && ver != "" {
				return nm + "@" + ver
			}
		}
	}
	return ""
}
