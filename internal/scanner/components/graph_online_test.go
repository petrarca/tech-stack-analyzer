package components

import (
	"errors"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// stubNoFileProvider satisfies types.Provider with no filesystem; only
// GetBasePath is meaningful (used as an online-cache key).
type stubNoFileProvider struct{}

func (stubNoFileProvider) ListDir(string) ([]types.File, error) { return nil, errors.New("no tree") }
func (stubNoFileProvider) Open(string) (string, error)          { return "", errors.New("no tree") }
func (stubNoFileProvider) Exists(string) (bool, error)          { return false, nil }
func (stubNoFileProvider) IsDir(string) (bool, error)           { return false, nil }
func (stubNoFileProvider) ReadFile(string) ([]byte, error)      { return nil, errors.New("no tree") }
func (stubNoFileProvider) GetBasePath() string                  { return "/stub" }

func buildTree() *types.Payload {
	root := types.NewPayload("app", nil)
	root.SetComponentType("nodejs")
	root.Dependencies = []types.Dependency{{Type: "npm", Name: "express", Version: "4.18.2", Direct: true}}
	child := types.NewPayload("py", nil)
	child.SetComponentType("python")
	child.Dependencies = []types.Dependency{{Type: "pypi", Name: "requests", Version: "2.32.3", Direct: true}}
	noDeps := types.NewPayload("empty", nil) // no dependencies -> skipped
	root.Children = []*types.Payload{child, noDeps}
	return root
}

func TestResolvePayloadGraphOnline_OffMode(t *testing.T) {
	SetDependencyGraphMode(types.DependencyGraphOff)
	defer SetDependencyGraphMode(types.DependencyGraphOff)

	n := ResolvePayloadGraphOnline(buildTree(), stubNoFileProvider{})
	if n != 0 {
		t.Errorf("off mode must resolve nothing, got %d", n)
	}
}

func TestResolvePayloadGraphOnline_NilPayload(t *testing.T) {
	SetDependencyGraphMode(types.DependencyGraphFull)
	defer SetDependencyGraphMode(types.DependencyGraphOff)
	if n := ResolvePayloadGraphOnline(nil, stubNoFileProvider{}); n != 0 {
		t.Errorf("nil payload must return 0, got %d", n)
	}
}

func TestResolvePayloadGraphOnline_WalksComponentsWithDeps(t *testing.T) {
	// With deps.dev disabled and no Maven repo, the online resolvers are no-ops,
	// but the walker still visits each component that has dependencies. This
	// verifies the tree walk and the "skip components without deps" rule
	// without any network access.
	SetDependencyGraphMode(types.DependencyGraphFull)
	SetUseDepsDev(false)
	SetMavenGraphSource("none")
	defer func() {
		SetDependencyGraphMode(types.DependencyGraphOff)
		SetMavenGraphSource("")
	}()

	n := ResolvePayloadGraphOnline(buildTree(), stubNoFileProvider{})
	// root (npm) + child (pypi) have deps; the empty child is skipped.
	if n != 2 {
		t.Errorf("expected 2 components processed (root + python child), got %d", n)
	}
}
