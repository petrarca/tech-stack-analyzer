package components

import "sync"

// Global registry for component detectors
var (
	detectors []Detector
	mu        sync.RWMutex
)

// Register adds a component detector to the registry
func Register(detector Detector) {
	mu.Lock()
	defer mu.Unlock()
	detectors = append(detectors, detector)
}

// GetDetectors returns all registered component detectors
func GetDetectors() []Detector {
	mu.RLock()
	defer mu.RUnlock()
	return detectors
}
