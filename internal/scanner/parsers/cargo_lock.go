package parsers

import (
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// cargoTableVersionRegex extracts the version field from a Cargo.toml table-form
// dependency, e.g. { version = "1.0", features = [...] }.
var cargoTableVersionRegex = regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)

// ParseCargoLockGraph parses Cargo.lock and returns the dependencies plus the
// package-to-package edges, honoring the requested graph mode. It implements
// the GraphProducer contract (ParseGraphFunc). Cargo.lock is self-contained:
// the [[package]] dependencies array states the locked edges, so no Cargo.toml
// is needed to build the full graph.
func ParseCargoLockGraph(input GraphInput) LockGraph {
	// Parse once; derive both the flat dependency list and the graph from the
	// same entries slice -- no re-parsing (F-03).
	entries := parseCargoLockEntries(string(input.Lockfile))

	// Map bare "name" references to a single locked version when unambiguous.
	// Built here and passed into helpers so it is not rebuilt per call (F-04).
	versionByName := make(map[string]string, len(entries))
	for _, e := range entries {
		versionByName[e.Name] = e.Version
	}

	result := LockGraph{Dependencies: cargoEntriesAsDeps(entries)}

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		// Prefer manifest-declared direct deps (accurate even when a direct dep
		// is also pulled transitively); fall back to the not-referenced
		// heuristic when no Cargo.toml is supplied.
		if len(input.Manifest) > 0 {
			result.Edges = cargoDirectEdgesFromManifest(string(input.Manifest), versionByName)
		} else {
			result.Edges = cargoDirectEdges(entries, versionByName)
		}
	case types.DependencyGraphFull:
		result.Edges = cargoFullEdges(entries, versionByName)
	}
	return result
}

// cargoEntriesAsDeps converts parsed entries into a flat Dependency slice.
// Used so ParseCargoLockGraph does not need to re-parse the lockfile.
func cargoEntriesAsDeps(entries []cargoLockEntry) []types.Dependency {
	deps := make([]types.Dependency, 0, len(entries))
	for _, e := range entries {
		if e.Name != "" && e.Version != "" {
			deps = append(deps, types.Dependency{
				Type:       DependencyTypeRust,
				Name:       e.Name,
				Version:    e.Version,
				SourceFile: "Cargo.lock",
			})
		}
	}
	return deps
}

// cargoDirectEdgesFromManifest builds root -> direct edges from the deps
// declared in Cargo.toml, resolved to their locked versions. The synthetic "."
// marker is the from node.
func cargoDirectEdgesFromManifest(cargoToml string, versionByName map[string]string) []types.DependencyEdge {
	directDeps := extractDirectDepsFromCargoToml(cargoToml)
	var edges []types.DependencyEdge
	for name, scope := range directDeps {
		if v, ok := versionByName[name]; ok {
			edges = append(edges, types.DependencyEdge{From: ".", To: name + "@" + v, Scope: scope})
		}
	}
	return edges
}

// cargoNodeID normalizes a Cargo.lock dependency reference ("name" or
// "name version") into a stable "name@version" node identity. Bare names are
// resolved via versionByName when the package appears once in the lockfile.
func cargoNodeID(ref string, versionByName map[string]string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.IndexByte(ref, ' '); i >= 0 {
		name := strings.TrimSpace(ref[:i])
		// Drop any trailing source spec after the version (rare in lockfiles).
		rest := strings.TrimSpace(ref[i+1:])
		if j := strings.IndexByte(rest, ' '); j >= 0 {
			rest = rest[:j]
		}
		return name + "@" + rest
	}
	if v, ok := versionByName[ref]; ok {
		return ref + "@" + v
	}
	return ref
}

