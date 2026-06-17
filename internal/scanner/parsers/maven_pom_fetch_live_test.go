//go:build online

package parsers

import (
	"strings"
	"testing"
)

// TestMavenPomFetcher_Live fetches a real, public BOM from Maven Central to
// validate the URL shape and response handling end to end. Opt-in (build tag
// "online"); excluded from the default test suite. Run with:
//
//	go test -tags online -run Live ./internal/scanner/parsers/
func TestMavenPomFetcher_Live(t *testing.T) {
	f := NewMavenPomFetcher("", nil)

	// A long-published, stable BOM that will not disappear.
	content, _, ok := f.FetchPOM("org.springframework.boot", "spring-boot-dependencies", "3.2.0")
	if !ok {
		t.Fatal("expected to fetch spring-boot-dependencies BOM from Maven Central")
	}
	if !strings.Contains(string(content), "<dependencyManagement>") {
		t.Error("fetched POM should contain a dependencyManagement section")
	}

	// A coordinate that does not exist must report not-found, not error.
	if _, _, ok := f.FetchPOM("com.example.nonexistent", "no-such-bom", "9.9.9"); ok {
		t.Error("expected not-found for a nonexistent coordinate")
	}
}
