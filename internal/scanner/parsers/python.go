package parsers

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PythonParser handles Python-specific file parsing with deps.dev patterns
type PythonParser struct{}

// NewPythonParser creates a new Python parser
func NewPythonParser() *PythonParser {
	return &PythonParser{}
}

// ParsePyprojectTOML parses pyproject.toml with enhanced dependency parsing
func (p *PythonParser) ParsePyprojectTOML(content string) []types.Dependency {
	parser := &pyprojectParserEnhanced{
		enhancedParser: p,
	}
	return parser.parse(content)
}

// ParseRequirementsTxt parses requirements.txt with full PEP 508 compliance
func (p *PythonParser) ParseRequirementsTxt(content string) []types.Dependency {
	dependencies := make([]types.Dependency, 0)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		dep, err := p.parsePEP508Dependency(line)
		if err != nil {
			continue // Skip invalid lines
		}

		if dep.Name != "" {
			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypePython,
				Name:     p.canonPackageName(dep.Name),
				Version:  p.resolveVersion(dep.Constraint),
				Scope:    types.ScopeProd, // requirements.txt defaults to production
				Direct:   true,
				Metadata: types.NewMetadata(MetadataSourceRequirementsTxt),
			})
		}
	}

	return dependencies
}

// PythonDependency represents a PEP 508 compliant dependency (deps.dev pattern)
type PythonDependency struct {
	Name        string // Package name
	Extras      string // [extra1,extra2]
	Constraint  string // >=1.0,<2.0
	Environment string // ; python_version >= "3.8"
}

// parsePEP508Dependency parses a Python requirement statement according to PEP 508
// Based on deps.dev/util/pypi/metadata.go ParseDependency function
func (p *PythonParser) parsePEP508Dependency(v string) (PythonDependency, error) {
	var d PythonDependency
	if v == "" {
		return d, fmt.Errorf("invalid python requirement: empty string")
	}

	const whitespace = " \t" // according to the PEP this is the only allowed whitespace
	s := strings.Trim(v, whitespace)

	// Extract name - characters ending with space or start of something else
	nameEnd := strings.IndexAny(s, whitespace+"[(;<=!~>")
	if nameEnd == 0 {
		return d, fmt.Errorf("invalid python requirement: empty name")
	}
	if nameEnd < 0 {
		d.Name = p.canonPackageName(s)
		return d, nil
	}

	d.Name = p.canonPackageName(s[:nameEnd])
	s = strings.TrimLeft(s[nameEnd:], whitespace)

	// Parse extras [extra1,extra2]
	if len(s) > 0 && s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return d, fmt.Errorf("invalid python requirement: %q has unterminated extras section", v)
		}
		d.Extras = strings.Trim(s[1:end], whitespace)
		s = s[end+1:]
	}

	// Parse constraint
	if len(s) > 0 && s[0] != ';' {
		end := strings.IndexByte(s, ';')
		if end < 0 {
			end = len(s) // all of the remainder is the constraint
		}
		d.Constraint = strings.Trim(s[:end], whitespace)
		// Remove parentheses if present
		if strings.HasPrefix(d.Constraint, "(") && strings.HasSuffix(d.Constraint, ")") {
			d.Constraint = d.Constraint[1 : len(d.Constraint)-1]
		}
		s = s[end:]
	}

	// Parse environment markers
	if len(s) > 0 && s[0] != ';' {
		return d, fmt.Errorf("invalid python requirement: internal parse error on %q", v)
	}
	if s != "" {
		d.Environment = strings.Trim(s[1:], whitespace) // s[1] == ';'
	}

	return d, nil
}

// canonPackageName returns the canonical form of a PyPI package name
// Based on deps.dev/util/pypi/metadata.go CanonPackageName function
func (p *PythonParser) canonPackageName(name string) string {
	// https://github.com/pypa/pip/blob/20.0.2/src/pip/_vendor/packaging/utils.py
	// https://www.python.org/dev/peps/pep-503/
	// Names may only be [-_.A-Za-z0-9].
	// Replace runs of [-_.] with a single "-", then lowercase everything.
	var out bytes.Buffer
	run := false // whether a run of [-_.] has started.
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
			out.WriteByte(c)
			run = false
		case 'A' <= c && c <= 'Z':
			out.WriteByte(c + ('a' - 'A'))
			run = false
		case c == '-' || c == '_' || c == '.':
			if !run {
				out.WriteByte('-')
			}
			run = true
		default:
			run = false
		}
	}
	return out.String()
}

// resolveVersion normalizes version strings using PEP 440 canonicalization
func (p *PythonParser) resolveVersion(constraint string) string {
	if constraint == "" {
		return "latest"
	}

	// Use semver package to normalize version according to PEP 440
	// Returns original string if parsing fails
	return semver.Normalize(semver.PyPI, constraint)
}

