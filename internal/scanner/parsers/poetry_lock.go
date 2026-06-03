package parsers

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParsePoetryLockGraph parses poetry.lock and returns the dependencies plus the
// package-to-package edges, honoring the requested graph mode. It implements
// the GraphProducer contract (ParseGraphFunc). poetry.lock is self-contained:
// each [[package]] has a [package.dependencies] sub-table naming its
// dependencies, and every package has a single locked version, so edges resolve
// to clean "name@version" nodes.
func ParsePoetryLockGraph(content []byte, mode types.DependencyGraphMode) LockGraph {
	// The flat parser needs pyproject.toml to identify direct deps; the graph
	// does not, so dependencies are best-effort here.
	result := LockGraph{Dependencies: ParsePoetryLock(content, "")}

	if mode == types.DependencyGraphOff {
		return result
	}

	entries := parsePoetryEntries(string(content))
	versionByName := make(map[string]string, len(entries))
	for _, e := range entries {
		versionByName[normalizePackageName(e.Name)] = e.Version
	}

	switch mode {
	case types.DependencyGraphDirect:
		// poetry.lock does not mark the root project, so direct edges require
		// pyproject.toml. Resolve the declared direct deps to locked versions.
		result.Edges = poetryDirectEdges(content, versionByName)
	case types.DependencyGraphFull:
		result.Edges = poetryFullEdges(entries, versionByName)
	}
	return result
}

// poetryLockEntry is a resolved [[package]] with its declared dependency names.
type poetryLockEntry struct {
	Name         string
	Version      string
	Dependencies []string // dependency names from [package.dependencies]
}

// poetryNodeID resolves a (normalized) dependency name to a "name@version" node
// via the locked version map. Returns "" when the package is absent.
func poetryNodeID(name string, versionByName map[string]string) string {
	v, ok := versionByName[normalizePackageName(name)]
	if !ok || v == "" {
		return ""
	}
	return name + "@" + v
}

