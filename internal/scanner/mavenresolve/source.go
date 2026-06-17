// Package mavenresolve resolves Maven artifact versions offline-first, from the
// scanned tree and (opt-in) local/remote Maven repositories.
//
// Maven omits a dependency's version when it is managed by a parent POM or an
// imported BOM (scope=import). Recovering those versions requires locating the
// managing POM. This package encapsulates the different places a POM can come
// from behind a single PomSource seam, and composes them into a precedence
// chain (local-first, network opt-in) -- mirroring internal/scanner/resolver,
// which applies the same pattern to dependency-graph edges.
//
// Resolution tiers, in precedence order:
//
//  1. in-repo source index   -- BOMs committed to the scanned tree (offline)
//  2. local ~/.m2 repository -- previously built/downloaded POMs (offline)
//  3. remote Maven repository -- Central or a configured mirror/JFrog (opt-in)
//
// Cross-module propagation and pre-generated resolved files (dependency-list /
// dependency-tree) are handled separately by the detector, before this chain.
package mavenresolve

// PomSource locates the raw POM for a Maven coordinate. It returns ok=false
// when the coordinate is not available from this source, letting a chain fall
// through to the next source. A concrete version is required (a repository path
// needs one); callers pass the version after resolving any property reference.
//
// The signature matches parsers.BomResolver so a PomSource (or a chain of them)
// can be adapted to the Maven parser's BOM-resolution hook without coupling the
// parser to this package.
type PomSource interface {
	// Name is a short identifier for diagnostics.
	Name() string
	// FetchPOM returns the POM bytes for the coordinate and the directory it
	// was found in (for relative-path resolution; empty when not applicable).
	FetchPOM(groupID, artifactID, version string) (content []byte, dir string, ok bool)
}

// Chain tries sources in order and returns the first hit. It is itself a
// PomSource, so chains compose. A nil or empty chain resolves nothing.
type Chain struct {
	sources []PomSource
}

// NewChain builds a chain from sources in precedence order. Nil sources are
// skipped so callers can include opt-in tiers unconditionally.
func NewChain(sources ...PomSource) *Chain {
	filtered := make([]PomSource, 0, len(sources))
	for _, s := range sources {
		if s != nil {
			filtered = append(filtered, s)
		}
	}
	return &Chain{sources: filtered}
}

// Name implements PomSource.
func (c *Chain) Name() string { return "chain" }

// FetchPOM tries each source in order, returning the first that resolves.
func (c *Chain) FetchPOM(groupID, artifactID, version string) ([]byte, string, bool) {
	if c == nil {
		return nil, "", false
	}
	for _, s := range c.sources {
		if content, dir, ok := s.FetchPOM(groupID, artifactID, version); ok {
			return content, dir, true
		}
	}
	return nil, "", false
}

// Empty reports whether the chain has no sources (so callers can avoid wiring a
// no-op BOM resolver).
func (c *Chain) Empty() bool {
	return c == nil || len(c.sources) == 0
}
