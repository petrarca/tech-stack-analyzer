package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/blobcache"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// graphCacheMu guards graphCache, which memoizes one blob cache per provider
// base path. The cache is shared across every component in a scan so a network
// fetcher (Maven POM, deps.dev response) fetches a given upstream artifact at
// most once, instead of rebuilding an empty cache per component.
var (
	graphCacheMu sync.Mutex
	graphCache   = map[string]blobcache.Cache{}
)

// GetGraphCache returns the scan-wide blob cache for the tree behind provider,
// creating it on first use and keying it by the provider's base path. Safe for
// concurrent callers.
func GetGraphCache(provider types.Provider) blobcache.Cache {
	base := provider.GetBasePath()

	graphCacheMu.Lock()
	defer graphCacheMu.Unlock()
	if c, ok := graphCache[base]; ok {
		return c
	}
	c := blobcache.NewMemory()
	graphCache[base] = c
	return c
}
