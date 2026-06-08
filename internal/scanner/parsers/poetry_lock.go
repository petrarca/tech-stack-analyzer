package parsers

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

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
