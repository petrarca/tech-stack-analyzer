package mavenresolve

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubDoer returns a fixed response per requested URL and records calls.
type stubDoer struct {
	responses map[string]int
	bodies    map[string]string
	calls     int
	lastAuth  string
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.calls++
	s.lastAuth = req.Header.Get("Authorization")
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

func TestRemoteSource_FetchPOM(t *testing.T) {
	const url = "https://repo1.maven.org/maven2/io/quarkus/quarkus-bom/3.6.0/quarkus-bom-3.6.0.pom"
	doer := &stubDoer{
		responses: map[string]int{url: http.StatusOK},
		bodies:    map[string]string{url: "<project><artifactId>quarkus-bom</artifactId></project>"},
	}
	s := NewRemoteSource(RemoteOptions{Client: doer})

	content, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", "3.6.0")
	if !ok || !strings.Contains(string(content), "quarkus-bom") {
		t.Fatalf("expected POM, got ok=%v content=%q", ok, content)
	}
	// Cached: no second HTTP call.
	if _, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", "3.6.0"); !ok {
		t.Fatal("cached fetch should be ok")
	}
	if doer.calls != 1 {
		t.Errorf("expected 1 HTTP call, got %d", doer.calls)
	}
}

func TestRemoteSource_BearerToken(t *testing.T) {
	const url = "https://jfrog.example.com/artifactory/virtual/com/example/bom/1.0/bom-1.0.pom"
	doer := &stubDoer{
		responses: map[string]int{url: http.StatusOK},
		bodies:    map[string]string{url: "<project/>"},
	}
	s := NewRemoteSource(RemoteOptions{
		BaseURL: "https://jfrog.example.com/artifactory/virtual",
		Token:   "secret-token",
		Client:  doer,
	})
	if _, _, ok := s.FetchPOM("com.example", "bom", "1.0"); !ok {
		t.Fatal("expected ok")
	}
	if doer.lastAuth != "Bearer secret-token" {
		t.Errorf("expected bearer auth header, got %q", doer.lastAuth)
	}
}

func TestRemoteSource_TransientNotCached(t *testing.T) {
	const url = "https://repo1.maven.org/maven2/io/x/y/1/y-1.pom"
	// 429 -> transient: not cached, so a retry hits the network again.
	doer := &stubDoer{responses: map[string]int{url: http.StatusTooManyRequests}}
	s := NewRemoteSource(RemoteOptions{Client: doer})

	if _, _, ok := s.FetchPOM("io.x", "y", "1"); ok {
		t.Error("429 should yield not ok")
	}
	_, _, _ = s.FetchPOM("io.x", "y", "1")
	if doer.calls != 2 {
		t.Errorf("429 must not be cached; expected 2 calls, got %d", doer.calls)
	}
}

func TestRemoteSource_NotFoundCached(t *testing.T) {
	doer := &stubDoer{responses: map[string]int{}} // 404 for everything
	s := NewRemoteSource(RemoteOptions{Client: doer})
	if _, _, ok := s.FetchPOM("com.private", "bom", "1.0"); ok {
		t.Error("404 should yield not ok")
	}
	_, _, _ = s.FetchPOM("com.private", "bom", "1.0")
	if doer.calls != 1 {
		t.Errorf("404 should be cached; expected 1 call, got %d", doer.calls)
	}
}

func TestRemoteSource_RequiresConcreteVersion(t *testing.T) {
	doer := &stubDoer{}
	s := NewRemoteSource(RemoteOptions{Client: doer})
	if _, _, ok := s.FetchPOM("io.quarkus", "quarkus-bom", ""); ok {
		t.Error("empty version should yield not ok")
	}
	if doer.calls != 0 {
		t.Errorf("expected no HTTP calls, got %d", doer.calls)
	}
}
