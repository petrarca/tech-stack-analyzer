// Package store provides a unified, lazily-opened SQLite store shared by
// scanner features that need cross-run, multi-process caching (currency now;
// the dependency-resolution blob cache later).
//
// The store is a thin facade over a single SQLite database file. It owns the
// file lifecycle, the connection pragmas (WAL + busy_timeout for safe
// multi-process access), and a schema-version record. It knows nothing about
// what consumers store: each consumer creates and owns its own table(s) via
// CREATE TABLE IF NOT EXISTS on first use.
//
// Lazy creation is a hard rule: opening the store creates the database file if
// it does not exist, so callers must only Open when a feature actually needs to
// cache something. A plain scan, an SBOM run, or any command that does not use a
// store-backed feature must never call Open. Importing this package has no side
// effects.
//
// The database must live on a local filesystem; SQLite file locking is
// unreliable on network filesystems (NFS/SMB).
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// SchemaVersion is the current store schema version. The facade records it in a
// meta table; consumers may consult it for migrations. Bumped when the shared
// meta layout changes (not when a consumer changes its own table).
const SchemaVersion = 1

// Store is a handle to the shared SQLite database. Safe for concurrent use by
// multiple goroutines; the underlying SQLite file is safe for multiple
// processes via WAL mode and busy_timeout.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (creating if necessary) the SQLite store at path. It creates the
// parent directory, applies the connection pragmas, and ensures the meta table
// with the schema version. Because it creates the file, callers must only Open
// when a store-backed feature is actually used.
//
// busyTimeoutMS is the SQLite busy_timeout in milliseconds (how long a writer
// waits for a competing writer before erroring); 5000 is a sane default.
func Open(path string, busyTimeoutMS int) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store: empty database path")
	}
	if busyTimeoutMS <= 0 {
		busyTimeoutMS = 5000
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create cache directory: %w", err)
	}

	// WAL: concurrent readers + a single writer, across processes.
	// busy_timeout: wait rather than fail on brief write contention.
	// synchronous=NORMAL: safe with WAL, much faster than FULL.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)",
		path, busyTimeoutMS,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	s := &Store{db: db, path: path}
	if err := s.initMeta(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB returns the underlying *sql.DB so consumers can create and operate their
// own tables. Consumers must use CREATE TABLE IF NOT EXISTS and must not alter
// the meta table.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// initMeta ensures the meta table exists and records the schema version. The
// meta table is the only table the facade itself owns.
func (s *Store) initMeta() error {
	const ddl = `CREATE TABLE IF NOT EXISTS store_meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	) WITHOUT ROWID;`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("store: init meta: %w", err)
	}
	// Record the schema version if not already present.
	const ins = `INSERT INTO store_meta(key, value) VALUES('schema_version', ?)
		ON CONFLICT(key) DO NOTHING;`
	if _, err := s.db.Exec(ins, SchemaVersion); err != nil {
		return fmt.Errorf("store: record schema version: %w", err)
	}
	return nil
}
