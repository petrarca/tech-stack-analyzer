package scanner

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ComponentRegistry indexes components by their package identifiers per technology
type ComponentRegistry struct {
	byDependencyType map[string]map[string]*types.Payload // dependency_type -> package_name -> component
}

// NewComponentRegistry creates a new component registry
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		byDependencyType: make(map[string]map[string]*types.Payload),
	}
}

// Register adds a component to the registry using registered package providers
func (r *ComponentRegistry) Register(component *types.Payload) {
	if component.Properties == nil {
		return
	}

	// Use each registered provider to extract packages
	for depType, provider := range providers.GetAll() {
		packageNames := provider.ExtractPackageNames(component)
		for _, pkgName := range packageNames {
			if pkgName != "" {
				// Register in dependency-type-specific map
				if r.byDependencyType[depType] == nil {
					r.byDependencyType[depType] = make(map[string]*types.Payload)
				}
				r.byDependencyType[depType][pkgName] = component
			}
		}
	}
}

// buildComponentRegistry recursively builds the registry from all components
func (s *Scanner) buildComponentRegistry(payload *types.Payload, registry *ComponentRegistry) {
	// Register current component if it has package identifiers
	registry.Register(payload)

	// Recursively register child components
	for _, child := range payload.Children {
		s.buildComponentRegistry(child, registry)
	}
}

// resolveComponentRefs resolves inter-component references
func (s *Scanner) resolveComponentRefs(root *types.Payload) {
	// Build registry of all components by their package names
	registry := NewComponentRegistry()
	s.buildComponentRegistry(root, registry)

	// Resolve dependencies for all components
	s.resolveComponentRefsRecursive(root, registry)
}

// resolveComponentRefsRecursive walks the tree and resolves component references
func (s *Scanner) resolveComponentRefsRecursive(payload *types.Payload, registry *ComponentRegistry) {
	// Resolve dependencies for current component
	for _, dep := range payload.Dependencies {
		// Try to find a matching component
		targetComponent := s.findMatchingComponent(dep, registry)
		if targetComponent != nil && targetComponent.ID != payload.ID {
			// Create component reference
			compRef := types.ComponentRef{
				TargetID:    targetComponent.ID,
				PackageName: dep.Name,
			}
			payload.ComponentRefs = append(payload.ComponentRefs, compRef)
		}
	}

	// Recursively process child components
	for _, child := range payload.Children {
		s.resolveComponentRefsRecursive(child, registry)
	}
}

// findMatchingComponent tries to find a component that provides the given dependency
func (s *Scanner) findMatchingComponent(dep types.Dependency, registry *ComponentRegistry) *types.Payload {
	provider := providers.Get(dep.Type)

	if provider == nil {
		// No provider registered for this dependency type
		return nil
	}

	// Look up in dependency-type-specific registry
	componentsForType, exists := registry.byDependencyType[dep.Type]
	if !exists {
		return nil
	}

	// Try direct match first
	if component, found := componentsForType[dep.Name]; found {
		return component
	}

	// Try custom matching logic from provider
	if provider.MatchFunc != nil {
		for pkgName, component := range componentsForType {
			if provider.MatchFunc(pkgName, dep.Name) {
				return component
			}
		}
	}

	return nil
}
