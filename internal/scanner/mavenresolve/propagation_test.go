package mavenresolve

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestPropagateVersions_FillsVersionlessSiblings(t *testing.T) {
	// Module A resolves jackson-core@2.17.0; module B declares it versionless.
	root := &types.Payload{
		Children: []*types.Payload{
			{Dependencies: []types.Dependency{
				{Type: "maven", Name: "com.fasterxml.jackson.core:jackson-core", Version: "2.17.0"},
			}},
			{Dependencies: []types.Dependency{
				{Type: "maven", Name: "com.fasterxml.jackson.core:jackson-core", Version: "latest"},
				{Type: "maven", Name: "org.example:only-here", Version: "1.0.0"},
			}},
		},
	}

	PropagateVersions(root)

	b := root.Children[1].Dependencies[0]
	if b.Version != "2.17.0" {
		t.Errorf("versionless sibling = %q, want 2.17.0", b.Version)
	}
	if b.Metadata["source"] != "cross-module" {
		t.Errorf("source = %v, want cross-module", b.Metadata["source"])
	}
	if b.Metadata[types.MetadataKeyDeclared] != "latest" {
		t.Errorf("declared = %v, want latest", b.Metadata[types.MetadataKeyDeclared])
	}
}

func TestPropagateVersions_DoesNotOverwriteConcrete(t *testing.T) {
	root := &types.Payload{
		Children: []*types.Payload{
			{Dependencies: []types.Dependency{
				{Type: "maven", Name: "g:a", Version: "2.0.0"},
			}},
			{Dependencies: []types.Dependency{
				{Type: "maven", Name: "g:a", Version: "1.0.0"}, // concrete: must stay
			}},
		},
	}
	PropagateVersions(root)
	if got := root.Children[1].Dependencies[0].Version; got != "1.0.0" {
		t.Errorf("concrete version overwritten: got %q, want 1.0.0", got)
	}
}

func TestPropagateVersions_NonMavenIgnored(t *testing.T) {
	root := &types.Payload{
		Dependencies: []types.Dependency{
			{Type: "npm", Name: "react", Version: "18.2.0"},
			{Type: "npm", Name: "react", Version: "latest"},
		},
	}
	PropagateVersions(root)
	// npm is not propagated by this Maven pass.
	if root.Dependencies[1].Version != "latest" {
		t.Error("npm dependency must not be touched by Maven propagation")
	}
}
