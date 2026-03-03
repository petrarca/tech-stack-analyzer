package components

import "sync"

// Global registry for component detectors
var (
	detectors    []Detector
	mu           sync.RWMutex
	useLockFiles = true // Default to true
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

// SetUseLockFiles sets whether lock files should be used for dependency resolution
func SetUseLockFiles(use bool) {
	mu.Lock()
	defer mu.Unlock()
	useLockFiles = use
}

// UseLockFiles returns whether lock files should be used for dependency resolution
func UseLockFiles() bool {
	mu.RLock()
	defer mu.RUnlock()
	return useLockFiles
}
