package parsers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PythonParser handles Python-specific file parsing (pyproject.toml, requirements.txt)
type PythonParser struct{}

// NewPythonParser creates a new Python parser
func NewPythonParser() *PythonParser {
	return &PythonParser{}
}

// ParsePyprojectTOML parses pyproject.toml and extracts dependencies with versions
func (p *PythonParser) ParsePyprojectTOML(content string) []types.Dependency {
	parser := &pyprojectParser{
		lineReg: regexp.MustCompile(`(^([a-zA-Z0-9._-]+)$|^([a-zA-Z0-9._-]+)(([>=]+)([0-9.]+)))`),
	}
	return parser.parse(content)
}

// ParseRequirementsTxt parses requirements.txt and extracts package names with versions
// Matches TypeScript logic: dependencies.push(['python', name, version || 'latest'])
func (p *PythonParser) ParseRequirementsTxt(content string) []types.Dependency {
	var dependencies []types.Dependency
	// Match TypeScript regex exactly: /(^([a-zA-Z0-9._-]+)$|^([a-zA-Z0-9._-]+)(([>=]+)([0-9.]+)))/
	// Group 2/3: name, Group 5: version (without comparison operator)
	lineReg := regexp.MustCompile(`(^([a-zA-Z0-9._-]+)$|^([a-zA-Z0-9._-]+)(([>=]+)([0-9.]+)))`)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		match := lineReg.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		// Extract name (group 2/3) and version (group 6 or 'latest') to match TypeScript exactly
		name := match[2]
		if name == "" {
			name = match[3]
		}
		version := match[6]
		if version == "" {
			version = "latest"
		}

		if name != "" {
			dependencies = append(dependencies, types.Dependency{
				Type:       "python",
				Name:       name,
				Version:    version,
				SourceFile: "pyproject.toml",
			})
		}
	}

	return dependencies
}

// ExtractProjectName extracts the project name from pyproject.toml
func (p *PythonParser) ExtractProjectName(content string) string {
	lines := strings.Split(content, "\n")
	inProjectSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for [project] section
		if line == "[project]" {
			inProjectSection = true
			continue
		}

		// If we hit a new section, reset flag
		if strings.HasPrefix(line, "[") && line != "[project]" {
			inProjectSection = false
			continue
		}

		// In [project] section, look for name = "..."
		if inProjectSection && strings.HasPrefix(line, "name") {
			// Extract name value: name = "project-name"
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

// DetectLicense detects licenses from pyproject.toml content (like TypeScript)
func (p *PythonParser) DetectLicense(content string, payload *types.Payload) {
	// Look for license field in [project] section (like TypeScript: if ('license' in prj))
	lines := strings.Split(content, "\n")
	inProjectSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for [project] section
		if strings.HasPrefix(line, "[project]") {
			inProjectSection = true
			continue
		}

		// Check for new section (exit project section)
		if strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[project]") {
			inProjectSection = false
			continue
		}

		// Look for license field in project section
		if inProjectSection && strings.HasPrefix(line, "license") {
			// Extract license value (like TypeScript: const tmp = typeof prj.license === 'string' ? prj.license : prj.license?.text)
			licenseValue := p.extractLicenseValue(line)
			if licenseValue != "" {
				// Simple license detection for common licenses (like spdx-expression-parse)
				if p.addLicenseIfMatch(licenseValue, payload) {
					return // Found license, exit
				}
			}
		}
	}
}

// extractLicenseValue extracts license value from license line
func (p *PythonParser) extractLicenseValue(line string) string {
	// Handle formats: license = "MIT", license = 'MIT', license = MIT
	afterEqual := strings.SplitN(line, "=", 2)
	if len(afterEqual) < 2 {
		return ""
	}

	licenseValue := strings.TrimSpace(afterEqual[1])
	// Remove quotes if present
	licenseValue = strings.Trim(licenseValue, `"`)
	licenseValue = strings.Trim(licenseValue, `'`)

	return licenseValue
}

