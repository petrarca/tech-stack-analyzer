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

// poetryMultiVersionFixture mirrors the real nicegui case: numpy is locked at
// two versions, and consumers reference it via a string range, a single-version
// range, and an array-of-tables with environment markers.
const poetryMultiVersionFixture = `[[package]]
name = "numpy"
version = "1.24.4"
description = "array computing"

[[package]]
name = "numpy"
version = "1.26.4"
description = "array computing"

[[package]]
name = "matplotlib"
version = "3.7.5"
description = "plotting"

[package.dependencies]
numpy = ">=1.20,<2"

[[package]]
name = "contourpy"
version = "1.1.1"
description = "contours"

[package.dependencies]
numpy = ">=1.16,<2.0"

[[package]]
name = "scipy"
version = "1.10.1"
description = "scientific"

[package.dependencies]
numpy = [
    {version = ">=1.16,<1.25", markers = "python_version <= \"3.11\""},
    {version = ">=1.26.0,<2.0", markers = "python_version >= \"3.12\""},
]
`

func TestParsePoetryLockGraph_MultiVersionRangeMatch(t *testing.T) {
	graph := ParsePoetryLockGraph([]byte(poetryMultiVersionFixture), types.DependencyGraphFull)

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}

	// matplotlib's ">=1.20,<2" matches both 1.24.4 and 1.26.4 -> edge to each.
	if !got["matplotlib@3.7.5->numpy@1.24.4"] || !got["matplotlib@3.7.5->numpy@1.26.4"] {
		t.Errorf("matplotlib: expected edges to BOTH numpy versions, got %v", got)
	}

	// contourpy's ">=1.16,<2.0" also matches both.
	if !got["contourpy@1.1.1->numpy@1.24.4"] || !got["contourpy@1.1.1->numpy@1.26.4"] {
		t.Errorf("contourpy: expected edges to both numpy versions, got %v", got)
	}

	// scipy uses an array-of-tables: ">=1.16,<1.25" selects 1.24.4,
	// ">=1.26.0,<2.0" selects 1.26.4. Union of both marker branches.
	if !got["scipy@1.10.1->numpy@1.24.4"] {
		t.Errorf("scipy: first marker constraint should select numpy 1.24.4, got %v", got)
	}
	if !got["scipy@1.10.1->numpy@1.26.4"] {
		t.Errorf("scipy: second marker constraint should select numpy 1.26.4, got %v", got)
	}
}

func TestParsePoetryLockGraph_RangeExcludesNonMatching(t *testing.T) {
	// numpy locked at 1.24.4 and 1.26.4; a strict "<1.25" range must select
	// ONLY 1.24.4, proving we do not blindly emit all versions.
	fixture := `[[package]]
name = "numpy"
version = "1.24.4"

[[package]]
name = "numpy"
version = "1.26.4"

[[package]]
name = "legacy-lib"
version = "1.0.0"

[package.dependencies]
numpy = ">=1.16,<1.25"
`
	graph := ParsePoetryLockGraph([]byte(fixture), types.DependencyGraphFull)
	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	if !got["legacy-lib@1.0.0->numpy@1.24.4"] {
		t.Errorf("expected edge to numpy 1.24.4, got %v", got)
	}
	if got["legacy-lib@1.0.0->numpy@1.26.4"] {
		t.Errorf("must NOT emit edge to numpy 1.26.4 (excluded by <1.25), got %v", got)
	}
}
