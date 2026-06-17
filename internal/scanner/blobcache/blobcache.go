// Package blobcache provides a small, concurrency-safe keyed blob store with a
// negative (not-found) cache. It backs the network fetchers used during
// dependency resolution -- Maven POM bytes and deps.dev responses -- so a given
// upstream artifact is fetched at most once per scan, shared across all
// components, instead of being re-fetched per component.
//
// The store is an interface so the in-memory implementation used today can be
// replaced later by a persistent one (file/SQLite) for cross-run caching, since
// published POMs and resolved-graph responses are immutable. A persistent
// implementation may choose to ignore or TTL the negative cache, because a 404
// today can become a published artifact later.
package blobcache

import "sync"

// Cache is a keyed blob store with a negative cache. Keys are caller-namespaced
// strings (e.g. "maven:group:artifact:version", "depsdev:system:name:version").
// Implementations must be safe for concurrent use.
type Cache interface {
	// Get returns the cached blob for key. found reports a positive hit;
	// notFound reports a cached negative result. When both are false the key is
	// unknown and the caller should fetch.
	Get(key string) (blob []byte, found bool, notFound bool)
	// Put stores a blob for key (a positive result).
	Put(key string, blob []byte)
	// PutNotFound records that key is known-absent (a negative result).
	PutNotFound(key string)
}

// Memory is an in-memory Cache. The zero value is not usable; use NewMemory.
type Memory struct {
	mu       sync.RWMutex
	blobs    map[string][]byte
	notFound map[string]bool
}

// NewMemory returns an empty in-memory cache.
func NewMemory() *Memory {
	return &Memory{
		blobs:    make(map[string][]byte),
		notFound: make(map[string]bool),
	}
}

// Get implements Cache.
func (m *Memory) Get(key string) ([]byte, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b, ok := m.blobs[key]; ok {
		return b, true, false
	}
	if m.notFound[key] {
		return nil, false, true
	}
	return nil, false, false
}

// Put implements Cache.
func (m *Memory) Put(key string, blob []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[key] = blob
}

// PutNotFound implements Cache.
func (m *Memory) PutNotFound(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notFound[key] = true
}