// addLicenseIfMatch adds license if it matches known licenses
func (p *PythonParser) addLicenseIfMatch(licenseText string, payload *types.Payload) bool {
	// Convert to lowercase for comparison
	licenseText = strings.ToLower(strings.TrimSpace(licenseText))

	// Map common license texts to standard names (like SPDX detection)
	licenseMap := map[string]string{
		"mit":          "MIT",
		"apache-2.0":   "Apache-2.0",
		"apache 2.0":   "Apache-2.0",
		"gpl-3.0":      "GPL-3.0",
		"gpl 3.0":      "GPL-3.0",
		"bsd":          "BSD",
		"bsd-3-clause": "BSD-3-Clause",
		"isc":          "ISC",
	}

	if standardLicense, exists := licenseMap[licenseText]; exists {
		license := types.License{
			LicenseName:   standardLicense,
			DetectionType: "file_based",
			SourceFile:    "setup.py",
			Confidence:    1.0,
		}
		payload.AddLicense(license)
		reason := fmt.Sprintf("license detected: %s (from setup.py)", standardLicense)
		payload.AddLicenseReason(reason)
		return true
	}

	// Check for exact match with standard license
	standardLicenses := []string{"MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause", "ISC", "BSD"}
	for _, licenseName := range standardLicenses {
		if strings.EqualFold(licenseText, licenseName) {
			license := types.License{
				LicenseName:   licenseName,
				DetectionType: "file_based",
				SourceFile:    "setup.py",
				Confidence:    1.0,
			}
			payload.AddLicense(license)
			reason := fmt.Sprintf("license detected: %s (from setup.py)", licenseName)
			payload.AddLicenseReason(reason)
			return true
		}
	}

	return false
}

// pyprojectParser handles TOML parsing state
type pyprojectParser struct {
	lineReg               *regexp.Regexp
	inProjectSection      bool
	inDependenciesSection bool
	expectingDependencies bool
	dependencies          []types.Dependency
}

func (p *pyprojectParser) parse(content string) []types.Dependency {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		p.processLine(line)
	}
	return p.dependencies
}

func (p *pyprojectParser) processLine(line string) {
	if p.handleSectionHeader(line) {
		return
	}
	if p.handleDependencyKeyword(line) {
		return
	}
	if p.shouldParseDependency(line) {
		p.parseDependencyLine(line)
	}
	if p.expectingDependencies && line == "]" {
		p.expectingDependencies = false
	}
}

func (p *pyprojectParser) handleSectionHeader(line string) bool {
	if line == "[project]" {
		p.inProjectSection = true
		return true
	}
	if line == "[project.dependencies]" || line == "[tool.poetry.dependencies]" {
		p.inDependenciesSection = true
		return true
	}
	if strings.HasPrefix(line, "[") {
		p.inProjectSection = false
		p.inDependenciesSection = false
		p.expectingDependencies = false
		return true
	}
	return false
}

func (p *pyprojectParser) handleDependencyKeyword(line string) bool {
	if p.inProjectSection && strings.HasPrefix(line, "dependencies") {
		p.expectingDependencies = true
		return true
	}
	return false
}

func (p *pyprojectParser) shouldParseDependency(line string) bool {
	return (p.inDependenciesSection || p.expectingDependencies) &&
		line != "" &&
		!strings.HasPrefix(line, "#")
}

func (p *pyprojectParser) parseDependencyLine(line string) {
	line = p.cleanLine(line)
	match := p.lineReg.FindStringSubmatch(line)
	if match == nil {
		return
	}

	name := p.extractName(match)
	version := p.extractVersion(match)

	if name != "" {
		p.dependencies = append(p.dependencies, types.Dependency{
			Type:       "python",
			Name:       name,
			Version:    version,
			SourceFile: "requirements.txt",
		})
	}
}

func (p *pyprojectParser) cleanLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, `"`)
	line = strings.TrimSuffix(line, `",`)
	line = strings.TrimSuffix(line, `"`)
	return line
}

func (p *pyprojectParser) extractName(match []string) string {
	if match[2] != "" {
		return match[2]
	}
	return match[3]
}

func (p *pyprojectParser) extractVersion(match []string) string {
	if match[6] != "" {
		return match[6]
	}
	return "latest"
}
