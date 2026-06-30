package license

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNpmHarvester(t *testing.T) {
	root := t.TempDir()
	// Modern string form.
	writeFile(t, filepath.Join(root, "mylib", "package.json"),
		`{"name":"mylib","version":"1.0.0","license":"Apache License, Version 2.0"}`)
	// Scoped package, legacy array form.
	writeFile(t, filepath.Join(root, "@myorg", "tool", "package.json"),
		`{"name":"@myorg/tool","licenses":[{"type":"MIT"}]}`)
	// Legacy object form.
	writeFile(t, filepath.Join(root, "oldlib", "package.json"),
		`{"name":"oldlib","license":{"type":"ISC"}}`)

	h := NewNpmHarvester(HarvestRoots{InTree: []string{root}})
	if h.Ecosystem() != "npm" {
		t.Fatalf("ecosystem = %q", h.Ecosystem())
	}
	cases := map[string]string{
		"mylib":       "Apache-2.0", // normalized from the declared string
		"@myorg/tool": "MIT",
		"oldlib":      "ISC",
		"missing":     "",
	}
	for name, want := range cases {
		if got := h.License(name, "1.0.0"); got != want {
			t.Errorf("License(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestNugetHarvester(t *testing.T) {
	root := t.TempDir()
	// Global-packages layout: <id>/<version>/<id>.nuspec, all lower-cased.
	nuspec := `<?xml version="1.0"?>
<package><metadata>
  <id>MyLib</id>
  <version>2.1.0</version>
  <license type="expression">MIT</license>
</metadata></package>`
	writeFile(t, filepath.Join(root, "mylib", "2.1.0", "mylib.nuspec"), nuspec)

	// A package whose license is a non-expression type must not resolve.
	fileType := `<?xml version="1.0"?>
<package><metadata>
  <id>OtherLib</id>
  <license type="file">LICENSE.txt</license>
</metadata></package>`
	writeFile(t, filepath.Join(root, "otherlib", "1.0.0", "otherlib.nuspec"), fileType)

	h := NewNugetHarvester(HarvestRoots{CacheRoots: []string{root}})
	if got := h.License("MyLib", "2.1.0"); got != "MIT" {
		t.Errorf("License(MyLib,2.1.0) = %q, want MIT (case-insensitive lookup)", got)
	}
	if got := h.License("OtherLib", "1.0.0"); got != "" {
		t.Errorf("file-type license should not resolve, got %q", got)
	}
	if got := h.License("MyLib", ""); got != "" {
		t.Errorf("missing version should not resolve, got %q", got)
	}
}

func TestHarvestRoots_AllRoots_FiltersMissing(t *testing.T) {
	existing := t.TempDir()
	roots := HarvestRoots{InTree: []string{existing, "/no/such/dir", ""}}
	got := roots.allRoots()
	if len(got) != 1 || got[0] != existing {
		t.Errorf("allRoots() = %v, want [%q]", got, existing)
	}
}
