package components

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
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
// payload using the first matching lockfile graph producer found in currentPath.
//
// It is the single, generic entry point every detector uses -- there is no
// per-ecosystem special-casing. A detector supplies an ordered list of
// lockfile -> graph parser for the ecosystem(s) it handles; this helper honors
// the global dependency-graph mode and is a no-op when the mode is off.
//
// Producers are tried in slice order and the first lockfile that exists wins,
// so callers must list them in the same priority order they use for flat
// dependency extraction.
func AttachLockfileGraph(payload *types.Payload, currentPath string, provider types.Provider, producers []LockfileGraphProducer) {
	mode := DependencyGraphMode()
	if mode == types.DependencyGraphOff || !UseLockFiles() {
		return
	}
	for _, producer := range producers {
		content, err := provider.ReadFile(filepath.Join(currentPath, producer.Lockfile))
		if err != nil || len(content) == 0 {
			continue
		}
		graph := producer.Parse(content, mode)
		if len(graph.Edges) > 0 {
			payload.DependencyEdges = append(payload.DependencyEdges, graph.Edges...)
		}
		// First matching lockfile wins (mirrors dependency extraction priority).
		return
	}
}
