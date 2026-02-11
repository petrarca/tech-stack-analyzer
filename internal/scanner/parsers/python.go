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
