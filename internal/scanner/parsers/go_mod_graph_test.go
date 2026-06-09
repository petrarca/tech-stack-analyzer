package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const goModGraphFixture = `example.com/app golang.org/x/text@v0.14.0
example.com/app github.com/spf13/cobra@v1.8.0
github.com/spf13/cobra@v1.8.0 github.com/spf13/pflag@v1.0.5
golang.org/x/text@v0.14.0 golang.org/x/tools@v0.1.0
`

func TestParseGoModGraph_FullEdges(t *testing.T) {
	g := ParseGoModGraph(GraphInput{Lockfile: []byte(goModGraphFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		".->golang.org/x/text@v0.14.0", // root edge from "."
		".->github.com/spf13/cobra@v1.8.0",
		"github.com/spf13/cobra@v1.8.0->github.com/spf13/pflag@v1.0.5",
		"golang.org/x/text@v0.14.0->golang.org/x/tools@v0.1.0",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseGoModGraph_Direct(t *testing.T) {
	gd := ParseGoModGraph(GraphInput{Lockfile: []byte(goModGraphFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["golang.org/x/text@v0.14.0"] || !got["github.com/spf13/cobra@v1.8.0"] {
		t.Errorf("direct: expected root edges, got %v", got)
	}
	if got["github.com/spf13/pflag@v1.0.5"] {
		t.Error("direct: must not include transitive pflag")
	}
}

func TestParseGoModDirectGraph_FromGoMod(t *testing.T) {
	goMod := `module example.com/app

go 1.22

require (
	github.com/spf13/cobra v1.8.0
	golang.org/x/text v0.14.0 // indirect
)

require github.com/stretchr/testify v1.9.0
`
	g := ParseGoModDirectGraph(GraphInput{Lockfile: []byte(goMod), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.To] = true
	}
	if !got["github.com/spf13/cobra@v1.8.0"] {
		t.Errorf("expected cobra direct edge, got %v", got)
	}
	if !got["github.com/stretchr/testify@v1.9.0"] {
		t.Errorf("expected single-line require testify, got %v", got)
	}
	if got["golang.org/x/text@v0.14.0"] {
		t.Error("indirect requirement must not be a direct edge")
	}
}

func TestParseGoModGraph_Off(t *testing.T) {
	if g := ParseGoModGraph(GraphInput{Lockfile: []byte(goModGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
