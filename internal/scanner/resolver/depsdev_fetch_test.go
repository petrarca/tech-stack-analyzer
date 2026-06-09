package resolver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// depsDevDAG is the validated deps.dev :dependencies response shape: a
// deduplicated DAG with a SELF root, DIRECT/INDIRECT nodes, and integer-indexed
// edges.
const depsDevDAG = `{
  "nodes": [
    {"versionKey": {"system": "MAVEN", "name": "com.example:app", "version": "1.0.0"}, "relation": "SELF"},
    {"versionKey": {"system": "MAVEN", "name": "com.google.guava:guava", "version": "32.1.3-jre"}, "relation": "DIRECT"},
    {"versionKey": {"system": "MAVEN", "name": "com.google.guava:failureaccess", "version": "1.0.1"}, "relation": "INDIRECT"}
  ],
  "edges": [
    {"fromNode": 0, "toNode": 1, "requirement": "32.1.3-jre"},
    {"fromNode": 1, "toNode": 2, "requirement": "1.0.1"}
  ]
}`

func newDepsDevServer(t *testing.T, calls *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		// Verify the gRPC-transcoding ":dependencies" verb and encoded colon.
		if !strings.HasSuffix(r.URL.Path, ":dependencies") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Path, "/v3/systems/maven/packages/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(depsDevDAG))
	}))
}

func TestDepsDevFetcher_FullAndDirect(t *testing.T) {
	var calls int32
	srv := newDepsDevServer(t, &calls)
	defer srv.Close()

	fetch := NewDepsDevFetcher(srv.URL, srv.Client())

	// full: root edge (from ".") + transitive edge.
	full, err := fetch("maven", "com.example:app", "1.0.0", types.DependencyGraphFull)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range full {
		got[e.From+"->"+e.To] = true
	}
	if !got[".->com.google.guava:guava@32.1.3-jre"] {
		t.Errorf("full: missing root edge, got %v", got)
	}
	if !got["com.google.guava:guava@32.1.3-jre->com.google.guava:failureaccess@1.0.1"] {
		t.Errorf("full: missing transitive edge, got %v", got)
	}

	// direct: only the root's edges.
	direct, err := fetch("maven", "com.example:app", "1.0.0", types.DependencyGraphDirect)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != 1 || direct[0].From != "." || direct[0].To != "com.google.guava:guava@32.1.3-jre" {
		t.Errorf("direct: expected [. -> guava], got %v", direct)
	}
}

func TestDepsDevFetcher_CachesPerCoordinate(t *testing.T) {
	var calls int32
	srv := newDepsDevServer(t, &calls)
	defer srv.Close()
	fetch := NewDepsDevFetcher(srv.URL, srv.Client())

	for i := 0; i < 3; i++ {
		if _, err := fetch("maven", "com.example:app", "1.0.0", types.DependencyGraphFull); err != nil {
			t.Fatal(err)
		}
	}
	// Same coordinate+mode resolved three times -> exactly one HTTP call.
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", n)
	}
}

func TestDepsDevFetcher_NotFoundIsEmptyNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	fetch := NewDepsDevFetcher(srv.URL, srv.Client())

	edges, err := fetch("maven", "no:such", "9.9.9", types.DependencyGraphFull)
	if err != nil {
		t.Errorf("404 must not be an error, got %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("404 must yield no edges, got %v", edges)
	}
}

func TestDepsDevFetcher_RateLimitedIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	fetch := NewDepsDevFetcher(srv.URL, srv.Client())

	if _, err := fetch("maven", "x:y", "1.0", types.DependencyGraphFull); err == nil {
		t.Error("429 must surface as an error")
	}
}

func TestDepsDevFetcher_EndpointOverride(t *testing.T) {
	// A facade/mirror at a custom base URL must be used verbatim.
	var hit int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hit, 1)
		_, _ = w.Write([]byte(depsDevDAG))
	}))
	defer srv.Close()

	fetch := NewDepsDevFetcher(srv.URL+"/", srv.Client()) // trailing slash trimmed
	if _, err := fetch("maven", "com.example:app", "1.0.0", types.DependencyGraphFull); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hit) != 1 {
		t.Error("custom endpoint was not used")
	}
}
