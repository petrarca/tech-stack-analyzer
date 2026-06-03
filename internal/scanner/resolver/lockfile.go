package resolver

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
)

// LockfileProducer pairs a lockfile (or pre-generated tree file) name with its
// graph parser. The list is ordered: the first file that exists wins, matching
// each ecosystem's flat-extraction priority (npm: package-lock > pnpm > yarn;
// python: uv > poetry).
type LockfileProducer struct {
	Lockfile string
	Parse    parsers.ParseGraphFunc
}

// LockfileResolver resolves edges by reading the highest-priority lockfile that
// exists in the component directory. This is the offline, authoritative source.
type LockfileResolver struct {
	producers []LockfileProducer
}

// NewLockfileResolver builds a resolver over an ordered producer list.
func NewLockfileResolver(producers ...LockfileProducer) *LockfileResolver {
	return &LockfileResolver{producers: producers}
}

// Name implements DependencyResolver.
func (r *LockfileResolver) Name() string { return "lockfile" }

// Resolve reads the first matching lockfile and returns its edges. It returns
// Resolved=false when no producer's lockfile is present, so the Chain can fall
// through to an online resolver.
func (r *LockfileResolver) Resolve(req Request) (Result, error) {
	for _, p := range r.producers {
		content, err := req.Provider.ReadFile(filepath.Join(req.Dir, p.Lockfile))
		if err != nil || len(content) == 0 {
			continue
		}
		graph := p.Parse(content, req.Mode)
		// A present lockfile resolves the component even if it yields zero edges
		// (e.g. a leaf with no dependencies); that is still authoritative and
		// must not fall through to an online approximation.
		return Result{
			Edges:    graph.Edges,
			Source:   SourceLockfile,
			Resolved: true,
		}, nil
	}
	return Result{Resolved: false}, nil
}
