// Package license provides shared license detection, normalization, SPDX
// expression parsing, risk categorization, and per-dependency license
// harvesting.
package license

import (
	"os"
	"path/filepath"
	"strings"
)

// Harvester resolves the declared license of a single resolved package by
// looking it up in one or more local roots (a package cache or an in-tree
// install directory). Implementations are ecosystem-specific.
type Harvester interface {
	// Ecosystem returns the dependency type (PURL type) this harvester serves,
	// e.g. "npm" or "nuget".
	Ecosystem() string
	// License returns the normalized SPDX license id for the given package
	// name/version, or "" when it cannot be resolved from the roots.
	License(name, version string) string
}

// HarvestRoots describes where a harvester may look. InTree roots are always
// searched (they are inside the scanned project). CacheRoots are global,
// out-of-tree caches and are only included when license-cache harvesting is
// enabled (the --harvest-licenses opt-in).
type HarvestRoots struct {
	InTree     []string
	CacheRoots []string
}

// allRoots returns the combined, existence-filtered search roots.
func (r HarvestRoots) allRoots() []string {
	var roots []string
	for _, p := range append(append([]string{}, r.InTree...), r.CacheRoots...) {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			roots = append(roots, p)
		}
	}
	return roots
}

// readFileLimited reads up to maxLicenseFileSize bytes from path, returning ""
// on any error or when the file is empty. Used to read small manifest/license
// files safely.
func readFileLimited(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxLicenseFileSize {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// firstExisting returns the first path that exists as a regular file, or "".
func firstExisting(paths ...string) string {
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// joinLower joins path segments, lower-casing each segment. Several package
// caches store packages under lower-cased directory names.
func joinLower(base string, segments ...string) string {
	parts := make([]string, 0, len(segments)+1)
	parts = append(parts, base)
	for _, s := range segments {
		parts = append(parts, strings.ToLower(s))
	}
	return filepath.Join(parts...)
}