// poetryFullEdges builds every package -> dependency edge stated by poetry.lock.
func poetryFullEdges(entries []poetryLockEntry, versionByName map[string]string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for _, e := range entries {
		from := poetryNodeID(e.Name, versionByName)
		if from == "" {
			continue
		}
		for _, dep := range e.Dependencies {
			if to := poetryNodeID(dep, versionByName); to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges
}

// poetryDirectEdges builds root -> direct-dependency edges by resolving the
// direct deps declared in pyproject.toml to their locked versions. The
// synthetic "." marker is the from node.
func poetryDirectEdges(lockContent []byte, versionByName map[string]string) []types.DependencyEdge {
	// poetry.lock has no embedded pyproject; the graph producer only sees the
	// lockfile. Fall back to the metadata.files-less approach: every package
	// not referenced as a dependency by another package is a root-level dep.
	entries := parsePoetryEntries(string(lockContent))
	referenced := make(map[string]bool)
	for _, e := range entries {
		for _, dep := range e.Dependencies {
			referenced[normalizePackageName(dep)] = true
		}
	}
	var edges []types.DependencyEdge
	for _, e := range entries {
		if referenced[normalizePackageName(e.Name)] {
			continue
		}
		if to := poetryNodeID(e.Name, versionByName); to != "" {
			edges = append(edges, types.DependencyEdge{From: ".", To: to})
		}
	}
	return edges
}

// parsePoetryEntries extracts every [[package]] with its [package.dependencies]
// sub-table names from poetry.lock content.
func parsePoetryEntries(content string) []poetryLockEntry {
	var entries []poetryLockEntry
	lines := strings.Split(content, "\n")
	state := &poetryEntryState{}

	flush := func() {
		if state.name != "" && state.version != "" {
			entries = append(entries, poetryLockEntry{
				Name:         state.name,
				Version:      state.version,
				Dependencies: state.deps,
			})
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[[package]]" {
			flush()
			state = &poetryEntryState{inPackage: true}
			continue
		}
		if !state.inPackage {
			continue
		}
		processPoetryEntryLine(trimmed, state)
	}
	flush()
	return entries
}

// poetryEntryState tracks parsing of a single [[package]] block including its
// [package.dependencies] sub-table.
type poetryEntryState struct {
	inPackage bool
	inDeps    bool
	name      string
	version   string
	deps      []string
}

func processPoetryEntryLine(line string, state *poetryEntryState) {
	switch {
	case line == "[package.dependencies]":
		state.inDeps = true
		return
	case strings.HasPrefix(line, "[package."):
		// Another package sub-table (extras, source, etc.) ends deps.
		state.inDeps = false
		return
	case line == "[[package]]":
		return
	}

	if strings.HasPrefix(line, "name = ") {
		state.name = extractQuotedValuePoetry(line, "name = ")
		return
	}
	if strings.HasPrefix(line, "version = ") && state.name != "" {
		state.version = extractQuotedValuePoetry(line, "version = ")
		return
	}
	if state.inDeps {
		if name := poetryDepName(line); name != "" {
			state.deps = append(state.deps, name)
		}
	}
}

// poetryDepName extracts the dependency name from a [package.dependencies] line.
// Lines look like: name = ">=1.0", name = {version = "*", ...}, or a TOML array
// header for multi-constraint deps (name = [ ... ]).
func poetryDepName(line string) string {
	if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
		return ""
	}
	name := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
	// Quoted names (rare): "package-name" = "..."
	name = strings.Trim(name, `"'`)
	if name == "" || name == "python" {
		return ""
	}
	return name
}

// ParsePoetryLock parses poetry.lock content and returns direct dependencies with resolved versions
// Direct dependencies are identified by cross-referencing with pyproject.toml
func ParsePoetryLock(lockContent []byte, pyprojectContent string) []types.Dependency {
	// Extract direct dependency names and scopes from pyproject.toml
	directDeps := extractDirectDepsFromPyproject(pyprojectContent)
	if len(directDeps) == 0 {
		return nil
	}
	declaredConstraints := extractDeclaredConstraintsFromPyproject(pyprojectContent)

	// Parse poetry.lock to get resolved versions
	packages := parsePoetryPackages(string(lockContent))

	// Build dependency list with resolved versions for direct deps only
	var dependencies []types.Dependency
	for name, version := range packages {
		// Normalize name for comparison (poetry uses lowercase with hyphens)
		normalizedName := normalizePackageName(name)
		if scope, exists := directDeps[normalizedName]; exists {
			dep := types.Dependency{
				Type:       DependencyTypePython,
				Name:       name,
				Version:    version,
				SourceFile: "poetry.lock",
				Scope:      scope,
				Direct:     true,
			}
			dep.SetDeclaredVersion(declaredConstraints[normalizedName])
			dependencies = append(dependencies, dep)
		}
	}

	return dependencies
}

// pyprojectParseState tracks the current parsing state for pyproject.toml
type pyprojectParseState struct {
	inDepsSection    bool
	inDevDepsSection bool
	inArrayDeps      bool
}

// extractDirectDepsFromPyproject extracts direct dependency names and scopes from pyproject.toml
func extractDirectDepsFromPyproject(content string) map[string]string {
	deps := make(map[string]string) // name -> scope
	lines := strings.Split(content, "\n")
	state := &pyprojectParseState{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		state = updatePyprojectState(state, trimmed)

		if name := extractDepFromLine(trimmed, state); name != "" {
			var scope string
			if state.inDepsSection {
				scope = types.ScopeProd
			} else if state.inDevDepsSection {
				scope = types.ScopeDev
			} else if state.inArrayDeps {
				scope = types.ScopeOptional
			}
			deps[normalizePackageName(name)] = scope
		}
	}

	return deps
}

// extractDeclaredConstraintsFromPyproject captures the declared version
// constraint for each direct dependency (e.g. fastapi = "^0.100" or
// "fastapi>=0.100" in array form), keyed by normalized name.
func extractDeclaredConstraintsFromPyproject(content string) map[string]string {
	constraints := make(map[string]string)
	state := &pyprojectParseState{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		state = updatePyprojectState(state, trimmed)
		if !state.inDepsSection && !state.inDevDepsSection && !state.inArrayDeps {
			continue
		}
		name, constraint := parsePyprojectConstraint(trimmed, state)
		if name != "" && constraint != "" {
			constraints[normalizePackageName(name)] = constraint
		}
	}
	return constraints
}

// parsePyprojectConstraint returns the dependency name and its declared
// constraint from a pyproject line. Handles table form (name = "^1.0") and
// PEP 621 array form ("name>=1.0").
func parsePyprojectConstraint(line string, state *pyprojectParseState) (name, constraint string) {
	if state.inArrayDeps {
		// PEP 621 array form: "fastapi>=0.100", possibly with a trailing comma.
		if !strings.HasPrefix(line, `"`) {
			return "", ""
		}
		spec := strings.Trim(strings.TrimRight(strings.TrimSpace(line), ","), `"'`)
		if i := strings.IndexAny(spec, "<>=~!^ ("); i > 0 {
			return spec[:i], strings.TrimSpace(spec[i:])
		}
		return spec, ""
	}
	if !strings.Contains(line, "=") || strings.HasPrefix(line, "#") {
		return "", ""
	}
	parts := strings.SplitN(line, "=", 2)
	name = strings.TrimSpace(parts[0])
	if name == "" || name == "python" {
		return "", ""
	}
	constraint = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	return name, constraint
}

func updatePyprojectState(state *pyprojectParseState, line string) *pyprojectParseState {
	newState := *state

	switch {
	case line == "[tool.poetry.dependencies]":
		newState = pyprojectParseState{inDepsSection: true}
	case line == "[tool.poetry.dev-dependencies]" || line == "[tool.poetry.group.dev.dependencies]":
		newState = pyprojectParseState{inDevDepsSection: true}
	case line == "[project.dependencies]":
		newState = pyprojectParseState{inArrayDeps: true}
	case strings.HasPrefix(line, "[project.optional-dependencies"):
		newState = pyprojectParseState{inArrayDeps: true}
	case strings.HasPrefix(line, "[") && !strings.Contains(line, "dependencies"):
		newState = pyprojectParseState{}
	}

	return &newState
}

func extractDepFromLine(line string, state *pyprojectParseState) string {
	if state.inDepsSection || state.inDevDepsSection {
		return extractPoetryDep(line)
	}
	if state.inArrayDeps {
		return extractArrayDep(line)
	}
	return ""
}

func extractPoetryDep(line string) string {
	if !strings.Contains(line, "=") || strings.HasPrefix(line, "#") {
		return ""
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 1 {
		return ""
	}
	name := strings.TrimSpace(parts[0])
	if name == "" || name == "python" {
		return ""
	}
	return name
}

func extractArrayDep(line string) string {
	if !strings.HasPrefix(line, `"`) {
		return ""
	}
	name := extractPackageNameFromQuoted(line)
	if name == "" || name == "python" {
		return ""
	}
	return name
}

// parsePoetryPackages extracts package name -> version mapping from poetry.lock
func parsePoetryPackages(content string) map[string]string {
	packages := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentName string
	inPackage := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[[package]]" {
			inPackage = true
			currentName = ""
			continue
		}

		if !inPackage {
			continue
		}

		// Parse name = "value"
		if strings.HasPrefix(trimmed, "name = ") {
			currentName = extractQuotedValuePoetry(trimmed, "name = ")
			continue
		}

		// Parse version = "value"
		if strings.HasPrefix(trimmed, "version = ") && currentName != "" {
			version := extractQuotedValuePoetry(trimmed, "version = ")
			if version != "" {
				packages[currentName] = version
			}
			inPackage = false
			continue
		}

		// End of package section
		if strings.HasPrefix(trimmed, "[") && trimmed != "[[package]]" {
			inPackage = false
		}
	}

	return packages
}

// normalizePackageName normalizes a Python package name for comparison
// Python package names are case-insensitive and treat hyphens/underscores as equivalent
func normalizePackageName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// extractQuotedValuePoetry extracts a quoted value from a line
func extractQuotedValuePoetry(line, prefix string) string {
	rest := line[len(prefix):]
	if len(rest) >= 2 && rest[0] == '"' {
		end := 1
		for end < len(rest) && rest[end] != '"' {
			end++
		}
		return rest[1:end]
	}
	return ""
}

// extractPackageNameFromQuoted extracts package name from a quoted dependency string
// e.g., "requests>=2.31.0" -> "requests"
func extractPackageNameFromQuoted(line string) string {
	// Remove quotes and trailing comma
	line = strings.Trim(line, `"',`)

	// Find the end of the package name (before version specifier)
	for i, c := range line {
		if c == '>' || c == '<' || c == '=' || c == '!' || c == '[' || c == ';' {
			return line[:i]
		}
	}
	return line
}
