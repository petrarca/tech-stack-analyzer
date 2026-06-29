package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnvCachePath is the environment variable that overrides the default cache DB
// path. It follows the established STACK_ANALYZER_* convention and is the only
// new env var the currency feature introduces.
const EnvCachePath = "STACK_ANALYZER_CURRENCY_CACHE"

// DefaultFileName is the cache database file name under the app cache dir.
const DefaultFileName = "currency.db"

// appCacheSubdir is the per-app subdirectory under os.UserCacheDir().
const appCacheSubdir = "stack-analyzer"

// PathSource identifies where a resolved cache path came from, for `cache info`.
type PathSource string

const (
	SourceFlag    PathSource = "flag"
	SourceEnv     PathSource = "env"
	SourceDefault PathSource = "default"
)

// ResolvePath determines the cache DB path with precedence flag > env > default.
// flagPath is the value of --currency-cache (empty if unset). It does NOT create
// anything; it only computes the path and reports which source set it.
func ResolvePath(flagPath string) (path string, source PathSource, err error) {
	if flagPath != "" {
		return flagPath, SourceFlag, nil
	}
	if env := os.Getenv(EnvCachePath); env != "" {
		return env, SourceEnv, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", SourceDefault, fmt.Errorf("store: resolve default cache dir: %w", err)
	}
	return filepath.Join(dir, appCacheSubdir, DefaultFileName), SourceDefault, nil
}
