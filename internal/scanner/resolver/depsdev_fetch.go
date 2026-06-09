package resolver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DefaultDepsDevBaseURL is the public deps.dev v3 API base. It can be overridden
// (SetResolveOnlineEndpoint / config) to point at any API-compatible facade or
// mirror that exposes the same
// /v3/systems/{system}/packages/{name}/versions/{version}:dependencies shape.
const DefaultDepsDevBaseURL = "https://api.deps.dev"

// HTTPDoer is the minimal HTTP client interface (satisfied by *http.Client),
// injectable for testing.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// depsDevClient implements a DepsDevFetcher against a deps.dev-compatible API.
// It caches responses per (system, name, version) for the lifetime of the
// client (one scan run) so a coordinate is fetched at most once; pinned
// versions are immutable, so in-run caching is always safe.
type depsDevClient struct {
	baseURL string
	http    HTTPDoer

	mu    sync.Mutex
	cache map[string][]types.DependencyEdge
}

// NewDepsDevFetcher builds a DepsDevFetcher targeting baseURL. A nil client uses
// a default http.Client with a sane timeout; an empty baseURL uses the public
// deps.dev API.
func NewDepsDevFetcher(baseURL string, client HTTPDoer) DepsDevFetcher {
	if baseURL == "" {
		baseURL = DefaultDepsDevBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	c := &depsDevClient{
		baseURL: baseURL,
		http:    client,
		cache:   make(map[string][]types.DependencyEdge),
	}
	return c.fetch
}

// depsDevResponse is the resolved-graph response: a deduplicated DAG of nodes
// and integer-indexed edges (validated shape).
type depsDevResponse struct {
	Nodes []struct {
		VersionKey struct {
			System  string `json:"system"`
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"versionKey"`
		Relation string `json:"relation"` // SELF | DIRECT | INDIRECT
	} `json:"nodes"`
	Edges []struct {
		FromNode int `json:"fromNode"`
		ToNode   int `json:"toNode"`
	} `json:"edges"`
}

// fetch performs the resolved-graph request and maps it to our edge model,
// honoring the mode. It is the DepsDevFetcher.
func (c *depsDevClient) fetch(system, name, version string, mode types.DependencyGraphMode) ([]types.DependencyEdge, error) {
	key := system + "|" + name + "|" + version + "|" + string(mode)
	c.mu.Lock()
	if cached, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	resp, err := c.request(system, name, version)
	if err != nil {
		return nil, err
	}
	edges := mapDepsDevGraph(resp, mode)

	c.mu.Lock()
	c.cache[key] = edges
	c.mu.Unlock()
	return edges, nil
}

// request calls the resolved-graph endpoint. The endpoint uses the
// gRPC-transcoding ":dependencies" verb; the package-name segment is
// URL-encoded (Maven names contain a colon).
func (c *depsDevClient) request(system, name, version string) (depsDevResponse, error) {
	var out depsDevResponse
	endpoint := fmt.Sprintf("%s/v3/systems/%s/packages/%s/versions/%s:dependencies",
		c.baseURL, url.PathEscape(strings.ToLower(system)), url.PathEscape(name), url.PathEscape(version))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return out, fmt.Errorf("deps.dev request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	switch res.StatusCode {
	case http.StatusOK:
		// proceed
	case http.StatusNotFound:
		// Unknown coordinate: not an error, just no graph.
		return out, nil
	case http.StatusTooManyRequests:
		return out, fmt.Errorf("deps.dev rate limited (429) for %s/%s@%s", system, name, version)
	default:
		return out, fmt.Errorf("deps.dev returned %d for %s/%s@%s", res.StatusCode, system, name, version)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("deps.dev decode error: %w", err)
	}
	return out, nil
}

// mapDepsDevGraph projects the deps.dev DAG onto our {from,to} edges. The SELF
// node is the root; its edges become "." edges. direct mode keeps only the
// root's edges (to DIRECT nodes); full mode keeps all edges.
func mapDepsDevGraph(resp depsDevResponse, mode types.DependencyGraphMode) []types.DependencyEdge {
	if len(resp.Nodes) == 0 {
		return nil
	}
	nodeID := make([]string, len(resp.Nodes))
	selfIdx := -1
	for i, n := range resp.Nodes {
		nodeID[i] = n.VersionKey.Name + "@" + n.VersionKey.Version
		if n.Relation == "SELF" {
			selfIdx = i
		}
	}

	var edges []types.DependencyEdge
	for _, e := range resp.Edges {
		if e.FromNode < 0 || e.FromNode >= len(nodeID) || e.ToNode < 0 || e.ToNode >= len(nodeID) {
			continue
		}
		isRootEdge := e.FromNode == selfIdx
		if mode == types.DependencyGraphDirect && !isRootEdge {
			continue
		}
		from := nodeID[e.FromNode]
		if isRootEdge {
			from = "."
		}
		edges = append(edges, types.DependencyEdge{From: from, To: nodeID[e.ToNode]})
	}
	return edges
}
