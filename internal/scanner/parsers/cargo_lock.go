package parsers

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseCargoLock parses Cargo.lock content and returns direct dependencies with resolved versions
// Direct dependencies are identified by cross-referencing with Cargo.toml
func ParseCargoLock(lockContent []byte, cargoTomlContent string) []types.Dependency {
	// Extract direct dependency names from Cargo.toml
	directDeps := extractDirectDepsFromCargoToml(cargoTomlContent)
	if len(directDeps) == 0 {
		return nil
	}

	// Parse Cargo.lock to get resolved versions
	packages := parseCargoLockPackages(string(lockContent))

	// Build dependency list with resolved versions for direct deps only
	var dependencies []types.Dependency
	for name, version := range packages {
		if directDeps[name] {
			dependencies = append(dependencies, types.Dependency{
				Type:       "cargo",
				Name:       name,
				Version:    version,
				SourceFile: "Cargo.lock",
			})
		}
	}

	return dependencies
}

// extractDirectDepsFromCargoToml extracts direct dependency names from Cargo.toml
func extractDirectDepsFromCargoToml(content string) map[string]bool {
	deps := make(map[string]bool)
	lines := strings.Split(content, "\n")
	state := &cargoTomlParseState{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		state = updateCargoTomlState(state, trimmed)

		if name := extractCargoDepName(trimmed, state); name != "" {
			deps[name] = true
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
