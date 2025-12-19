package parsers

import (
	"gopkg.in/yaml.v3"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// PnpmLockfile represents the structure of pnpm-lock.yaml
type PnpmLockfile struct {
	LockfileVersion string                  `yaml:"lockfileVersion"`
	Importers       map[string]PnpmImporter `yaml:"importers"`
}

// PnpmImporter represents an importer in pnpm-lock.yaml
type PnpmImporter struct {
	Dependencies    map[string]PnpmDependency `yaml:"dependencies"`
	DevDependencies map[string]PnpmDependency `yaml:"devDependencies"`
}

// PnpmDependency represents a dependency in pnpm-lock.yaml
type PnpmDependency struct {
	Specifier string `yaml:"specifier"`
	Version   string `yaml:"version"`
}

// ParsePnpmLock parses pnpm-lock.yaml content and returns direct dependencies with resolved versions
func ParsePnpmLock(content []byte) []types.Dependency {
	var lockfile PnpmLockfile
	if err := yaml.Unmarshal(content, &lockfile); err != nil {
		return nil
	}

	var dependencies []types.Dependency

	// Get the root importer (current project)
	rootImporter, exists := lockfile.Importers["."]
	if !exists {
		return nil
	}

	// Extract direct dependencies with prod scope
	for name, dep := range rootImporter.Dependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       "npm",
			Name:       name,
			Version:    dep.Version,
			SourceFile: "pnpm-lock.yaml",
			Scope:      types.ScopeProd,
		})
	}

	// Extract dev dependencies with dev scope
	for name, dep := range rootImporter.DevDependencies {
		dependencies = append(dependencies, types.Dependency{
			Type:       "npm",
			Name:       name,
			Version:    dep.Version,
			SourceFile: "pnpm-lock.yaml",
			Scope:      types.ScopeDev,
		})
	}

	return dependencies
}
