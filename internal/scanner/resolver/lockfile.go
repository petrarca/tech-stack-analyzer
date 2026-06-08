package resolver

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
)

// LockfileProducer pairs a lockfile (or pre-generated tree file) name with its
// graph parser, plus an optional manifest filename for the same component
// (e.g. Cargo.toml for Cargo.lock, package.json for package-lock.json). The
// list is ordered: the first file that exists wins, matching each ecosystem's
// flat-extraction priority (npm: package-lock > pnpm > yarn; python: uv >
// poetry).
type LockfileProducer struct {
	Lockfile string
	Manifest string // optional; read and passed to the producer for direct/scope
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
		// Read the optional manifest for direct-dependency and scope derivation.
		// Absent or unreadable manifest is fine -- producers tolerate nil.
		var manifest []byte
		if p.Manifest != "" {
			if m, mErr := req.Provider.ReadFile(filepath.Join(req.Dir, p.Manifest)); mErr == nil {
				manifest = m
			}
		}
		graph := p.Parse(parsers.GraphInput{
			Lockfile: content,
			Manifest: manifest,
			Mode:     req.Mode,
		})
		// A present lockfile resolves the component even if it yields zero edges
		// (e.g. a leaf with no dependencies); that is still authoritative and
		// must not fall through to an online approximation.
		return Result{
			Edges:      graph.Edges,
			Source:     SourceLockfile,
			Resolved:   true,
			Unresolved: graph.Unresolved,
		}, nil
	}
	return Result{Resolved: false}, nil
}
