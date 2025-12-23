package parsers

import (
	"encoding/json"
	"log"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PHPParser handles PHP-specific file parsing (composer.json)
type PHPParser struct{}

// NewPHPParser creates a new PHP parser
func NewPHPParser() *PHPParser {
	return &PHPParser{}
}

// ComposerJSON represents the structure of composer.json
type ComposerJSON struct {
	Name       string            `json:"name"`
	License    string            `json:"license"`
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

// ParseComposerJSON parses composer.json and extracts project info and dependencies
func (p *PHPParser) ParseComposerJSON(content string) (string, string, []types.Dependency) {
	var composerJSON ComposerJSON
	if err := json.Unmarshal([]byte(content), &composerJSON); err != nil {
		log.Printf("Warning: Failed to parse composer.json: %v", err)
		return "", "", nil
	}

	// Extract project name
	projectName := composerJSON.Name

	// Extract license
	license := composerJSON.License

	// Merge require and require-dev dependencies
	dependencies := make([]types.Dependency, 0)
	metadata := types.NewMetadata(MetadataSourceComposerJSON)

	// Process production dependencies (nil-safe)
	if composerJSON.Require != nil {
		for name, version := range composerJSON.Require {
			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypePHP,
				Name:     name,
				Version:  version,
				Scope:    types.ScopeProd,
				Direct:   true,
				Metadata: metadata,
			})
		}
	}

	// Process development dependencies (nil-safe)
	if composerJSON.RequireDev != nil {
		for name, version := range composerJSON.RequireDev {
			dependencies = append(dependencies, types.Dependency{
				Type:     DependencyTypePHP,
				Name:     name,
				Version:  version,
				Scope:    types.ScopeDev,
				Direct:   true,
				Metadata: metadata,
			})
		}
	}

	return projectName, license, dependencies
}
