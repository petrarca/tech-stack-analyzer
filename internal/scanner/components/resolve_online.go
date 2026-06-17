package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// The public online sources are off by default to preserve the offline
// guarantee, and are enabled independently:
//
//   - deps.dev: online dependency-graph resolution (edges) for manifest-only
//     ecosystems. Endpoint configurable for an API-compatible facade/mirror.
//   - Maven Central: the public fallback for Maven BOM/parent POM fetch.
//
// They are separate concerns (graph vs. version resolution, different
// services), so each has its own switch rather than one umbrella flag. An
// explicitly configured private Maven repo (--maven-repo-url) is a distinct,
// always-on opt-in and is not gated here.
var (
	useDepsDevMu sync.RWMutex
	useDepsDev   bool

	depsDevEndpointMu sync.RWMutex
	depsDevEndpoint   string

	useMavenCentralMu sync.RWMutex
	useMavenCentral   bool

	mavenGraphSourceMu sync.RWMutex
	mavenGraphSource   string // "" (default) | "repo" | "deps-dev" | "none"
)

// SetMavenGraphSource sets the Maven transitive-graph source override. Empty
// means "follow the global --deps-dev default".
func SetMavenGraphSource(source string) {
	mavenGraphSourceMu.Lock()
	defer mavenGraphSourceMu.Unlock()
	mavenGraphSource = source
}

// MavenGraphSource returns the Maven graph-source override ("" = follow
// --deps-dev). Values: "repo" (crawl the POM-source chain), "deps-dev", "none".
func MavenGraphSource() string {
	mavenGraphSourceMu.RLock()
	defer mavenGraphSourceMu.RUnlock()
	return mavenGraphSource
}

// SetUseDepsDev enables or disables online dependency-graph resolution via
// deps.dev.
func SetUseDepsDev(enable bool) {
	useDepsDevMu.Lock()
	defer useDepsDevMu.Unlock()
	useDepsDev = enable
}

// UseDepsDev reports whether deps.dev resolution is enabled.
func UseDepsDev() bool {
	useDepsDevMu.RLock()
	defer useDepsDevMu.RUnlock()
	return useDepsDev
}

// SetDepsDevEndpoint overrides the deps.dev base URL. Empty uses the public
// deps.dev API. A deps.dev-API-compatible facade or mirror (same
// /v3/systems/.../versions/...:dependencies shape) can be supplied here.
func SetDepsDevEndpoint(endpoint string) {
	depsDevEndpointMu.Lock()
	defer depsDevEndpointMu.Unlock()
	depsDevEndpoint = endpoint
}

// DepsDevEndpoint returns the configured deps.dev base URL ("" = public).
func DepsDevEndpoint() string {
	depsDevEndpointMu.RLock()
	defer depsDevEndpointMu.RUnlock()
	return depsDevEndpoint
}

// SetUseMavenCentral enables or disables the public Maven Central fallback for
// Maven BOM/parent POM fetch.
func SetUseMavenCentral(enable bool) {
	useMavenCentralMu.Lock()
	defer useMavenCentralMu.Unlock()
	useMavenCentral = enable
}

// UseMavenCentral reports whether the public Maven Central fallback is enabled.
func UseMavenCentral() bool {
	useMavenCentralMu.RLock()
	defer useMavenCentralMu.RUnlock()
	return useMavenCentral
}

// Maven repository settings for offline-first version resolution. The local
// ~/.m2 read and the remote repository are independent opt-ins layered under
// the same offline-by-default principle: reading outside the scanned tree
// (local cache) and the network (remote) are both explicit.
var (
	mavenSettingsMu   sync.RWMutex
	useMavenLocalRepo bool                   // read ~/.m2/repository for BOM/parent POMs
	mavenLocalRepoDir string                 // override for the local repo path ("" = default)
	mavenRepoURL      string                 // remote Maven repo base ("" = Maven Central)
	mavenRepoUser     string                 // username for Basic auth (paired with token)
	mavenRepoToken    string                 // token: Basic password (with user) or Bearer
	mavenSettings     *mavenresolve.Settings // parsed settings.xml (repos, creds, local repo)
)

// SetUseMavenLocalRepo enables reading the local ~/.m2 repository as a POM
// source (offline; reads outside the scanned tree).
func SetUseMavenLocalRepo(enable bool) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	useMavenLocalRepo = enable
}

