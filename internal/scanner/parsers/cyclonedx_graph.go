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
	Components []struct {
		BOMRef  string `json:"bom-ref"`
		Name    string `json:"name"`
		Version string `json:"version"`
		PURL    string `json:"purl"`
	} `json:"components"`
	Dependencies []struct {
		Ref       string   `json:"ref"`
		DependsOn []string `json:"dependsOn"`
	} `json:"dependencies"`
	Metadata struct {
		Component struct {
			BOMRef string `json:"bom-ref"`
		} `json:"component"`
	} `json:"metadata"`
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

	// Map bom-ref -> "name@version". Fall back to PURL-derived name@version.
	nodeByRef := make(map[string]string, len(bom.Components))
	for _, c := range bom.Components {
		node := cyclonedxNode(c.Name, c.Version, c.PURL)
		if node == "" {
			continue
		}
		if c.BOMRef != "" {
			nodeByRef[c.BOMRef] = node
		}
	}
	rootRef := bom.Metadata.Component.BOMRef

	var unresolved []string
	for _, d := range bom.Dependencies {
		from := nodeByRef[d.Ref]
		isRoot := d.Ref == rootRef || from == ""
		for _, depRef := range d.DependsOn {
			to := nodeByRef[depRef]
			if to == "" {
				unresolved = append(unresolved, d.Ref+" -> "+depRef)
				continue
			}
			switch input.Mode {
			case types.DependencyGraphDirect:
				if isRoot {
					result.Edges = append(result.Edges, types.DependencyEdge{From: ".", To: to})
				}
			case types.DependencyGraphFull:
				edgeFrom := from
				if isRoot {
					edgeFrom = "."
				}
				result.Edges = append(result.Edges, types.DependencyEdge{From: edgeFrom, To: to})
			}
		}
	}
	result.Unresolved = unresolved
	return result
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
