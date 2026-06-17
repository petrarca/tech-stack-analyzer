package components

import (
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
)

// Online (deps.dev) dependency resolution is off by default to preserve the
// offline guarantee; the online resolver only fills gaps where no local
// lockfile/tree-file is present, and only when explicitly enabled. The endpoint
// is configurable so an API-compatible facade or mirror can be used instead of
// the public deps.dev API.
// resolveOnlineMu and resolveOnlineEndpointMu are intentionally separate: the
// two settings are logically independent and set by different callers at
// startup. Sharing one lock would couple unrelated state (F-14).
var (
	resolveOnlineMu sync.RWMutex
	resolveOnline   bool

	resolveOnlineEndpointMu sync.RWMutex
	resolveOnlineEndpoint   string
)

// SetResolveOnline enables or disables the online dependency-resolution
// fallback.
func SetResolveOnline(enable bool) {
	resolveOnlineMu.Lock()
	defer resolveOnlineMu.Unlock()
	resolveOnline = enable
}

// ResolveOnline reports whether the online fallback is enabled.
func ResolveOnline() bool {
	resolveOnlineMu.RLock()
	defer resolveOnlineMu.RUnlock()
	return resolveOnline
}

// SetResolveOnlineEndpoint overrides the online-resolver base URL. Empty uses
// the public deps.dev API. A deps.dev-API-compatible facade or mirror (same
// /v3/systems/.../versions/...:dependencies shape) can be supplied here.
func SetResolveOnlineEndpoint(endpoint string) {
	resolveOnlineEndpointMu.Lock()
	defer resolveOnlineEndpointMu.Unlock()
	resolveOnlineEndpoint = endpoint
}

// ResolveOnlineEndpoint returns the configured online-resolver base URL ("" =
// public deps.dev).
func ResolveOnlineEndpoint() string {
	resolveOnlineEndpointMu.RLock()
	defer resolveOnlineEndpointMu.RUnlock()
	return resolveOnlineEndpoint
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
// endpoint when online resolution is enabled. When disabled it carries no
// Online resolver and falls through (no edges, no network), preserving the
// offline default.
func depsDevResolver() *resolver.DepsDevResolver {
	r := &resolver.DepsDevResolver{Enabled: ResolveOnline()}
	if r.Enabled {
		r.Online = resolver.NewDepsDevFetcher(ResolveOnlineEndpoint(), nil)
	}
	return r
}
