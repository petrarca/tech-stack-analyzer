package providers

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PackageProvider defines how a detector extracts and matches packages
type PackageProvider struct {
	DependencyType      string                                                    // "nuget", "npm", "maven", etc.
	ExtractPackageNames func(component *types.Payload) []string                   // Extract package names from properties
	MatchFunc           func(componentPkgName string, dependencyName string) bool // Custom matching logic (e.g., case-insensitive)
}

var (
	packageProviders = make(map[string]*PackageProvider) // dependency_type -> provider
	providersMutex   sync.RWMutex
)

// Register allows detectors to register their package extraction and matching logic
func Register(provider *PackageProvider) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	packageProviders[provider.DependencyType] = provider
}

// Get returns a registered package provider by dependency type
func Get(dependencyType string) *PackageProvider {
	providersMutex.RLock()
	defer providersMutex.RUnlock()
	return packageProviders[dependencyType]
}

// GetAll returns all registered providers
func GetAll() map[string]*PackageProvider {
	providersMutex.RLock()
	defer providersMutex.RUnlock()

	// Return copy to avoid concurrent modification issues
	result := make(map[string]*PackageProvider, len(packageProviders))
	for k, v := range packageProviders {
		result[k] = v
	}
	return result
}

// --- Helper functions for common extraction patterns ---

// SinglePropertyExtractor creates an ExtractPackageNames function that extracts
// a single property from component.Properties[techKey][propName]
func SinglePropertyExtractor(techKey, propName string) func(*types.Payload) []string {
	return func(component *types.Payload) []string {
		props := getPropsMap(component, techKey)
		if props == nil {
			return nil
		}
		if name := props[propName]; name != "" {
			return []string{name}
		}
		return nil
	}
}

// GroupArtifactExtractor creates an ExtractPackageNames function that extracts
// groupId:artifactId from component.Properties[techKey]
func GroupArtifactExtractor(techKey string) func(*types.Payload) []string {
	return func(component *types.Payload) []string {
		props := getPropsMap(component, techKey)
		if props == nil {
			return nil
		}
		groupID := props["group_id"]
		artifactID := props["artifact_id"]
		if groupID != "" && artifactID != "" {
			return []string{groupID + ":" + artifactID}
		}
		return nil
	}
}

// getPropsMap extracts the properties map for a given tech key, handling both
// map[string]string and map[string]interface{} types
func getPropsMap(component *types.Payload, techKey string) map[string]string {
	if component.Properties == nil {
		return nil
	}

	// Try map[string]string first (most common)
	if props, ok := component.Properties[techKey].(map[string]string); ok {
		return props
	}

	// Try map[string]interface{} and convert
	if props, ok := component.Properties[techKey].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range props {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
		return result
	}

	return nil
}
