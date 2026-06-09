package parsers

import (
	"encoding/json"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// RenvParser parses R's renv.lock (JSON). The Packages object maps a package
// name to {Package, Version, Source, Requirements[]}; Requirements lists the
// package names it depends on (a real dependency graph).
type RenvParser struct{}

// NewRenvParser creates a new renv.lock parser.
func NewRenvParser() *RenvParser {
	return &RenvParser{}
}

type renvLock struct {
	Packages map[string]renvPackage `json:"Packages"`
}

type renvPackage struct {
	Package      string   `json:"Package"`
	Version      string   `json:"Version"`
	Source       string   `json:"Source"`
	Requirements []string `json:"Requirements"`
}

// ParseRenvLock parses renv.lock into resolved dependencies.
func (p *RenvParser) ParseRenvLock(content string) []types.Dependency {
	var lock renvLock
	if err := json.Unmarshal([]byte(content), &lock); err != nil {
		return nil
	}
	var deps []types.Dependency
	for name, pkg := range lock.Packages {
		ver := pkg.Version
		if pkg.Package == "" && ver == "" {
			continue
		}
		if pkg.Package != "" {
			name = pkg.Package
		}
		if ver == "" {
			continue
		}
		deps = append(deps, types.Dependency{
			Type:       DependencyTypeR,
			Name:       name,
			Version:    ver,
			SourceFile: "renv.lock",
		})
	}
	return deps
}

// rBasePackage reports whether a name is base/recommended R (shipped with R,
// not a CRAN package node) -- "R" itself and the standard library packages.
func rBasePackage(name string) bool {
	switch name {
	case "R", "base", "compiler", "datasets", "graphics", "grDevices", "grid",
		"methods", "parallel", "splines", "stats", "stats4", "tcltk", "tools",
		"utils", "MASS", "Matrix":
		return true
	}
	return false
}
