package components

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LockfileGraphProducer pairs a lockfile name with its graph parser. Detectors
// supply an ordered slice so the highest-priority lockfile that exists wins,
// matching how each ecosystem already prioritizes its lockfiles for flat
// dependency extraction (e.g. npm: package-lock > pnpm > yarn; python: uv >
// poetry).
type LockfileGraphProducer struct {
	Lockfile string
	Parse    parsers.ParseGraphFunc
}

// AttachLockfileGraph attaches package-to-package dependency edges to the
// payload, resolving them through the dependency-resolver chain.
//
// It is the single, generic entry point every detector uses -- there is no
// per-ecosystem special-casing. A detector supplies an ordered list of
// lockfile -> graph parser for the ecosystem(s) it handles; this helper builds
// a resolver chain (local lockfile first, online deps.dev fallback) and honors
// the global dependency-graph mode, remaining a no-op when the mode is off.
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
		resolver.NewLockfileResolver(toLockfileProducers(producers)...),
		// Online fallback. Disabled by default; wired only when online
		// resolution is explicitly enabled. Safe to include unconditionally --
		// it falls through when not enabled.
		depsDevResolver(),
	)

	res, err := chain.Resolve(resolver.Request{
		Dir:      currentPath,
		Provider: provider,
		Mode:     mode,
		// Ecosystem/Coordinates are not yet supplied by detectors; the online
		// resolver falls through without them, so behavior is unchanged.
	})
	if err != nil || len(res.Edges) == 0 {
		return
	}
	payload.DependencyEdges = append(payload.DependencyEdges, res.Edges...)
}

// toLockfileProducers adapts the detector-facing producer type to the resolver
// package's type. The shapes are identical; this keeps the detector API stable.
func toLockfileProducers(producers []LockfileGraphProducer) []resolver.LockfileProducer {
	out := make([]resolver.LockfileProducer, len(producers))
	for i, p := range producers {
		out[i] = resolver.LockfileProducer{Lockfile: p.Lockfile, Parse: p.Parse}
	}
	return out
}
