package parsers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// UvLockfile represents the structure of uv.lock (TOML format)
type UvLockfile struct {
	Version  int         `yaml:"version"`
	Packages []UvPackage `yaml:"package"`
}

// UvPackage represents a package entry in uv.lock
type UvPackage struct {
	Name                 string                       `yaml:"name"`
	Version              string                       `yaml:"version"`
	Source               UvSource                     `yaml:"source"`
	Dependencies         []UvDependencyRef            `yaml:"dependencies"`
	OptionalDependencies map[string][]UvDependencyRef `yaml:"optional-dependencies"`
}

// UvSource represents the source of a package
type UvSource struct {
	Editable string `yaml:"editable"`
	Registry string `yaml:"registry"`
	Git      string `yaml:"git"`
}

// UvDependencyRef represents a dependency reference
type UvDependencyRef struct {
	Name  string `yaml:"name"`
	Extra string `yaml:"extra"`
}

// ParseUvLock parses uv.lock content and returns direct dependencies with resolved versions
// Direct dependencies are identified by finding the project's own package entry (source.editable = ".")
// and extracting its dependencies list
func ParseUvLock(content []byte, projectName string) []types.Dependency {
	var lockfile UvLockfile

	// uv.lock uses TOML format, but we need a TOML parser
	// For now, we'll use a simple approach: find the project package and extract deps
	if err := parseUvLockTOML(content, &lockfile); err != nil {
		return nil
	}

	// Build a map of package name -> version for quick lookup
	packageVersions := make(map[string]string)
	for _, pkg := range lockfile.Packages {
		packageVersions[pkg.Name] = pkg.Version
	}

	// Find the project's own package (editable = ".")
	var directDepNames []string
	for _, pkg := range lockfile.Packages {
		if pkg.Source.Editable == "." || pkg.Name == projectName {
			// This is the project itself - its dependencies are direct deps
			for _, dep := range pkg.Dependencies {
				directDepNames = append(directDepNames, dep.Name)
			}
			// Also include optional dependencies (dev, etc.)
			for _, deps := range pkg.OptionalDependencies {
				for _, dep := range deps {
					directDepNames = append(directDepNames, dep.Name)
				}
			}
			break
		}
	}

	// Build dependency list with resolved versions
	var dependencies []types.Dependency
	seen := make(map[string]bool)
	for _, name := range directDepNames {
		if seen[name] {
			continue
		}
		seen[name] = true

		version := packageVersions[name]
		if version == "" {
			continue
		}

		dependencies = append(dependencies, types.Dependency{
			Type:       "python",
			Name:       name,
			Version:    version,
			SourceFile: "uv.lock",
		})
	}

	return dependencies
}

// parseUvLockTOML is a simple TOML parser for uv.lock format
// This is a simplified parser that handles the specific structure of uv.lock
func parseUvLockTOML(content []byte, lockfile *UvLockfile) error {
	// uv.lock is TOML, but we need to parse it manually since we don't have a TOML library
	// For simplicity, we'll parse the key structures we need

	lines := string(content)

	// Use a simple state machine to parse [[package]] entries
	packages := parseUvPackages(lines)
	lockfile.Packages = packages

	return nil
}

// uvParseState tracks the current parsing state for uv.lock
type uvParseState struct {
	currentPkg      *UvPackage
	inDependencies  bool
	inOptionalDeps  bool
	currentOptGroup string
}

// parseUvPackages extracts package information from uv.lock content
func parseUvPackages(content string) []UvPackage {
	var packages []UvPackage
	lines := splitLines(content)
	state := &uvParseState{}

	for _, line := range lines {
		trimmed := trimSpace(line)

		if trimmed == "[[package]]" {
			if state.currentPkg != nil {
				packages = append(packages, *state.currentPkg)
			}
			state = &uvParseState{
				currentPkg: &UvPackage{
					OptionalDependencies: make(map[string][]UvDependencyRef),
				},
			}
			continue
		}

		if state.currentPkg == nil {
			continue
		}

		processUvLine(trimmed, state)
	}

	// Don't forget the last package
	if state.currentPkg != nil {
		packages = append(packages, *state.currentPkg)
	}

	return packages
}

func processUvLine(line string, state *uvParseState) {
	switch {
	case hasPrefix(line, "name = "):
		state.currentPkg.Name = extractQuotedValue(line, "name = ")
	case hasPrefix(line, "version = "):
		state.currentPkg.Version = extractQuotedValue(line, "version = ")
	case hasPrefix(line, "source = "):
		if contains(line, "editable = \".\"") {
			state.currentPkg.Source.Editable = "."
		}
	case line == "dependencies = [":
		state.inDependencies = true
		state.inOptionalDeps = false
	case line == "[package.optional-dependencies]":
		state.inOptionalDeps = true
		state.inDependencies = false
	case state.inOptionalDeps && contains(line, " = ["):
		state.currentOptGroup = extractKey(line)
	case (state.inDependencies || state.inOptionalDeps) && hasPrefix(line, "{ name = "):
		addUvDependency(line, state)
	case line == "]":
		if state.inDependencies {
			state.inDependencies = false
		}
	}
}

func addUvDependency(line string, state *uvParseState) {
	depName := extractDepName(line)
	if depName == "" {
		return
	}

	ref := UvDependencyRef{Name: depName}
	if state.inOptionalDeps && state.currentOptGroup != "" {
		state.currentPkg.OptionalDependencies[state.currentOptGroup] = append(
			state.currentPkg.OptionalDependencies[state.currentOptGroup], ref)
	} else {
		state.currentPkg.Dependencies = append(state.currentPkg.Dependencies, ref)
	}
}

// Helper functions for parsing
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractQuotedValue(line, prefix string) string {
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

func extractKey(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '=' {
			return line[:i]
		}
	}
	return ""
}

func extractDepName(line string) string {
	// Parse: { name = "package-name" } or { name = "package-name", ... }
	prefix := "{ name = \""
	if !hasPrefix(line, prefix) {
		return ""
	}
	rest := line[len(prefix):]
	end := 0
	for end < len(rest) && rest[end] != '"' {
		end++
	}
	return rest[:end]
}
