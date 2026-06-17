package mavenresolve

import "path/filepath"

// PomReader reads a file's bytes by provider-relative path. It is the minimal
// slice of a file provider this package needs, kept as a tiny interface so the
// package does not depend on the scanner's provider or component machinery
// (avoiding an import cycle).
type PomReader interface {
	ReadFile(path string) ([]byte, error)
}

// CoordinateLookup returns the manifest path(s) declaring a "groupId:artifactId"
// coordinate within the scanned tree, in preference order. It is satisfied by
// the components source index without this package importing it.
type CoordinateLookup func(coordinate string) []string

// RepoSource resolves POMs that are committed to the scanned tree, via a
// coordinate->path lookup (the source index) plus a provider to read them.
// This is the offline, in-repo tier: a BOM or parent POM that lives in the
// repository resolves with no cache and no network.
type RepoSource struct {
	lookup CoordinateLookup
	reader PomReader
}

// NewRepoSource builds an in-repo source. Returns nil when either dependency is
// nil, so it can be included in a chain unconditionally.
func NewRepoSource(lookup CoordinateLookup, reader PomReader) *RepoSource {
	if lookup == nil || reader == nil {
		return nil
	}
	return &RepoSource{lookup: lookup, reader: reader}
}

// Name implements PomSource.
func (s *RepoSource) Name() string { return "repo(source-index)" }

// FetchPOM implements PomSource by locating the coordinate's POM in the tree.
// The version is ignored: an in-repo module has a single POM regardless of the
// declared version, and BOM coordinates are unique per repo.
func (s *RepoSource) FetchPOM(groupID, artifactID, _ string) ([]byte, string, bool) {
	if s == nil || groupID == "" || artifactID == "" {
		return nil, "", false
	}
	paths := s.lookup(groupID + ":" + artifactID)
	if len(paths) == 0 {
		return nil, "", false
	}
	content, err := s.reader.ReadFile(paths[0])
	if err != nil {
		return nil, "", false
	}
	return content, filepath.Dir(paths[0]), true
}
