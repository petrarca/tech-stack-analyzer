package components

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// stubIndexer indexes files named "marker.txt", using their content as the
// coordinate, under a fixed ecosystem.
type stubIndexer struct{ eco string }

func (s stubIndexer) Ecosystem() string            { return s.eco }
func (s stubIndexer) Matches(fileName string) bool { return fileName == "marker.txt" }
func (s stubIndexer) Coordinate(c []byte) string   { return string(c) }

// treeProvider is a minimal in-memory provider exposing a fixed directory tree.
type treeProvider struct {
	base  string
	dirs  map[string][]types.File
	files map[string]string
}

func (t *treeProvider) ListDir(path string) ([]types.File, error) { return t.dirs[path], nil }
func (t *treeProvider) Open(path string) (string, error)          { return t.files[path], nil }
func (t *treeProvider) Exists(path string) (bool, error)          { _, ok := t.files[path]; return ok, nil }
func (t *treeProvider) IsDir(path string) (bool, error)           { _, ok := t.dirs[path]; return ok, nil }
func (t *treeProvider) ReadFile(path string) ([]byte, error)      { return []byte(t.files[path]), nil }
func (t *treeProvider) GetBasePath() string                       { return t.base }

func TestSourceIndex_BuildAndLookup(t *testing.T) {
	resetSourceIndexCacheForTest()
	sourceIndexersMu.Lock()
	saved := sourceIndexers
	sourceIndexers = []SourceIndexer{stubIndexer{eco: "test"}}
	sourceIndexersMu.Unlock()
	defer func() {
		sourceIndexersMu.Lock()
		sourceIndexers = saved
		sourceIndexersMu.Unlock()
		resetSourceIndexCacheForTest()
	}()

	provider := &treeProvider{
		base: "/repo",
		dirs: map[string][]types.File{
			".": {
				{Name: "marker.txt", Type: "file"},
				{Name: "sub", Type: "dir"},
			},
			"sub": {
				{Name: "marker.txt", Type: "file"},
			},
		},
		files: map[string]string{
			"marker.txt":     "com.example:root",
			"sub/marker.txt": "com.example:child",
		},
	}

	idx := GetSourceIndex(provider)
	if got := idx.Lookup("test", "com.example:root"); len(got) != 1 || got[0] != "marker.txt" {
		t.Errorf("root lookup = %v, want [marker.txt]", got)
	}
	if got := idx.Lookup("test", "com.example:child"); len(got) != 1 || got[0] != "sub/marker.txt" {
		t.Errorf("child lookup = %v, want [sub/marker.txt]", got)
	}
	if got := idx.Lookup("test", "missing"); got != nil {
		t.Errorf("missing lookup = %v, want nil", got)
	}
	if got := idx.Lookup("other", "com.example:root"); got != nil {
		t.Errorf("unknown ecosystem lookup = %v, want nil", got)
	}
}

func TestSourceIndex_NilSafe(t *testing.T) {
	var idx *SourceIndex
	if got := idx.Lookup("test", "x"); got != nil {
		t.Errorf("nil index lookup = %v, want nil", got)
	}
}
