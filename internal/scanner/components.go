package scanner

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ComponentDetector handles component-based detection for technologies
// that haven't been migrated to the plugin-based detector system yet
// (currently only GitHub Actions).
type ComponentDetector struct {
	provider    types.Provider
	depDetector *DependencyDetector
}

// NewComponentDetector creates a new component detector
func NewComponentDetector(depDetector *DependencyDetector, provider types.Provider, rules []types.Rule) *ComponentDetector {
	return &ComponentDetector{
		provider:    provider,
		depDetector: depDetector,
	}
}
