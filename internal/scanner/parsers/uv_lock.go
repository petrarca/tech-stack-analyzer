package parsers

import (
	"github.com/BurntSushi/toml"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// UvLockfile represents the structure of uv.lock (TOML format)
type UvLockfile struct {
	Version  int         `toml:"version"`
	Packages []UvPackage `toml:"package"`
}

// UvPackage represents a package entry in uv.lock
type UvPackage struct {
	Name                 string                       `toml:"name"`
	Version              string                       `toml:"version"`
	Source               UvSource                     `toml:"source"`
	Dependencies         []UvDependencyRef            `toml:"dependencies"`
	OptionalDependencies map[string][]UvDependencyRef `toml:"optional-dependencies"`
}

// UvSource represents the source of a package
type UvSource struct {
	Editable string `toml:"editable"`
	Registry string `toml:"registry"`
	Git      string `toml:"git"`
}

// UvDependencyRef represents a dependency reference
type UvDependencyRef struct {
	Name  string `toml:"name"`
	Extra string `toml:"extra"`
}

// ParseUvLockGraph parses uv.lock and returns the dependencies plus the
// package-to-package edges, honoring the requested graph mode. It implements
// the GraphProducer contract (ParseGraphFunc). uv.lock is self-contained: each
// [[package]] lists its resolved dependencies by name, and every package has a
// single locked version, so edges resolve to clean "name@version" nodes.
func ParseUvLockGraph(input GraphInput) LockGraph {
	content := input.Lockfile
	// The flat parser needs the project name to isolate direct deps; the graph
	// does not, so dependencies are best-effort here.
	result := LockGraph{Dependencies: ParseUvLock(content, "")}

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	lockfile, err := decodeUvLockGraph(content)
	if err != nil {
		return result
	}

	versionByName := make(map[string]string, len(lockfile.Packages))
	for _, pkg := range lockfile.Packages {
		versionByName[pkg.Name] = pkg.Version
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = uvDirectEdges(lockfile, versionByName)
	case types.DependencyGraphFull:
		result.Edges = uvFullEdges(lockfile, versionByName)
	}
	return result
}

// uvLockTOML is the TOML view of uv.lock for graph extraction. uv.lock is TOML;
// using a real decoder (rather than the line-based flat parser) is robust to
// formatting and captures the per-package dependency lists directly.
type uvLockTOML struct {
	Packages []struct {
		Name                 string                 `toml:"name"`
		Version              string                 `toml:"version"`
		Source               map[string]any         `toml:"source"`
		Dependencies         []uvDepTOML            `toml:"dependencies"`
		OptionalDependencies map[string][]uvDepTOML `toml:"optional-dependencies"`
		DevDependencies      map[string][]uvDepTOML `toml:"dev-dependencies"`
	} `toml:"package"`
}

type uvDepTOML struct {
	Name string `toml:"name"`
}

// decodeUvLockGraph decodes uv.lock into the UvLockfile shape used by the edge
// builders, via a real TOML decoder.
func decodeUvLockGraph(content []byte) (UvLockfile, error) {
	var raw uvLockTOML
	if err := toml.Unmarshal(content, &raw); err != nil {
		return UvLockfile{}, err
	}
	var out UvLockfile
	for _, p := range raw.Packages {
		pkg := UvPackage{
			Name:                 p.Name,
			Version:              p.Version,
			OptionalDependencies: make(map[string][]UvDependencyRef),
		}
		if ed, ok := p.Source["editable"].(string); ok {
			pkg.Source.Editable = ed
		}
		for _, d := range p.Dependencies {
			pkg.Dependencies = append(pkg.Dependencies, UvDependencyRef{Name: d.Name})
		}
		for group, deps := range p.OptionalDependencies {
			for _, d := range deps {
				pkg.OptionalDependencies[group] = append(pkg.OptionalDependencies[group], UvDependencyRef{Name: d.Name})
			}
		}
		out.Packages = append(out.Packages, pkg)
	}
	return out, nil
}

// uvNodeID resolves a dependency name to a "name@version" node via the locked
// version map. Returns "" when the package is not present in the lockfile.
func uvNodeID(name string, versionByName map[string]string) string {
	v, ok := versionByName[name]
	if !ok || v == "" {
		return ""
	}
	return name + "@" + v
}

// uvFullEdges builds every package -> dependency edge stated by uv.lock,
// including optional-dependency groups.
func uvFullEdges(lockfile UvLockfile, versionByName map[string]string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	seen := make(map[string]bool)
	for _, pkg := range lockfile.Packages {
		from := uvNodeID(pkg.Name, versionByName)
		if from == "" {
			continue
		}
		add := func(refs []UvDependencyRef) {
			for _, ref := range refs {
				if to := uvNodeID(ref.Name, versionByName); to != "" {
					// A dep can appear in both dependencies and an
					// optional-dependency group; emit each edge once.
					if key := from + "|" + to; !seen[key] {
						seen[key] = true
						edges = append(edges, types.DependencyEdge{From: from, To: to})
					}
				}
			}
		}
		add(pkg.Dependencies)
		for _, group := range pkg.OptionalDependencies {
			add(group)
		}
	}
	return edges
}

// uvDirectEdges builds root -> direct-dependency edges from the project's own
// package entry (source.editable = "."). The synthetic "." marker is the from
// node.
func uvDirectEdges(lockfile UvLockfile, versionByName map[string]string) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for _, pkg := range lockfile.Packages {
		if pkg.Source.Editable != "." {
			continue
		}
		add := func(refs []UvDependencyRef, scope string) {
			for _, ref := range refs {
				if to := uvNodeID(ref.Name, versionByName); to != "" {
					edges = append(edges, types.DependencyEdge{From: ".", To: to, Scope: scope})
				}
			}
		}
		add(pkg.Dependencies, types.ScopeProd)
		for _, group := range pkg.OptionalDependencies {
			add(group, types.ScopeOptional)
		}
	}
	return edges
}

