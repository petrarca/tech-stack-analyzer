//go:build online

// Live deps.dev integration test. Excluded from the default test suite (it
// makes a real network call to api.deps.dev). Run explicitly with:
//
//	go test -tags online ./internal/scanner/resolver/ -run Live
//
// or via the Taskfile test:online target.
package resolver

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// TestLive_DepsDevFetch_Maven hits the real deps.dev API for a well-known
// published Maven artifact and verifies a non-trivial transitive graph comes
// back. This is the only test that touches the network; it is opt-in.
func TestLive_DepsDevFetch_Maven(t *testing.T) {
	fetch := NewDepsDevFetcher("", nil) // public deps.dev, default client

	edges, err := fetch.ResolveGraph("maven", "org.springframework.boot:spring-boot-starter-web", "3.2.0", types.DependencyGraphFull)
	if err != nil {
		t.Fatalf("live deps.dev call failed: %v", err)
	}
	if len(edges) < 20 {
		t.Fatalf("expected a non-trivial Spring Boot graph, got %d edges", len(edges))
	}

	// spring-core is the high-fan-in hub; it must appear as a dependency target.
	found := false
	for _, e := range edges {
		if e.To == "org.springframework:spring-core@6.1.1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected spring-core@6.1.1 in the resolved graph")
	}
}

// TestLive_DepsDevFetch_NotFound verifies a private/unknown coordinate returns
// ErrCoordinateNotFound (so the resolver chain falls through) against the real API.
func TestLive_DepsDevFetch_NotFound(t *testing.T) {
	fetch := NewDepsDevFetcher("", nil)
	_, err := fetch.ResolveGraph("maven", "com.example.private:nonexistent-artifact-xyz", "9.9.9", types.DependencyGraphFull)
	if err == nil {
		t.Fatal("expected an error for an unknown coordinate")
	}
}
