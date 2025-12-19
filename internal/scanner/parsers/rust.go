package parsers

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// RustParser handles Rust-specific file parsing (Cargo.toml)
type RustParser struct{}

// NewRustParser creates a new Rust parser
func NewRustParser() *RustParser {
	return &RustParser{}
}

// Dependency represents different Cargo dependency formats
type Dependency struct {
	Version string
	Path    string
	Git     string
	Branch  string
	Rev     string
}

// CargoToml represents the structure of Cargo.toml ( the parts we need)
type CargoToml struct {
	Package struct {
		Name    string `toml:"name"`
		License string `toml:"license"`
	} `toml:"package"`
	Dependencies      map[string]interface{} `toml:"dependencies"`
	DevDependencies   map[string]interface{} `toml:"dev-dependencies"`
	BuildDependencies map[string]interface{} `toml:"build-dependencies"`
	WorkspaceDeps     map[string]interface{} `toml:"workspace.dependencies"`
}

// ParseCargoToml parses Cargo.toml and extracts project info and dependencies
func (p *RustParser) ParseCargoToml(content string) (string, string, []types.Dependency, bool) {
	lines := strings.Split(content, "\n")

	var projectName, license string
	var dependencies []types.Dependency
	isWorkspace := false

	// Parse the TOML manually to avoid external dependencies
	var currentSection string
	var inDependencies bool
	var inDevDependencies bool
	var inBuildDependencies bool
	var inWorkspaceDeps bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if p.shouldSkipLine(line) {
			continue
		}

		if p.isSectionHeader(line) {
			currentSection, inDependencies, inDevDependencies, inBuildDependencies, inWorkspaceDeps, isWorkspace =
				p.parseSectionHeader(line, isWorkspace)
			continue
		}

		if currentSection == "package" {
			projectName, license = p.parsePackageSection(line, projectName, license)
		}

		if p.isDependencySection(inDependencies, inDevDependencies, inBuildDependencies, inWorkspaceDeps) {
			dep := p.parseDependencyLine(line)
			if dep.Name != "" && dep.Version != "" {
				dependencies = append(dependencies, dep)
			}
		}
	}

	return projectName, license, dependencies, isWorkspace
}

// shouldSkipLine checks if a line should be skipped (empty or comment)
func (p *RustParser) shouldSkipLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "#")
}

// isSectionHeader checks if a line is a TOML section header
func (p *RustParser) isSectionHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

// parseSectionHeader parses a section header and returns section info
func (p *RustParser) parseSectionHeader(line string, currentIsWorkspace bool) (string, bool, bool, bool, bool, bool) {
	section := strings.Trim(line, "[]")

	inDependencies := (section == "dependencies")
	inDevDependencies := (section == "dev-dependencies")
	inBuildDependencies := (section == "build-dependencies")
	inWorkspaceDeps := (section == "workspace.dependencies")

	if section == "workspace" {
		currentIsWorkspace = true
	}

	return section, inDependencies, inDevDependencies, inBuildDependencies, inWorkspaceDeps, currentIsWorkspace
}

// parsePackageSection parses package section fields
func (p *RustParser) parsePackageSection(line, currentName, currentLicense string) (string, string) {
	if strings.HasPrefix(line, "name") {
		return p.parsePackageField(line, currentName, currentLicense)
	} else if strings.HasPrefix(line, "license") {
		return p.parseLicenseField(line, currentName, currentLicense)
	}
	return currentName, currentLicense
}

// parsePackageField parses the name field from package section
func (p *RustParser) parsePackageField(line, currentName, currentLicense string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 2 {
		value := strings.TrimSpace(parts[1])
		value = p.removeInlineComments(value)
		return strings.Trim(value, `"`), currentLicense
	}
	return currentName, currentLicense
}

// parseLicenseField parses the license field from package section
func (p *RustParser) parseLicenseField(line, currentName, currentLicense string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 2 {
		value := strings.TrimSpace(parts[1])
		value = p.removeInlineComments(value)
		return currentName, strings.Trim(value, `"`)
	}
	return currentName, currentLicense
}

// removeInlineComments removes inline comments from a value
func (p *RustParser) removeInlineComments(value string) string {
	if idx := strings.Index(value, "#"); idx != -1 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

// isDependencySection checks if we're in a dependency section
func (p *RustParser) isDependencySection(inDeps, inDevDeps, inBuildDeps, inWorkspaceDeps bool) bool {
	return inDeps || inDevDeps || inBuildDeps || inWorkspaceDeps
}

// parseDependencyLine parses a single dependency line from Cargo.toml
func (p *RustParser) parseDependencyLine(line string) types.Dependency {
	// Remove comments
	if idx := strings.Index(line, "#"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	if line == "" {
		return types.Dependency{}
	}

	// Split on = to get name and value
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return types.Dependency{}
	}

	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Handle different dependency formats
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		// Simple string version: "serde = "1.0""
		version := strings.Trim(value, `"`)
		return types.Dependency{
			Type:    "cargo",
			Name:    name,
			Version: version,
		}
	} else if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		// Object format: "serde = { version = "1.0", features = ["derive"] }"
		return p.parseObjectDependency(name, value)
	}

	return types.Dependency{}
}

// parseObjectDependency parses object-style dependencies
func (p *RustParser) parseObjectDependency(name, value string) types.Dependency {
	// Remove braces and split by lines
	content := strings.Trim(value, "{}")
	lines := strings.Split(content, ",")

	depInfo := &dependencyInfo{}

	for _, line := range lines {
		depInfo.parseLine(line)
	}

	return p.buildDependency(name, depInfo)
}

// dependencyInfo holds parsed dependency information
type dependencyInfo struct {
	version, path, git, branch, tag, rev string
}

// parseLine extracts dependency information from a single line
func (d *dependencyInfo) parseLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}

	value := strings.Trim(strings.Trim(parts[1], " "), `"`)
	key := strings.TrimSpace(parts[0])

	switch key {
	case "version":
		d.version = value
	case "path":
		d.path = value
	case "git":
		d.git = value
	case "branch":
		d.branch = value
	case "tag":
		d.tag = value
	case "rev":
		d.rev = value
	}
}

// buildDependency creates a dependency from parsed information
func (p *RustParser) buildDependency(name string, info *dependencyInfo) types.Dependency {
	var version string

	switch {
	case info.path != "":
		version = p.buildPathExample(info)
	case info.git != "":
		version = p.buildGitExample(info)
	default:
		version = info.version
		if version == "" {
			// If no version, path, or git info, this is likely an empty/malformed dependency
			return types.Dependency{}
		}
	}

	return types.Dependency{
		Type:    "cargo",
		Name:    name,
		Version: version,
	}
}

// buildPathExample creates a path-based example string
func (p *RustParser) buildPathExample(info *dependencyInfo) string {
	example := "path:" + info.path
	if info.version != "" {
		example += ":" + info.version
	}
	return example
}

// buildGitExample creates a git-based example string
func (p *RustParser) buildGitExample(info *dependencyInfo) string {
	example := "git:" + info.git

	ref := info.branch
	if ref == "" {
		ref = info.tag
	}
	if ref == "" {
		ref = info.rev
	}

	if ref != "" {
		example += "#" + ref
	} else {
		example = "latest"
	}

	return example
}
