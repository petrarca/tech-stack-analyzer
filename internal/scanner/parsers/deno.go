package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DenoParser handles Deno-specific file parsing (deno.lock)
type DenoParser struct{}

// NewDenoParser creates a new Deno parser
func NewDenoParser() *DenoParser {
	return &DenoParser{}
}

// DenoLock represents the structure of deno.lock
type DenoLock struct {
	Version string            `json:"version"`
	Remote  map[string]string `json:"remote"`
}

// ParseDenoLock parses deno.lock and extracts version and dependencies
func (p *DenoParser) ParseDenoLock(content string) (string, []types.Dependency) {
	var denoLock DenoLock
	if err := json.Unmarshal([]byte(content), &denoLock); err != nil {
		return "", nil
	}

	// Extract version
	version := denoLock.Version

	// Extract dependencies from remote URLs
	dependencies := make([]types.Dependency, 0)

	for url, hash := range denoLock.Remote {
		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeDeno,
			Name:     url,
			Version:  hash,
			Scope:    types.ScopeProd,
			Direct:   true,
			Metadata: types.NewMetadata(MetadataSourceDenoLock),
		})
	}

	return version, dependencies
}
