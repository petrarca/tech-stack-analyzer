package parsers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseCpanfileSnapshotGraph parses cpanfile.snapshot and returns the
// distribution-to-distribution edges. It implements the GraphProducer contract.
//
// cpanfile.snapshot states a real graph: each distribution's requires: block
// lists the modules it needs, and each distribution's provides: block maps
// modules to that distribution. Resolving a required module to its providing
// distribution yields distribution -> distribution edges ("name@version").
// Direct deps are not marked (they live in cpanfile), so direct mode uses the
// not-referenced heuristic: a distribution required by no other is a root.
func ParseCpanfileSnapshotGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	dists := parseCpanDistributions(string(input.Lockfile))

	// module name -> providing distribution node ("name@version").
	distByModule := make(map[string]string)
	for _, d := range dists {
		if d.Version == "" {
			continue
		}
		node := d.Name + "@" + d.Version
		for _, m := range d.Provides {
			distByModule[m] = node
		}
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = cpanDirectEdges(dists, distByModule)
	case types.DependencyGraphFull:
		result.Edges, result.Unresolved = cpanFullEdges(dists, distByModule)
	}
	return result
}

// cpanFullEdges builds every distribution -> required-distribution edge.
func cpanFullEdges(dists []cpanDist, distByModule map[string]string) (edges []types.DependencyEdge, unresolved []string) {
	seen := make(map[string]bool)
	for _, d := range dists {
		if d.Version == "" {
			continue
		}
		from := d.Name + "@" + d.Version
		for _, mod := range d.Requires {
			to, ok := distByModule[mod]
			if !ok {
				// Core/Perl modules (e.g. strict, warnings, Carp) are not
				// shipped as distributions in the snapshot; skip silently for
				// well-known cases, report the rest.
				if !cpanCoreModule(mod) {
					unresolved = append(unresolved, from+" -> "+mod)
				}
				continue
			}
			if to == from {
				continue
			}
			if key := from + "|" + to; !seen[key] {
				seen[key] = true
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges, unresolved
}

// cpanDirectEdges builds root -> direct edges. A root distribution is one not
// required (transitively referenced) by any other distribution.
func cpanDirectEdges(dists []cpanDist, distByModule map[string]string) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, d := range dists {
		for _, mod := range d.Requires {
			if to, ok := distByModule[mod]; ok {
				referenced[to] = true
			}
		}
	}
	var edges []types.DependencyEdge
	for _, d := range dists {
		if d.Version == "" {
			continue
		}
		node := d.Name + "@" + d.Version
		if referenced[node] {
			continue
		}
		edges = append(edges, types.DependencyEdge{From: ".", To: node})
	}
	return edges
}

// cpanCoreModule reports whether a module is a Perl core/pragma module that is
// not shipped as a CPAN distribution in the snapshot (so its absence from the
// provides index is expected, not an unresolved reference).
func cpanCoreModule(mod string) bool {
	switch mod {
	case "perl", "Perl", "strict", "warnings", "Carp", "Exporter", "Scalar::Util",
		"List::Util", "Cwd", "File::Spec", "File::Basename", "Data::Dumper",
		"POSIX", "Encode", "Fcntl", "Socket", "Time::HiRes", "Config":
		return true
	}
	return false
}
