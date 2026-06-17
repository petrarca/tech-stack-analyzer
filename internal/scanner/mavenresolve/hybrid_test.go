package mavenresolve

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// fakeDepsDev is a stub deps.dev resolver: it resolves listed public coords and
// reports the rest as unresolved.
type fakeDepsDev struct {
	public map[string][]types.DependencyEdge // "name@version" -> edges
}

func (f fakeDepsDev) Name() string { return "fake-deps-dev" }
func (f fakeDepsDev) Resolve(req resolver.Request) (resolver.Result, error) {
	var edges []types.DependencyEdge
	var unresolved []string
	any := false
	for _, d := range req.Dependencies {
		key := d.Name + "@" + d.Version
		if e, ok := f.public[key]; ok {
			edges = append(edges, e...)
			any = true
		} else {
			unresolved = append(unresolved, key)
		}
	}
	return resolver.Result{Edges: edges, Resolved: any, Unresolved: unresolved}, nil
}

func TestHybridResolver_DepsDevPlusRepoFallback(t *testing.T) {
	// deps.dev resolves the public dep's subtree; the private dep is unresolved
	// and must be crawled from the repo.
	depsDev := fakeDepsDev{public: map[string][]types.DependencyEdge{
		"org.public:lib@1.0": {
			{From: ".", To: "org.public:lib@1.0"},
			{From: "org.public:lib@1.0", To: "org.public:transitive@2.0"},
		},
	}}
	// Repo has the private artifact and its child.
	repoSrc := pomMap{
		"com.private:app:1.0":  pom("com.private", "app", "1.0", "com.private:core:1.0"),
		"com.private:core:1.0": pom("com.private", "core", "1.0"),
	}
	repo := NewGraphResolver(repoSrc)

	h := NewHybridResolver(depsDev, repo)
	if h == nil {
		t.Fatal("expected a hybrid resolver")
	}

	res, err := h.Resolve(resolver.Request{
		Mode: types.DependencyGraphFull,
		Dependencies: []resolver.Coordinates{
			{Name: "org.public:lib", Version: "1.0"},
			{Name: "com.private:app", Version: "1.0"},
		},
	})
	if err != nil || !res.Resolved {
		t.Fatalf("resolve failed: err=%v resolved=%v", err, res.Resolved)
	}

	got := edgeSet(res.Edges)
	// deps.dev public subtree present.
	if !got["org.public:lib@1.0 -> org.public:transitive@2.0"] {
		t.Error("missing deps.dev public transitive edge")
	}
	// repo-crawled private subtree present.
	if !got["com.private:app@1.0 -> com.private:core@1.0"] {
		t.Error("missing repo-crawled private edge")
	}
}

func TestHybridResolver_NoUnresolvedSkipsCrawl(t *testing.T) {
	// All public: hybrid returns deps.dev result without crawling.
	depsDev := fakeDepsDev{public: map[string][]types.DependencyEdge{
		"org.public:lib@1.0": {{From: ".", To: "org.public:lib@1.0"}},
	}}
	// A repo source that would panic if queried (it won't be).
	repo := NewGraphResolver(pomMap{})

	h := NewHybridResolver(depsDev, repo)
	res, err := h.Resolve(resolver.Request{
		Mode:         types.DependencyGraphFull,
		Dependencies: []resolver.Coordinates{{Name: "org.public:lib", Version: "1.0"}},
	})
	if err != nil || !res.Resolved {
		t.Fatalf("resolve failed: err=%v resolved=%v", err, res.Resolved)
	}
	if !edgeSet(res.Edges)[". -> org.public:lib@1.0"] {
		t.Error("missing deps.dev edge")
	}
}

func TestNewHybridResolver_NilArgs(t *testing.T) {
	if NewHybridResolver(nil, NewGraphResolver(pomMap{})) != nil {
		t.Error("nil deps.dev should yield nil hybrid")
	}
	if NewHybridResolver(fakeDepsDev{}, nil) != nil {
		t.Error("nil repo should yield nil hybrid")
	}
}
