package mavenresolve

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// Settings is the subset of a Maven settings.xml this package needs: the local
// repository path, the configured repositories (with their server id), and the
// server credentials keyed by id. It lets the scanner reuse a developer/CI
// machine's existing Maven configuration -- repository URLs and credentials --
// without re-specifying them, exactly as Maven and Trivy do.
type Settings struct {
	LocalRepository string
	Repositories    []SettingsRepository
	Servers         map[string]SettingsServer // by id
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

// RemoteSources builds a RemoteSource for each repository in the settings,
// attaching the matching server's Basic-auth credentials by repository id.
// Repositories without a URL are skipped. The client (nil = default) is shared.
func (s *Settings) RemoteSources(client httpDoer) []PomSource {
	if s == nil {
		return nil
	}
	var sources []PomSource
	for _, repo := range s.Repositories {
		opts := RemoteOptions{BaseURL: repo.URL, Client: client}
		if srv, ok := s.Servers[repo.ID]; ok {
			opts.Username = srv.Username
			opts.Password = srv.Password
		}
		sources = append(sources, NewRemoteSource(opts))
	}
	return sources
}
