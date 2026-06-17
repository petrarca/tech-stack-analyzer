package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/blobcache"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// graphCacheMu guards graphCache, which memoizes one blob cache per provider
// base path. The cache is shared across every component in a scan so a network
// fetcher (Maven POM, deps.dev response) fetches a given upstream artifact at
// most once, instead of rebuilding an empty cache per component.
var (
	graphCacheMu sync.Mutex
	graphCache   = map[string]blobcache.Cache{}

	mavenMemoMu sync.Mutex
	mavenMemo   = map[string]*mavenresolve.ChildMemo{}
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

// GetMavenChildMemo returns the scan-wide Maven graph child memo for the tree
// behind provider, keyed by base path. Sharing it across all components lets a
// coordinate's transitive subtree be resolved once instead of per component.
func GetMavenChildMemo(provider types.Provider) *mavenresolve.ChildMemo {
	base := provider.GetBasePath()
	mavenMemoMu.Lock()
	defer mavenMemoMu.Unlock()
	if m, ok := mavenMemo[base]; ok {
		return m
	}
	m := mavenresolve.NewChildMemo()
	mavenMemo[base] = m
	return m
}
