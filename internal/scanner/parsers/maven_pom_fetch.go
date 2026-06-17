package parsers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultMavenCentralBaseURL is the public Maven Central repository base. A POM
// is addressed at {base}/{group/path}/{artifact}/{version}/{artifact}-{version}.pom.
// It can be overridden to point at a mirror or an API-compatible proxy.
const DefaultMavenCentralBaseURL = "https://repo1.maven.org/maven2"

// pomHTTPDoer is the minimal HTTP client interface (satisfied by *http.Client),
// injectable for testing.
type pomHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// MavenPomFetcher fetches BOM POMs from a Maven repository over HTTP. It exists
// to resolve scope=import BOMs that are NOT present in the scanned tree (e.g.
// third-party BOMs like the Quarkus or Spring BOM): fetching the published POM
// yields the EXACT managed versions the build uses, unlike guessing the latest
// release. Only public coordinates resolve; private artifacts return not-found.
//
// Network access is opt-in: the fetcher is only wired when online resolution is
// enabled, preserving the offline-by-default guarantee. Responses are cached
// per coordinate for the run (published POMs are immutable).
type MavenPomFetcher struct {
	baseURL string
	http    pomHTTPDoer

	mu       sync.Mutex
	cache    map[string][]byte
	notFound map[string]bool
}

// NewMavenPomFetcher builds a fetcher targeting baseURL (empty uses Maven
// Central). A nil client uses a default http.Client with a sane timeout.
func NewMavenPomFetcher(baseURL string, client pomHTTPDoer) *MavenPomFetcher {
	if baseURL == "" {
		baseURL = DefaultMavenCentralBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &MavenPomFetcher{
		baseURL:  baseURL,
		http:     client,
		cache:    make(map[string][]byte),
		notFound: make(map[string]bool),
	}
}

// FetchPOM returns the raw POM for the given coordinates, or ok=false when the
// coordinate is unknown to the repository (404) or cannot be fetched. A missing
// or non-concrete version yields ok=false (a repository path needs a concrete
// version). The signature matches what a BomResolver needs.
func (f *MavenPomFetcher) FetchPOM(groupID, artifactID, version string) (content []byte, dir string, ok bool) {
	if groupID == "" || artifactID == "" || version == "" {
		return nil, "", false
	}
	key := groupID + ":" + artifactID + ":" + version

	f.mu.Lock()
	if f.notFound[key] {
		f.mu.Unlock()
		return nil, "", false
	}
	if cached, hit := f.cache[key]; hit {
		f.mu.Unlock()
		return cached, "", true
	}
	f.mu.Unlock()

	body, definitive := f.request(groupID, artifactID, version)

	f.mu.Lock()
	defer f.mu.Unlock()
	if body == nil {
		// Cache only definitive misses (404): the coordinate is genuinely not
		// on the repository. Transient failures (429 rate-limit, 5xx, network
		// errors) are not cached, so another module's lookup may still succeed.
		// Unlike Trivy, a rate-limit never aborts the scan -- the dependency
		// is simply left versionless.
		if definitive {
			f.notFound[key] = true
		}
		return nil, "", false
	}
	f.cache[key] = body
	return body, "", true
}

// request performs the HTTP GET for a published POM. Returns (body, definitive)
// where body!=nil means success; body==nil with definitive=true means a 404
// (genuine miss to cache); definitive=false means a transient failure (429,
// 5xx, network) that should not be cached.
func (f *MavenPomFetcher) request(groupID, artifactID, version string) (body []byte, definitive bool) {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	endpoint := fmt.Sprintf("%s/%s/%s/%s/%s-%s.pom",
		f.baseURL, groupPath, artifactID, version, artifactID, version)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, true // malformed request: will never succeed
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, false // network error: transient
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxPomFetchBytes))
		if err != nil {
			return nil, false
		}
		return data, true
	case http.StatusNotFound, http.StatusGone:
		return nil, true // genuine miss
	default:
		return nil, false // 429 / 5xx / other: transient, do not cache
	}
}

// maxPomFetchBytes caps a fetched POM size to guard against pathological
// responses (a POM is normally a few KB).
const maxPomFetchBytes = 4 << 20 // 4 MiB
