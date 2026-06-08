package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// mixLockFixture is a trimmed real mix.lock: hex entries with dependency
// tuples, plus a git entry (no version) that must be ignored for versions.
const mixLockFixture = "%{\n" +
	`  "bandit": {:hex, :bandit, "1.11.1", "hashA", [:mix], [{:hpax, "~> 1.0", [hex: :hpax, repo: "hexpm", optional: false]}, {:plug, "~> 1.18", [hex: :plug, repo: "hexpm", optional: false]}], "hexpm", "hashB"},` + "\n" +
	`  "ecto": {:hex, :ecto, "3.14.0", "hashC", [:mix], [{:decimal, "~> 3.0", [hex: :decimal, repo: "hexpm", optional: false]}, {:telemetry, "~> 1.0", [hex: :telemetry, repo: "hexpm", optional: false]}], "hexpm", "hashD"},` + "\n" +
	`  "hpax": {:hex, :hpax, "1.0.3", "hashE", [:mix], [], "hexpm", "hashF"},` + "\n" +
	`  "plug": {:hex, :plug, "1.18.0", "hashG", [:mix], [], "hexpm", "hashH"},` + "\n" +
	`  "decimal": {:hex, :decimal, "3.1.0", "hashI", [:mix], [], "hexpm", "hashJ"},` + "\n" +
	`  "telemetry": {:hex, :telemetry, "1.2.1", "hashK", [:mix], [], "hexpm", "hashL"},` + "\n" +
	`  "heroicons": {:git, "https://github.com/tailwindlabs/heroicons.git", "0435d4c", [tag: "v2.2.0"]},` + "\n" +
	"}\n"

func TestElixirParser_ParseMixLock(t *testing.T) {
	deps := NewElixirParser().ParseMixLock(mixLockFixture)
	byName := map[string]string{}
	for _, d := range deps {
		byName[d.Name] = d.Version
	}
	if byName["bandit"] != "1.11.1" || byName["ecto"] != "3.14.0" || byName["telemetry"] != "1.2.1" {
		t.Errorf("versions wrong: %v", byName)
	}
	// git dep has no version -> not emitted.
	if _, ok := byName["heroicons"]; ok {
		t.Errorf("git dep heroicons should be skipped (no version), got %v", byName)
	}
}

func TestParseMixLockGraph_FullEdges(t *testing.T) {
	g := ParseMixLockGraph(GraphInput{Lockfile: []byte(mixLockFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"bandit@1.11.1->hpax@1.0.3",
		"bandit@1.11.1->plug@1.18.0",
		"ecto@3.14.0->decimal@3.1.0",
		"ecto@3.14.0->telemetry@1.2.1",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseMixLockGraph_Direct(t *testing.T) {
	// bandit and ecto are not referenced by any other package -> roots.
	// hpax/plug/decimal/telemetry are transitive.
	gd := ParseMixLockGraph(GraphInput{Lockfile: []byte(mixLockFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["bandit@1.11.1"] || !got["ecto@3.14.0"] {
		t.Errorf("direct: expected bandit and ecto roots, got %v", got)
	}
	if got["hpax@1.0.3"] || got["telemetry@1.2.1"] {
		t.Errorf("direct: transitive deps must not appear, got %v", got)
	}
}

func TestParseMixLockGraph_Off(t *testing.T) {
	if g := ParseMixLockGraph(GraphInput{Lockfile: []byte(mixLockFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
