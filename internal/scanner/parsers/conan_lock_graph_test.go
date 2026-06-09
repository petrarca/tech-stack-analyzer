package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// conanLockV1Fixture mirrors a conan.lock v1: graph_lock.nodes keyed by id,
// node "0" is the root. Refs carry user/channel and revision suffixes.
const conanLockV1Fixture = `{
  "version": "0.4",
  "graph_lock": {
    "nodes": {
      "0": {"requires": ["1", "3"]},
      "1": {"ref": "poco/1.12.4@_/_#abc123", "requires": ["2"]},
      "2": {"ref": "zlib/1.3.1"},
      "3": {"ref": "openssl/3.2.0@user/stable"}
    }
  }
}`

func TestParseConanLockGraph_FullEdges(t *testing.T) {
	g := ParseConanLockGraph(GraphInput{Lockfile: []byte(conanLockV1Fixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		".->poco@1.12.4",   // root -> direct (revision stripped)
		".->openssl@3.2.0", // root -> direct (user/channel stripped)
		"poco@1.12.4->zlib@1.3.1",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseConanLockGraph_Direct(t *testing.T) {
	gd := ParseConanLockGraph(GraphInput{Lockfile: []byte(conanLockV1Fixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["poco@1.12.4"] || !got["openssl@3.2.0"] {
		t.Errorf("direct: expected poco and openssl, got %v", got)
	}
	if got["zlib@1.3.1"] {
		t.Error("direct: must not include transitive zlib")
	}
}

func TestParseConanLockGraph_V2NoGraph(t *testing.T) {
	// conan.lock v2 has only a flat requires list -> no edges.
	v2 := `{"version": "0.5", "requires": ["zlib/1.3.1", "poco/1.12.4"]}`
	g := ParseConanLockGraph(GraphInput{Lockfile: []byte(v2), Mode: types.DependencyGraphFull})
	if len(g.Edges) != 0 {
		t.Errorf("v2: expected 0 edges (no graph section), got %v", g.Edges)
	}
}

func TestParseConanLockGraph_Off(t *testing.T) {
	if g := ParseConanLockGraph(GraphInput{Lockfile: []byte(conanLockV1Fixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
