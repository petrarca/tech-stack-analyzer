//go:build online

package mavenresolve

import (
	"strings"
	"testing"
)

// TestRemoteSource_Live fetches a real, public BOM from Maven Central. Opt-in
// (build tag "online"); excluded from the default test suite. Run with:
//
//	go test -tags online -run Live ./internal/scanner/mavenresolve/
func TestRemoteSource_Live(t *testing.T) {
	s := NewRemoteSource(RemoteOptions{}) // Maven Central, no auth

	content, _, ok := s.FetchPOM("org.springframework.boot", "spring-boot-dependencies", "3.2.0")
	if !ok {
		t.Fatal("expected to fetch spring-boot-dependencies BOM from Maven Central")
	}
	if !strings.Contains(string(content), "<dependencyManagement>") {
		t.Error("fetched POM should contain a dependencyManagement section")
	}

	if _, _, ok := s.FetchPOM("com.example.nonexistent", "no-such-bom", "9.9.9"); ok {
		t.Error("expected not-found for a nonexistent coordinate")
	}
}

// TestRemoteSource_Live_RealBOMParses guards the User-Agent requirement: Maven
// Central returns a 200 "abusive tool" notice (not the POM) when the
// User-Agent is missing/default. A real BOM must fetch and contain its managed
// dependencies, not an error notice.
func TestRemoteSource_Live_RealBOMParses(t *testing.T) {
	s := NewRemoteSource(RemoteOptions{})
	content, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", "3.36.0")
	if !ok {
		t.Fatal("expected to fetch quarkus-bom")
	}
	if !strings.Contains(string(content), "<dependencyManagement>") ||
		!strings.Contains(string(content), "quarkus-arc") {
		t.Errorf("fetched content is not the real BOM (User-Agent block?): %.80s", content)
	}
}
