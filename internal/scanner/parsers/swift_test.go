package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// v2/v3: top-level pins with identity/location/state.version.
const packageResolvedV2 = `{
  "pins" : [
    {
      "identity" : "alamofire",
      "kind" : "remoteSourceControl",
      "location" : "https://github.com/Alamofire/Alamofire.git",
      "state" : { "revision" : "abc", "version" : "5.8.0" }
    },
    {
      "identity" : "swift-log",
      "location" : "https://github.com/apple/swift-log.git",
      "state" : { "version" : "1.5.3" }
    }
  ],
  "version" : 2
}`

// v1: nested object.pins with package/repositoryURL.
const packageResolvedV1 = `{
  "object" : {
    "pins" : [
      {
        "package" : "Alamofire",
        "repositoryURL" : "https://github.com/Alamofire/Alamofire.git",
        "state" : { "version" : "5.8.0" }
      }
    ]
  },
  "version" : 1
}`

func TestSwiftParser_ParsePackageResolvedV2(t *testing.T) {
	deps := NewSwiftParser().ParsePackageResolved(packageResolvedV2)
	byName := map[string]string{}
	for _, d := range deps {
		byName[d.Name] = d.Version
	}
	if byName["alamofire"] != "5.8.0" || byName["swift-log"] != "1.5.3" {
		t.Errorf("v2 parse wrong: %v", byName)
	}
}

func TestSwiftParser_ParsePackageResolvedV1(t *testing.T) {
	deps := NewSwiftParser().ParsePackageResolved(packageResolvedV1)
	if len(deps) != 1 || deps[0].Name != "alamofire" || deps[0].Version != "5.8.0" {
		t.Errorf("v1 parse wrong (name derived from repositoryURL): %+v", deps)
	}
}

func TestParsePackageResolvedGraph_Modes(t *testing.T) {
	if g := ParsePackageResolvedGraph(GraphInput{Lockfile: []byte(packageResolvedV2), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off: expected 0 edges, got %d", len(g.Edges))
	}

	g := ParsePackageResolvedGraph(GraphInput{Lockfile: []byte(packageResolvedV2), Mode: types.DependencyGraphFull})
	got := map[string]bool{}
	for _, e := range g.Edges {
		if e.From != "." {
			t.Errorf("expected root-rooted from='.', got %q", e.From)
		}
		got[e.To] = true
	}
	if !got["alamofire@5.8.0"] || !got["swift-log@1.5.3"] {
		t.Errorf("expected root edges to both pins, got %v", got)
	}
}
