package components

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// AttachLockfileGraph attaches package-to-package dependency edges to the
// payload using the first matching lockfile graph producer found in currentPath.
//
// It is the single, generic entry point every detector uses -- there is no
// per-ecosystem special-casing. A detector supplies a map of lockfile name ->
// graph parser for the ecosystem(s) it handles; this helper honors the global
// dependency-graph mode and is a no-op when the mode is off.
//
// Producers are tried in map-iteration order is non-deterministic, so callers
// that have a lockfile priority should pass an ordered list instead; for the
// common single-lockfile case the map form is sufficient.
func AttachLockfileGraph(payload *types.Payload, currentPath string, provider types.Provider, producers map[string]parsers.ParseGraphFunc) {
	mode := DependencyGraphMode()
	if mode == types.DependencyGraphOff || !UseLockFiles() {
		return
	}
	for lockName, parse := range producers {
		content, err := provider.ReadFile(filepath.Join(currentPath, lockName))
		if err != nil || len(content) == 0 {
			continue
		}
		graph := parse(content, mode)
		if len(graph.Edges) > 0 {
			payload.DependencyEdges = append(payload.DependencyEdges, graph.Edges...)
		}
		// First matching lockfile wins (mirrors dependency extraction priority).
		return
	}
}
