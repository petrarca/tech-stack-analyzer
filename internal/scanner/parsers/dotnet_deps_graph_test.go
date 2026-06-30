package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// dotnetDepsFixture is a minimal but representative .NET Core <App>.deps.json:
// a root project depending on a direct package, which pulls a transitive one,
// plus a bundled runtime pack carrying the synthetic "runtimepack." prefix.
const dotnetDepsFixture = `{
  "runtimeTarget": { "name": ".NETCoreApp,Version=v8.0/linux-x64" },
  "targets": {
    ".NETCoreApp,Version=v8.0/linux-x64": {
      "MyApp/1.0.0": {
        "dependencies": {
          "Serilog.AspNetCore": "8.0.0",
          "Microsoft.NETCore.App.Runtime.linux-x64": "8.0.0"
        }
      },
      "Serilog.AspNetCore/8.0.0": {
        "dependencies": { "Serilog": "3.1.1" }
      },
      "Serilog/3.1.1": {},
      "runtimepack.Microsoft.NETCore.App.Runtime.linux-x64/8.0.0": {}
    }
  },
  "libraries": {
    "MyApp/1.0.0": { "type": "project" },
    "Serilog.AspNetCore/8.0.0": { "type": "package" },
    "Serilog/3.1.1": { "type": "package" },
    "runtimepack.Microsoft.NETCore.App.Runtime.linux-x64/8.0.0": { "type": "runtimepack" }
  }
}`

func TestParseDotNetDepsGraph_FullEdges(t *testing.T) {
	graph := ParseDotNetDepsGraph(GraphInput{Lockfile: []byte(dotnetDepsFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		// Direct edges from the synthetic root.
		".->Serilog.AspNetCore@8.0.0",
		// The runtimepack. prefix is stripped so the root edge resolves to the
		// same node the runtime library declares.
		".->Microsoft.NETCore.App.Runtime.linux-x64@8.0.0",
		// Transitive edge.
		"Serilog.AspNetCore@8.0.0->Serilog@3.1.1",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseDotNetDepsGraph_Modes(t *testing.T) {
	if g := ParseDotNetDepsGraph(GraphInput{Lockfile: []byte(dotnetDepsFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: only the root project's two direct dependencies.
	gd := ParseDotNetDepsGraph(GraphInput{Lockfile: []byte(dotnetDepsFixture), Mode: types.DependencyGraphDirect})
	if len(gd.Edges) != 2 {
		t.Fatalf("direct mode: expected 2 edges, got %d (%v)", len(gd.Edges), gd.Edges)
	}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct mode: edge should originate at root, got %q -> %q", e.From, e.To)
		}
	}
}

func TestParseDotNetDepsGraph_Malformed(t *testing.T) {
	if g := ParseDotNetDepsGraph(GraphInput{Lockfile: []byte("not json"), Mode: types.DependencyGraphFull}); len(g.Edges) != 0 {
		t.Errorf("malformed input: expected 0 edges, got %d", len(g.Edges))
	}
	if g := ParseDotNetDepsGraph(GraphInput{Lockfile: []byte(`{}`), Mode: types.DependencyGraphFull}); len(g.Edges) != 0 {
		t.Errorf("empty libraries: expected 0 edges, got %d", len(g.Edges))
	}
}
