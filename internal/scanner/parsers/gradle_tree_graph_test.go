package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// gradleTreeFixture mirrors `gradle dependencies` output: nested connectors,
// conflict resolution (-> version), and the (*) "seen elsewhere" marker.
const gradleTreeFixture = `
runtimeClasspath - Runtime classpath of source set 'main'.
+--- org.springframework:spring-web:6.1.0
|    +--- org.springframework:spring-beans:6.1.0
|    \--- org.springframework:spring-core:6.1.0
|         \--- org.springframework:spring-jcl:6.1.0
+--- com.google.guava:guava:32.1.0-jre -> 32.1.3-jre
\--- org.slf4j:slf4j-api:2.0.7

(*) - Indicates repeated occurrences of a transitive dependency subtree.
`

func TestParseGradleTreeGraph_FullEdges(t *testing.T) {
	g := ParseGradleTreeGraph(GraphInput{Lockfile: []byte(gradleTreeFixture), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		".->org.springframework:spring-web@6.1.0",
		"org.springframework:spring-web@6.1.0->org.springframework:spring-beans@6.1.0",
		"org.springframework:spring-web@6.1.0->org.springframework:spring-core@6.1.0",
		"org.springframework:spring-core@6.1.0->org.springframework:spring-jcl@6.1.0",
		".->com.google.guava:guava@32.1.3-jre", // conflict-resolved version
		".->org.slf4j:slf4j-api@2.0.7",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseGradleTreeGraph_Direct(t *testing.T) {
	gd := ParseGradleTreeGraph(GraphInput{Lockfile: []byte(gradleTreeFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["org.springframework:spring-web@6.1.0"] || !got["com.google.guava:guava@32.1.3-jre"] || !got["org.slf4j:slf4j-api@2.0.7"] {
		t.Errorf("direct: expected the three top-level deps, got %v", got)
	}
	if got["org.springframework:spring-beans@6.1.0"] {
		t.Error("direct: must not include transitive spring-beans")
	}
}

func TestParseGradleTreeGraph_Off(t *testing.T) {
	if g := ParseGradleTreeGraph(GraphInput{Lockfile: []byte(gradleTreeFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}
}
