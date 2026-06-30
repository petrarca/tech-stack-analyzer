package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/require"
)

// collectEdges walks the component tree and returns every dependency edge.
func collectEdges(p *types.Payload) []types.DependencyEdge {
	var edges []types.DependencyEdge
	var walk func(n *types.Payload)
	walk = func(n *types.Payload) {
		edges = append(edges, n.DependencyEdges...)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(p)
	return edges
}

// TestScanner_DependencyGraph_EndToEnd scans a real fixture project (package.json
// + a real npm package-lock.json from a public express install) with the
// dependency graph enabled, and verifies edges flow all the way through the
// scan: detector -> AttachLockfileGraph -> resolver chain -> producer ->
// payload. This is the regression guard the per-parser unit tests cannot give:
// it exercises the full wiring, not just the parse function.
func TestScanner_DependencyGraph_EndToEnd(t *testing.T) {
	// Real lockfile fixture (public packages only).
	lock, err := os.ReadFile(filepath.Join("parsers", "testdata", "lockfiles", "package-lock.json"))
	require.NoError(t, err)

	tempDir, err := os.MkdirTemp("", "scanner-depgraph")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	pkg := `{"name": "fixture-app", "version": "1.0.0", "dependencies": {"express": "^5.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(pkg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package-lock.json"), lock, 0o644))

	// Enable the graph for this test; restore afterwards so other tests are
	// unaffected (the mode is process-global).
	prev := components.DependencyGraphMode()
	components.SetDependencyGraphMode(types.DependencyGraphFull)
	defer components.SetDependencyGraphMode(prev)

	scanner, err := NewScanner(tempDir)
	require.NoError(t, err)
	result, err := scanner.Scan()
	require.NoError(t, err)

	edges := collectEdges(result)
	require.NotEmpty(t, edges, "expected dependency edges with --dependency-graph full")

	// express must reach its transitives, all tagged source=lockfile.
	got := map[string]string{}
	for _, e := range edges {
		got[e.From+"->"+e.To] = e.Source
	}
	require.Equal(t, "lockfile", got["express@5.2.1->router@2.2.0"], "express -> router edge, source lockfile")
	require.Equal(t, "lockfile", got["express@5.2.1->vary@1.1.2"], "express -> vary edge, source lockfile")
}

// TestScanner_DependencyGraph_OffByDefault verifies the graph is NOT emitted
// when the mode is off (the default), so a normal scan is unchanged.
func TestScanner_DependencyGraph_OffByDefault(t *testing.T) {
	lock, err := os.ReadFile(filepath.Join("parsers", "testdata", "lockfiles", "package-lock.json"))
	require.NoError(t, err)

	tempDir, err := os.MkdirTemp("", "scanner-depgraph-off")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	pkg := `{"name": "fixture-app", "version": "1.0.0", "dependencies": {"express": "^5.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(pkg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package-lock.json"), lock, 0o644))

	// Default is off; assert explicitly without changing global state.
	prev := components.DependencyGraphMode()
	components.SetDependencyGraphMode(types.DependencyGraphOff)
	defer components.SetDependencyGraphMode(prev)

	scanner, err := NewScanner(tempDir)
	require.NoError(t, err)
	result, err := scanner.Scan()
	require.NoError(t, err)

	require.Empty(t, collectEdges(result), "off mode must emit no dependency edges")
}

// TestScanner_DependencyGraph_DotNetDepsJSON verifies the full .deps.json wiring:
// a .csproj plus an <App>.deps.json (the .NET SDK's resolved build output) flows
// through the detector -> graph producer discovery -> resolver chain -> payload,
// emitting the transitive graph with the runtimepack prefix stripped.
func TestScanner_DependencyGraph_DotNetDepsJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scanner-depgraph-dotnet")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	csproj := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><AssemblyName>MyApp</AssemblyName><TargetFramework>net8.0</TargetFramework></PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Serilog.AspNetCore" Version="8.0.0" />
  </ItemGroup>
</Project>`
	depsJSON := `{
  "runtimeTarget": { "name": ".NETCoreApp,Version=v8.0" },
  "targets": {
    ".NETCoreApp,Version=v8.0": {
      "MyApp/1.0.0": { "dependencies": { "Serilog.AspNetCore": "8.0.0" } },
      "Serilog.AspNetCore/8.0.0": { "dependencies": { "Serilog": "3.1.1" } },
      "Serilog/3.1.1": {}
    }
  },
  "libraries": {
    "MyApp/1.0.0": { "type": "project" },
    "Serilog.AspNetCore/8.0.0": { "type": "package" },
    "Serilog/3.1.1": { "type": "package" }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "MyApp.csproj"), []byte(csproj), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "MyApp.deps.json"), []byte(depsJSON), 0o644))

	prev := components.DependencyGraphMode()
	components.SetDependencyGraphMode(types.DependencyGraphFull)
	defer components.SetDependencyGraphMode(prev)

	scanner, err := NewScanner(tempDir)
	require.NoError(t, err)
	result, err := scanner.Scan()
	require.NoError(t, err)

	got := map[string]string{}
	for _, e := range collectEdges(result) {
		got[e.From+"->"+e.To] = e.Source
	}
	require.Equal(t, "lockfile", got["Serilog.AspNetCore@8.0.0->Serilog@3.1.1"],
		"transitive edge from .deps.json should flow through, source lockfile; got %v", got)
}
