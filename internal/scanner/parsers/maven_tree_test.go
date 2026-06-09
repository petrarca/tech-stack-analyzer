package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// mavenTreeFixture is a trimmed real `mvn dependency:tree -DoutputType=json`
// output: the project root with two direct deps (guava with a transitive child,
// junit with a transitive child).
const mavenTreeFixture = `{
  "groupId": "com.example",
  "artifactId": "sample-app",
  "version": "1.0.0",
  "type": "jar",
  "scope": "",
  "children": [
    {
      "groupId": "com.google.guava",
      "artifactId": "guava",
      "version": "32.1.3-jre",
      "type": "jar",
      "scope": "compile",
      "children": [
        {
          "groupId": "com.google.guava",
          "artifactId": "failureaccess",
          "version": "1.0.1",
          "type": "jar",
          "scope": "compile"
        }
      ]
    },
    {
      "groupId": "junit",
      "artifactId": "junit",
      "version": "4.13.2",
      "type": "jar",
      "scope": "test",
      "children": [
        {
          "groupId": "org.hamcrest",
          "artifactId": "hamcrest-core",
          "version": "1.3",
          "type": "jar",
          "scope": "test"
        }
      ]
    }
  ]
}`

func TestParseMavenTreeGraph_FullEdges(t *testing.T) {
	graph := ParseMavenTreeGraph(GraphInput{Lockfile: []byte(mavenTreeFixture), Mode: types.DependencyGraphFull})

	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	for _, want := range []string{
		"com.example:sample-app@1.0.0->com.google.guava:guava@32.1.3-jre",
		"com.google.guava:guava@32.1.3-jre->com.google.guava:failureaccess@1.0.1",
		"com.example:sample-app@1.0.0->junit:junit@4.13.2",
		"junit:junit@4.13.2->org.hamcrest:hamcrest-core@1.3",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParseMavenTreeGraph_Modes(t *testing.T) {
	if g := ParseMavenTreeGraph(GraphInput{Lockfile: []byte(mavenTreeFixture), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}

	// direct: root -> its two direct children only, from the synthetic "." node.
	gd := ParseMavenTreeGraph(GraphInput{Lockfile: []byte(mavenTreeFixture), Mode: types.DependencyGraphDirect})
	got := map[string]bool{}
	for _, e := range gd.Edges {
		if e.From != "." {
			t.Errorf("direct mode: expected from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["com.google.guava:guava@32.1.3-jre"] || !got["junit:junit@4.13.2"] {
		t.Errorf("direct mode: expected root edges to guava and junit, got %v", got)
	}
	if got["org.hamcrest:hamcrest-core@1.3"] {
		t.Error("direct mode: must not include transitive hamcrest-core edge")
	}
}
