package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const uvLockGraphFixture = `version = 1

[[package]]
name = "myapp"
version = "0.1.0"
source = { editable = "." }
dependencies = [
    { name = "fastapi" },
    { name = "requests" },
]

[[package]]
name = "fastapi"
version = "0.110.0"
dependencies = [
    { name = "starlette" },
]

[[package]]
name = "starlette"
version = "0.36.3"

[[package]]
name = "requests"
version = "2.31.0"
`

func TestParseUvLockGraph_FullEdges(t *testing.T) {
	graph := ParseUvLockGraph(GraphInput{Lockfile: []byte(uvLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"myapp@0.1.0->fastapi@0.110.0",
		"myapp@0.1.0->requests@2.31.0",
		"fastapi@0.110.0->starlette@0.36.3",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseUvLockGraph_Modes(t *testing.T) {
	if g := ParseUvLockGraph(GraphInput{Lockfile: []byte(uvLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	gd := ParseUvLockGraph(GraphInput{Lockfile: []byte(uvLockGraphFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct mode: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["fastapi@0.110.0"] || !got["requests@2.31.0"] {
		t.Errorf("direct mode: expected root edges to fastapi and requests, got %v", got)
	}
	if got["starlette@0.36.3"] {
		t.Error("direct mode: must not include transitive starlette edge")
	}
}
