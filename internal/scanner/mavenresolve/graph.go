package mavenresolve

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// maxGraphDepth bounds the transitive crawl depth (deep enough for real trees,
// a guard against pathological cases on top of the visited-set cycle break).
const maxGraphDepth = 64

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
		source: r.source,
		chosen: make(map[string]string), // coordinate -> mediated version
		edges:  make([]coordEdge, 0),    // edges keyed by coordinate (from/to)
		seen:   make(map[string]bool),   // coordinate-edge dedup
	}

	type qitem struct {
		coord, version string
		depth          int
	}
	var queue []qitem

	// Seed: root -> each declared dependency. The synthetic "." root matches the
	// node identity convention of the other resolvers.
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

	expanded := make(map[string]bool) // coordinate already expanded (cycle guard)
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if it.depth > maxGraphDepth || expanded[it.coord] {
			continue
		}
		expanded[it.coord] = true

		group, artifact := splitMavenName(it.coord)
		if group == "" || artifact == "" {
			continue
		}
		// Fetch the version chosen for this coordinate (nearest-wins).
		content, _, ok := c.source.FetchPOM(group, artifact, c.chosen[it.coord])
		if !ok {
			continue // private/absent or transient: no children, never aborts
		}
		for _, child := range parsers.ParsePomDependenciesForGraph(content, c.source.FetchPOM) {
			if !semver.IsResolved(child.Version) {
				continue
			}
			c.choose(child.Name, child.Version) // first (nearest) request wins
			c.addEdge(it.coord, child.Name)
			queue = append(queue, qitem{child.Name, child.Version, it.depth + 1})
		}
	}

	edges := c.resolveEdges()
	if len(edges) == 0 {
		return resolver.Result{Resolved: false}, nil
	}
	return resolver.Result{Edges: edges, Source: resolver.SourceMavenRepo, Resolved: true}, nil
}

// coordEdge is a parent->child edge keyed by coordinate (no version); the
// version is applied from the mediated choice when materializing.
type coordEdge struct{ from, to string }

// crawl holds the mutable state of one transitive walk.
type crawl struct {
	source PomSource
	chosen map[string]string // coordinate -> mediated (nearest-wins) version
	edges  []coordEdge
	seen   map[string]bool
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
