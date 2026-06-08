package parsers

import (
	"gopkg.in/yaml.v3"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// pubspecLockGraph is the lockfile view for graph extraction. Each package
// records its resolved version and dependency kind. pubspec.lock does not list
// per-package transitive edges, so the full graph is reconstructed from the
// manifest (direct) plus the lock's transitive set is exposed as direct-from-
// root only; true package->package edges require pubspec.yaml dependency data
// which the lock does not carry. We therefore emit root -> direct edges (from
// the manifest or the lock's "direct" markers) and, in full mode, also root ->
// transitive edges so the full closure is still represented.
type pubspecLockGraph struct {
	Packages map[string]struct {
		Dependency string `yaml:"dependency"`
		Version    string `yaml:"version"`
	} `yaml:"packages"`
}

// ParsePubspecLockGraph parses pubspec.lock into root-rooted edges. It
// implements the GraphProducer contract.
//
// pubspec.lock records resolved versions and whether each package is direct or
// transitive, but not the package-to-package edges. The graph is therefore
// rooted at the synthetic "." node:
//   - direct mode: "." -> each "direct main"/"direct dev" package
//   - full mode:   "." -> every resolved package (direct + transitive closure)
//
// This captures the full reachable set; finer package-to-package edges are not
// stated by the lockfile (a known limitation, like other flat-pin lockfiles).
func ParsePubspecLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock pubspecLockGraph
	if err := yaml.Unmarshal(input.Lockfile, &lock); err != nil {
		return result
	}

	for name, pkg := range lock.Packages {
		if pkg.Version == "" {
			continue
		}
		direct := pkg.Dependency == "direct main" || pkg.Dependency == "direct dev"
		if input.Mode == types.DependencyGraphDirect && !direct {
			continue
		}
		scope := types.ScopeProd
		if pkg.Dependency == "direct dev" {
			scope = types.ScopeDev
		}
		result.Edges = append(result.Edges, types.DependencyEdge{
			From:  ".",
			To:    name + "@" + pkg.Version,
			Scope: scope,
		})
	}
	return result
}
