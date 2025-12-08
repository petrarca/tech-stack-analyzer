package components

import "github.com/petrarca/tech-stack-analyzer/internal/types"

// Detector is the interface that all component detectors must implement
type Detector interface {
	// Name returns the name of this detector (e.g., "nodejs", "python")
	Name() string

	// Detect scans files in the current directory and returns detected components
	// Returns nil if no components are detected
	Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector DependencyDetector) []*types.Payload
}

// DependencyDetector interface for matching dependencies
type DependencyDetector interface {
	MatchDependencies(dependencies []string, depType string) map[string][]string
}
