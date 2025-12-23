package parsers

import (
	"golang.org/x/mod/modfile"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// GolangParser handles Go-specific file parsing (go.mod)
type GolangParser struct{}

// GoModInfo contains metadata about the Go module
type GoModInfo struct {
	ModulePath string
	GoVersion  string
}

// NewGolangParser creates a new Go parser
func NewGolangParser() *GolangParser {
	return &GolangParser{}
}

// buildGoMetadata creates metadata map for Go dependencies
func (p *GolangParser) buildGoMetadata(depPath string, replaceMap map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add source file
	metadata["source"] = MetadataSourceGoMod

	// Add replace directive if this dependency is replaced
	if replacement, exists := replaceMap[depPath]; exists {
		metadata["replaced_by"] = replacement
	}

	return metadata
}

// ParseGoModWithInfo parses go.mod and returns both dependencies and module info
func (p *GolangParser) ParseGoModWithInfo(content string) ([]types.Dependency, *GoModInfo) {
	dependencies := make([]types.Dependency, 0)
	info := &GoModInfo{}

	// Parse the go.mod file using the official parser
	file, err := modfile.Parse("go.mod", []byte(content), nil)
	if err != nil {
		// If parsing fails, return empty dependencies and info
		return dependencies, info
	}

	// Extract module path
	if file.Module != nil {
		info.ModulePath = file.Module.Mod.Path
	}

	// Extract Go version
	if file.Go != nil {
		info.GoVersion = file.Go.Version
	}

	// Build replace map for quick lookup
	replaceMap := make(map[string]string)
	for _, replace := range file.Replace {
		replaceMap[replace.Old.Path] = replace.New.Path + "@" + replace.New.Version
	}

	// Extract dependencies from the require section
	for _, req := range file.Require {
		// Skip indirect dependencies
		if req.Indirect {
			continue
		}

		metadata := p.buildGoMetadata(req.Mod.Path, replaceMap)

		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeGolang,
			Name:     req.Mod.Path,
			Version:  req.Mod.Version,
			Scope:    types.ScopeProd, // Go modules default to production
			Direct:   true,
			Metadata: metadata,
		})
	}

	return dependencies, info
}
