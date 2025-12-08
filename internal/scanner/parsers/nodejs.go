package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// NodeJSParser handles Node.js-specific file parsing (package.json)
type NodeJSParser struct{}

// NewNodeJSParser creates a new Node.js parser
func NewNodeJSParser() *NodeJSParser {
	return &NodeJSParser{}
}

// PackageJSON represents the structure of package.json
type PackageJSON struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParsePackageJSON parses package.json content and returns the parsed structure
func (p *NodeJSParser) ParsePackageJSON(content []byte) (*PackageJSON, error) {
	var packageJSON PackageJSON
	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return nil, err
	}
	return &packageJSON, nil
}

// ExtractDependencies extracts all dependency names from package.json (dependencies + devDependencies)
func (p *NodeJSParser) ExtractDependencies(pkg *PackageJSON) []string {
	var dependencies []string

	// Add regular dependencies
	for name := range pkg.Dependencies {
		dependencies = append(dependencies, name)
	}

	// Add dev dependencies
	for name := range pkg.DevDependencies {
		dependencies = append(dependencies, name)
	}

	return dependencies
}

// GetPackageName returns the package name with a fallback if empty
func (p *NodeJSParser) GetPackageName(pkg *PackageJSON) string {
	if pkg.Name != "" {
		return pkg.Name
	}
	return "nodejs-component"
}

// CreateDependencies creates a list of Dependency objects from package.json
func (p *NodeJSParser) CreateDependencies(pkg *PackageJSON, depNames []string) []types.Dependency {
	var dependencies []types.Dependency

	for _, name := range depNames {
		version := pkg.Dependencies[name]
		if version == "" {
			version = pkg.DevDependencies[name]
		}

		dependencies = append(dependencies, types.Dependency{
			Type:    "npm",
			Name:    name,
			Example: version,
		})
	}

	return dependencies
}