// pyprojectParserEnhanced handles TOML parsing with enhanced dependency support
type pyprojectParserEnhanced struct {
	enhancedParser *PythonParser

	inProjectSection      bool
	inDependenciesSection bool
	expectingDependencies bool
	dependencies          []types.Dependency
}

func (p *pyprojectParserEnhanced) parse(content string) []types.Dependency {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		p.processLine(line)
	}
	return p.dependencies
}

func (p *pyprojectParserEnhanced) processLine(line string) {
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

func (p *pyprojectParserEnhanced) handleSectionHeader(line string) bool {
	if line == "[project]" {
		p.inProjectSection = true
		return true
	}
	if line == "[project.dependencies]" {
		p.inDependenciesSection = true
		return true
	}
	// Note: Poetry dependencies ([tool.poetry.dependencies]) are not currently supported
	if strings.HasPrefix(line, "[") {
		p.inProjectSection = false
		p.inDependenciesSection = false
		p.expectingDependencies = false
		return true
	}
	return false
}

func (p *pyprojectParserEnhanced) handleDependencyKeyword(line string) bool {
	if p.inProjectSection {
		// Check for exact "dependencies" keyword followed by whitespace, =, or [
		// This prevents matching "dependencies_extra" or similar fields
		if line == "dependencies" ||
			strings.HasPrefix(line, "dependencies ") ||
			strings.HasPrefix(line, "dependencies=") ||
			strings.HasPrefix(line, "dependencies[") {
			p.expectingDependencies = true
			return true
		}
	}
	return false
}

func (p *pyprojectParserEnhanced) shouldParseDependency(line string) bool {
	return (p.inDependenciesSection || p.expectingDependencies) &&
		line != "" &&
		!strings.HasPrefix(line, "#")
}

func (p *pyprojectParserEnhanced) parseDependencyLine(line string) {
	// Clean quotes and trailing commas from dependency line
	// Note: line is already trimmed in parse() before processLine() is called
	line = strings.TrimPrefix(line, `"`)
	line = strings.TrimSuffix(line, `",`)
	line = strings.TrimSuffix(line, `"`)

	// Use enhanced PEP 508 parsing
	dep, err := p.enhancedParser.parsePEP508Dependency(line)
	if err != nil {
		return
	}

	if dep.Name != "" {
		p.dependencies = append(p.dependencies, types.Dependency{
			Type:       DependencyTypePython,
			Name:       p.enhancedParser.canonPackageName(dep.Name),
			Version:    p.enhancedParser.resolveVersion(dep.Constraint),
			SourceFile: "pyproject.toml",
			Direct:     true,
		})
	}
}

// ExtractProjectName extracts the project name from pyproject.toml (same as original)
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

// DetectLicense detects licenses from pyproject.toml content (same as original)
func (p *PythonParser) DetectLicense(content string, payload *types.Payload) {
	// Look for license field in [project] section
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
			licenseValue := p.extractLicenseValue(line)
			if licenseValue != "" {
				if p.addLicenseIfMatch(licenseValue, payload) {
					return // Found license, exit
				}
			}
		}
	}
}

// extractLicenseValue extracts license value from license line
func (p *PythonParser) extractLicenseValue(line string) string {
	afterEqual := strings.SplitN(line, "=", 2)
	if len(afterEqual) < 2 {
		return ""
	}

	licenseValue := strings.TrimSpace(afterEqual[1])
	licenseValue = strings.Trim(licenseValue, `"`)
	licenseValue = strings.Trim(licenseValue, `'`)

	return licenseValue
}

// addLicenseIfMatch adds license if it matches known licenses
func (p *PythonParser) addLicenseIfMatch(licenseText string, payload *types.Payload) bool {
	licenseText = strings.ToLower(strings.TrimSpace(licenseText))

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
			SourceFile:    "pyproject.toml",
			Confidence:    1.0,
		}
		payload.AddLicense(license)
		reason := fmt.Sprintf("license detected: %s (from pyproject.toml)", standardLicense)
		payload.AddLicenseReason(reason)
		return true
	}

	standardLicenses := []string{"MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause", "ISC", "BSD"}
	for _, licenseName := range standardLicenses {
		if strings.EqualFold(licenseText, licenseName) {
			license := types.License{
				LicenseName:   licenseName,
				DetectionType: "file_based",
				SourceFile:    "pyproject.toml",
				Confidence:    1.0,
			}
			payload.AddLicense(license)
			reason := fmt.Sprintf("license detected: %s (from pyproject.toml)", licenseName)
			payload.AddLicenseReason(reason)
			return true
		}
	}

	return false
}
