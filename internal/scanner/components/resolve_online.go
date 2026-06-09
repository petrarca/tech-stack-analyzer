package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
)

// Online (deps.dev) dependency resolution is off by default to preserve the
// offline guarantee; the online resolver only fills gaps where no local
// lockfile/tree-file is present, and only when explicitly enabled. The endpoint
// is configurable so an API-compatible facade or mirror can be used instead of
// the public deps.dev API.
var (
	resolveOnlineMu       sync.RWMutex
	resolveOnline         bool
	resolveOnlineEndpoint string
)

// SetResolveOnline enables or disables the online dependency-resolution
// fallback.
func SetResolveOnline(enable bool) {
	resolveOnlineMu.Lock()
	defer resolveOnlineMu.Unlock()
	resolveOnline = enable
}

// ResolveOnline reports whether the online fallback is enabled.
func ResolveOnline() bool {
	resolveOnlineMu.RLock()
	defer resolveOnlineMu.RUnlock()
	return resolveOnline
}

// SetResolveOnlineEndpoint overrides the online-resolver base URL. Empty uses
// the public deps.dev API. A deps.dev-API-compatible facade or mirror (same
// /v3/systems/.../versions/...:dependencies shape) can be supplied here.
func SetResolveOnlineEndpoint(endpoint string) {
	resolveOnlineMu.Lock()
	defer resolveOnlineMu.Unlock()
	resolveOnlineEndpoint = endpoint
}

// ResolveOnlineEndpoint returns the configured online-resolver base URL ("" =
// public deps.dev).
func ResolveOnlineEndpoint() string {
	resolveOnlineMu.RLock()
	defer resolveOnlineMu.RUnlock()
	return resolveOnlineEndpoint
}

// depsDevResolver builds the online fallback resolver, wired to the configured
// endpoint when online resolution is enabled. When disabled it carries no
// Online resolver and falls through (no edges, no network), preserving the
// offline default.
func depsDevResolver() *resolver.DepsDevResolver {
	r := &resolver.DepsDevResolver{Enabled: ResolveOnline()}
	if r.Enabled {
		r.Online = resolver.NewDepsDevFetcher(ResolveOnlineEndpoint(), nil)
	}
	return r
}
