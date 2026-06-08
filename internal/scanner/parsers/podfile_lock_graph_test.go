package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// podfileLockFixture mirrors a Podfile.lock: PODS with bare and map entries,
// a subspec (Alamofire/Core), and a DEPENDENCIES section.
const podfileLockFixture = `PODS:
  - Alamofire (5.8.0)
  - AlamofireImage (4.3.0):
    - Alamofire (~> 5.6)
  - SnapKit (5.6.0)
  - Kingfisher (7.9.1):
    - Kingfisher/Core (= 7.9.1)
  - Kingfisher/Core (7.9.1)

DEPENDENCIES:
  - AlamofireImage (~> 4.3)
  - SnapKit
  - Kingfisher

SPEC CHECKSUMS:
  Alamofire: abc123

COCOAPODS: 1.12.1
`

func TestParsePodfileLockGraph_FullEdges(t *testing.T) {
	g := ParsePodfileLockGraph(GraphInput{Lockfile: []byte(podfileLockFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	// AlamofireImage depends on Alamofire (constraint resolves to the locked
	// Alamofire 5.8.0).
	if !got["AlamofireImage@4.3.0->Alamofire@5.8.0"] {
		t.Errorf("missing AlamofireImage -> Alamofire edge; got %v", got)
	}
	// Kingfisher/Core subspec collapses to the Kingfisher root pod; the
	// self-edge (Kingfisher -> Kingfisher) is dropped.
	if got["Kingfisher@7.9.1->Kingfisher@7.9.1"] {
		t.Error("self-edge from subspec must be dropped")
	}
}

func TestParsePodfileLockGraph_Direct(t *testing.T) {
	gd := ParsePodfileLockGraph(GraphInput{Lockfile: []byte(podfileLockFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["AlamofireImage@4.3.0"] || !got["SnapKit@5.6.0"] || !got["Kingfisher@7.9.1"] {
		t.Errorf("direct: expected the three DEPENDENCIES pods, got %v", got)
	}
	// Alamofire is transitive only (not in DEPENDENCIES).
	if got["Alamofire@5.8.0"] {
		t.Error("direct: Alamofire is transitive only; must not be a direct edge")
	}
}

func TestParsePodfileLockGraph_Off(t *testing.T) {
	if g := ParsePodfileLockGraph(GraphInput{Lockfile: []byte(podfileLockFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
