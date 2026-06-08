package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const gemfileLockGraphFixture = `GEM
  remote: https://rubygems.org/
  specs:
    actionpack (7.1.0)
      actionview (= 7.1.0)
      rack (>= 2.2.4)
    actionview (7.1.0)
      builder (~> 3.1)
    rack (3.0.8)
    builder (3.2.4)

PLATFORMS
  ruby

DEPENDENCIES
  actionpack (~> 7.1)
  rack

BUNDLED WITH
   2.4.10
`

func TestParseGemfileLockGraph_FullEdges(t *testing.T) {
	graph := ParseGemfileLockGraph(GraphInput{Lockfile: []byte(gemfileLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"actionpack@7.1.0->actionview@7.1.0",
		"actionpack@7.1.0->rack@3.0.8",
		"actionview@7.1.0->builder@3.2.4",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseGemfileLockGraph_Modes(t *testing.T) {
	if g := ParseGemfileLockGraph(GraphInput{Lockfile: []byte(gemfileLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: DEPENDENCIES section -> actionpack, rack only.
	gd := ParseGemfileLockGraph(GraphInput{Lockfile: []byte(gemfileLockGraphFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct mode: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["actionpack@7.1.0"] || !got["rack@3.0.8"] {
		t.Errorf("direct mode: expected actionpack and rack, got %v", got)
	}
	if got["builder@3.2.4"] {
		t.Error("direct mode: must not include transitive builder")
	}
}
