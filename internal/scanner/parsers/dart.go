package parsers

import (
	"gopkg.in/yaml.v3"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DartParser handles Dart/Flutter pubspec.yaml and pubspec.lock parsing.
type DartParser struct{}

// NewDartParser creates a new Dart parser.
func NewDartParser() *DartParser {
	return &DartParser{}
}

// pubspecYAML is the manifest. dependencies/dev_dependencies map a package name
// to a version constraint (string) or a source map (git/path/hosted).
type pubspecYAML struct {
	Name            string         `yaml:"name"`
	Dependencies    map[string]any `yaml:"dependencies"`
	DevDependencies map[string]any `yaml:"dev_dependencies"`
}

// pubspecLock is the lockfile. packages maps a name to its resolved version,
// dependency kind (direct main/dev, transitive), and the dependencies it pulls.
type pubspecLock struct {
	Packages map[string]pubspecLockPackage `yaml:"packages"`
}

type pubspecLockPackage struct {
	Dependency string `yaml:"dependency"` // "direct main" | "direct dev" | "transitive"
	Version    string `yaml:"version"`
}

// ParsePubspecName extracts the project name from pubspec.yaml.
func (p *DartParser) ParsePubspecName(content string) string {
	var doc pubspecYAML
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return ""
	}
	return doc.Name
}

// ParsePubspecLock parses pubspec.lock into resolved dependencies. Direct deps
// are marked from the "dependency" field ("direct main"/"direct dev").
func (p *DartParser) ParsePubspecLock(content string) []types.Dependency {
	var lock pubspecLock
	if err := yaml.Unmarshal([]byte(content), &lock); err != nil {
		return nil
	}
	var deps []types.Dependency
	for name, pkg := range lock.Packages {
		if pkg.Version == "" {
			continue
		}
		scope := types.ScopeProd
		if pkg.Dependency == "direct dev" {
			scope = types.ScopeDev
		}
		deps = append(deps, types.Dependency{
			Type:       DependencyTypeDart,
			Name:       name,
			Version:    pkg.Version,
			Scope:      scope,
			Direct:     pkg.Dependency == "direct main" || pkg.Dependency == "direct dev",
			SourceFile: "pubspec.lock",
		})
	}
	return deps
}

// ParsePubspecYAML parses pubspec.yaml dependencies (fallback when no lockfile).
func (p *DartParser) ParsePubspecYAML(content string) []types.Dependency {
	var doc pubspecYAML
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	var deps []types.Dependency
	add := func(m map[string]any, scope string) {
		for name, v := range m {
			ver := "latest"
			if s, ok := v.(string); ok && s != "" {
				ver = s
			}
			deps = append(deps, types.Dependency{
				Type:       DependencyTypeDart,
				Name:       name,
				Version:    ver,
				Scope:      scope,
				Direct:     true,
				SourceFile: "pubspec.yaml",
			})
		}
	}
	add(doc.Dependencies, types.ScopeProd)
	add(doc.DevDependencies, types.ScopeDev)
	return deps
}
