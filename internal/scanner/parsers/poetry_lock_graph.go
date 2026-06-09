package parsers

import (
	"github.com/BurntSushi/toml"
	pep440 "github.com/aquasecurity/go-pep440-version"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// poetryGraphLockfile is the TOML view of poetry.lock used for graph building.
// Dependency values are intentionally `any`: poetry uses three shapes --
// a string range ("^1.0"), a table ({version = "...", markers = "..."}), and an
// array of tables (multi-constraint by environment marker). All three are
// handled in poetryDepConstraints.
type poetryGraphLockfile struct {
	Packages []poetryGraphPackage `toml:"package"`
}

type poetryGraphPackage struct {
	Name         string         `toml:"name"`
	Version      string         `toml:"version"`
	Dependencies map[string]any `toml:"dependencies"`
}

// ParsePoetryLockGraph parses poetry.lock and returns the dependencies plus the
// package-to-package edges, honoring the requested graph mode. It implements
// the GraphProducer contract (ParseGraphFunc).
//
// poetry.lock is self-contained: each [[package]] has a [package.dependencies]
// table naming its dependencies with version ranges. A package may be locked at
// MULTIPLE versions (e.g. numpy 1.24.4 and 1.26.4 for different Python
// versions), so each dependency range is matched (PEP 440) against every locked
// version of that package and an edge is emitted to each satisfying version.
// This mirrors Trivy's poetry resolution and correctly captures the reachable
// graph surface for blast-radius analysis.
func ParsePoetryLockGraph(input GraphInput) LockGraph {
	content := input.Lockfile
	// The flat parser needs pyproject.toml to identify direct deps; the graph
	// does not, so dependencies are best-effort here.
	result := LockGraph{Dependencies: ParsePoetryLock(content, "")}

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lockfile poetryGraphLockfile
	if err := toml.Unmarshal(content, &lockfile); err != nil {
		return result
	}

	// Keep ALL locked versions per (normalized) name, and a reverse map to the
	// lockfile's canonical name. The canonical name is used for the To node id
	// so direct and full edges have consistent, matching node identities (F-06).
	versionsByName := make(map[string][]string)
	canonicalName := make(map[string]string) // normalized -> lockfile name
	for _, pkg := range lockfile.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		key := normalizePackageName(pkg.Name)
		versionsByName[key] = append(versionsByName[key], pkg.Version)
		canonicalName[key] = pkg.Name
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		// Prefer pyproject.toml-declared direct deps; fall back to the
		// not-referenced heuristic when no manifest is supplied.
		if len(input.Manifest) > 0 {
			result.Edges = poetryDirectEdgesFromManifest(string(input.Manifest), versionsByName, canonicalName)
		} else {
			result.Edges = poetryDirectEdges(lockfile, versionsByName)
		}
	case types.DependencyGraphFull:
		result.Edges, result.Unresolved = poetryFullEdges(lockfile, versionsByName)
	}
	return result
}

// poetryDirectEdgesFromManifest builds root -> direct edges from the deps
// declared in pyproject.toml, resolved to their locked versions. The synthetic
// "." marker is the from node. When a package is locked at multiple versions,
// an edge is emitted to each (markers not evaluated).
// poetryDirectEdgesFromManifest builds root -> direct edges from the deps
// declared in pyproject.toml, resolved to their locked versions. The To node
// id uses the lockfile's canonical name (from canonicalName) so it matches the
// From node ids emitted by poetryFullEdges (F-06).
func poetryDirectEdgesFromManifest(pyproject string, versionsByName map[string][]string, canonicalName map[string]string) []types.DependencyEdge {
	directDeps := extractDirectDepsFromPyproject(pyproject)
	var edges []types.DependencyEdge
	for name, scope := range directDeps {
		key := normalizePackageName(name)
		canonical, ok := canonicalName[key]
		if !ok {
			canonical = name // manifest name as fallback if not in lock
		}
		for _, v := range versionsByName[key] {
			edges = append(edges, types.DependencyEdge{From: ".", To: canonical + "@" + v, Scope: scope})
		}
	}
	return edges
}

