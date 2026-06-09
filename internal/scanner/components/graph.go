package components

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LockfileGraphProducer is an alias for parsers.LockfileProducer. It is re-
// exported here so detectors only need to import "components", not both
// "components" and "parsers", keeping the detector API surface small.
type LockfileGraphProducer = parsers.LockfileProducer

// AttachLockfileGraph attaches package-to-package dependency edges to the
// payload, resolving them through the dependency-resolver chain.
//
// It is the single, generic entry point every detector uses -- there is no
// per-ecosystem special-casing. A detector supplies an ordered list of
// lockfile -> graph parser for the ecosystem(s) it handles; this helper builds
// a resolver chain (local lockfile first, online fallback) and honors the
// global dependency-graph mode, remaining a no-op when the mode is off.
//
// Producers are tried in slice order and the first lockfile that exists wins,
// so callers must list them in the same priority order they use for flat
// dependency extraction.
func AttachLockfileGraph(payload *types.Payload, currentPath string, provider types.Provider, producers []LockfileGraphProducer) {
	mode := DependencyGraphMode()
	if mode == types.DependencyGraphOff || !UseLockFiles() {
		return
	}

	chain := resolver.NewChain(
		resolver.NewLockfileResolver(producers...),
		// Online fallback. Disabled by default; wired only when online
		// resolution is explicitly enabled. Safe to include unconditionally --
		// it falls through when not enabled.
		depsDevResolver(),
	)

	req := resolver.Request{
		Dir:       currentPath,
		Provider:  provider,
		Mode:      mode,
		Ecosystem: payload.ComponentType,
	}
	// Fan-out online resolution over the component's declared dependencies.
	// Any dep with a name and version is a candidate coordinate; the online
	// resolver skips 404s (private deps) and unions the rest. No detector
	// changes are needed: payload.Dependencies is always set by the flat parser.
	for _, dep := range payload.Dependencies {
		if dep.Name != "" && dep.Version != "" {
			req.Dependencies = append(req.Dependencies, resolver.Coordinates{
				Name:    dep.Name,
				Version: dep.Version,
			})
		}
	}

	res, err := chain.Resolve(req)
	if err != nil {
		// F-02: surface chain errors rather than silently swallowing them.
		payload.SetComponentProperty("dependency_graph", "error", err.Error())
		return
	}

	payload.DependencyEdges = append(payload.DependencyEdges, res.Edges...)

	// Surface unresolved dependency references (lockfile drift, unparseable
	// refs) rather than dropping them silently, so consumers can detect gaps.
	if len(res.Unresolved) > 0 {
		payload.SetComponentProperty("dependency_graph", "unresolved", res.Unresolved)
	}
}
