package components

import (
	"path/filepath"
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// SourceIndex maps an ecosystem coordinate (e.g. a Maven "groupId:artifactId"
// or an npm package name) to the path(s) of the manifest file(s) that declare
// it within the scanned tree. It answers the question Trivy answers by reaching
// into ~/.m2 or a remote repository -- "where is the POM/manifest for this
// coordinate?" -- but strictly from sources present in the repository, keeping
// the scan offline.
//
// It is ecosystem-agnostic on purpose: the same "coordinate -> source path"
// lookup is useful beyond Maven BOM imports (e.g. resolving workspace members
// in npm/Go/Python monorepos). Ecosystems opt in by registering a
// SourceIndexer; an ecosystem with no registered indexer contributes nothing.
type SourceIndex struct {
	// byEcosystem[ecosystem][coordinate] = manifest paths (provider-relative).
	byEcosystem map[string]map[string][]string
}

// Lookup returns the manifest path(s) declaring coordinate in the given
// ecosystem, or nil when none are indexed. The first element is the preferred
// match (insertion order during the walk).
func (idx *SourceIndex) Lookup(ecosystem, coordinate string) []string {
	if idx == nil {
		return nil
	}
	if m, ok := idx.byEcosystem[ecosystem]; ok {
		return m[coordinate]
	}
	return nil
}

// SourceIndexer extracts the coordinate a manifest file declares, so the file
// can be found later by that coordinate. Implementations are pure (parse only)
// and must not assume any particular directory layout.
type SourceIndexer interface {
	// Ecosystem is the index namespace (e.g. "maven", "npm").
	Ecosystem() string
	// Matches reports whether fileName is a manifest this indexer understands.
	Matches(fileName string) bool
	// Coordinate parses content and returns the coordinate the manifest
	// declares (e.g. "groupId:artifactId"), or "" when it declares none.
	Coordinate(content []byte) string
}

var (
	sourceIndexersMu sync.RWMutex
	sourceIndexers   []SourceIndexer

	// sourceIndexCache memoizes a built index per provider base path so the
	// tree is walked at most once per scan, regardless of how many detectors
	// query it.
	sourceIndexCacheMu sync.Mutex
	sourceIndexCache   = map[string]*SourceIndex{}
)

// RegisterSourceIndexer adds an indexer. Called from detector init().
func RegisterSourceIndexer(indexer SourceIndexer) {
	sourceIndexersMu.Lock()
	defer sourceIndexersMu.Unlock()
	sourceIndexers = append(sourceIndexers, indexer)
}

// GetSourceIndex returns the source index for the scanned tree behind provider,
// building it on first use and caching it by the provider's base path. Safe for
// concurrent callers. Returns an empty (non-nil) index when no indexers are
// registered or the tree cannot be walked, so callers never need a nil check
// beyond Lookup's own guard.
func GetSourceIndex(provider types.Provider) *SourceIndex {
	base := provider.GetBasePath()

	sourceIndexCacheMu.Lock()
	defer sourceIndexCacheMu.Unlock()
	if idx, ok := sourceIndexCache[base]; ok {
		return idx
	}

	idx := buildSourceIndex(provider)
	sourceIndexCache[base] = idx
	return idx
}

// buildSourceIndex walks the tree once and applies every registered indexer to
// each file, recording coordinate -> path entries.
func buildSourceIndex(provider types.Provider) *SourceIndex {
	idx := &SourceIndex{byEcosystem: map[string]map[string][]string{}}

	sourceIndexersMu.RLock()
	indexers := append([]SourceIndexer(nil), sourceIndexers...)
	sourceIndexersMu.RUnlock()
	if len(indexers) == 0 {
		return idx
	}

	walkSourceTree(provider, ".", func(relPath, fileName string) {
		var content []byte
		for _, indexer := range indexers {
			if !indexer.Matches(fileName) {
				continue
			}
			if content == nil {
				data, err := provider.ReadFile(relPath)
				if err != nil {
					return
				}
				content = data
			}
			coord := indexer.Coordinate(content)
			if coord == "" {
				continue
			}
			eco := indexer.Ecosystem()
			if idx.byEcosystem[eco] == nil {
				idx.byEcosystem[eco] = map[string][]string{}
			}
			idx.byEcosystem[eco][coord] = append(idx.byEcosystem[eco][coord], relPath)
		}
	})

	return idx
}

// walkSourceTree does a bounded depth-first walk of the provider tree, invoking
// visit for every file with its provider-relative path and base name. Directory
// traversal errors are skipped so a single unreadable directory does not abort
// the index.
func walkSourceTree(provider types.Provider, dir string, visit func(relPath, fileName string)) {
	const maxDepth = 64 // guard against symlink loops / pathological trees
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maxDepth {
			return
		}
		entries, err := provider.ListDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			child := e.Name
			if dir != "." {
				child = filepath.Join(dir, e.Name)
			}
			if e.Type == "dir" {
				walk(child, depth+1)
				continue
			}
			visit(child, e.Name)
		}
	}
	walk(dir, 0)
}

// resetSourceIndexCacheForTest clears the build cache. Test-only.
func resetSourceIndexCacheForTest() {
	sourceIndexCacheMu.Lock()
	defer sourceIndexCacheMu.Unlock()
	sourceIndexCache = map[string]*SourceIndex{}
}