// poetryFullEdges builds every package -> dependency edge. Each dependency
// range is resolved to all locked versions it satisfies. Dependency names not
// present in the lockfile are returned as unresolved references rather than
// dropped silently.
func poetryFullEdges(lockfile poetryGraphLockfile, versionsByName map[string][]string) (edges []types.DependencyEdge, unresolved []string) {
	for _, pkg := range lockfile.Packages {
		if pkg.Version == "" {
			continue
		}
		from := pkg.Name + "@" + pkg.Version
		for depName, depVal := range pkg.Dependencies {
			targets := poetryResolveTargets(depName, depVal, versionsByName)
			if len(targets) == 0 {
				unresolved = append(unresolved, from+" -> "+depName)
				continue
			}
			for _, to := range targets {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges, unresolved
}

// poetryDirectEdges builds root -> direct-dependency edges. poetry.lock does not
// mark the project root, so a root is approximated as any package not referenced
// as a dependency by another package. The synthetic "." marker is the from node.
func poetryDirectEdges(lockfile poetryGraphLockfile, versionsByName map[string][]string) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, pkg := range lockfile.Packages {
		for depName := range pkg.Dependencies {
			referenced[normalizePackageName(depName)] = true
		}
	}
	var edges []types.DependencyEdge
	for _, pkg := range lockfile.Packages {
		if pkg.Version == "" || referenced[normalizePackageName(pkg.Name)] {
			continue
		}
		edges = append(edges, types.DependencyEdge{From: ".", To: pkg.Name + "@" + pkg.Version})
	}
	return edges
}

// poetryResolveTargets resolves a dependency (name + range value) to the
// "name@version" nodes of every locked version that satisfies the constraint.
// When no version matches (e.g. an unparseable range), it falls back to all
// locked versions so the edge is not silently dropped.
func poetryResolveTargets(name string, value any, versionsByName map[string][]string) []string {
	key := normalizePackageName(name)
	versions, ok := versionsByName[key]
	if !ok || len(versions) == 0 {
		return nil
	}
	// A single locked version: emit it without range matching (cheap + robust).
	if len(versions) == 1 {
		return []string{name + "@" + versions[0]}
	}

	constraints := poetryDepConstraints(value)
	var targets []string
	seen := make(map[string]bool)
	for _, ver := range versions {
		if poetryVersionMatchesAny(ver, constraints) {
			id := name + "@" + ver
			if !seen[id] {
				seen[id] = true
				targets = append(targets, id)
			}
		}
	}
	if len(targets) == 0 {
		// No constraint matched (or none parseable): keep all versions rather
		// than drop the edge -- over-approximation is safer for blast-radius.
		for _, ver := range versions {
			targets = append(targets, name+"@"+ver)
		}
	}
	return targets
}

// poetryDepConstraints extracts the version constraint string(s) from a poetry
// dependency value, handling the three shapes: string, table, array of tables.
func poetryDepConstraints(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case map[string]any:
		if c := tomlString(v["version"]); c != "" {
			return []string{c}
		}
	case []map[string]any:
		var out []string
		for _, m := range v {
			if c := tomlString(m["version"]); c != "" {
				out = append(out, c)
			}
		}
		return out
	case []any:
		var out []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if c := tomlString(m["version"]); c != "" {
					out = append(out, c)
				}
			}
		}
		return out
	}
	return nil
}

// poetryVersionMatchesAny reports whether ver satisfies any of the constraints.
// An empty constraint set or an unparseable version matches nothing (the caller
// applies the keep-all fallback).
func poetryVersionMatchesAny(ver string, constraints []string) bool {
	if len(constraints) == 0 {
		return false
	}
	v, err := pep440.Parse(ver)
	if err != nil {
		return false
	}
	for _, c := range constraints {
		spec, err := pep440.NewSpecifiers(c, pep440.WithPreRelease(true))
		if err != nil {
			continue
		}
		if spec.Check(v) {
			return true
		}
	}
	return false
}

// tomlString safely converts a decoded TOML value to a string.
func tomlString(v any) string {
	s, _ := v.(string)
	return s
}
