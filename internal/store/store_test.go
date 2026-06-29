package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOpenCreatesFileAndMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "currency.db") // sub dir must be created

	s, err := Open(path, 5000)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
	var v string
	if err := s.DB().QueryRow(`SELECT value FROM store_meta WHERE key='schema_version'`).Scan(&v); err != nil {
		t.Fatalf("schema_version not recorded: %v", err)
	}
	if v != "1" {
		t.Errorf("schema_version = %q, want 1", v)
	}
}

// Stat must NOT create the file (the lazy-creation rule for `cache info`).
func TestStatDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "currency.db")

	info, err := Stat(path, SourceDefault)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Exists {
		t.Error("Stat reported Exists=true for a missing file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Stat created the database file; it must not")
	}
}

func TestStatReportsTablesAndRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "currency.db")

	s, err := Open(path, 5000)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := s.DB().Exec(`CREATE TABLE t(k TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, err := s.DB().Exec(`INSERT INTO t(k) VALUES(?)`, k); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	_ = s.Close()

	info, err := Stat(path, SourceDefault)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.Exists || info.SizeBytes == 0 {
		t.Errorf("expected existing non-empty db, got %+v", info)
	}
	if info.SchemaVersion != "1" {
		t.Errorf("schema version = %q, want 1", info.SchemaVersion)
	}
	if info.TableRows["t"] != 3 {
		t.Errorf("table t rows = %d, want 3", info.TableRows["t"])
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	t.Setenv(EnvCachePath, "/env/path/currency.db")

	if p, src, _ := ResolvePath("/flag/path.db"); p != "/flag/path.db" || src != SourceFlag {
		t.Errorf("flag precedence: got %q/%s", p, src)
	}
	if p, src, _ := ResolvePath(""); p != "/env/path/currency.db" || src != SourceEnv {
		t.Errorf("env precedence: got %q/%s", p, src)
	}
	t.Setenv(EnvCachePath, "")
	p, src, err := ResolvePath("")
	if err != nil {
		t.Fatalf("default path: %v", err)
	}
	if src != SourceDefault || filepath.Base(p) != DefaultFileName {
		t.Errorf("default precedence: got %q/%s", p, src)
	}
}

// Two store handles writing the same file concurrently (proxy for multi-process)
// must not corrupt the DB. WAL + busy_timeout handles contention.
func TestConcurrentWritesNoCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "currency.db")

	open := func() *Store {
		s, err := Open(path, 10000)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		return s
	}
	s1, s2 := open(), open()
	defer s1.Close()
	defer s2.Close()
	if _, err := s1.DB().Exec(`CREATE TABLE c(k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	writer := func(s *Store, prefix string) {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_, err := s.DB().Exec(
				`INSERT INTO c(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
				prefix+string(rune('0'+i%10))+itoa(i), "x")
			if err != nil {
				t.Errorf("concurrent write: %v", err)
				return
			}
		}
	}
	wg.Add(2)
	go writer(s1, "a")
	go writer(s2, "b")
	wg.Wait()

	var ok string
	if err := s1.DB().QueryRow(`PRAGMA integrity_check`).Scan(&ok); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if ok != "ok" {
		t.Errorf("integrity_check = %q, want ok", ok)
	}
}

// itoa avoids strconv import noise in the test fixture.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
