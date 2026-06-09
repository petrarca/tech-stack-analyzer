package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const renvLockFixture = `{
  "R": {"Version": "4.3.1", "Repositories": [{"Name": "CRAN", "URL": "https://cran.rstudio.com"}]},
  "Packages": {
    "dplyr": {
      "Package": "dplyr", "Version": "1.1.2", "Source": "Repository",
      "Requirements": ["R", "cli", "rlang"]
    },
    "cli": {
      "Package": "cli", "Version": "3.6.1", "Source": "Repository",
      "Requirements": ["R", "utils"]
    },
    "rlang": {
      "Package": "rlang", "Version": "1.1.1", "Source": "Repository",
      "Requirements": ["R"]
    }
  }
}`

func TestParseRenvLock(t *testing.T) {
	deps := NewRenvParser().ParseRenvLock(renvLockFixture)
	byName := map[string]string{}
	for _, d := range deps {
		byName[d.Name] = d.Version
	}
	if byName["dplyr"] != "1.1.2" || byName["cli"] != "3.6.1" || byName["rlang"] != "1.1.1" {
		t.Errorf("renv parse wrong: %v", byName)
	}
}

func TestParseRenvLockGraph_FullEdges(t *testing.T) {
	g := ParseRenvLockGraph(GraphInput{Lockfile: []byte(renvLockFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	if !got["dplyr@1.1.2->cli@3.6.1"] || !got["dplyr@1.1.2->rlang@1.1.1"] {
		t.Errorf("missing dplyr edges; got %v", got)
	}
	// "R" and "utils" are base packages -> must not appear as edge targets.
	for e := range got {
		if e == "dplyr@1.1.2->R@" || e == "cli@3.6.1->utils@" {
			t.Errorf("base R package leaked into graph: %s", e)
		}
	}
	for _, e := range g.Edges {
		if e.To == "R@" || e.To == "utils@" {
			t.Errorf("base package edge present: %v", e)
		}
	}
}

func TestParseRenvLockGraph_Direct(t *testing.T) {
	// dplyr is required by nobody -> root; cli/rlang are transitive.
	gd := ParseRenvLockGraph(GraphInput{Lockfile: []byte(renvLockFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["dplyr@1.1.2"] {
		t.Errorf("direct: expected dplyr root, got %v", got)
	}
	if got["cli@3.6.1"] || got["rlang@1.1.1"] {
		t.Errorf("direct: transitive packages must not appear, got %v", got)
	}
}

func TestParseRenvLockGraph_Off(t *testing.T) {
	if g := ParseRenvLockGraph(GraphInput{Lockfile: []byte(renvLockFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
