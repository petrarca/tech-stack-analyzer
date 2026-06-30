package components

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Per-dependency license harvesting. In-tree sources (e.g. a node_modules
// directory inside the scanned project) are always read -- they are part of
// what is being scanned and keep results deterministic. Out-of-tree global
// caches (e.g. ~/.nuget/packages) are read only when license-cache harvesting
// is enabled via SetHarvestLicenseCaches, mirroring the --maven-local-repo
// opt-in for the Maven ~/.m2 cache.

var (
	harvestCachesMu sync.RWMutex
	harvestCaches   bool
)

// SetHarvestLicenseCaches enables or disables reading out-of-tree global package
// caches (e.g. ~/.nuget/packages) for per-dependency license harvesting.
func SetHarvestLicenseCaches(enable bool) {
	harvestCachesMu.Lock()
	defer harvestCachesMu.Unlock()
	harvestCaches = enable
}

// HarvestLicenseCaches reports whether out-of-tree cache harvesting is enabled.
func HarvestLicenseCaches() bool {
	harvestCachesMu.RLock()
	defer harvestCachesMu.RUnlock()
	return harvestCaches
}

// HarvestLicenses annotates the payload tree's dependencies with licenses
// resolved from local sources. In-tree sources under basePath are always used;
// global caches are added when HarvestLicenseCaches() is enabled. Returns the
// number of licenses added.
func HarvestLicenses(payload *types.Payload, basePath string) int {
	harvesters := []license.Harvester{
		license.NewNpmHarvester(npmHarvestRoots(basePath)),
		license.NewNugetHarvester(nugetHarvestRoots()),
	}
	return license.ApplyHarvesters(payload, harvesters)
}

// npmHarvestRoots returns the npm search roots: a node_modules directory at the
// scan root (in-tree) plus the global npm cache when cache harvesting is on.
func npmHarvestRoots(basePath string) license.HarvestRoots {
	roots := license.HarvestRoots{
		InTree: []string{filepath.Join(basePath, "node_modules")},
	}
	if HarvestLicenseCaches() {
		// The npm v7+ cache stores tarballs, not extracted package.json trees,
		// so there is no global directory analogous to node_modules to read
		// declared licenses from. Only the in-tree node_modules is used for npm.
		_ = roots
	}
	return roots
}

// nugetHarvestRoots returns the NuGet global-packages-folder roots. NuGet has no
// in-tree install layout (packages live only in the global folder), so these
// are cache roots, gated by HarvestLicenseCaches().
func nugetHarvestRoots() license.HarvestRoots {
	if !HarvestLicenseCaches() {
		return license.HarvestRoots{}
	}
	return license.HarvestRoots{CacheRoots: nugetGlobalPackagesDirs()}
}

// nugetGlobalPackagesDirs returns candidate NuGet global-packages folders,
// honoring the NUGET_PACKAGES override and falling back to ~/.nuget/packages.
func nugetGlobalPackagesDirs() []string {
	if env := os.Getenv("NUGET_PACKAGES"); env != "" {
		return []string{env}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".nuget", "packages")}
}
