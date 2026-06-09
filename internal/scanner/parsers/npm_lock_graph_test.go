package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const npmLockGraphFixture = `{
  "name": "myapp",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "myapp",
      "version": "1.0.0",
      "dependencies": {
        "express": "^4.18.0"
      }
    },
    "node_modules/express": {
      "version": "4.18.2",
      "dependencies": {
        "accepts": "~1.3.8",
        "body-parser": "1.20.1"
      }
    },
    "node_modules/accepts": {
      "version": "1.3.8"
    },
    "node_modules/body-parser": {
      "version": "1.20.1",
      "dependencies": {
        "accepts": "~1.3.8"
      }
    }
  }
}`

func TestParsePackageLockGraph_FullEdges(t *testing.T) {
	graph := ParsePackageLockGraph(GraphInput{Lockfile: []byte(npmLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"express@4.18.2->accepts@1.3.8", // ~1.3.8 resolved via hoisted node_modules
		"express@4.18.2->body-parser@1.20.1",
		"body-parser@1.20.1->accepts@1.3.8",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParsePackageLockGraph_Modes(t *testing.T) {
	if g := ParsePackageLockGraph(GraphInput{Lockfile: []byte(npmLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	gd := ParsePackageLockGraph(GraphInput{Lockfile: []byte(npmLockGraphFixture), Mode: types.DependencyGraphDirect})
	if len(gd.Edges) != 1 || gd.Edges[0].From != "." || gd.Edges[0].To != "express@4.18.2" {
		t.Errorf("direct mode: expected [. -> express@4.18.2], got %v", gd.Edges)
	}
}
