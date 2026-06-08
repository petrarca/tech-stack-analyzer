package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

const cyclonedxGraphFixture = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "metadata": {
    "component": {"bom-ref": "pkg:maven/com.example/app@1.0.0", "name": "app", "version": "1.0.0"}
  },
  "components": [
    {"bom-ref": "ref-guava", "name": "guava", "version": "32.1.3-jre"},
    {"bom-ref": "ref-failureaccess", "name": "failureaccess", "version": "1.0.1"},
    {"bom-ref": "pkg:maven/org.slf4j/slf4j-api@2.0.7", "purl": "pkg:maven/org.slf4j/slf4j-api@2.0.7"}
  ],
  "dependencies": [
    {"ref": "pkg:maven/com.example/app@1.0.0", "dependsOn": ["ref-guava", "pkg:maven/org.slf4j/slf4j-api@2.0.7"]},
    {"ref": "ref-guava", "dependsOn": ["ref-failureaccess"]}
  ]
}`

func TestParseCycloneDXGraph_FullEdges(t *testing.T) {
	g := ParseCycloneDXGraph(GraphInput{Lockfile: []byte(cyclonedxGraphFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		".->guava@32.1.3-jre", // root (metadata.component) edge
		".->slf4j-api@2.0.7",  // PURL-derived node id
		"guava@32.1.3-jre->failureaccess@1.0.1",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseCycloneDXGraph_Direct(t *testing.T) {
	gd := ParseCycloneDXGraph(GraphInput{Lockfile: []byte(cyclonedxGraphFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["guava@32.1.3-jre"] || !got["slf4j-api@2.0.7"] {
		t.Errorf("direct: expected root deps, got %v", got)
	}
	if got["failureaccess@1.0.1"] {
		t.Error("direct: must not include transitive failureaccess")
	}
}

func TestParseCycloneDXGraph_Off(t *testing.T) {
	if g := ParseCycloneDXGraph(GraphInput{Lockfile: []byte(cyclonedxGraphFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
