// Package cache provides a TTL-backed, multi-process currency cache that
// decorates a currency.CurrencyResolver. It owns the `currency` table inside the
// shared internal/store database; the store facade itself knows nothing about
// currency.
//
// Caching is keyed by (system, name) -- not version -- because "latest version"
// is a property of the package, not of any installed pin. Entries carry a
// per-row TTL (latest versions change when upstream ships releases), and
// not-found results are cached as negative entries (also TTL'd) so internal/
// yanked packages are not re-queried every run.
package cache

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/currency"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
	"github.com/petrarca/tech-stack-analyzer/internal/store"
)

const createTable = `CREATE TABLE IF NOT EXISTS currency (
	system        TEXT NOT NULL,
	name          TEXT NOT NULL,
	latest        TEXT,
	is_deprecated INTEGER NOT NULL DEFAULT 0,
	published_at  TEXT,
	not_found     INTEGER NOT NULL DEFAULT 0,
	fetched_at    INTEGER NOT NULL,
	ttl_seconds   INTEGER NOT NULL,
	PRIMARY KEY (system, name)
) WITHOUT ROWID;`

// Resolver decorates an inner currency.CurrencyResolver with the SQLite cache.
type Resolver struct {
	inner currency.CurrencyResolver
	db    *sql.DB
	ttl   time.Duration
	force bool // when true, skip cache reads (re-fetch), still write back
}

// New builds a cache-backed resolver over s, wrapping inner. ttl is the per-entry
// lifetime. When force is true, every lookup skips the cache read and re-fetches
// from inner, writing the fresh result back (existing entries are overwritten,
// never pre-deleted; a transient error leaves the stale entry intact).
func New(s *store.Store, inner currency.CurrencyResolver, ttl time.Duration, force bool) (*Resolver, error) {
	if _, err := s.DB().Exec(createTable); err != nil {
		return nil, fmt.Errorf("currency cache: create table: %w", err)
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Resolver{inner: inner, db: s.DB(), ttl: ttl, force: force}, nil
}

// LatestVersion implements currency.CurrencyResolver with a read-through,
// write-back cache. A cached negative (not_found) within TTL returns
// currency.ErrNotFound without hitting the network.
func (r *Resolver) LatestVersion(system, name string) (currency.LatestInfo, error) {
	if !r.force {
		if info, notFound, hit := r.read(system, name); hit {
			resolvestats.AddCurrencyCacheHit()
			if notFound {
				return currency.LatestInfo{}, currency.ErrNotFound
			}
			return info, nil
		}
	}

	info, err := r.inner.LatestVersion(system, name)
	switch {
	case err == nil:
		r.write(system, name, info, false)
		return info, nil
	case errors.Is(err, currency.ErrNotFound):
		r.write(system, name, currency.LatestInfo{}, true) // negative cache
		return currency.LatestInfo{}, currency.ErrNotFound
	default:
		// Transient error: do not touch the cache (stale entry, if any, survives).
		return currency.LatestInfo{}, err
	}
}

// read returns (info, notFound, hit). hit is false on a miss or an expired entry.
func (r *Resolver) read(system, name string) (currency.LatestInfo, bool, bool) {
	var (
		latest     sql.NullString
		published  sql.NullString
		deprecated int
		notFound   int
		fetchedAt  int64
		ttlSeconds int64
	)
	row := r.db.QueryRow(
		`SELECT latest, is_deprecated, published_at, not_found, fetched_at, ttl_seconds
		 FROM currency WHERE system=? AND name=?`, system, name)
	if err := row.Scan(&latest, &deprecated, &published, &notFound, &fetchedAt, &ttlSeconds); err != nil {
		return currency.LatestInfo{}, false, false // miss (incl. sql.ErrNoRows)
	}
	if time.Now().Unix() > fetchedAt+ttlSeconds {
		return currency.LatestInfo{}, false, false // expired -> treat as miss
	}
	return currency.LatestInfo{
		Latest:       latest.String,
		IsDeprecated: deprecated != 0,
		PublishedAt:  published.String,
	}, notFound != 0, true
}

// write upserts a positive or negative entry with a fresh fetched_at + TTL.
func (r *Resolver) write(system, name string, info currency.LatestInfo, notFound bool) {
	nf := 0
	if notFound {
		nf = 1
	}
	dep := 0
	if info.IsDeprecated {
		dep = 1
	}
	// A cache write failure degrades to a miss on the next lookup but is not
	// fatal (the cache is a best-effort performance layer; the caller already
	// received the resolved value). Assign to _ to satisfy static analysis;
	// when a logger is threaded into the cache, replace with a debug log.
	_, _ = r.db.Exec(
		`INSERT INTO currency(system,name,latest,is_deprecated,published_at,not_found,fetched_at,ttl_seconds)
		 VALUES(?,?,?,?,?,?,?,?)
		 ON CONFLICT(system,name) DO UPDATE SET
		   latest=excluded.latest, is_deprecated=excluded.is_deprecated,
		   published_at=excluded.published_at, not_found=excluded.not_found,
		   fetched_at=excluded.fetched_at, ttl_seconds=excluded.ttl_seconds`,
		system, name, nullStr(info.Latest), dep, nullStr(info.PublishedAt), nf,
		time.Now().Unix(), int64(r.ttl.Seconds()),
	)
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ClearAll removes every currency entry from the store. Returns the number of
// rows deleted. The currency table is created first if absent (so callers get a
// clean 0 rather than an error on a fresh store).
func ClearAll(s *store.Store) (int64, error) {
	if _, err := s.DB().Exec(createTable); err != nil {
		return 0, err
	}
	res, err := s.DB().Exec(`DELETE FROM currency`)
	if err != nil {
		return 0, fmt.Errorf("currency cache: clear: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ClearExpired removes only currency entries whose TTL has elapsed. Returns the
// number of rows deleted.
func ClearExpired(s *store.Store) (int64, error) {
	if _, err := s.DB().Exec(createTable); err != nil {
		return 0, err
	}
	res, err := s.DB().Exec(
		`DELETE FROM currency WHERE (fetched_at + ttl_seconds) <= ?`, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("currency cache: clear expired: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
