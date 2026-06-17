package mavenresolve

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// maxGraphDepth bounds the transitive crawl depth (deep enough for real trees,
// a guard against pathological cases on top of the visited-set cycle break).
const maxGraphDepth = 64

// graphFetchConcurrency bounds parallel POM fetches within one BFS level. POM
// fetches are independent, latency-dominated network calls, so a moderate fan-
// out cuts wall-clock time without overwhelming a repository.
const graphFetchConcurrency = 16

// GraphResolver produces the transitive Maven dependency graph by crawling POMs
// from the repository chain (in-repo source index, local ~/.m2, configured
// remote) -- the way Maven and Trivy resolve, but offline-first and tolerant of
// rate limits. Unlike deps.dev it covers private artifacts, because the chain
// includes internal repositories.
//
// It implements resolver.DependencyResolver and is selected for Maven by
// --maven-graph-source=repo. A POM that cannot be fetched (private artifact
// absent from every tier, or a transient failure) contributes no children and
// never aborts the scan.
type GraphResolver struct {
	// source fetches a POM by coordinates (the composed POM-source chain).
	source PomSource
}

// NewGraphResolver builds a GraphResolver over a POM source (typically a Chain).
// Returns nil when source is nil so callers can include it conditionally.
func NewGraphResolver(source PomSource) *GraphResolver {
	if source == nil {
		return nil
	}
	return &GraphResolver{source: source}
}

// Name implements resolver.DependencyResolver.
func (r *GraphResolver) Name() string { return "maven-repo" }

// Resolve crawls the transitive graph from the request's declared dependencies.
// Returns Resolved=false when there is nothing to seed from, so the chain can
// fall through.
func (r *GraphResolver) Resolve(req resolver.Request) (resolver.Result, error) {
	if r == nil || r.source == nil || len(req.Dependencies) == 0 {
		return resolver.Result{Resolved: false}, nil
	}

	full := req.Mode == types.DependencyGraphFull

	// Maven's conflict mediation is nearest-wins: among versions requested for a
	// coordinate, the one closest to the root wins, and there is exactly one
	// version per coordinate in the result. We reproduce it with a breadth-first
	// walk -- BFS visits shallower (nearer) requests first -- recording one
	// chosen version per coordinate, edges by coordinate, then rewriting edge
	// endpoints to the chosen version at the end.
	c := &crawl{
		source:   r.source,
		chosen:   make(map[string]string), // coordinate -> mediated version
		edges:    make([]coordEdge, 0),    // edges keyed by coordinate (from/to)
		seen:     make(map[string]bool),   // coordinate-edge dedup
		expanded: make(map[string]bool),   // coordinate already expanded (cycle guard)
	}

	// Seed: root -> each declared dependency. The synthetic "." root matches the
	// node identity convention of the other resolvers.
	var queue []qitem
	for _, dep := range req.Dependencies {
		if dep.Name == "" || !semver.IsResolved(dep.Version) {
			continue
		}
		c.choose(dep.Name, dep.Version)
		c.addEdge(".", dep.Name)
		if full {
			queue = append(queue, qitem{dep.Name, dep.Version, 1})
		}
	}

	// Expand the queue level by level; each level fetches concurrently.
	for len(queue) > 0 {
		queue = c.processLevel(queue)
	}

	edges := c.resolveEdges()
	if len(edges) == 0 {
		return resolver.Result{Resolved: false}, nil
	}
	return resolver.Result{Edges: edges, Source: resolver.SourceMavenRepo, Resolved: true}, nil
}

// qitem is a queued coordinate to expand at a given BFS depth.
type qitem struct {
	coord, version string
	depth          int
}

// processLevel expands one BFS level and returns the next level's queue. POM
// fetches for the level's coordinates run concurrently (bounded) -- they are
// independent, latency-dominated network calls. Edge recording, version
// mediation (nearest-wins, preserved by BFS order), and building the next
// frontier run sequentially after the fetches, so the result is deterministic.
func (c *crawl) processLevel(level []qitem) []qitem {
	// Deduplicate the level and skip already-expanded / too-deep items.
	var items []qitem
	for _, it := range level {
		if it.depth > maxGraphDepth || c.expanded[it.coord] {
			continue
		}
		c.expanded[it.coord] = true
		items = append(items, qitem{it.coord, c.chosen[it.coord], it.depth})
	}

	// Fetch + parse each item's children concurrently.
	results := make([][]types.Dependency, len(items))
	sem := make(chan struct{}, graphFetchConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		group, artifact := splitMavenName(items[i].coord)
		if group == "" || artifact == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, group, artifact, version string) {
			defer wg.Done()
			defer func() { <-sem }()
			content, _, ok := c.source.FetchPOM(group, artifact, version)
			if !ok {
				return // private/absent or transient: no children, never aborts
			}
			results[i] = parsers.ParsePomDependenciesForGraph(content, c.source.FetchPOM)
		}(i, group, artifact, items[i].version)
	}
	wg.Wait()

	// Merge: record edges, mediate versions, enqueue the next level.
	var next []qitem
	for i, item := range items {
		for _, child := range results[i] {
			if !semver.IsResolved(child.Version) {
				continue
			}
			c.choose(child.Name, child.Version)
			c.addEdge(item.coord, child.Name)
			next = append(next, qitem{child.Name, child.Version, item.depth + 1})
		}
	}
	return next
}

// coordEdge is a parent->child edge keyed by coordinate (no version); the
// version is applied from the mediated choice when materializing.
type coordEdge struct{ from, to string }

// crawl holds the mutable state of one transitive walk.
type crawl struct {
	source   PomSource
	chosen   map[string]string // coordinate -> mediated (nearest-wins) version
	edges    []coordEdge
	seen     map[string]bool // coordinate-edge dedup
	expanded map[string]bool // coordinate already expanded (cycle guard)
}

// choose records the mediated version for a coordinate: the first (nearest in
// BFS order) request wins; later, deeper requests do not override it.
func (c *crawl) choose(coord, version string) {
	if _, ok := c.chosen[coord]; !ok {
		c.chosen[coord] = version
	}
}

// addEdge records a unique coordinate edge ("." for the synthetic root).
func (c *crawl) addEdge(from, to string) {
	key := from + "|" + to
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.edges = append(c.edges, coordEdge{from: from, to: to})
}

// resolveEdges materializes coordinate edges into name@version edges using the
// mediated version for each coordinate, so every endpoint of a coordinate uses
// the single chosen version (one version per coordinate, as Maven resolves).
func (c *crawl) resolveEdges() []types.DependencyEdge {
	out := make([]types.DependencyEdge, 0, len(c.edges))
	node := func(coord string) string {
		if coord == "." {
			return "."
		}
		return coord + "@" + c.chosen[coord]
	}
	for _, e := range c.edges {
		out = append(out, types.DependencyEdge{From: node(e.from), To: node(e.to)})
	}
	return out
}

// splitMavenName splits "groupId:artifactId" into its parts.
func splitMavenName(name string) (group, artifact string) {
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			return name[:i], name[i+1:]
		}
	}
	return "", ""
}
