package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// bunLockFixture mirrors bun.lock (JSONC, note the trailing commas): a root
// workspace with direct deps and a packages map whose entries are
// [ident, registry, info, hash].
const bunLockFixture = `{
  "lockfileVersion": 1,
  "workspaces": {
    "": {
      "name": "app",
      "dependencies": {
        "express": "^4.18.0",
      },
      "devDependencies": {
        "typescript": "^5.0.0",
      },
    },
  },
  "packages": {
    "express": ["express@4.18.2", "", { "dependencies": { "accepts": "~1.3.8", "body-parser": "1.20.1" } }, "sha512-aaa"],
    "accepts": ["accepts@1.3.8", "", {}, "sha512-bbb"],
    "body-parser": ["body-parser@1.20.1", "", { "dependencies": { "accepts": "~1.3.8" } }, "sha512-ccc"],
    "typescript": ["typescript@5.4.5", "", {}, "sha512-ddd"],
  },
}`

func TestParseBunLockGraph_FullEdges(t *testing.T) {
	g := ParseBunLockGraph(GraphInput{Lockfile: []byte(bunLockFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"express@4.18.2->accepts@1.3.8",
		"express@4.18.2->body-parser@1.20.1",
		"body-parser@1.20.1->accepts@1.3.8",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseBunLockGraph_Direct(t *testing.T) {
	gd := ParseBunLockGraph(GraphInput{Lockfile: []byte(bunLockFixture), Mode: types.DependencyGraphDirect})
	got := map[string]string{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = e.Scope
	}
	if got["express@4.18.2"] != types.ScopeProd {
		t.Errorf("direct: expected express prod, got %v", got)
	}
	if got["typescript@5.4.5"] != types.ScopeDev {
		t.Errorf("direct: expected typescript dev, got %v", got)
	}
	if _, ok := got["accepts@1.3.8"]; ok {
		t.Error("direct: transitive accepts must not appear")
	}
}

func TestParseBunLockGraph_Off(t *testing.T) {
	if g := ParseBunLockGraph(GraphInput{Lockfile: []byte(bunLockFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
