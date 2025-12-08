package python

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Python component detection
type Detector struct{}

// Name returns the detector name
func (d *Detector) Name() string {
	return "python"
}

// Detect scans for Python projects (pyproject.toml - supports Poetry, uv, and other PEP 518 tools)
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "pyproject.toml" {
			continue
		}

		// Read pyproject.toml
		content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		// Extract project name
		projectName := extractProjectName(string(content))
		if projectName == "" {
			continue
		}

		// Create payload with specific file path
		relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
		if relativeFilePath == "." {
			relativeFilePath = "/"
		} else {
			relativeFilePath = "/" + relativeFilePath
		}

		payload := types.NewPayloadWithPath(projectName, relativeFilePath)

		// Set tech field to python
		payload.AddPrimaryTech("python")

		// Parse dependencies
		dependencies := parseDependencies(string(content))

		// Extract dependency names for tech matching
		var depNames []string
		for _, dep := range dependencies {
			depNames = append(depNames, dep.Name)
		}

		// Match dependencies against rules
		if len(dependencies) > 0 {
			matchedTechs := depDetector.MatchDependencies(depNames, "python")
			for tech, reasons := range matchedTechs {
				for _, reason := range reasons {
					payload.AddTech(tech, reason)
				}
			}

			payload.Dependencies = dependencies
		}

		// Detect license
		detectLicense(string(content), payload)

		payloads = append(payloads, payload)
	}

	return payloads
}

// extractProjectName extracts the project name from pyproject.toml
func extractProjectName(content string) string {
	lines := strings.Split(content, "\n")
	inProjectSection := false
	inPoetrySection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[project]" {
			inProjectSection = true
			inPoetrySection = false
			continue
		}

		if line == "[tool.poetry]" {
			inPoetrySection = true
			inProjectSection = false
			continue
		}

		if strings.HasPrefix(line, "[") {
			inProjectSection = false
			inPoetrySection = false
			continue
		}

		if (inProjectSection || inPoetrySection) && strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"'`)
				if name != "" {
					return name
				}
			}
		}
	}

	return ""
}

// parseDependencies parses dependencies from pyproject.toml
func parseDependencies(content string) []types.Dependency {
	var dependencies []types.Dependency
	lineReg := regexp.MustCompile(`^([a-zA-Z0-9._-]+)(\s*=\s*(.*))?$`)
	arrayDepReg := regexp.MustCompile(`^([a-zA-Z0-9._\-\[\]]+)([>=<]+[^"]*)?`)

	lines := strings.Split(content, "\n")
	state := &dependencyParseState{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		state = updateDependencyState(state, line)

		if shouldParseDependency(state, line) {
			if dep := parseDependencyLine(line, state.inArrayDependencies, lineReg, arrayDepReg); dep != nil {
				dependencies = append(dependencies, *dep)
			}
		}
	}

	return dependencies
}

// dependencyParseState tracks the current parsing state
type dependencyParseState struct {
	inProjectSection      bool
	inDependenciesSection bool
	inArrayDependencies   bool
	expectingDependencies bool
}

// updateDependencyState updates the parsing state based on the current line
func updateDependencyState(state *dependencyParseState, line string) *dependencyParseState {
	newState := *state // copy state

	if line == "[project]" {
		newState.inProjectSection = true
	} else if line == "[project.dependencies]" {
		newState.inDependenciesSection = true
		newState.inArrayDependencies = true
	} else if line == "[tool.poetry.dependencies]" || line == "[tool.uv.sources]" {
		newState.inDependenciesSection = true
		newState.inArrayDependencies = false
	} else if strings.HasPrefix(line, "[") {
		// Reset all state on any other section
		newState = dependencyParseState{}
	} else if newState.inProjectSection && strings.HasPrefix(line, "dependencies") {
		newState.expectingDependencies = true
		newState.inArrayDependencies = true
	}

	return &newState
}

// shouldParseDependency determines if the current line should be parsed as a dependency
func shouldParseDependency(state *dependencyParseState, line string) bool {
	return (state.inDependenciesSection || state.expectingDependencies) &&
		line != "" && !strings.HasPrefix(line, "#") && line != "]" && line != "[" &&
		!strings.HasPrefix(line, "dependencies") && !strings.HasPrefix(line, "[")
}

