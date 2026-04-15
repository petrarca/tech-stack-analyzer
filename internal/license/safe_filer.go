package license

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-enry/go-license-detector/v4/licensedb/filer"
)

// maxLicenseFileSize is the maximum file size the license detector will read.
// Legitimate license files are plain text and rarely exceed a few KB.
const maxLicenseFileSize = 512 * 1024

// safeFiler wraps a filer.Filer and guards against two classes of files that
// would cause the upstream go-license-detector to hang or consume excessive
// memory:
//
//  1. Binary files -- go-license-detector matches filenames containing license
//     keywords (e.g. "lgpl", "mit") and runs expensive Unicode normalization on
//     the raw content. Binary files cause this to run indefinitely.
//  2. Oversized files -- legitimate license files are small plain text; files
//     larger than maxLicenseFileSize are skipped unconditionally.
type safeFiler struct {
	inner filer.Filer
	// dirPath is the base directory, needed to stat files before reading them.
	dirPath string
}

// newSafeFiler creates a safeFiler wrapping the given filer.
func newSafeFiler(inner filer.Filer, dirPath string) *safeFiler {
	return &safeFiler{inner: inner, dirPath: dirPath}
}

func (f *safeFiler) ReadFile(path string) ([]byte, error) {
	// Check file size before reading to avoid loading large binaries into memory.
	info, err := os.Stat(filepath.Join(f.dirPath, path))
	if err == nil && info.Size() > maxLicenseFileSize {
		return nil, fmt.Errorf("skipped oversized file (%d bytes): %s", info.Size(), path)
	}

	content, err := f.inner.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use go-enry's binary detection on the actual content (checks first 8KB).
	if enry.IsBinary(content) {
		return nil, fmt.Errorf("skipped binary file: %s", path)
	}

	return content, nil
}

func (f *safeFiler) ReadDir(path string) ([]filer.File, error) {
	entries, err := f.inner.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// Pre-filter oversized files to avoid even attempting to read them.
	filtered := make([]filer.File, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			info, statErr := os.Stat(filepath.Join(f.dirPath, path, e.Name))
			if statErr == nil && info.Size() > maxLicenseFileSize {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

func (f *safeFiler) Close() {
	f.inner.Close()
}

func (f *safeFiler) PathsAreAlwaysSlash() bool {
	return f.inner.PathsAreAlwaysSlash()
}
