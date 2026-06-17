package mavenresolve

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/blobcache"
)

// Settings is the subset of a Maven settings.xml this package needs: the local
// repository path, the configured repositories (with their server id), and the
// server credentials keyed by id. It lets the scanner reuse a developer/CI
// machine's existing Maven configuration -- repository URLs and credentials --
// without re-specifying them, exactly as Maven and Trivy do.
type Settings struct {
	LocalRepository string
	Repositories    []SettingsRepository
	Mirrors         []SettingsMirror
	Servers         map[string]SettingsServer // by id
}

// SettingsMirror redirects matching repositories to a single URL, per Maven's
// <mirror> semantics. MirrorOf is a comma-separated pattern list ("*", explicit
// ids, "external:*", "!excluded"). Credentials come from the <server> whose id
// equals the mirror id.
type SettingsMirror struct {
	ID       string
	URL      string
	MirrorOf string
}

// SettingsRepository is a repository URL and the server id supplying its creds.
type SettingsRepository struct {
	ID  string
	URL string
}

// SettingsServer holds credentials for a repository id. The password is often a
// JFrog/Artifactory reference token, used here as HTTP Basic auth.
type SettingsServer struct {
	Username string
	Password string
}

// xmlSettings mirrors the settings.xml elements we read.
type xmlSettings struct {
	XMLName         xml.Name `xml:"settings"`
	LocalRepository string   `xml:"localRepository"`
	Servers         []struct {
		ID       string `xml:"id"`
		Username string `xml:"username"`
		Password string `xml:"password"`
	} `xml:"servers>server"`
	Mirrors []struct {
		ID       string `xml:"id"`
		URL      string `xml:"url"`
		MirrorOf string `xml:"mirrorOf"`
	} `xml:"mirrors>mirror"`
	Profiles []struct {
		Repositories []struct {
			ID  string `xml:"id"`
			URL string `xml:"url"`
		} `xml:"repositories>repository"`
	} `xml:"profiles>profile"`
}

// DefaultSettingsPath returns the conventional user settings.xml path
// ($HOME/.m2/settings.xml), or "" when the home directory is unknown.
func DefaultSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".m2", "settings.xml")
}

// LoadSettings parses a Maven settings.xml. Returns nil (no error) when the file
// is absent, so callers can treat "no settings" as "no extra repos/creds".
func LoadSettings(path string) (*Settings, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is the user's own settings.xml
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseSettings(data)
}

// parseSettings unmarshals settings.xml content into Settings.
func parseSettings(data []byte) (*Settings, error) {
	var x xmlSettings
	if err := xml.Unmarshal(data, &x); err != nil {
		return nil, err
	}

	s := &Settings{
		LocalRepository: strings.TrimSpace(x.LocalRepository),
		Servers:         make(map[string]SettingsServer),
	}
	for _, srv := range x.Servers {
		id := strings.TrimSpace(srv.ID)
		if id == "" {
			continue
		}
		s.Servers[id] = SettingsServer{
			Username: strings.TrimSpace(srv.Username),
			Password: strings.TrimSpace(srv.Password),
		}
	}
	for _, m := range x.Mirrors {
		url := strings.TrimSpace(m.URL)
		if url == "" {
			continue
		}
		s.Mirrors = append(s.Mirrors, SettingsMirror{
			ID:       strings.TrimSpace(m.ID),
			URL:      url,
			MirrorOf: strings.TrimSpace(m.MirrorOf),
		})
	}
	seen := make(map[string]bool)
	for _, prof := range x.Profiles {
		for _, repo := range prof.Repositories {
			url := strings.TrimSpace(repo.URL)
			if url == "" || seen[url] {
				continue
			}
			seen[url] = true
			s.Repositories = append(s.Repositories, SettingsRepository{
				ID:  strings.TrimSpace(repo.ID),
				URL: url,
			})
		}
	}
	return s, nil
}

// mirrorFor returns the mirror that handles the given repository id, or nil when
// none matches. Maven applies the first matching mirror.
func (s *Settings) mirrorFor(repoID string) *SettingsMirror {
	for i := range s.Mirrors {
		if mirrorMatches(s.Mirrors[i].MirrorOf, repoID) {
			return &s.Mirrors[i]
		}
	}
	return nil
}

// mirrorMatches implements Maven's mirrorOf pattern matching: comma-separated
// tokens of "*", "external:*" (treated as "*" here -- we have no notion of
// local repos), explicit ids, and "!id" exclusions.
func mirrorMatches(mirrorOf, repoID string) bool {
	matched := false
	for _, p := range strings.Split(mirrorOf, ",") {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
			continue
		case len(p) > 1 && p[0] == '!':
			if p[1:] == repoID {
				return false // explicit exclusion wins
			}
		case p == repoID:
			matched = true
		case p == "*" || p == "external:*":
			matched = true
		}
	}
	return matched
}

// RemoteSources builds the remote POM sources implied by the settings, honoring
// Maven mirror semantics. Each declared repository is routed through a matching
// mirror (using the mirror's URL and the mirror id's credentials) or used
// directly with its own server credentials. Mirrors are deduplicated, so a
// catch-all mirror (mirrorOf=*) collapses all repositories to one source. A
// catch-all mirror with no declared repositories still yields a source (it is
// the effective repository). The client (nil = default) is shared.
func (s *Settings) RemoteSources(client httpDoer, cache blobcache.Cache) []PomSource {
	if s == nil {
		return nil
	}

	var sources []PomSource
	seen := make(map[string]bool)
	add := func(url, serverID string) {
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		opts := RemoteOptions{BaseURL: url, Client: client, Cache: cache}
		if srv, ok := s.Servers[serverID]; ok {
			opts.Username = srv.Username
			opts.Password = srv.Password
		}
		sources = append(sources, NewRemoteSource(opts))
	}

	for _, repo := range s.Repositories {
		if m := s.mirrorFor(repo.ID); m != nil {
			add(m.URL, m.ID)
		} else {
			add(repo.URL, repo.ID)
		}
	}

	// A catch-all mirror with no (matched) declared repositories is still the
	// effective source -- include it so settings with only a mirror work.
	for i := range s.Mirrors {
		if mirrorMatches(s.Mirrors[i].MirrorOf, "central") || s.Mirrors[i].MirrorOf == "*" {
			add(s.Mirrors[i].URL, s.Mirrors[i].ID)
		}
	}
	return sources
}
