package mavenresolve

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleSettings = `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0">
  <localRepository>/custom/repo</localRepository>
  <servers>
    <server>
      <id>releases</id>
      <username>builder</username>
      <password>token-secret</password>
    </server>
  </servers>
  <profiles>
    <profile>
      <id>artifactory</id>
      <repositories>
        <repository>
          <id>releases</id>
          <url>https://repo.example.com/artifactory/releases.virtual</url>
        </repository>
        <repository>
          <id>snapshots</id>
          <url>https://repo.example.com/artifactory/snapshots.virtual</url>
        </repository>
      </repositories>
    </profile>
  </profiles>
</settings>`

func TestParseSettings(t *testing.T) {
	s, err := parseSettings([]byte(sampleSettings))
	if err != nil {
		t.Fatal(err)
	}
	if s.LocalRepository != "/custom/repo" {
		t.Errorf("localRepository = %q", s.LocalRepository)
	}
	if len(s.Repositories) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(s.Repositories))
	}
	if s.Repositories[0].ID != "releases" || s.Repositories[0].URL != "https://repo.example.com/artifactory/releases.virtual" {
		t.Errorf("unexpected repo[0]: %+v", s.Repositories[0])
	}
	srv, ok := s.Servers["releases"]
	if !ok || srv.Username != "builder" || srv.Password != "token-secret" {
		t.Errorf("unexpected server creds: %+v ok=%v", srv, ok)
	}
}

func TestSettings_RemoteSources_AttachCreds(t *testing.T) {
	s, err := parseSettings([]byte(sampleSettings))
	if err != nil {
		t.Fatal(err)
	}
	sources := s.RemoteSources(nil)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	// The "releases" repo source must carry the matching Basic creds.
	rs, ok := sources[0].(*RemoteSource)
	if !ok {
		t.Fatal("expected *RemoteSource")
	}
	if rs.username != "builder" || rs.password != "token-secret" {
		t.Errorf("creds not attached: user=%q", rs.username)
	}
	// The "snapshots" repo has no matching server -> anonymous.
	rs2 := sources[1].(*RemoteSource)
	if rs2.username != "" || rs2.password != "" {
		t.Error("snapshots repo should be anonymous (no matching server)")
	}
}

func TestLoadSettings_AbsentFileIsNil(t *testing.T) {
	s, err := LoadSettings(filepath.Join(t.TempDir(), "nope.xml"))
	if err != nil || s != nil {
		t.Errorf("absent file should yield (nil, nil), got (%v, %v)", s, err)
	}
	if s, err := LoadSettings(""); err != nil || s != nil {
		t.Errorf("empty path should yield (nil, nil), got (%v, %v)", s, err)
	}
}

func TestLoadSettings_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.xml")
	if err := os.WriteFile(path, []byte(sampleSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings(path)
	if err != nil || s == nil {
		t.Fatalf("expected settings, got (%v, %v)", s, err)
	}
	if len(s.Repositories) != 2 {
		t.Errorf("expected 2 repositories, got %d", len(s.Repositories))
	}
}

const mirrorSettings = `<?xml version="1.0"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0">
  <servers>
    <server><id>central</id><username>u1</username><password>p1</password></server>
    <server><id>mirror.virtual</id><username>mu</username><password>mp</password></server>
  </servers>
  <mirrors>
    <mirror>
      <id>mirror.virtual</id>
      <mirrorOf>*</mirrorOf>
      <url>https://repo.example.com/artifactory/mirror.virtual</url>
    </mirror>
  </mirrors>
  <profiles>
    <profile>
      <id>artifactory</id>
      <repositories>
        <repository><id>central</id><url>https://repo.example.com/artifactory/releases</url></repository>
        <repository><id>snapshots</id><url>https://repo.example.com/artifactory/snapshots</url></repository>
      </repositories>
    </profile>
  </profiles>
</settings>`

func TestSettings_MirrorOfStarCollapsesToMirror(t *testing.T) {
	s, err := parseSettings([]byte(mirrorSettings))
	if err != nil {
		t.Fatal(err)
	}
	sources := s.RemoteSources(nil)
	// mirrorOf=* routes both repos to the single mirror -> one deduplicated source.
	if len(sources) != 1 {
		t.Fatalf("expected 1 mirror source, got %d", len(sources))
	}
	rs := sources[0].(*RemoteSource)
	if rs.baseURL != "https://repo.example.com/artifactory/mirror.virtual" {
		t.Errorf("expected mirror URL, got %q", rs.baseURL)
	}
	// Credentials come from the mirror id's <server>, not the original repo's.
	if rs.username != "mu" || rs.password != "mp" {
		t.Errorf("expected mirror creds mu/mp, got %q", rs.username)
	}
}

func TestMirrorMatches(t *testing.T) {
	cases := []struct {
		mirrorOf, repoID string
		want             bool
	}{
		{"*", "central", true},
		{"external:*", "central", true},
		{"central", "central", true},
		{"central", "snapshots", false},
		{"*,!central", "central", false}, // exclusion wins
		{"*,!central", "snapshots", true},
		{"a,b,c", "b", true},
		{"", "central", false},
	}
	for _, c := range cases {
		if got := mirrorMatches(c.mirrorOf, c.repoID); got != c.want {
			t.Errorf("mirrorMatches(%q,%q) = %v, want %v", c.mirrorOf, c.repoID, got, c.want)
		}
	}
}

// RemoteSources on a nil *Settings must be safe.
func TestSettings_RemoteSources_Nil(t *testing.T) {
	var s *Settings
	if got := s.RemoteSources(nil); got != nil {
		t.Errorf("nil settings should yield nil sources, got %v", got)
	}
}
