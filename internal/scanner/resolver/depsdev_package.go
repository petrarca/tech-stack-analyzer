package resolver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PackageInfo is the deps.dev GetPackage result distilled to what a currency
// check needs: the latest stable version and its publish/deprecation status.
type PackageInfo struct {
	LatestVersion string // the "default" version (greatest, ignoring pre-releases)
	IsDeprecated  bool   // deprecation flag of the default version
	PublishedAt   string // publish time of the default version, if known (RFC3339)
}

// depsDevPackageSystems maps a dependency tuple's ecosystem label (as it appears
// in the aggregate, e.g. "npm", "maven", "nuget") to a deps.dev "system"
// identifier for the GetPackage endpoint. Unlike the graph resolver's map (keyed
// by component type and limited to systems with resolved-graph data), GetPackage
// is available for all seven deps.dev systems -- so currency covers a broader
// set. Ecosystems absent from this map have no deps.dev coverage and are treated
// by callers as unsupported.
var depsDevPackageSystems = map[string]string{
	"npm":    "npm",
	"maven":  "maven",
	"gradle": "maven", // Gradle artifacts use Maven coordinates
	"pypi":   "pypi",
	"nuget":  "nuget",
	"cargo":  "cargo",
	"golang": "go",
	"gem":    "rubygems",
}

// DepsDevPackageSystem maps a dependency-tuple ecosystem label to a deps.dev
// GetPackage system identifier. ok is false when the ecosystem has no deps.dev
// coverage (e.g. delphi, native-lib, docker, conan, dotnet-ref). Callers record
// such dependencies as unsupported rather than querying.
func DepsDevPackageSystem(ecosystem string) (system string, ok bool) {
	system, ok = depsDevPackageSystems[strings.ToLower(ecosystem)]
	return system, ok
}

// PackageClient calls the deps.dev GetPackage endpoint. It reuses the same
// transport as the graph resolver (endpoint composition, Accept header, 404/429
// handling) via the shared getJSON helper -- it is a second caller of that
// transport, not a second HTTP client.
type PackageClient struct {
	c *depsDevClient
}

// NewPackageClient builds a PackageClient. A nil client uses a default
// http.Client with a sane timeout; an empty baseURL uses the public deps.dev
// API. baseURL accepts the same --deps-dev-endpoint override as the graph
// resolver.
func NewPackageClient(baseURL string, client HTTPDoer) *PackageClient {
	if baseURL == "" {
		baseURL = DefaultDepsDevBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &PackageClient{c: &depsDevClient{baseURL: baseURL, http: client}}
}

// depsDevPackage is the subset of the GetPackage response we read.
type depsDevPackage struct {
	Versions []struct {
		VersionKey struct {
			Version string `json:"version"`
		} `json:"versionKey"`
		IsDefault    bool   `json:"isDefault"`
		IsDeprecated bool   `json:"isDeprecated"`
		PublishedAt  string `json:"publishedAt"`
	} `json:"versions"`
}

// GetPackage returns the latest-version info for a package, or
// ErrCoordinateNotFound when deps.dev does not know it (HTTP 404). system is a
// deps.dev system identifier (npm, maven, pypi, nuget, cargo, go, rubygems);
// name is the package name (Maven uses "group:artifact").
func (p *PackageClient) GetPackage(system, name string) (PackageInfo, error) {
	path := fmt.Sprintf("/v3/systems/%s/packages/%s",
		url.PathEscape(strings.ToLower(system)), url.PathEscape(name))
	label := system + "/" + name

	body, notFound, err := p.c.getJSON(path, label)
	if err != nil {
		return PackageInfo{}, err
	}
	if notFound {
		return PackageInfo{}, ErrCoordinateNotFound
	}

	var pkg depsDevPackage
	if err := json.Unmarshal(body, &pkg); err != nil {
		return PackageInfo{}, fmt.Errorf("deps.dev decode error: %w", err)
	}
	for _, v := range pkg.Versions {
		if v.IsDefault {
			return PackageInfo{
				LatestVersion: v.VersionKey.Version,
				IsDeprecated:  v.IsDeprecated,
				PublishedAt:   v.PublishedAt,
			}, nil
		}
	}
	// No default marked: deps.dev knows the package but has no usable latest.
	return PackageInfo{}, ErrCoordinateNotFound
}
