package license

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-enry/go-license-detector/v4/licensedb/filer"
)

func TestSafeFiler_SkipsBinaryContent(t *testing.T) {
	// A file with license-like name but binary content (null bytes signal binary).
	binaryContent := make([]byte, 1024)
	binaryContent[0] = 0x00
	binaryContent[1] = 0x01

	// Write to a temp dir so os.Stat works in safeFiler.
	dir := t.TempDir()
	binFile := "some-lib-mit.jar"
	if err := os.WriteFile(filepath.Join(dir, binFile), binaryContent, 0600); err != nil {
		t.Fatal(err)
	}

	inner, err := filer.FromDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	sf := newSafeFiler(inner, dir)
	defer sf.Close()

	_, readErr := sf.ReadFile(binFile)
	if readErr == nil {
		t.Error("expected binary file to be rejected, but ReadFile succeeded")
	}
}

func TestSafeFiler_AllowsPlainTextLicense(t *testing.T) {
	licenseText := []byte("MIT License\n\nCopyright (c) 2024\n\nPermission is hereby granted...")

	dir := t.TempDir()
	licFile := "LICENSE.txt"
	if err := os.WriteFile(filepath.Join(dir, licFile), licenseText, 0600); err != nil {
		t.Fatal(err)
	}

	inner, err := filer.FromDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	sf := newSafeFiler(inner, dir)
	defer sf.Close()

	content, err := sf.ReadFile(licFile)
	if err != nil {
		t.Errorf("expected plain text license to be allowed, got error: %v", err)
	}
	if string(content) != string(licenseText) {
		t.Error("content mismatch")
	}
}

func TestSafeFiler_SkipsOversizedFiles(t *testing.T) {
	// Create a plain text file larger than maxLicenseFileSize.
	oversized := make([]byte, maxLicenseFileSize+1)
	for i := range oversized {
		oversized[i] = 'a' // all ASCII -- not binary
	}

	dir := t.TempDir()
	bigFile := "LICENSE"
	if err := os.WriteFile(filepath.Join(dir, bigFile), oversized, 0600); err != nil {
		t.Fatal(err)
	}

	inner, err := filer.FromDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	sf := newSafeFiler(inner, dir)
	defer sf.Close()

	_, readErr := sf.ReadFile(bigFile)
	if readErr == nil {
		t.Error("expected oversized file to be rejected, but ReadFile succeeded")
	}
}

func TestSafeFiler_ReadDir_FiltersOversized(t *testing.T) {
	dir := t.TempDir()

	// One normal file, one oversized file.
	if err := os.WriteFile(filepath.Join(dir, "LICENSE.txt"), []byte("MIT"), 0600); err != nil {
		t.Fatal(err)
	}
	oversized := make([]byte, maxLicenseFileSize+1)
	if err := os.WriteFile(filepath.Join(dir, "COPYING"), oversized, 0600); err != nil {
		t.Fatal(err)
	}

	inner, err := filer.FromDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	sf := newSafeFiler(inner, dir)
	defer sf.Close()

	entries, err := sf.ReadDir("")
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if e.Name == "COPYING" {
			t.Error("oversized file should have been filtered from ReadDir results")
		}
	}
}
