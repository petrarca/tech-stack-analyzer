package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// cpanfileSnapshotFixture mirrors a real Carton cpanfile.snapshot: Plack
// requires Try::Tiny and YAML (which resolve to their providing distributions);
// core modules (strict, Carp) are not distributions.
const cpanfileSnapshotFixture = `# carton snapshot format: version 1.0
DISTRIBUTIONS
  YAML-0.84
    pathname: M/MS/MSTROUT/YAML-0.84.tar.gz
    provides:
      YAML  0.84
      YAML::Loader  0.84
    requires:
      Carp  0
  Try-Tiny-0.30
    pathname: D/DO/DOY/Try-Tiny-0.30.tar.gz
    provides:
      Try::Tiny  0.30
    requires:
      strict  0
  Plack-1.0039
    pathname: M/MI/MIYAGAWA/Plack-1.0039.tar.gz
    provides:
      Plack  1.0039
    requires:
      Try::Tiny  0.09
      YAML  0.68
`

func TestParseCpanfileSnapshot(t *testing.T) {
	deps := NewCpanfileSnapshotParser().ParseCpanfileSnapshot(cpanfileSnapshotFixture)
	byName := map[string]string{}
	for _, d := range deps {
		byName[d.Name] = d.Version
	}
	if byName["YAML"] != "0.84" || byName["Try-Tiny"] != "0.30" || byName["Plack"] != "1.0039" {
		t.Errorf("distributions parsed wrong: %v", byName)
	}
}

func TestParseCpanfileSnapshotGraph_FullEdges(t *testing.T) {
	g := ParseCpanfileSnapshotGraph(GraphInput{Lockfile: []byte(cpanfileSnapshotFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	// Plack requires Try::Tiny and YAML -> edges to their distributions.
	if !got["Plack@1.0039->Try-Tiny@0.30"] {
		t.Errorf("missing Plack -> Try-Tiny edge; got %v", got)
	}
	if !got["Plack@1.0039->YAML@0.84"] {
		t.Errorf("missing Plack -> YAML edge; got %v", got)
	}
	// Core module "Carp"/"strict" must not appear as unresolved.
	for _, u := range g.Unresolved {
		if u == "YAML-0.84 -> Carp" || u == "Try-Tiny-0.30 -> strict" {
			t.Errorf("core module reported as unresolved: %s", u)
		}
	}
}

func TestParseCpanfileSnapshotGraph_Direct(t *testing.T) {
	// Plack is required by nobody -> root. YAML/Try-Tiny are required by Plack.
	gd := ParseCpanfileSnapshotGraph(GraphInput{Lockfile: []byte(cpanfileSnapshotFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["Plack@1.0039"] {
		t.Errorf("direct: expected Plack root, got %v", got)
	}
	if got["YAML@0.84"] || got["Try-Tiny@0.30"] {
		t.Errorf("direct: transitive distributions must not appear, got %v", got)
	}
}

func TestParseCpanfileSnapshotGraph_Off(t *testing.T) {
	if g := ParseCpanfileSnapshotGraph(GraphInput{Lockfile: []byte(cpanfileSnapshotFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
