package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const cargoLockGraphFixture = `version = 3

[[package]]
name = "myapp"
version = "0.1.0"
dependencies = [
 "serde",
 "tokio 1.35.1",
]

[[package]]
name = "serde"
version = "1.0.197"
dependencies = [
 "serde_derive",
]

[[package]]
name = "serde_derive"
version = "1.0.197"

[[package]]
name = "tokio"
version = "1.35.1"
dependencies = [
 "serde",
]
`

func TestParseCargoLockGraph_DirectFromManifest(t *testing.T) {
	// Cargo.toml declares serde + tokio as direct deps. serde is ALSO pulled
	// transitively (tokio -> serde), which would confuse a root heuristic; the
	// manifest path must still classify serde as direct.
	cargoToml := `[package]
name = "myapp"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = "1.35"
`
	gd := ParseCargoLockGraph(GraphInput{
		Lockfile: []byte(cargoLockGraphFixture),
		Manifest: []byte(cargoToml),
		Mode:     types.DependencyGraphDirect,
	})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["serde@1.0.197"] || !got["tokio@1.35.1"] {
		t.Errorf("expected direct edges to serde and tokio, got %v", got)
	}
	if got["serde_derive@1.0.197"] {
		t.Error("must not include transitive serde_derive as direct")
	}
}

func TestParseCargoLockGraph_FullEdges(t *testing.T) {
	graph := ParseCargoLockGraph(GraphInput{Lockfile: []byte(cargoLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"myapp@0.1.0->serde@1.0.197", // bare "serde" resolved to its locked version
		"myapp@0.1.0->tokio@1.35.1",  // explicit "tokio 1.35.1"
		"serde@1.0.197->serde_derive@1.0.197",
		"tokio@1.35.1->serde@1.0.197",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseCargoLockGraph_Modes(t *testing.T) {
	// off: no edges
	if g := ParseCargoLockGraph(GraphInput{Lockfile: []byte(cargoLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: root (myapp, not referenced by anyone) -> its direct deps only
	gd := ParseCargoLockGraph(GraphInput{Lockfile: []byte(cargoLockGraphFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct mode: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["serde@1.0.197"] || !got["tokio@1.35.1"] {
		t.Errorf("direct mode: expected root edges to serde and tokio, got %v", got)
	}
	if got["serde_derive@1.0.197"] {
		t.Error("direct mode: must not include transitive serde_derive edge")
	}

	// full: transitive edge present
	gf := ParseCargoLockGraph(GraphInput{Lockfile: []byte(cargoLockGraphFixture), Mode: types.DependencyGraphFull})
	found := false
	for _, e := range gf.Edges {
		if e.From == "serde@1.0.197" && e.To == "serde_derive@1.0.197" {
			found = true
		}
	}
	if !found {
		t.Errorf("full mode: expected serde@1.0.197 -> serde_derive@1.0.197, got %v", gf.Edges)
	}
}
