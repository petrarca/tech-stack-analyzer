package resolver

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// mapProvider is a minimal in-memory Provider keyed by full path.
type mapProvider struct{ files map[string][]byte }

func (m mapProvider) ListDir(string) ([]types.File, error) { return nil, nil }
func (m mapProvider) Open(string) (string, error)          { return "", nil }
func (m mapProvider) Exists(string) (bool, error)          { return false, nil }
func (m mapProvider) IsDir(string) (bool, error)           { return false, nil }
func (m mapProvider) GetBasePath() string                  { return "/" }
func (m mapProvider) ReadFile(p string) ([]byte, error) {
	if b, ok := m.files[p]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}

// stubParse returns a fixed edge set regardless of content, honoring off.
func stubParse(edges ...types.DependencyEdge) parsers.ParseGraphFunc {
	return func(input parsers.GraphInput) parsers.LockGraph {
		if input.Mode == types.DependencyGraphOff {
			return parsers.LockGraph{}
		}
		return parsers.LockGraph{Edges: edges}
	}
}

func edgeSet(edges []types.DependencyEdge) map[string]string {
	out := map[string]string{}
	for _, e := range edges {
		out[e.From+"->"+e.To] = e.Source
	}
	return out
}

func TestChain_LockfileWins(t *testing.T) {
	prov := mapProvider{files: map[string][]byte{
		"/app/pnpm-lock.yaml": []byte("x"),
	}}
	lock := NewLockfileResolver(
		LockfileProducer{Lockfile: "pnpm-lock.yaml", Parse: stubParse(
			types.DependencyEdge{From: "a@1", To: "b@2"},
		)},
	)
	online := &DepsDevResolver{Enabled: true, Fetch: func(_, _, _ string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) {
		t.Fatal("online resolver must not be consulted when a lockfile resolves")
		return nil, nil
	}}
	chain := NewChain(lock, online)

	res, err := chain.Resolve(Request{Dir: "/app", Provider: prov, Mode: types.DependencyGraphFull})
	if err != nil {
		t.Fatal(err)
	}
	got := edgeSet(res.Edges)
	if got["a@1->b@2"] != string(SourceLockfile) {
		t.Errorf("expected lockfile-tagged edge, got %v", got)
	}
}

func TestChain_OnlineFallbackFillsGap(t *testing.T) {
	prov := mapProvider{files: map[string][]byte{}} // no lockfile present
	lock := NewLockfileResolver(
		LockfileProducer{Lockfile: "pom.xml", Parse: stubParse()},
	)
	online := &DepsDevResolver{
		Enabled: true,
		Fetch: func(system, name, version string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) {
			if system != "maven" || name != "g:a" || version != "1.0" {
				t.Fatalf("unexpected coordinates: %s %s %s", system, name, version)
			}
			return []types.DependencyEdge{{From: "g:a@1.0", To: "g:b@2.0"}}, nil
		},
	}
	chain := NewChain(lock, online)

	res, err := chain.Resolve(Request{
		Dir:         "/svc",
		Provider:    prov,
		Mode:        types.DependencyGraphFull,
		Ecosystem:   "java",
		Coordinates: &Coordinates{Name: "g:a", Version: "1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := edgeSet(res.Edges)
	if got["g:a@1.0->g:b@2.0"] != string(SourceDepsDev) {
		t.Errorf("expected deps.dev-tagged fallback edge, got %v", got)
	}
}

func TestChain_OffShortCircuits(t *testing.T) {
	online := &DepsDevResolver{Enabled: true, Fetch: func(_, _, _ string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) {
		t.Fatal("must not resolve in off mode")
		return nil, nil
	}}
	chain := NewChain(online)
	res, err := chain.Resolve(Request{Mode: types.DependencyGraphOff})
	if err != nil || len(res.Edges) != 0 {
		t.Errorf("off mode must yield no edges, got %v err=%v", res.Edges, err)
	}
}

func TestLockfileResolver_PresentButEmptyDoesNotFallThrough(t *testing.T) {
	prov := mapProvider{files: map[string][]byte{"/leaf/uv.lock": []byte("x")}}
	lock := NewLockfileResolver(LockfileProducer{Lockfile: "uv.lock", Parse: stubParse()})
	online := &DepsDevResolver{Enabled: true, Fetch: func(_, _, _ string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) {
		t.Fatal("present lockfile (even with zero edges) is authoritative; must not fall through")
		return nil, nil
	}}
	chain := NewChain(lock, online)

	res, err := chain.Resolve(Request{Dir: "/leaf", Provider: prov, Mode: types.DependencyGraphDirect})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edges) != 0 {
		t.Errorf("expected no edges from empty lockfile, got %v", res.Edges)
	}
}

func TestDepsDevResolver_DisabledOrNoFetcherFallsThrough(t *testing.T) {
	cases := []*DepsDevResolver{
		{Enabled: false, Fetch: func(_, _, _ string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) { return nil, nil }},
		{Enabled: true, Fetch: nil},
	}
	for i, r := range cases {
		res, err := r.Resolve(Request{
			Mode:        types.DependencyGraphFull,
			Ecosystem:   "java",
			Coordinates: &Coordinates{Name: "g:a", Version: "1.0"},
		})
		if err != nil || res.Resolved {
			t.Errorf("case %d: expected fall-through (Resolved=false), got resolved=%v err=%v", i, res.Resolved, err)
		}
	}
}

func TestDepsDevResolver_PropagatesError(t *testing.T) {
	boom := errors.New("network down")
	r := &DepsDevResolver{Enabled: true, Fetch: func(_, _, _ string, _ types.DependencyGraphMode) ([]types.DependencyEdge, error) {
		return nil, boom
	}}
	_, err := r.Resolve(Request{
		Mode:        types.DependencyGraphFull,
		Ecosystem:   "java",
		Coordinates: &Coordinates{Name: "g:a", Version: "1.0"},
	})
	if !errors.Is(err, boom) {
		t.Errorf("expected error propagation, got %v", err)
	}
}