// parseDependencyLine parses a single dependency line
func parseDependencyLine(line string, isArrayFormat bool, lineReg, arrayDepReg *regexp.Regexp) *types.Dependency {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, `"`)
	line = strings.TrimSuffix(line, `",`)
	line = strings.TrimSuffix(line, `"`)

	if isArrayFormat {
		return parseArrayDependency(line, arrayDepReg)
	}
	return parseKeyValueDependency(line, lineReg)
}

// parseArrayDependency parses array format dependencies like "fastapi>=0.104.0"
func parseArrayDependency(line string, arrayDepReg *regexp.Regexp) *types.Dependency {
	match := arrayDepReg.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	name := match[1]
	version := match[2]
	if version == "" {
		version = "latest"
	} else {
		// Clean version by stripping operators
		version = extractVersion(version)
	}

	return &types.Dependency{
		Type:    "python",
		Name:    name,
		Example: version,
	}
}

// parseKeyValueDependency parses key-value format dependencies like "fastapi = ^0.104.1"
func parseKeyValueDependency(line string, lineReg *regexp.Regexp) *types.Dependency {
	match := lineReg.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	name := match[1]
	version := "latest"

	if len(match) > 3 && match[3] != "" {
		version = extractVersion(match[3])
	}

	return &types.Dependency{
		Type:    "python",
		Name:    name,
		Example: version,
	}
}

// extractVersion extracts version from various pyproject.toml formats
func extractVersion(valueStr string) string {
	valueStr = strings.TrimSpace(valueStr)
	valueStr = strings.Trim(valueStr, `"`)

	// Handle simple version strings with operators
	if strings.HasPrefix(valueStr, "^") || strings.HasPrefix(valueStr, "~") ||
		strings.HasPrefix(valueStr, ">=") || strings.HasPrefix(valueStr, "<=") ||
		strings.HasPrefix(valueStr, "==") || strings.HasPrefix(valueStr, "!=") ||
		strings.HasPrefix(valueStr, ">") || strings.HasPrefix(valueStr, "<") {
		// Strip the operator for clean version - check 2-char operators first
		if len(valueStr) > 2 && (strings.HasPrefix(valueStr, ">=") || strings.HasPrefix(valueStr, "<=") || strings.HasPrefix(valueStr, "==") || strings.HasPrefix(valueStr, "!=")) {
			return valueStr[2:]
		}
		if len(valueStr) > 1 && (strings.HasPrefix(valueStr, "^") || strings.HasPrefix(valueStr, "~") || strings.HasPrefix(valueStr, ">") || strings.HasPrefix(valueStr, "<")) {
			return valueStr[1:]
		}
		return valueStr
	}

	// Handle complex format with version field
	if strings.Contains(valueStr, "version") {
		versionReg := regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)
		if match := versionReg.FindStringSubmatch(valueStr); len(match) > 1 {
			return match[1]
		}
	}

	// Handle Git URLs
	if strings.Contains(valueStr, "git") {
		gitReg := regexp.MustCompile(`git\+https?://[^@]+@([^#]+)#?([^"]*)?`)
		if match := gitReg.FindStringSubmatch(valueStr); len(match) > 1 {
			version := "git: " + match[1]
			if len(match) > 2 && match[2] != "" {
				version += "@" + match[2]
			}
			return version
		}
		return "git"
	}

	// Handle local paths
	if strings.Contains(valueStr, "path") {
		return "local"
	}

	// Handle simple quoted version
	if !strings.Contains(valueStr, "{") && !strings.Contains(valueStr, "[") {
		return valueStr
	}

	return "latest"
}

// detectLicense detects license from pyproject.toml
func detectLicense(content string, payload *types.Payload) {
	lines := strings.Split(content, "\n")
	inProjectSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[project]" {
			inProjectSection = true
			continue
		}

		if strings.HasPrefix(line, "[") && line != "[project]" {
			inProjectSection = false
			continue
		}

		if inProjectSection && strings.HasPrefix(line, "license") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				license := strings.TrimSpace(parts[1])
				license = strings.Trim(license, `"'`)
				if license != "" {
					payload.Licenses = append(payload.Licenses, license)
				}
			}
		}
	}
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
