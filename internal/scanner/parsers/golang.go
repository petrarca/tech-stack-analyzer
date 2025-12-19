package parsers

import (
	"golang.org/x/mod/modfile"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// GolangParser handles Go-specific file parsing (go.mod)
type GolangParser struct{}

// NewGolangParser creates a new Go parser
func NewGolangParser() *GolangParser {
	return &GolangParser{}
}

// ParseGoMod parses go.mod and extracts dependencies using the official modfile parser
func (p *GolangParser) ParseGoMod(content string) []types.Dependency {
	var dependencies []types.Dependency

	// Parse the go.mod file using the official parser
	file, err := modfile.Parse("go.mod", []byte(content), nil)
	if err != nil {
		// If parsing fails, return empty dependencies
		return dependencies
	}

	// Extract dependencies from the require section
	for _, req := range file.Require {
		// Skip indirect dependencies
		if req.Indirect {
			continue
		}

		// Create dependency with version
		dependencies = append(dependencies, types.Dependency{
			Type:    "golang",
			Name:    req.Mod.Path,
			Version: req.Mod.Version,
		})
	}

	return dependencies
}
