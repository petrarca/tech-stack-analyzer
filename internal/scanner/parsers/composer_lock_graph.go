package parsers

import (
	"encoding/json"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// composerLock is the JSON view of composer.lock. packages/packages-dev each
// list resolved packages with a require map naming their dependencies.
type composerLock struct {
	Packages    []composerLockPackage `json:"packages"`
	PackagesDev []composerLockPackage `json:"packages-dev"`
}

type composerLockPackage struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Require map[string]string `json:"require"`
}

// ParseComposerLockGraph parses composer.lock and returns the package-to-package
// edges, honoring the requested graph mode. It implements the GraphProducer
// contract (ParseGraphFunc).
//
// Each package's require map names its dependencies; every package is locked at
// a single version so names resolve cleanly to "name@version". Platform
// requirements (php, ext-*, lib-*) are not real packages and are skipped.
// Direct deps come from composer.json (require/require-dev) when supplied.
func ParseComposerLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock composerLock
	if err := json.Unmarshal(input.Lockfile, &lock); err != nil {
		return result
	}

	all := append(append([]composerLockPackage{}, lock.Packages...), lock.PackagesDev...)
	versionByName := make(map[string]string, len(all))
	for _, p := range all {
		if p.Name != "" && p.Version != "" {
			versionByName[p.Name] = p.Version
		}
	}
	node := func(name string) string {
		if v, ok := versionByName[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		result.Edges = composerDirectEdges(input.Manifest, node)
	case types.DependencyGraphFull:
		var unresolved []string
		for _, p := range all {
			from := node(p.Name)
			if from == "" {
				continue
			}
			for depName := range p.Require {
				if isComposerPlatformReq(depName) {
					continue
				}
				if to := node(depName); to != "" {
					result.Edges = append(result.Edges, types.DependencyEdge{From: from, To: to})
				} else {
					unresolved = append(unresolved, from+" -> "+depName)
				}
			}
		}
		result.Unresolved = unresolved
	}
	return result
}

// composerDirectEdges derives root -> direct edges from composer.json's require
// (prod) and require-dev (dev) maps, resolved to locked versions.
func composerDirectEdges(manifest []byte, node func(string) string) []types.DependencyEdge {
	if len(manifest) == 0 {
		return nil
	}
	var composerJSON struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal(manifest, &composerJSON); err != nil {
		return nil
	}
	var edges []types.DependencyEdge
	emit := func(reqs map[string]string, scope string) {
		for name := range reqs {
			if isComposerPlatformReq(name) {
				continue
			}
			if to := node(name); to != "" {
				edges = append(edges, types.DependencyEdge{From: ".", To: to, Scope: scope})
			}
		}
	}
	emit(composerJSON.Require, types.ScopeProd)
	emit(composerJSON.RequireDev, types.ScopeDev)
	return edges
}

// isComposerPlatformReq reports whether a require key is a platform requirement
// (php, hhvm, ext-*, lib-*, composer-*) rather than an installable package.
func isComposerPlatformReq(name string) bool {
	switch name {
	case "php", "hhvm":
		return true
	}
	for _, prefix := range []string{"ext-", "lib-", "composer-", "php-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	// Packagist package names always contain a vendor/name slash; platform reqs
	// do not.
	return !strings.Contains(name, "/")
}
