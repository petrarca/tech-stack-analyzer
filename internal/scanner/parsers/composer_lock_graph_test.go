package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const composerLockGraphFixture = `{
  "packages": [
    {
      "name": "laravel/framework",
      "version": "v10.0.0",
      "require": {
        "php": "^8.1",
        "symfony/console": "^6.2",
        "monolog/monolog": "^3.0"
      }
    },
    {
      "name": "symfony/console",
      "version": "v6.2.5",
      "require": {
        "php": "^8.1",
        "symfony/string": "^6.2"
      }
    },
    {
      "name": "symfony/string",
      "version": "v6.2.5",
      "require": {"php": "^8.1"}
    },
    {
      "name": "monolog/monolog",
      "version": "3.2.0",
      "require": {"php": "^8.1"}
    }
  ],
  "packages-dev": [
    {
      "name": "phpunit/phpunit",
      "version": "10.0.0",
      "require": {"php": "^8.1"}
    }
  ]
}`

func TestParseComposerLockGraph_FullEdges(t *testing.T) {
	graph := ParseComposerLockGraph(GraphInput{Lockfile: []byte(composerLockGraphFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"laravel/framework@v10.0.0->symfony/console@v6.2.5",
		"laravel/framework@v10.0.0->monolog/monolog@3.2.0",
		"symfony/console@v6.2.5->symfony/string@v6.2.5",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
	// Platform requirement "php" must never produce an edge.
	for e := range got {
		if want := "->php@"; len(e) > len(want) && e[len(e)-len(want)-3:] == want+"^8" {
			t.Errorf("platform requirement leaked into graph: %q", e)
		}
	}
}

func TestParseComposerLockGraph_DirectFromManifest(t *testing.T) {
	manifest := `{
  "require": {"php": "^8.1", "laravel/framework": "^10.0"},
  "require-dev": {"phpunit/phpunit": "^10.0"}
}`
	gd := ParseComposerLockGraph(GraphInput{
		Lockfile: []byte(composerLockGraphFixture),
		Manifest: []byte(manifest),
		Mode:     types.DependencyGraphDirect,
	})
	got := map[string]string{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("expected from='.', got %q", e.From)
		}
		got[e.To] = e.Scope
	}
	if got["laravel/framework@v10.0.0"] != types.ScopeProd {
		t.Errorf("expected laravel/framework prod direct edge, got %v", got)
	}
	if got["phpunit/phpunit@10.0.0"] != types.ScopeDev {
		t.Errorf("expected phpunit dev direct edge, got %v", got)
	}
	if _, ok := got["symfony/console@v6.2.5"]; ok {
		t.Error("symfony/console is transitive only; must not be direct")
	}
}

func TestParseComposerLockGraph_Off(t *testing.T) {
	if g := ParseComposerLockGraph(GraphInput{Lockfile: []byte(composerLockGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}
}
