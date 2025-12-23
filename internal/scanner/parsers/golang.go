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

// ParseGoMod parses go.mod and extracts dependencies using the official modfile parser
func (p *GolangParser) ParseGoMod(content string) []types.Dependency {
	var dependencies []types.Dependency

	// Parse the go.mod file using the official parser
	file, err := modfile.Parse("go.mod", []byte(content), nil)
	if err != nil {
		// If parsing fails, return empty dependencies
		return dependencies
	}

	// Build replace map for quick lookup
	replaceMap := make(map[string]string)
	for _, replace := range file.Replace {
		replaceMap[replace.Old.Path] = replace.New.Path + "@" + replace.New.Version
	}

	// Extract Go version for metadata
	goVersion := ""
	if file.Go != nil {
		goVersion = file.Go.Version
	}

	// Extract dependencies from the require section
	for _, req := range file.Require {
		// Skip indirect dependencies
		if req.Indirect {
			continue
		}

		metadata := p.buildGoMetadata(req.Mod.Path, goVersion, replaceMap)

		dependencies = append(dependencies, types.Dependency{
			Type:     "golang",
			Name:     req.Mod.Path,
			Version:  req.Mod.Version,
			Direct:   true,
			Metadata: metadata,
		})
	}

	return dependencies
}

// buildGoMetadata creates metadata map for Go dependencies
func (p *GolangParser) buildGoMetadata(depPath, goVersion string, replaceMap map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add source file
	metadata["source"] = "go.mod"

	// Add Go version if available (language requirement for this dependency)
	if goVersion != "" {
		metadata["go_version"] = goVersion
	}

	// Add replace directive if this dependency is replaced
	if replacement, exists := replaceMap[depPath]; exists {
		metadata["replaced_by"] = replacement
	}

	return metadata
}

// ParseGoModWithInfo parses go.mod and returns both dependencies and module info
func (p *GolangParser) ParseGoModWithInfo(content string) ([]types.Dependency, *GoModInfo) {
	var dependencies []types.Dependency
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

		metadata := p.buildGoMetadata(req.Mod.Path, info.GoVersion, replaceMap)

		dependencies = append(dependencies, types.Dependency{
			Type:     "golang",
			Name:     req.Mod.Path,
			Version:  req.Mod.Version,
			Direct:   true,
			Metadata: metadata,
		})
	}

	return dependencies, info
}
