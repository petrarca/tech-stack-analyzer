package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const poetryLockGraphFixture = `[[package]]
name = "fastapi"
version = "0.110.0"
description = ""

[package.dependencies]
starlette = ">=0.36.3,<0.37.0"
pydantic = ">=1.7.4"

[[package]]
name = "starlette"
version = "0.36.3"
description = ""

[package.dependencies]
anyio = ">=3.4.0,<5"

[[package]]
name = "pydantic"
version = "2.6.1"
description = ""

[[package]]
name = "anyio"
version = "4.2.0"
description = ""

[[package]]
name = "requests"
version = "2.31.0"
description = ""
`

func TestParsePoetryLockGraph_FullEdges(t *testing.T) {
	graph := ParsePoetryLockGraph([]byte(poetryLockGraphFixture), types.DependencyGraphFull)

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"fastapi@0.110.0->starlette@0.36.3",
		"fastapi@0.110.0->pydantic@2.6.1",
		"starlette@0.36.3->anyio@4.2.0",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParsePoetryLockGraph_Modes(t *testing.T) {
	if g := ParsePoetryLockGraph([]byte(poetryLockGraphFixture), types.DependencyGraphOff); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: packages not referenced by any other package are roots
	// (fastapi, requests). starlette/pydantic/anyio are transitive.
	gd := ParsePoetryLockGraph([]byte(poetryLockGraphFixture), types.DependencyGraphDirect)
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
	if got["anyio@4.2.0"] {
		t.Error("direct mode: must not include transitive anyio edge")
	}
}
