package aggregator

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// dep is a small constructor for readable test fixtures.
func dep(name, version, scope string, direct bool) types.Dependency {
	return types.Dependency{Type: "npm", Name: name, Version: version, Scope: scope, Direct: direct}
}

// findDep returns the aggregated dependency for a name@version, or fails.
func findDep(t *testing.T, deps []types.Dependency, name, version string) types.Dependency {
	t.Helper()
	for _, d := range deps {
		if d.Name == name && d.Version == version {
			return d
		}
	}
	t.Fatalf("dependency %s@%s not found in aggregated output", name, version)
	return types.Dependency{}
}

// When the same package appears in multiple modules with conflicting scope/direct
// flags, aggregation must MERGE them (Direct = OR, Scope = most-exposed wins)
// instead of letting the last-walked occurrence overwrite the others.
func TestCollectDependencies_MergesConflictingFlags(t *testing.T) {
	// root has a child; the same packages appear in both with different flags.
	root := &types.Payload{
		Dependencies: []types.Dependency{
			// transitive/dev occurrence walked FIRST
			dep("fastify", "4.29.1", "prod", false),
			dep("tslib", "2.8.1", "prod", false),
			dep("uuid", "8.3.2", "dev", false),
		},
		Children: []*types.Payload{
			{
				Dependencies: []types.Dependency{
					// direct occurrence walked LATER for fastify; for tslib a less-exposed
					// scope is walked later (must NOT win); uuid becomes direct+prod later.
					dep("fastify", "4.29.1", "prod", true),
					dep("tslib", "2.8.1", "optional", false),
					dep("uuid", "8.3.2", "prod", true),
				},
			},
		},
	}

	a := NewAggregator([]string{"dependencies"})
	deps := a.collectDependencies(&types.Payload{Children: []*types.Payload{root}})

	// fastify: direct anywhere -> direct=true; prod everywhere -> prod.
	if d := findDep(t, deps, "fastify", "4.29.1"); !d.Direct || d.Scope != "prod" {
		t.Errorf("fastify merged wrong: got direct=%v scope=%q, want direct=true scope=prod", d.Direct, d.Scope)
	}

	// tslib: prod must beat optional regardless of walk order; never direct.
	if d := findDep(t, deps, "tslib", "2.8.1"); d.Direct || d.Scope != "prod" {
		t.Errorf("tslib merged wrong: got direct=%v scope=%q, want direct=false scope=prod", d.Direct, d.Scope)
	}

	// uuid: direct anywhere -> true; prod must beat dev.
	if d := findDep(t, deps, "uuid", "8.3.2"); !d.Direct || d.Scope != "prod" {
		t.Errorf("uuid merged wrong: got direct=%v scope=%q, want direct=true scope=prod", d.Direct, d.Scope)
	}
}

// The merge result must be independent of the order modules are walked.
func TestCollectDependencies_OrderIndependent(t *testing.T) {
	directProd := dep("lodash", "4.17.21", "prod", true)
	transitiveDev := dep("lodash", "4.17.21", "dev", false)

	// Two trees with the occurrences in opposite order must yield the same merge.
	treeA := &types.Payload{Children: []*types.Payload{
		{Dependencies: []types.Dependency{transitiveDev}},
		{Dependencies: []types.Dependency{directProd}},
	}}
	treeB := &types.Payload{Children: []*types.Payload{
		{Dependencies: []types.Dependency{directProd}},
		{Dependencies: []types.Dependency{transitiveDev}},
	}}

	a := NewAggregator([]string{"dependencies"})
	da := findDep(t, a.collectDependencies(treeA), "lodash", "4.17.21")
	db := findDep(t, a.collectDependencies(treeB), "lodash", "4.17.21")

	if da.Direct != db.Direct || da.Scope != db.Scope {
		t.Fatalf("aggregation not order-independent: A{direct=%v scope=%q} B{direct=%v scope=%q}",
			da.Direct, da.Scope, db.Direct, db.Scope)
	}
	if !da.Direct || da.Scope != "prod" {
		t.Errorf("merged lodash wrong: got direct=%v scope=%q, want direct=true scope=prod", da.Direct, da.Scope)
	}
}

// A non-conflicting single occurrence must pass through unchanged.
func TestCollectDependencies_SingleOccurrenceUnchanged(t *testing.T) {
	root := &types.Payload{Dependencies: []types.Dependency{dep("react", "18.2.0", "prod", true)}}
	a := NewAggregator([]string{"dependencies"})
	d := findDep(t, a.collectDependencies(root), "react", "18.2.0")
	if !d.Direct || d.Scope != "prod" {
		t.Errorf("single occurrence altered: got direct=%v scope=%q, want direct=true scope=prod", d.Direct, d.Scope)
	}
}

func TestMergeScope(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"prod", "dev", "prod"},
		{"dev", "prod", "prod"},
		{"prod", "optional", "prod"},
		{"optional", "prod", "prod"},
		{"dev", "test", "dev"},
		{"optional", "dev", "optional"},
		{"", "dev", "dev"},
		{"dev", "", "dev"},
		{"weirdscope", "", "weirdscope"}, // unknown beats empty
		{"prod", "weirdscope", "prod"},   // known beats unknown
	}
	for _, c := range cases {
		if got := mergeScope(c.a, c.b); got != c.want {
			t.Errorf("mergeScope(%q,%q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}
