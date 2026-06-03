package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
)

// resolveOnline gates online (deps.dev) dependency resolution. It is off by
// default to preserve the offline guarantee; the online resolver only fills
// gaps where no local lockfile/tree-file is present, and only when explicitly
// enabled. The actual network fetcher is wired separately when the online
// integration lands; until then enabling this is a no-op (the resolver falls
// through without a fetcher).
var (
	resolveOnlineMu sync.RWMutex
	resolveOnline   bool
)

// SetResolveOnline enables or disables the online deps.dev resolver fallback.
func SetResolveOnline(enable bool) {
	resolveOnlineMu.Lock()
	defer resolveOnlineMu.Unlock()
	resolveOnline = enable
}

// ResolveOnline reports whether the online deps.dev fallback is enabled.
func ResolveOnline() bool {
	resolveOnlineMu.RLock()
	defer resolveOnlineMu.RUnlock()
	return resolveOnline
}

// depsDevResolver builds the online fallback resolver. It is enabled only when
// ResolveOnline() is true; the network fetcher is not yet wired, so an enabled
// resolver currently still falls through (no edges) until the online
// integration provides a DepsDevFetcher. This keeps the seam in place without
// crossing the offline boundary.
func depsDevResolver() *resolver.DepsDevResolver {
	return &resolver.DepsDevResolver{
		Enabled: ResolveOnline(),
		Fetch:   nil, // wired when online resolution is implemented
	}
}