// ParseUvLock parses uv.lock content and returns direct dependencies with
// resolved versions. It uses the TOML decoder (same as the graph path) to
// avoid maintaining a parallel state machine (F-11).
func ParseUvLock(content []byte, projectName string) []types.Dependency {
	lockfile, err := decodeUvLockGraph(content)
	if err != nil {
		return nil
	}

	packageVersions := make(map[string]string, len(lockfile.Packages))
	for _, pkg := range lockfile.Packages {
		packageVersions[pkg.Name] = pkg.Version
	}

	directDepNames := uvRootDependencyNames(lockfile.Packages, projectName)
	return uvDirectDependencies(directDepNames, packageVersions)
}

// uvRootDependencyNames returns the dependency names (regular + optional) of the
// root project package, identified by the editable "." source or the project
// name.
func uvRootDependencyNames(packages []UvPackage, projectName string) []string {
	var names []string
	for _, pkg := range packages {
		if pkg.Source.Editable != "." && pkg.Name != projectName {
			continue
		}
		for _, dep := range pkg.Dependencies {
			names = append(names, dep.Name)
		}
		for _, deps := range pkg.OptionalDependencies {
			for _, dep := range deps {
				names = append(names, dep.Name)
			}
		}
		break
	}
	return names
}

// uvDirectDependencies builds the deduplicated, versioned direct dependencies
// from the root dependency names.
func uvDirectDependencies(directDepNames []string, packageVersions map[string]string) []types.Dependency {
	var dependencies []types.Dependency
	seen := make(map[string]bool)
	for _, name := range directDepNames {
		if seen[name] {
			continue
		}
		seen[name] = true
		version := packageVersions[name]
		if version == "" {
			continue
		}
		dependencies = append(dependencies, types.Dependency{
			Type:       DependencyTypePython,
			Name:       name,
			Version:    version,
			SourceFile: "uv.lock",
			Direct:     true,
		})
	}
	return dependencies
}
