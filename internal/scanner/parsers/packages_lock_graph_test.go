package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const packagesLockGraphFixture = `{
  "version": 1,
  "dependencies": {
    ".NETCoreApp,Version=v8.0": {
      "Serilog.AspNetCore": {
        "type": "Direct",
        "requested": "[8.0.0, )",
        "resolved": "8.0.0",
        "dependencies": {
          "Serilog": "3.1.1",
          "Serilog.Sinks.Console": "5.0.0"
        }
      },
      "Serilog": {
        "type": "Transitive",
        "resolved": "3.1.1"
      },
      "Serilog.Sinks.Console": {
        "type": "Transitive",
        "resolved": "5.0.0",
        "dependencies": {
          "Serilog": "3.1.1"
        }
      },
      "my.app": {
        "type": "Project",
        "dependencies": {
          "Serilog.AspNetCore": "8.0.0"
        }
      }
    }
  }
}`

func TestParsePackagesLockGraph_FullEdges(t *testing.T) {
	graph := ParsePackagesLockGraph(GraphInput{Lockfile: []byte(packagesLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"Serilog.AspNetCore@8.0.0->Serilog@3.1.1",
		"Serilog.AspNetCore@8.0.0->Serilog.Sinks.Console@5.0.0",
		"Serilog.Sinks.Console@5.0.0->Serilog@3.1.1",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParsePackagesLockGraph_Modes(t *testing.T) {
	if g := ParsePackagesLockGraph(GraphInput{Lockfile: []byte(packagesLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: type=Direct -> Serilog.AspNetCore only.
	gd := ParsePackagesLockGraph(GraphInput{Lockfile: []byte(packagesLockGraphFixture), Mode: types.DependencyGraphDirect})
	if len(gd.Edges) != 1 || gd.Edges[0].From != "." || gd.Edges[0].To != "Serilog.AspNetCore@8.0.0" {
		t.Errorf("direct mode: expected [. -> Serilog.AspNetCore@8.0.0], got %v", gd.Edges)
	}
}
