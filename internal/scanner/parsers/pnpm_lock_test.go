package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestParsePnpmLock(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		wantDeps map[string]string
	}{
		{
			name: "basic dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
      lodash:
        specifier: ^4.17.21
        version: 4.17.21
`,
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"lodash":  "4.17.21",
			},
		},
		{
			name: "with dev dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
    devDependencies:
      jest:
        specifier: ^29.0.0
        version: 29.7.0
`,
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"jest":    "29.7.0",
			},
		},
		{
			name: "only dev dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    devDependencies:
      typescript:
        specifier: ^5.0.0
        version: 5.3.2
`,
			expected: 1,
			wantDeps: map[string]string{
				"typescript": "5.3.2",
			},
		},
		{
			name: "no root importer",
			content: `lockfileVersion: '9.0'

importers:
  packages/sub:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
`,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name: "v9 with packages block and peer-suffix versions",
			content: `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      mylib:
        specifier: ^1.2.0
        version: 1.2.11(peerdep@4.3.6)
      '@myorg/widget':
        specifier: ^5.0.0
        version: 5.2.8
    devDependencies:
      mytestlib:
        specifier: ^3.0.0
        version: 3.1.0(typescript@5.4.0)

packages:
  'mylib@1.2.11':
    resolution: {integrity: sha512-abc}
  '@myorg/widget@5.2.8':
    resolution: {integrity: sha512-def}
  'mytestlib@3.1.0':
    resolution: {integrity: sha512-ghi}
`,
			expected: 3,
			wantDeps: map[string]string{
				"mylib":         "1.2.11",
				"@myorg/widget": "5.2.8",
				"mytestlib":     "3.1.0",
			},
		},
		{
			name:     "empty content",
			content:  ``,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name:     "invalid yaml",
			content:  `{invalid: yaml: content`,
			expected: 0,
			wantDeps: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParsePnpmLock([]byte(tt.content))

			if len(deps) != tt.expected {
				t.Errorf("ParsePnpmLock() got %d dependencies, want %d", len(deps), tt.expected)
			}

			for _, dep := range deps {
				if dep.Type != "npm" {
					t.Errorf("ParsePnpmLock() dep.Type = %s, want npm", dep.Type)
				}
				if dep.SourceFile != "pnpm-lock.yaml" {
					t.Errorf("ParsePnpmLock() dep.SourceFile = %s, want pnpm-lock.yaml", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParsePnpmLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}

func TestParsePnpmLockGraph_V9Edges(t *testing.T) {
	content := `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      mylib:
        specifier: ^1.0.0
        version: 1.0.0

packages:
  'mylib@1.0.0':
    resolution: {integrity: sha512-aaa}
  'dep-a@2.0.0':
    resolution: {integrity: sha512-bbb}
  'dep-b@3.0.0':
    resolution: {integrity: sha512-ccc}

snapshots:
  'mylib@1.0.0(react@18.0.0)':
    dependencies:
      dep-a: 2.0.0
      dep-b: 3.0.0(peer@1.0.0)
  'dep-a@2.0.0':
    dependencies:
      dep-b: 3.0.0
`
	graph := ParsePnpmLockGraph(GraphInput{Lockfile: []byte(content), Mode: types.DependencyGraphFull})
	if len(graph.Dependencies) == 0 {
		t.Fatal("expected at least the direct dependency")
	}
	got := map[string]bool{}
	for _, e := range graph.Edges {
		got[e.From+"->"+e.To] = true
	}
	// Peer suffixes must be stripped from both endpoints.
	for _, want := range []string{
		"mylib@1.0.0->dep-a@2.0.0",
		"mylib@1.0.0->dep-b@3.0.0",
		"dep-a@2.0.0->dep-b@3.0.0",
	} {
		if !got[want] {
			t.Errorf("missing edge %q; got %v", want, got)
		}
	}
}

func TestParsePnpmLockGraph_Modes(t *testing.T) {
	content := `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      mylib:
        specifier: ^1.0.0
        version: 1.0.0

packages:
  'mylib@1.0.0':
    resolution: {integrity: sha512-aaa}
  'dep-a@2.0.0':
    resolution: {integrity: sha512-bbb}

snapshots:
  'mylib@1.0.0':
    dependencies:
      dep-a: 2.0.0
`
	// off: no edges
	if g := ParsePnpmLockGraph(GraphInput{Lockfile: []byte(content), Mode: types.DependencyGraphOff}); len(g.Edges) != 0 {
		t.Errorf("off mode: expected 0 edges, got %d", len(g.Edges))
	}
	// direct: only root -> direct edge
	gd := ParsePnpmLockGraph(GraphInput{Lockfile: []byte(content), Mode: types.DependencyGraphDirect})
	if len(gd.Edges) != 1 || gd.Edges[0].From != "." || gd.Edges[0].To != "mylib@1.0.0" {
		t.Errorf("direct mode: expected [. -> mylib@1.0.0], got %v", gd.Edges)
	}
	if gd.Edges[0].Scope != types.ScopeProd {
		t.Errorf("direct mode: expected prod scope, got %q", gd.Edges[0].Scope)
	}
	// full: transitive edge present
	gf := ParsePnpmLockGraph(GraphInput{Lockfile: []byte(content), Mode: types.DependencyGraphFull})
	found := false
	for _, e := range gf.Edges {
		if e.From == "mylib@1.0.0" && e.To == "dep-a@2.0.0" {
			found = true
		}
	}
	if !found {
		t.Errorf("full mode: expected mylib@1.0.0 -> dep-a@2.0.0, got %v", gf.Edges)
	}
}
