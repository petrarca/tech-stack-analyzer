// Package resolvestats holds process-wide counters for dependency-resolution
// activity (POM fetches, cache hits, deps.dev calls). The network fetchers
// increment them cheaply; a reporter samples them to surface progress through
// the normal progress/event mechanism (never raw stderr), so a long resolution
// phase is not silent.
//
// Counters are global and monotonic for the process. Callers that want a
// per-scan delta should snapshot at the start and subtract.
package resolvestats

import (
	"strings"
	"sync/atomic"

	"fmt"
)

var (
	pomFetched   atomic.Int64 // POMs fetched over the network (Maven repos)
	cacheHits    atomic.Int64 // POM/response cache hits (fetch avoided)
	depsDevCalls atomic.Int64 // deps.dev graph requests
	authFailures atomic.Int64 // 401/403 responses from a Maven repository
)

// AddPOMFetched records a network POM fetch.
func AddPOMFetched() { pomFetched.Add(1) }

// AddCacheHit records a cache hit (a fetch avoided).
func AddCacheHit() { cacheHits.Add(1) }

// AddDepsDevCall records a deps.dev graph request.
func AddDepsDevCall() { depsDevCalls.Add(1) }

// AddAuthFailure records a 401/403 from a Maven repository (missing/invalid
// credentials), so the scanner can warn that private artifacts went unresolved.
func AddAuthFailure() { authFailures.Add(1) }

// Snapshot is a point-in-time copy of the counters.
type Snapshot struct {
	POMFetched   int64
	CacheHits    int64
	DepsDevCalls int64
	AuthFailures int64
}

// Get returns the current counter values.
func Get() Snapshot {
	return Snapshot{
		POMFetched:   pomFetched.Load(),
		CacheHits:    cacheHits.Load(),
		DepsDevCalls: depsDevCalls.Load(),
		AuthFailures: authFailures.Load(),
	}
}

// Sub returns the delta s - base (per field), for per-scan reporting.
func (s Snapshot) Sub(base Snapshot) Snapshot {
	return Snapshot{
		POMFetched:   s.POMFetched - base.POMFetched,
		CacheHits:    s.CacheHits - base.CacheHits,
		DepsDevCalls: s.DepsDevCalls - base.DepsDevCalls,
		AuthFailures: s.AuthFailures - base.AuthFailures,
	}
}

// Active reports whether any resolution activity has been recorded in the delta.
func (s Snapshot) Active() bool {
	return s.POMFetched > 0 || s.CacheHits > 0 || s.DepsDevCalls > 0 || s.AuthFailures > 0
}

// Format renders a human-readable, source-broken-down summary of the counters
// (e.g. "12 POMs, 393 deps.dev, 40 cached"), as shown in the resolution-phase
// progress line. Returns "no fetches" when nothing was fetched.
func (s Snapshot) Format() string {
	var parts []string
	if s.POMFetched > 0 {
		parts = append(parts, fmt.Sprintf("%d POMs", s.POMFetched))
	}
	if s.DepsDevCalls > 0 {
		parts = append(parts, fmt.Sprintf("%d deps.dev", s.DepsDevCalls))
	}
	if s.CacheHits > 0 {
		parts = append(parts, fmt.Sprintf("%d cached", s.CacheHits))
	}
	if len(parts) == 0 {
		return "no fetches"
	}
	return strings.Join(parts, ", ")
}
