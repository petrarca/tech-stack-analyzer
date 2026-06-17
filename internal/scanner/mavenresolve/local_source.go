package mavenresolve

import (
	"os"
	"path/filepath"
	"strings"
)

// LocalRepoSource reads POMs from a local Maven repository on disk (the
// ~/.m2/repository cache by default), exactly as Maven and Trivy do. It is
// fully offline and needs no credentials: a developer/CI machine that has built
// the project typically has most of its POMs -- public and previously-built
// private artifacts -- already cached here, so this tier closes much of the gap
// with no network.
//
// A POM lives at {repo}/{group/path}/{artifact}/{version}/{artifact}-{version}.pom.
type LocalRepoSource struct {
	repoDir string
}

// DefaultLocalRepoDir returns the local Maven repository path, resolved the way
// Maven does, in precedence order:
//
//  1. MAVEN_REPO_LOCAL env var (convenience override; not a Maven standard but
//     commonly used in CI to relocate the cache without a settings.xml).
//  2. maven.repo.local system property exposed via the conventional
//     MAVEN_OPTS "-Dmaven.repo.local=PATH" (parsed best-effort).
//  3. $HOME/.m2/repository (Maven default).
//
// Returns "" when none can be determined. Note: <localRepository> in
// settings.xml is not read here -- that would require parsing settings.xml,
// which can be added as a separate source if needed.
func DefaultLocalRepoDir() string {
	if v := strings.TrimSpace(os.Getenv("MAVEN_REPO_LOCAL")); v != "" {
		return v
	}
	if v := mavenRepoLocalFromOpts(os.Getenv("MAVEN_OPTS")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".m2", "repository")
}

// mavenRepoLocalFromOpts extracts a "-Dmaven.repo.local=PATH" value from a
// MAVEN_OPTS string, or "" when absent.
func mavenRepoLocalFromOpts(opts string) string {
	const flag = "-Dmaven.repo.local="
	for _, tok := range strings.Fields(opts) {
		if strings.HasPrefix(tok, flag) {
			return strings.Trim(strings.TrimPrefix(tok, flag), `"'`)
		}
	}
	return ""
}

// NewLocalRepoSource builds a source rooted at repoDir (empty uses the default
// ~/.m2/repository). Returns nil when no usable directory can be determined, so
// callers can include it in a chain unconditionally.
func NewLocalRepoSource(repoDir string) *LocalRepoSource {
	if repoDir == "" {
		repoDir = DefaultLocalRepoDir()
	}
	if repoDir == "" {
		return nil
	}
	if info, err := os.Stat(repoDir); err != nil || !info.IsDir() {
		return nil
	}
	return &LocalRepoSource{repoDir: repoDir}
}

// Name implements PomSource.
func (s *LocalRepoSource) Name() string { return "local(~/.m2)" }

// FetchPOM implements PomSource by reading the POM file from the local repo.
func (s *LocalRepoSource) FetchPOM(groupID, artifactID, version string) ([]byte, string, bool) {
	if s == nil || groupID == "" || artifactID == "" || version == "" {
		return nil, "", false
	}
	groupPath := filepath.Join(strings.Split(groupID, ".")...)
	dir := filepath.Join(s.repoDir, groupPath, artifactID, version)
	pomPath := filepath.Join(dir, artifactID+"-"+version+".pom")

	content, err := os.ReadFile(pomPath) //nolint:gosec // path built from coordinate segments under repoDir
	if err != nil {
		return nil, "", false
	}
	return content, dir, true
}
