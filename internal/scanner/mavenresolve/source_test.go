package mavenresolve

import (
	"os"
	"path/filepath"
	"testing"
)

// fixedSource is a PomSource that returns a fixed result for one coordinate.
type fixedSource struct {
	name      string
	wantCoord string
	body      string
}

func (f fixedSource) Name() string { return f.name }
func (f fixedSource) FetchPOM(g, a, _ string) ([]byte, string, bool) {
	if g+":"+a == f.wantCoord {
		return []byte(f.body), "", true
	}
	return nil, "", false
}

func TestChain_FirstHitWins(t *testing.T) {
	c := NewChain(
		nil, // skipped
		fixedSource{name: "a", wantCoord: "g:miss", body: "A"},
		fixedSource{name: "b", wantCoord: "g:hit", body: "B"},
		fixedSource{name: "c", wantCoord: "g:hit", body: "C"},
	)
	content, _, ok := c.FetchPOM("g", "hit", "1.0")
	if !ok || string(content) != "B" {
		t.Errorf("expected first matching source B, got ok=%v content=%q", ok, content)
	}
	if _, _, ok := c.FetchPOM("g", "none", "1.0"); ok {
		t.Error("expected miss for unknown coordinate")
	}
}

func TestChain_Empty(t *testing.T) {
	if !NewChain().Empty() {
		t.Error("empty chain should report Empty")
	}
	if !NewChain(nil, nil).Empty() {
		t.Error("chain of only nils should report Empty")
	}
	if NewChain(fixedSource{}).Empty() {
		t.Error("non-empty chain should not report Empty")
	}
	var nilChain *Chain
	if !nilChain.Empty() {
		t.Error("nil chain should report Empty")
	}
}

func TestLocalRepoSource(t *testing.T) {
	dir := t.TempDir()
	// Lay out a POM at the Maven coordinate path.
	pomDir := filepath.Join(dir, "io", "quarkus", "quarkus-bom", "3.6.0")
	if err := os.MkdirAll(pomDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pom := filepath.Join(pomDir, "quarkus-bom-3.6.0.pom")
	if err := os.WriteFile(pom, []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLocalRepoSource(dir)
	if s == nil {
		t.Fatal("expected a source for an existing dir")
	}
	if _, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", "3.6.0"); !ok {
		t.Error("expected to read the local POM")
	}
	if _, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", "9.9.9"); ok {
		t.Error("missing version should not resolve")
	}

	if NewLocalRepoSource(filepath.Join(dir, "does-not-exist")) != nil {
		t.Error("nonexistent dir should yield nil source")
	}
}

func TestRepoSource(t *testing.T) {
	lookup := func(coord string) []string {
		if coord == "com.example:bom" {
			return []string{"sub/bom/pom.xml"}
		}
		return nil
	}
	reader := readerFunc(func(path string) ([]byte, error) {
		if path == "sub/bom/pom.xml" {
			return []byte("<project/>"), nil
		}
		return nil, os.ErrNotExist
	})
	s := NewRepoSource(lookup, reader)
	content, dir, ok := s.FetchPOM("com.example", "bom", "ignored")
	if !ok || string(content) != "<project/>" || dir != "sub/bom" {
		t.Errorf("unexpected result ok=%v content=%q dir=%q", ok, content, dir)
	}
	if _, _, ok := s.FetchPOM("com.example", "missing", ""); ok {
		t.Error("unknown coordinate should not resolve")
	}
	if NewRepoSource(nil, reader) != nil || NewRepoSource(lookup, nil) != nil {
		t.Error("nil dependencies should yield nil source")
	}
}

type readerFunc func(string) ([]byte, error)

func (f readerFunc) ReadFile(path string) ([]byte, error) { return f(path) }

func TestMavenRepoLocalFromOpts(t *testing.T) {
	cases := map[string]string{
		"-Dmaven.repo.local=/tmp/repo":             "/tmp/repo",
		"-Xmx1g -Dmaven.repo.local=/a/b -Dother=1": "/a/b",
		`-Dmaven.repo.local="/quoted/path"`:        "/quoted/path",
		"-Xmx1g":                                   "",
		"":                                         "",
	}
	for opts, want := range cases {
		if got := mavenRepoLocalFromOpts(opts); got != want {
			t.Errorf("mavenRepoLocalFromOpts(%q) = %q, want %q", opts, got, want)
		}
	}
}
