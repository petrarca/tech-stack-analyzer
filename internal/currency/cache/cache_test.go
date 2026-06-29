package cache

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

// countingResolver records how many times the inner resolver was called.
type countingResolver struct {
	calls int
	info  currency.LatestInfo
	err   error
}

func (c *countingResolver) LatestVersion(system, name string) (currency.LatestInfo, error) {
	c.calls++
	return c.info, c.err
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "currency.db"), 5000)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCacheReadThroughWriteBack(t *testing.T) {
	s := openStore(t)
	inner := &countingResolver{info: currency.LatestInfo{Latest: "2.0.0"}}
	r, err := New(s, inner, time.Hour, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// First call: miss -> inner called -> cached.
	if info, err := r.LatestVersion("npm", "x"); err != nil || info.Latest != "2.0.0" {
		t.Fatalf("first: %+v %v", info, err)
	}
	// Second call: hit -> inner NOT called again.
	if info, err := r.LatestVersion("npm", "x"); err != nil || info.Latest != "2.0.0" {
		t.Fatalf("second: %+v %v", info, err)
	}
	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1 (second should be a cache hit)", inner.calls)
	}
}

func TestCacheNegativeEntry(t *testing.T) {
	s := openStore(t)
	inner := &countingResolver{err: currency.ErrNotFound}
	r, _ := New(s, inner, time.Hour, false)

	for i := 0; i < 3; i++ {
		if _, err := r.LatestVersion("npm", "missing"); !errors.Is(err, currency.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	}
	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1 (negative result must be cached)", inner.calls)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	s := openStore(t)
	inner := &countingResolver{info: currency.LatestInfo{Latest: "2.0.0"}}
	r, _ := New(s, inner, time.Hour, false)

	// First call caches the entry.
	_, _ = r.LatestVersion("npm", "x")
	if inner.calls != 1 {
		t.Fatalf("after first call inner.calls=%d, want 1", inner.calls)
	}
	// Age the entry past its TTL by rewriting fetched_at into the far past
	// (TTL granularity is whole seconds, so simulate age directly).
	if _, err := s.DB().Exec(`UPDATE currency SET fetched_at = fetched_at - 100000 WHERE system='npm' AND name='x'`); err != nil {
		t.Fatalf("age entry: %v", err)
	}
	// Next call sees an expired entry -> re-fetch.
	_, _ = r.LatestVersion("npm", "x")
	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (entry should have expired)", inner.calls)
	}
}

func TestCacheForceBypass(t *testing.T) {
	s := openStore(t)
	inner := &countingResolver{info: currency.LatestInfo{Latest: "2.0.0"}}

	// Prime the cache with a normal resolver.
	warm, _ := New(s, inner, time.Hour, false)
	_, _ = warm.LatestVersion("npm", "x")
	if inner.calls != 1 {
		t.Fatalf("warm: inner calls = %d, want 1", inner.calls)
	}

	// Force resolver over the same store: must re-fetch despite a fresh entry.
	forced, _ := New(s, inner, time.Hour, true)
	_, _ = forced.LatestVersion("npm", "x")
	if inner.calls != 2 {
		t.Errorf("force: inner calls = %d, want 2 (force must bypass the cache read)", inner.calls)
	}
}

func TestCacheKeepsStaleOnError(t *testing.T) {
	s := openStore(t)
	// Prime a good value, then age it past TTL so the next read is a miss.
	good := &countingResolver{info: currency.LatestInfo{Latest: "2.0.0"}}
	r1, _ := New(s, good, time.Hour, false)
	_, _ = r1.LatestVersion("npm", "x")
	if _, err := s.DB().Exec(`UPDATE currency SET fetched_at = fetched_at - 100000 WHERE system='npm' AND name='x'`); err != nil {
		t.Fatalf("age entry: %v", err)
	}

	// Now the entry is expired; a transient error must NOT overwrite/delete it.
	failing := &countingResolver{err: errors.New("network down")}
	r2, _ := New(s, failing, time.Hour, false)
	if _, err := r2.LatestVersion("npm", "x"); err == nil {
		t.Fatal("expected transient error to surface")
	}
	// The stale row should still be present (not deleted by the error path).
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM currency WHERE system='npm' AND name='x'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("stale entry rows = %d, want 1 (error must not delete the entry)", n)
	}
}
