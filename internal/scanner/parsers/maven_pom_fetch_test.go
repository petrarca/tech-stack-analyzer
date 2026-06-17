package parsers

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubDoer returns a fixed response per requested URL.
type stubDoer struct {
	responses map[string]int    // url -> status
	bodies    map[string]string // url -> body
	calls     int
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.calls++
	url := req.URL.String()
	status, ok := s.responses[url]
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(s.bodies[url])),
	}, nil
}

func TestMavenPomFetcher_FetchPOM(t *testing.T) {
	const url = "https://repo1.maven.org/maven2/io/quarkus/quarkus-bom/3.6.0/quarkus-bom-3.6.0.pom"
	doer := &stubDoer{
		responses: map[string]int{url: http.StatusOK},
		bodies:    map[string]string{url: "<project><artifactId>quarkus-bom</artifactId></project>"},
	}
	f := NewMavenPomFetcher("", doer)

	content, _, ok := f.FetchPOM("io.quarkus", "quarkus-bom", "3.6.0")
	if !ok {
		t.Fatal("expected ok=true for a present POM")
	}
	if !strings.Contains(string(content), "quarkus-bom") {
		t.Errorf("unexpected content: %s", content)
	}

	// Second call is served from cache (no extra HTTP request).
	if _, _, ok := f.FetchPOM("io.quarkus", "quarkus-bom", "3.6.0"); !ok {
		t.Fatal("cached fetch should still be ok")
	}
	if doer.calls != 1 {
		t.Errorf("expected 1 HTTP call (then cache), got %d", doer.calls)
	}
}

func TestMavenPomFetcher_NotFound(t *testing.T) {
	doer := &stubDoer{responses: map[string]int{}} // everything 404
	f := NewMavenPomFetcher("", doer)

	if _, _, ok := f.FetchPOM("com.private", "internal-bom", "1.0.0"); ok {
		t.Error("expected ok=false for a missing POM")
	}
	// 404 is cached as not-found; no second HTTP call.
	_, _, _ = f.FetchPOM("com.private", "internal-bom", "1.0.0")
	if doer.calls != 1 {
		t.Errorf("expected 1 HTTP call (then not-found cache), got %d", doer.calls)
	}
}

func TestMavenPomFetcher_RequiresConcreteVersion(t *testing.T) {
	doer := &stubDoer{}
	f := NewMavenPomFetcher("", doer)

	for _, tc := range []struct{ g, a, v string }{
		{"", "bom", "1.0"},
		{"io.quarkus", "", "1.0"},
		{"io.quarkus", "quarkus-bom", ""},
	} {
		if _, _, ok := f.FetchPOM(tc.g, tc.a, tc.v); ok {
			t.Errorf("FetchPOM(%q,%q,%q) = ok, want not ok", tc.g, tc.a, tc.v)
		}
	}
	if doer.calls != 0 {
		t.Errorf("expected no HTTP calls for invalid coordinates, got %d", doer.calls)
	}
}
