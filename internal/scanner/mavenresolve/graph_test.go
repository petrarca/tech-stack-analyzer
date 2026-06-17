package mavenresolve

import (
	"sort"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// pomMap is a PomSource backed by an in-memory coordinate->POM map.
type pomMap map[string]string // "g:a:v" -> pom xml

func (m pomMap) Name() string { return "test" }
func (m pomMap) FetchPOM(g, a, v string) ([]byte, string, bool) {
	if pom, ok := m[g+":"+a+":"+v]; ok {
		return []byte(pom), "", true
	}
	return nil, "", false
}

func pom(group, artifact, version string, deps ...string) string {
	x := `<project><groupId>` + group + `</groupId><artifactId>` + artifact +
		`</artifactId><version>` + version + `</version><dependencies>`
	// deps are "g:a:v" triples
	for _, d := range deps {
		var g, a, v string
		parts := splitTriple(d)
		g, a, v = parts[0], parts[1], parts[2]
		x += `<dependency><groupId>` + g + `</groupId><artifactId>` + a +
			`</artifactId><version>` + v + `</version></dependency>`
	}
	return x + `</dependencies></project>`
}

func splitTriple(s string) [3]string {
	var out [3]string
	i := 0
	start := 0
	for j := 0; j < len(s) && i < 2; j++ {
		if s[j] == ':' {
			out[i] = s[start:j]
			i++
			start = j + 1
		}
	}
	out[2] = s[start:]
	return out
}

func edgeSet(edges []types.DependencyEdge) map[string]bool {
	m := make(map[string]bool, len(edges))
	for _, e := range edges {
		m[e.From+" -> "+e.To] = true
	}
	return m
}

func TestGraphResolver_Transitive(t *testing.T) {
	// root -> A -> B -> C ; A -> C(direct, nearer) at a different version.
	src := pomMap{
		"g:a:1.0": pom("g", "a", "1.0", "g:b:1.0", "g:c:2.0"),
		"g:b:1.0": pom("g", "b", "1.0", "g:c:1.0"),
		"g:c:2.0": pom("g", "c", "2.0"),
		"g:c:1.0": pom("g", "c", "1.0"),
	}
	r := NewGraphResolver(src, nil)

	res, err := r.Resolve(resolver.Request{
		Mode:         types.DependencyGraphFull,
		Dependencies: []resolver.Coordinates{{Name: "g:a", Version: "1.0"}},
	})
	if err != nil || !res.Resolved {
		t.Fatalf("resolve failed: err=%v resolved=%v", err, res.Resolved)
	}
	got := edgeSet(res.Edges)

	// Nearest-wins: g:c resolved via a's direct dep (depth 2, version 2.0) before
	// b's transitive dep (depth 3, version 1.0). So every g:c node is @2.0.
	for _, want := range []string{
		". -> g:a@1.0",
		"g:a@1.0 -> g:b@1.0",
		"g:a@1.0 -> g:c@2.0",
		"g:b@1.0 -> g:c@2.0", // rewritten to the mediated 2.0, not b's declared 1.0
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, keys(got))
		}
	}
	// The non-mediated version must NOT appear.
	if got["g:b@1.0 -> g:c@1.0"] {
		t.Error("g:c@1.0 leaked; mediation should rewrite it to 2.0")
	}
}

func TestGraphResolver_DirectModeNoTransitive(t *testing.T) {
	src := pomMap{"g:a:1.0": pom("g", "a", "1.0", "g:b:1.0")}
	r := NewGraphResolver(src, nil)
	res, _ := r.Resolve(resolver.Request{
		Mode:         types.DependencyGraphDirect,
		Dependencies: []resolver.Coordinates{{Name: "g:a", Version: "1.0"}},
	})
	got := edgeSet(res.Edges)
	if !got[". -> g:a@1.0"] {
		t.Error("direct edge missing")
	}
	if got["g:a@1.0 -> g:b@1.0"] {
		t.Error("direct mode must not expand transitive edges")
	}
}

func TestGraphResolver_Cycle(t *testing.T) {
	// a -> b -> a (cycle) must terminate.
	src := pomMap{
		"g:a:1.0": pom("g", "a", "1.0", "g:b:1.0"),
		"g:b:1.0": pom("g", "b", "1.0", "g:a:1.0"),
	}
	r := NewGraphResolver(src, nil)
	res, err := r.Resolve(resolver.Request{
		Mode:         types.DependencyGraphFull,
		Dependencies: []resolver.Coordinates{{Name: "g:a", Version: "1.0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := edgeSet(res.Edges)
	if !got["g:a@1.0 -> g:b@1.0"] || !got["g:b@1.0 -> g:a@1.0"] {
		t.Errorf("cycle edges missing: %v", keys(got))
	}
}

func TestGraphResolver_MissingPomNoChildren(t *testing.T) {
	// a's POM is absent (e.g. private/unreachable): direct edge stays, no crash.
	src := pomMap{}
	r := NewGraphResolver(src, nil)
	res, _ := r.Resolve(resolver.Request{
		Mode:         types.DependencyGraphFull,
		Dependencies: []resolver.Coordinates{{Name: "g:a", Version: "1.0"}},
	})
	if !edgeSet(res.Edges)[". -> g:a@1.0"] {
		t.Error("direct edge should be present even when POM is unreachable")
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
