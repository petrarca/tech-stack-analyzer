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

	// Currency counters: a separate group from the dependency-resolution
	// counters above. They are NOT mixed into Format()/Active(); currency
	// progress uses FormatCurrency()/CurrencyActive(). The currency cache
	// decorator increments currencyCacheHits (never the dependency cacheHits).
	currencyResolved    atomic.Int64 // packages resolved to a latest version
	currencyCacheHits   atomic.Int64 // currency cache hits (lookup avoided)
	currencyUnsupported atomic.Int64 // no deps.dev system for the ecosystem
	currencyUnknown     atomic.Int64 // queried, not found (incl. internal/yanked)
	currencyErrors      atomic.Int64 // transient lookup failures
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

// AddCurrencyResolved records a package resolved to a latest version.
func AddCurrencyResolved() { currencyResolved.Add(1) }

// AddCurrencyCacheHit records a currency cache hit (a lookup avoided). Distinct
// from AddCacheHit (the dependency-resolution POM/graph cache).
func AddCurrencyCacheHit() { currencyCacheHits.Add(1) }

// AddCurrencyUnsupported records an ecosystem with no deps.dev coverage.
func AddCurrencyUnsupported() { currencyUnsupported.Add(1) }

// AddCurrencyUnknown records a package queried but not found in any source.
func AddCurrencyUnknown() { currencyUnknown.Add(1) }

// AddCurrencyError records a transient currency lookup failure.
func AddCurrencyError() { currencyErrors.Add(1) }

// Snapshot is a point-in-time copy of the counters.
type Snapshot struct {
	POMFetched   int64
	CacheHits    int64
	DepsDevCalls int64
	AuthFailures int64

	// Currency counters (separate group).
	CurrencyResolved    int64
	CurrencyCacheHits   int64
	CurrencyUnsupported int64
	CurrencyUnknown     int64
	CurrencyErrors      int64
}

// Get returns the current counter values.
func Get() Snapshot {
	return Snapshot{
		POMFetched:          pomFetched.Load(),
		CacheHits:           cacheHits.Load(),
		DepsDevCalls:        depsDevCalls.Load(),
		AuthFailures:        authFailures.Load(),
		CurrencyResolved:    currencyResolved.Load(),
		CurrencyCacheHits:   currencyCacheHits.Load(),
		CurrencyUnsupported: currencyUnsupported.Load(),
		CurrencyUnknown:     currencyUnknown.Load(),
		CurrencyErrors:      currencyErrors.Load(),
	}
}

// Sub returns the delta s - base (per field), for per-scan reporting.
func (s Snapshot) Sub(base Snapshot) Snapshot {
	return Snapshot{
		POMFetched:          s.POMFetched - base.POMFetched,
		CacheHits:           s.CacheHits - base.CacheHits,
		DepsDevCalls:        s.DepsDevCalls - base.DepsDevCalls,
		AuthFailures:        s.AuthFailures - base.AuthFailures,
		CurrencyResolved:    s.CurrencyResolved - base.CurrencyResolved,
		CurrencyCacheHits:   s.CurrencyCacheHits - base.CurrencyCacheHits,
		CurrencyUnsupported: s.CurrencyUnsupported - base.CurrencyUnsupported,
		CurrencyUnknown:     s.CurrencyUnknown - base.CurrencyUnknown,
		CurrencyErrors:      s.CurrencyErrors - base.CurrencyErrors,
	}
}

// Active reports whether any dependency-resolution activity has been recorded in
// the delta. Currency activity is reported separately via CurrencyActive.
func (s Snapshot) Active() bool {
	return s.POMFetched > 0 || s.CacheHits > 0 || s.DepsDevCalls > 0 || s.AuthFailures > 0
}

// CurrencyActive reports whether any currency activity is in the delta.
func (s Snapshot) CurrencyActive() bool {
	return s.CurrencyResolved > 0 || s.CurrencyCacheHits > 0 ||
		s.CurrencyUnsupported > 0 || s.CurrencyUnknown > 0 || s.CurrencyErrors > 0
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

// FormatCurrency renders the currency-resolution progress line from the currency
// counters only (e.g. "312 resolved, 21 cached, 540 unsupported, 3 unknown"). It
// is separate from Format so the two phases never conflate.
func (s Snapshot) FormatCurrency() string {
	var parts []string
	if s.CurrencyResolved > 0 {
		parts = append(parts, fmt.Sprintf("%d resolved", s.CurrencyResolved))
	}
	if s.CurrencyCacheHits > 0 {
		parts = append(parts, fmt.Sprintf("%d cached", s.CurrencyCacheHits))
	}
	if s.CurrencyUnsupported > 0 {
		parts = append(parts, fmt.Sprintf("%d unsupported", s.CurrencyUnsupported))
	}
	if s.CurrencyUnknown > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", s.CurrencyUnknown))
	}
	if s.CurrencyErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", s.CurrencyErrors))
	}
	if len(parts) == 0 {
		return "no lookups"
	}
	return strings.Join(parts, ", ")
}
