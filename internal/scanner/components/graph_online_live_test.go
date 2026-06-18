//go:build online

package components

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// TestLiveResolvePayloadGraphOnline verifies that the off-scan resolver
// (used by the sbom command) resolves a real transitive dependency graph from
// coordinates alone -- no source files -- via deps.dev. Build-tag gated
// (online); excluded from the default test suite.
func TestLiveResolvePayloadGraphOnline(t *testing.T) {
	SetDependencyGraphMode(types.DependencyGraphFull)
	SetUseDepsDev(true)
	defer func() {
		SetDependencyGraphMode(types.DependencyGraphOff)
		SetUseDepsDev(false)
	}()

	root := types.NewPayload("app", nil)
	root.SetComponentType("nodejs")
	root.Dependencies = []types.Dependency{
		{Type: "npm", Name: "express", Version: "4.18.2", Direct: true},
	}

	n := ResolvePayloadGraphOnline(root, stubNoFileProvider{})
	if n != 1 {
		t.Fatalf("expected 1 component resolved, got %d", n)
	}
	if len(root.DependencyEdges) == 0 {
		t.Fatal("expected transitive edges from deps.dev, got none")
	}

	// express pulls in accepts, body-parser, etc. transitively.
	nodes := map[string]bool{}
	for _, e := range root.DependencyEdges {
		nodes[e.To] = true
	}
	found := false
	for n := range nodes {
		if len(n) >= 7 && n[:7] == "accepts" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected transitive node 'accepts@...' in resolved edges; got %d edges", len(root.DependencyEdges))
	}
}