// UseMavenLocalRepo reports whether the local ~/.m2 repository is used.
func UseMavenLocalRepo() bool {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return useMavenLocalRepo
}

// SetMavenLocalRepoDir overrides the local repository path ("" = Maven default
// resolution: MAVEN_REPO_LOCAL / MAVEN_OPTS / ~/.m2/repository).
func SetMavenLocalRepoDir(dir string) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	mavenLocalRepoDir = dir
}

// MavenLocalRepoDir returns the configured local repository path override.
func MavenLocalRepoDir() string {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return mavenLocalRepoDir
}

// SetMavenRepoURL sets the remote Maven repository base URL ("" = Central).
func SetMavenRepoURL(url string) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	mavenRepoURL = url
}

// MavenRepoURL returns the configured remote Maven repository base URL.
func MavenRepoURL() string {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return mavenRepoURL
}

// SetMavenRepoToken sets the bearer token for an authenticated remote Maven
// repository. Supplied from the environment, never persisted.
func SetMavenRepoToken(token string) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	mavenRepoToken = token
}

// MavenRepoToken returns the configured remote Maven repository token (Basic
// password when a user is set, otherwise a Bearer token).
func MavenRepoToken() string {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return mavenRepoToken
}

// SetMavenRepoUser sets the username for Basic auth against the configured
// remote Maven repository (env-sourced, paired with the token as password).
func SetMavenRepoUser(user string) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	mavenRepoUser = user
}

// MavenRepoUser returns the configured remote Maven repository username.
func MavenRepoUser() string {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return mavenRepoUser
}

// SetMavenSettings stores the loaded Maven settings.xml (repository URLs,
// credentials, and local-repository path) for reuse by the resolution chain.
func SetMavenSettings(s *mavenresolve.Settings) {
	mavenSettingsMu.Lock()
	defer mavenSettingsMu.Unlock()
	mavenSettings = s
}

// MavenSettings returns the loaded Maven settings.xml, or nil when none was
// configured/found.
func MavenSettings() *mavenresolve.Settings {
	mavenSettingsMu.RLock()
	defer mavenSettingsMu.RUnlock()
	return mavenSettings
}

// depsDevFetcherCache memoizes one deps.dev fetcher per provider base path, so
// its response cache is shared across every component in a scan instead of
// being rebuilt (empty) per component.
var (
	depsDevFetcherMu    sync.Mutex
	depsDevFetcherCache = map[string]resolver.OnlineGraphResolver{}
)

// depsDevResolver builds the online fallback resolver for the tree behind
// provider, wired to the configured endpoint when deps.dev resolution is
// enabled. The underlying fetcher (and its response cache) is shared per scan.
// When disabled it carries no Online resolver and falls through (no edges, no
// network), preserving the offline default.
func depsDevResolver(provider types.Provider) *resolver.DepsDevResolver {
	return newDepsDevResolver(provider, UseDepsDev())
}

// NewDepsDevResolver builds an enabled deps.dev resolver for the tree behind
// provider, sharing the scan-wide fetcher cache. It is used by ecosystem
// detectors that compose deps.dev explicitly (e.g. the Maven hybrid graph
// source), independent of the global --deps-dev default.
func NewDepsDevResolver(provider types.Provider) *resolver.DepsDevResolver {
	return newDepsDevResolver(provider, true)
}

func newDepsDevResolver(provider types.Provider, enabled bool) *resolver.DepsDevResolver {
	r := &resolver.DepsDevResolver{Enabled: enabled}
	if !r.Enabled {
		return r
	}
	base := provider.GetBasePath()
	depsDevFetcherMu.Lock()
	defer depsDevFetcherMu.Unlock()
	f, ok := depsDevFetcherCache[base]
	if !ok {
		f = resolver.NewDepsDevFetcher(DepsDevEndpoint(), nil)
		depsDevFetcherCache[base] = f
	}
	r.Online = f
	return r
}

// MavenEffectiveGraphSource resolves the effective Maven transitive-graph
// source: the explicit --maven-graph-source override when set, otherwise the
// global --deps-dev default ("deps-dev" when enabled, else "none").
func MavenEffectiveGraphSource() string {
	if s := MavenGraphSource(); s != "" {
		return s
	}
	if UseDepsDev() {
		return "deps-dev"
	}
	return "none"
}