// cargoFullEdges builds every package -> dependency edge stated by Cargo.lock.
func cargoFullEdges(entries []cargoLockEntry, versionByName map[string]string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for _, e := range entries {
		from := e.Name + "@" + e.Version
		for _, dep := range e.Dependencies {
			to := cargoNodeID(dep, versionByName)
			if to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges
}

// cargoDirectEdges builds root -> direct-dependency edges. The root is the only
// [[package]] entry not referenced as a dependency by any other entry (the
// workspace/binary crate). versionByName is passed in -- already built by
// ParseCargoLockGraph -- so it is not rebuilt here (F-04).
func cargoDirectEdges(entries []cargoLockEntry, versionByName map[string]string) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, e := range entries {
		for _, dep := range e.Dependencies {
			name := dep
			if i := strings.IndexByte(dep, ' '); i >= 0 {
				name = dep[:i]
			}
			referenced[strings.TrimSpace(name)] = true
		}
	}
	var edges []types.DependencyEdge
	for _, e := range entries {
		if referenced[e.Name] {
			continue // not a root crate
		}
		for _, dep := range e.Dependencies {
			to := cargoNodeID(dep, versionByName)
			if to != "" {
				edges = append(edges, types.DependencyEdge{From: ".", To: to})
			}
		}
	}
	return edges
}

// ParseCargoLock parses Cargo.lock content and returns direct dependencies with resolved versions
// Direct dependencies are identified by cross-referencing with Cargo.toml
func ParseCargoLock(lockContent []byte, cargoTomlContent string) []types.Dependency {
	// Extract direct dependency names and scopes from Cargo.toml
	directDeps := extractDirectDepsFromCargoToml(cargoTomlContent)
	if len(directDeps) == 0 {
		return nil
	}
	declaredConstraints := extractDeclaredConstraintsFromCargoToml(cargoTomlContent)

	// Parse Cargo.lock to get resolved versions
	packages := parseCargoLockPackages(string(lockContent))

	// Build dependency list with resolved versions for direct deps only
	var dependencies []types.Dependency
	for name, version := range packages {
		if scope, exists := directDeps[name]; exists {
			dep := types.Dependency{
				Type:       DependencyTypeRust,
				Name:       name,
				Version:    version,
				SourceFile: "Cargo.lock",
				Scope:      scope,
				Direct:     true,
			}
			dep.SetDeclaredVersion(declaredConstraints[name])
			dependencies = append(dependencies, dep)
		}
	}

	return dependencies
}

// extractDeclaredConstraintsFromCargoToml captures the declared version
// constraint for each direct dependency, handling both the string form
// (serde = "1.0") and the table form (serde = { version = "1.0", ... }).
func extractDeclaredConstraintsFromCargoToml(content string) map[string]string {
	constraints := make(map[string]string)
	state := &cargoTomlParseState{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		state = updateCargoTomlState(state, trimmed)
		name := extractCargoDepName(trimmed, state)
		if name == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		var constraint string
		if strings.HasPrefix(value, "{") {
			// Table form: pull the version = "..." field.
			if m := cargoTableVersionRegex.FindStringSubmatch(value); m != nil {
				constraint = m[1]
			}
		} else {
			constraint = strings.Trim(value, `"'`)
		}
		if constraint != "" {
			constraints[name] = constraint
		}
	}
	return constraints
}

// extractDirectDepsFromCargoToml extracts direct dependency names and scopes from Cargo.toml
func extractDirectDepsFromCargoToml(content string) map[string]string {
	deps := make(map[string]string) // name -> scope
	lines := strings.Split(content, "\n")
	state := &cargoTomlParseState{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		state = updateCargoTomlState(state, trimmed)

		if name := extractCargoDepName(trimmed, state); name != "" {
			var scope string
			if state.inDependencies {
				scope = types.ScopeProd
			} else if state.inDevDependencies {
				scope = types.ScopeDev
			} else if state.inBuildDependencies {
				scope = types.ScopeBuild
			}
			deps[name] = scope
		}
	}

	return deps
}

// cargoTomlParseState tracks the current parsing state for Cargo.toml
type cargoTomlParseState struct {
	inDependencies      bool
	inDevDependencies   bool
	inBuildDependencies bool
}

func updateCargoTomlState(state *cargoTomlParseState, line string) *cargoTomlParseState {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return state
	}

	section := strings.Trim(line, "[]")
	newState := &cargoTomlParseState{}

	switch section {
	case "dependencies":
		newState.inDependencies = true
	case "dev-dependencies":
		newState.inDevDependencies = true
	case "build-dependencies":
		newState.inBuildDependencies = true
	}

	return newState
}

func extractCargoDepName(line string, state *cargoTomlParseState) string {
	if !state.inDependencies && !state.inDevDependencies && !state.inBuildDependencies {
		return ""
	}

	// Skip comments and section headers
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
		return ""
	}

	// Parse: package-name = "version" or package-name = { version = "..." }
	if !strings.Contains(line, "=") {
		return ""
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 1 {
		return ""
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return ""
	}

	return name
}

// parseCargoLockPackages extracts package name -> version mapping from Cargo.lock
func parseCargoLockPackages(content string) map[string]string {
	packages := make(map[string]string)
	for _, pkg := range parseCargoLockEntries(content) {
		packages[pkg.Name] = pkg.Version
	}
	return packages
}

// cargoLockEntry is a resolved [[package]] entry with its locked dependencies.
type cargoLockEntry struct {
	Name         string
	Version      string
	Dependencies []string // raw entries: "name" or "name version"
}

// cargoLockTOML is the TOML view of Cargo.lock for graph extraction.
type cargoLockTOML struct {
	Packages []struct {
		Name         string   `toml:"name"`
		Version      string   `toml:"version"`
		Dependencies []string `toml:"dependencies"`
	} `toml:"package"`
}

// parseCargoLockEntries extracts every [[package]] entry from Cargo.lock,
// including the dependencies array used to build the package-to-package graph.
// Uses a real TOML decoder for robustness (multi-line arrays, comments, etc.).
func parseCargoLockEntries(content string) []cargoLockEntry {
	var lock cargoLockTOML
	if err := toml.Unmarshal([]byte(content), &lock); err != nil {
		return nil
	}
	entries := make([]cargoLockEntry, 0, len(lock.Packages))
	for _, p := range lock.Packages {
		if p.Name == "" || p.Version == "" {
			continue
		}
		entries = append(entries, cargoLockEntry{
			Name:         p.Name,
			Version:      p.Version,
			Dependencies: p.Dependencies,
		})
	}
	return entries
}
