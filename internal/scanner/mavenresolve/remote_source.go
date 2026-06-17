package mavenresolve

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/blobcache"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
)

// DefaultMavenCentralBaseURL is the public Maven Central repository base. A POM
// is addressed at {base}/{group/path}/{artifact}/{version}/{artifact}-{version}.pom.
const DefaultMavenCentralBaseURL = "https://repo1.maven.org/maven2"

// maxPomFetchBytes caps a fetched POM size (a POM is normally a few KB).
const maxPomFetchBytes = 4 << 20 // 4 MiB

// userAgent identifies the client to Maven repositories. Maven Central serves a
// 200 "abusive tool" notice (not the POM) to requests with a missing or default
// User-Agent, so a descriptive one is required.
const userAgent = "tech-stack-analyzer (+https://github.com/petrarca/tech-stack-analyzer)"

// httpDoer is the minimal HTTP client interface (satisfied by *http.Client),
// injectable for testing.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RemoteSource fetches POMs from a Maven repository over HTTP -- Maven Central
// by default, or a configured mirror/proxy (e.g. an internal JFrog Artifactory
// virtual repo that also serves private artifacts). It resolves scope=import
// BOMs and parent POMs not present in the scanned tree or the local cache.
//
// Network access is opt-in (only wired when online resolution is enabled).
// Optional bearer-token auth supports private repositories. Responses are
// cached per coordinate for the run (published POMs are immutable). A 429/5xx
// is treated as transient (not cached, never aborts the scan) -- unlike a
// recursive package-manager crawl that fails hard on rate-limiting.
type RemoteSource struct {
	baseURL  string
	token    string
	username string
	password string
	http     httpDoer
	cache    blobcache.Cache
}

// RemoteOptions configures a RemoteSource. Auth precedence: Username/Password
// (HTTP Basic, as Maven settings.xml uses) takes effect when set; otherwise
// Token (HTTP Bearer) is used when set; otherwise the request is anonymous.
type RemoteOptions struct {
	// BaseURL is the repository base ("" uses Maven Central).
	BaseURL string
	// Token, when non-empty, is sent as "Authorization: Bearer" (used when no
	// Username/Password is given).
	Token string
	// Username/Password, when set, are sent as HTTP Basic auth. JFrog reference
	// tokens are supplied as the password here, matching settings.xml.
	Username string
	Password string
	// Client overrides the HTTP client (nil uses a default with a timeout).
	Client httpDoer
	// Cache, when set, is shared across the scan so a POM is fetched at most
	// once regardless of how many components/modules need it. Nil uses a
	// private in-memory cache (per source).
	Cache blobcache.Cache
}

// NewRemoteSource builds a RemoteSource from options.
func NewRemoteSource(opts RemoteOptions) *RemoteSource {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultMavenCentralBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	cache := opts.Cache
	if cache == nil {
		cache = blobcache.NewMemory()
	}
	return &RemoteSource{
		baseURL:  baseURL,
		token:    opts.Token,
		username: opts.Username,
		password: opts.Password,
		http:     client,
		cache:    cache,
	}
}

// Name implements PomSource.
func (s *RemoteSource) Name() string { return "remote(" + s.baseURL + ")" }

// FetchPOM implements PomSource against the remote repository.
func (s *RemoteSource) FetchPOM(groupID, artifactID, version string) ([]byte, string, bool) {
	if groupID == "" || artifactID == "" || version == "" {
		return nil, "", false
	}
	// Key by repository base so distinct repos do not share entries.
	key := "maven|" + s.baseURL + "|" + groupID + ":" + artifactID + ":" + version

	if blob, found, notFound := s.cache.Get(key); found {
		resolvestats.AddCacheHit()
		return blob, "", true
	} else if notFound {
		resolvestats.AddCacheHit()
		return nil, "", false
	}

	resolvestats.AddPOMFetched()
	body, definitive := s.request(groupID, artifactID, version)

	if body == nil {
		// Cache only definitive misses (404). Transient failures (429/5xx/
		// network) are not cached so a later lookup may still succeed, and a
		// rate-limit never aborts the scan.
		if definitive {
			s.cache.PutNotFound(key)
		}
		return nil, "", false
	}
	s.cache.Put(key, body)
	return body, "", true
}

// request performs the HTTP GET. Returns (body, definitive): body!=nil is
// success; body==nil with definitive=true is a genuine 404; definitive=false is
// a transient failure not to be cached.
func (s *RemoteSource) request(groupID, artifactID, version string) (body []byte, definitive bool) {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	endpoint := fmt.Sprintf("%s/%s/%s/%s/%s-%s.pom",
		s.baseURL, groupPath, artifactID, version, artifactID, version)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, true
	}
	// Maven Central rejects requests with no/default User-Agent (it returns a
	// 200 "abusive tool" notice instead of the POM), so identify the client.
	req.Header.Set("User-Agent", userAgent)
	switch {
	case s.username != "" || s.password != "":
		req.SetBasicAuth(s.username, s.password)
	case s.token != "":
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, false
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
		return nil, true
	default:
		return nil, false // 401/403/429/5xx: transient or auth -- do not cache as a genuine miss
	}
}
