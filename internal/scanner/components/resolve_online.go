package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
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
)

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

// depsDevResolver builds the online fallback resolver, wired to the configured
// endpoint when deps.dev resolution is enabled. When disabled it carries no
// Online resolver and falls through (no edges, no network), preserving the
// offline default.
func depsDevResolver() *resolver.DepsDevResolver {
	r := &resolver.DepsDevResolver{Enabled: UseDepsDev()}
	if r.Enabled {
		r.Online = resolver.NewDepsDevFetcher(DepsDevEndpoint(), nil)
	}
	return r
}
