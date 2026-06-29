// Package currency reports how far each dependency is behind its latest
// available version (freshness), as a separate artifact from the scan output and
// SBOM. Latest versions are resolved from Google deps.dev, behind a resolver
// abstraction with a chain so additional sources (e.g. an internal registry) can
// be added later without changing callers.
package currency

import (
	"errors"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
)

// ErrNotFound signals that no resolver in the chain could resolve the package
// (e.g. deps.dev returned 404). It mirrors resolver.ErrCoordinateNotFound and is
// the value callers check to record a dependency as "unknown".
var ErrNotFound = errors.New("currency: package not found in any source")

// LatestInfo is the resolver result for a single package: its latest stable
// version and publish/deprecation status.
type LatestInfo struct {
	Latest       string
	IsDeprecated bool
	PublishedAt  string
	Source       string // which resolver answered (e.g. "deps.dev")
}

// CurrencyResolver resolves the latest stable version of a package. system is a
// deps.dev system identifier (npm, maven, pypi, nuget, cargo, go, rubygems);
// name is the package name (Maven uses "group:artifact"). Implementations return
// ErrNotFound when the source does not know the package.
//
// This interface is independent of the graph resolver (OnlineGraphResolver);
// only the underlying deps.dev transport is shared.
type CurrencyResolver interface {
	LatestVersion(system, name string) (LatestInfo, error)
}

// ChainResolver tries each resolver in order until one resolves the package or a
// hard error occurs. ErrNotFound from a resolver means "fall through to the
// next"; if every resolver reports not-found, the chain returns ErrNotFound.
// v1's chain has a single link (deps.dev); internal-registry resolvers can be
// appended later without changing callers.
type ChainResolver struct {
	resolvers []CurrencyResolver
}

// NewChainResolver builds a chain from the given resolvers, in priority order.
func NewChainResolver(resolvers ...CurrencyResolver) *ChainResolver {
	return &ChainResolver{resolvers: resolvers}
}

// LatestVersion implements CurrencyResolver.
func (c *ChainResolver) LatestVersion(system, name string) (LatestInfo, error) {
	for _, r := range c.resolvers {
		info, err := r.LatestVersion(system, name)
		if err == nil {
			return info, nil
		}
		if errors.Is(err, ErrNotFound) {
			continue // try the next source
		}
		return LatestInfo{}, err // hard error (network/5xx): surface it
	}
	return LatestInfo{}, ErrNotFound
}

// depsDevResolver is the deps.dev-backed CurrencyResolver. It is a thin adapter
// over resolver.PackageClient (which reuses the shared deps.dev transport).
type depsDevResolver struct {
	client *resolver.PackageClient
}

// NewDepsDevResolver builds a deps.dev currency resolver. endpoint is the
// --deps-dev-endpoint override (empty = public deps.dev).
func NewDepsDevResolver(endpoint string) CurrencyResolver {
	return &depsDevResolver{client: resolver.NewPackageClient(endpoint, nil)}
}

// LatestVersion implements CurrencyResolver via deps.dev GetPackage.
func (d *depsDevResolver) LatestVersion(system, name string) (LatestInfo, error) {
	info, err := d.client.GetPackage(system, name)
	if errors.Is(err, resolver.ErrCoordinateNotFound) {
		return LatestInfo{}, ErrNotFound
	}
	if err != nil {
		return LatestInfo{}, err
	}
	return LatestInfo{
		Latest:       info.LatestVersion,
		IsDeprecated: info.IsDeprecated,
		PublishedAt:  info.PublishedAt,
		Source:       "deps.dev",
	}, nil
}
