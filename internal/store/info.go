package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// Info is a read-only snapshot of a store's on-disk state, for `cache info`.
type Info struct {
	Path          string
	Source        PathSource
	Exists        bool
	SizeBytes     int64
	SchemaVersion string
	TableRows     map[string]int64 // table name -> row count
}

// Stat reports store state at path WITHOUT creating the file. If the file does
// not exist, Exists is false and the caller should report "no cache yet". This
// is what `cache info` uses so it never initializes a cache.
func Stat(path string, source PathSource) (Info, error) {
	info := Info{Path: path, Source: source, TableRows: map[string]int64{}}

	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return info, nil // Exists stays false
	}
	if err != nil {
		return info, fmt.Errorf("store: stat %s: %w", path, err)
	}
	info.Exists = true
	info.SizeBytes = fi.Size()

	// Open read-only so we never create or migrate from an info call.
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(2000)", path))
	if err != nil {
		return info, fmt.Errorf("store: open (ro) %s: %w", path, err)
	}
	defer func() { _ = db.Close() }()

	_ = db.QueryRow(`SELECT value FROM store_meta WHERE key='schema_version'`).Scan(&info.SchemaVersion)

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return info, fmt.Errorf("store: list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return info, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return info, err
	}
	for _, t := range tables {
		var n int64
		// Table names come from sqlite_master, not user input; safe to interpolate.
		if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, t)).Scan(&n); err == nil {
			info.TableRows[t] = n
		}
	}
	return info, nil
}

// Vacuum reclaims space after deletions. It opens the store (creating it if
// absent is avoided by the caller, which should check existence first).
func (s *Store) Vacuum() error {
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return fmt.Errorf("store: vacuum: %w", err)
	}
	return nil
}
