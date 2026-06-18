package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// LockfileGraphProducer is an alias for parsers.LockfileProducer. It is re-
// exported here so detectors only need to import "components", not both
// "components" and "parsers", keeping the detector API surface small.
type LockfileGraphProducer = parsers.LockfileProducer

// graphRequest is a deferred dependency-graph resolution captured during the
// scan walk and executed afterwards, in a dedicated resolution phase. Detectors
// register one per component (via AttachLockfileGraph*); the scanner drains the
// queue once the walk is complete (ResolveDeferredGraphs).
//
// Deferring keeps the (fast, local) scan walk separate from the (slow, network-
// bound) dependency resolution: detection produces components with their
// declared dependencies, then resolution enriches them with the graph. It also
// gives resolution a clear phase boundary for progress reporting.
type graphRequest struct {
	payload   *types.Payload
	dir       string
	provider  types.Provider
	producers []LockfileGraphProducer
	fallback  resolver.DependencyResolver
}

var (
	graphQueueMu sync.Mutex
	graphQueue   []graphRequest
)

// AttachLockfileGraph registers a deferred dependency-graph resolution for the
// component. It is the single, generic entry point every detector uses -- a
// detector supplies an ordered list of lockfile -> graph parser for its
// ecosystem(s). Nothing is resolved here; the work runs later in
// ResolveDeferredGraphs. It is a no-op when the dependency-graph mode is off.
//
// Producers are tried in slice order and the first lockfile that exists wins,
// so callers must list them in the same priority order they use for flat
// dependency extraction.
func AttachLockfileGraph(payload *types.Payload, currentPath string, provider types.Provider, producers []LockfileGraphProducer) {
	AttachLockfileGraphWithFallback(payload, currentPath, provider, producers, nil)
}

// AttachLockfileGraphWithFallback is AttachLockfileGraph with an optional
// caller-supplied fallback resolver tried after the committed lockfile/tree and
// before deps.dev (e.g. the Maven repo-crawl resolver per --maven-graph-source).
func AttachLockfileGraphWithFallback(payload *types.Payload, currentPath string, provider types.Provider, producers []LockfileGraphProducer, fallback resolver.DependencyResolver) {
	if DependencyGraphMode() == types.DependencyGraphOff || !UseLockFiles() {
		return
	}
	graphQueueMu.Lock()
	graphQueue = append(graphQueue, graphRequest{
		payload:   payload,
		dir:       currentPath,
		provider:  provider,
		producers: producers,
		fallback:  fallback,
	})
	graphQueueMu.Unlock()
}

// ResolveDeferredGraphs executes all deferred graph-resolution requests
// registered during the scan walk, then clears the queue. This is the
// dependency-resolution phase: it runs once, after detection, so the network-
// bound resolution is isolated from the file walk. Returns the number of
// requests processed.
func ResolveDeferredGraphs() int {
	graphQueueMu.Lock()
	queue := graphQueue
	graphQueue = nil
	graphQueueMu.Unlock()

	for _, r := range queue {
		resolveGraphRequest(r)
	}
	return len(queue)
}

// mavenGraphFallbackHook builds the Maven transitive-graph fallback resolver
// (repo crawl / deps.dev hybrid per --maven-graph-source). It is registered by
// the java detector via RegisterMavenGraphFallback so this package can build it
// without importing the java detector package (which imports this one).
var mavenGraphFallbackHook func(provider types.Provider) resolver.DependencyResolver

// RegisterMavenGraphFallback registers the Maven graph-fallback factory. Called
// once from the java detector's init().
func RegisterMavenGraphFallback(fn func(provider types.Provider) resolver.DependencyResolver) {
	mavenGraphFallbackHook = fn
}

// mavenEcosystems are the payload ComponentType values whose transitive graph
// can use the Maven repo-crawl fallback (they share Maven coordinates).
var mavenEcosystems = map[string]bool{"maven": true, "gradle": true, "java": true}

// ResolvePayloadGraphOnline resolves the transitive dependency graph for an
// already-built payload tree WITHOUT access to the original source files --
// purely from each component's resolved direct dependencies (coordinates),
// using the online resolvers (deps.dev, and the Maven repo crawl/hybrid when a
// repository is configured). It is the off-scan counterpart to
// ResolveDeferredGraphs, used by the `sbom` command to enrich a saved
// direct-only scan with transitive dependencies.
//
// It reuses resolveGraphRequest unchanged: each component is resolved with no
// lockfile producers (the source tree is gone, so the lockfile resolver no-ops
// and the chain falls through to the online resolvers) and, for Maven-coordinate
// ecosystems, the registered Maven fallback. Resolution honors the same global
// flags as a scan (--dependency-graph, --deps-dev, --maven-graph-source,
// --maven-repo-url, --maven-central, --maven-settings). Returns the number of
// components processed.
//
// provider only needs to satisfy the interface; its file reads are expected to
// fail (no source tree) -- only GetBasePath is used (as an online-cache key).
func ResolvePayloadGraphOnline(root *types.Payload, provider types.Provider) int {
	if root == nil || DependencyGraphMode() == types.DependencyGraphOff {
		return 0
	}
	count := 0
	var walk func(p *types.Payload)
	walk = func(p *types.Payload) {
		if len(p.Dependencies) > 0 {
			var fallback resolver.DependencyResolver
			if mavenGraphFallbackHook != nil && mavenEcosystems[p.ComponentType] {
				fallback = mavenGraphFallbackHook(provider)
			}
			resolveGraphRequest(graphRequest{
				payload:  p,
				dir:      "",
				provider: provider,
				// no producers: the source tree is gone; the lockfile resolver
				// no-ops and the chain uses the online resolvers.
				producers: nil,
				fallback:  fallback,
			})
			count++
		}
		for _, child := range p.Children {
			walk(child)
		}
	}
	walk(root)
	return count
}

// resolveGraphRequest runs one component's graph resolution: committed
// lockfile/tree -> fallback (if any) -> deps.dev, fanned out over the
// component's declared dependencies.
func resolveGraphRequest(r graphRequest) {
	mode := DependencyGraphMode()

	resolvers := []resolver.DependencyResolver{resolver.NewLockfileResolver(r.producers...)}
	if r.fallback != nil {
		resolvers = append(resolvers, r.fallback)
	}
	resolvers = append(resolvers, depsDevResolver(r.provider))
	chain := resolver.NewChain(resolvers...)

	req := resolver.Request{
		Dir:       r.dir,
		Provider:  r.provider,
		Mode:      mode,
		Ecosystem: r.payload.ComponentType,
	}
	for _, dep := range r.payload.Dependencies {
		if dep.Name != "" && dep.Version != "" {
			req.Dependencies = append(req.Dependencies, resolver.Coordinates{
				Name:    dep.Name,
				Version: dep.Version,
			})
		}
	}

	res, err := chain.Resolve(req)
	if err != nil {
		r.payload.SetComponentProperty("dependency_graph", "error", err.Error())
		return
	}
	r.payload.DependencyEdges = append(r.payload.DependencyEdges, res.Edges...)
	if len(res.Unresolved) > 0 {
		r.payload.SetComponentProperty("dependency_graph", "unresolved", res.Unresolved)
	}
}
