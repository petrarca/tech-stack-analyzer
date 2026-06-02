package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// cargoTableVersionRegex extracts the version field from a Cargo.toml table-form
// dependency, e.g. { version = "1.0", features = [...] }.
var cargoTableVersionRegex = regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)

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
	lines := strings.Split(content, "\n")
	state := &cargoLockParseState{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[[package]]" {
			// Save previous package if valid
			if state.currentName != "" && state.currentVersion != "" {
				packages[state.currentName] = state.currentVersion
			}
			state = &cargoLockParseState{}
			state.inPackage = true
			continue
		}

		if !state.inPackage {
			continue
		}

		processCargoLockLine(trimmed, state)
	}

	// Don't forget the last package
	if state.currentName != "" && state.currentVersion != "" {
		packages[state.currentName] = state.currentVersion
	}

	return packages
}

// cargoLockParseState tracks the current parsing state for Cargo.lock
type cargoLockParseState struct {
	inPackage      bool
	currentName    string
	currentVersion string
}

func processCargoLockLine(line string, state *cargoLockParseState) {
	switch {
	case strings.HasPrefix(line, "name = "):
		state.currentName = extractCargoLockQuotedValue(line, "name = ")
	case strings.HasPrefix(line, "version = "):
		state.currentVersion = extractCargoLockQuotedValue(line, "version = ")
	case strings.HasPrefix(line, "[") && line != "[[package]]":
		// End of package section (hit another section)
		state.inPackage = false
	}
}

func extractCargoLockQuotedValue(line, prefix string) string {
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
