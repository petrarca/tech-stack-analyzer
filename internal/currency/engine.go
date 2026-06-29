package currency

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolvestats"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// aggregateDoc is the minimal shape we read from an aggregate file: only the
// dependency list is needed for currency.
type aggregateDoc struct {
	Dependencies []types.Dependency `json:"dependencies"`
}

// Options configures an engine run.
type Options struct {
	SourceEndpoint string // --deps-dev-endpoint (empty = public deps.dev)
	TTLHours       int    // per-entry TTL (for the artifact header)
	DirectOnly     bool   // v1: true (resolve only direct dependencies)
	Concurrency    int    // parallel lookups; <=0 uses resolveConcurrency default
}

// ResolveAggregateFile reads an aggregate JSON file, resolves currency for its
// (direct) dependencies via resolver, and returns the artifact. resolver is
// normally the cache-decorated chain; the engine does not know or care.
func ResolveAggregateFile(path string, r CurrencyResolver, opt Options) (*Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("currency: read aggregate %s: %w", path, err)
	}
	var doc aggregateDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("currency: parse aggregate (expected a stack-analyzer -agg.json): %w", err)
	}
	return Resolve(doc.Dependencies, r, opt), nil
}

// resolveConcurrency bounds parallel deps.dev lookups. Lookups are independent,
// latency-dominated network calls, so a moderate fan-out cuts wall-clock time.
// Kept below the Maven graph crawl's 16 because deps.dev is a shared public
// service that returns 429 under heavy load; the cache absorbs repeats.
const resolveConcurrency = 10

// Resolve classifies the currency of each dependency, building the artifact.
// In v1 only direct dependencies are resolved (DirectOnly); transitive ones are
// skipped entirely (not recorded), keeping the artifact focused on what a team
// can act on. The design is transitive-ready: dropping the DirectOnly filter
// would extend coverage with no other change.
//
// Lookups run concurrently (bounded), mirroring the Maven graph crawl: each
// eligible dep is classified in its own goroutine into a pre-sized results slice
// (no shared-state mutation), then merged in order. Concurrency is the speed
// lever for large direct sets (thousands of network calls).
func Resolve(deps []types.Dependency, r CurrencyResolver, opt Options) *Artifact {
	art := newArtifact(opt.SourceEndpoint, opt.TTLHours)
	now := time.Now().UTC().Format(time.RFC3339)

	// Eligible deps (filtered once); their count is the progress denominator.
	var eligible []types.Dependency
	for _, dep := range deps {
		if opt.DirectOnly && !dep.Direct {
			continue
		}
		eligible = append(eligible, dep)
	}
	resolvestats.SetCurrencyTotal(len(eligible))

	conc := opt.Concurrency
	if conc <= 0 {
		conc = resolveConcurrency
	}

	// Classify each eligible dep concurrently into results[i] (index-keyed, so no
	// lock is needed on the slice). The resolver/cache and resolvestats counters
	// are all concurrency-safe.
	results := make([]Dependency, len(eligible))
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for i := range eligible {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = classifyDep(eligible[i], r, now)
		}(i)
	}
	wg.Wait()

	for _, entry := range results {
		art.Dependencies = append(art.Dependencies, entry)
		art.addToSummary(entry)
	}
	// Append entries in a deterministic order (system, name) for stable output.
	sort.SliceStable(art.Dependencies, func(i, j int) bool {
		if art.Dependencies[i].System != art.Dependencies[j].System {
			return art.Dependencies[i].System < art.Dependencies[j].System
		}
		return art.Dependencies[i].Name < art.Dependencies[j].Name
	})
	return art
}

// classifyDep resolves and classifies a single dependency, returning the built
// artifact entry. The caller appends it and updates the summary.
func classifyDep(dep types.Dependency, r CurrencyResolver, now string) Dependency {
	entry := Dependency{
		PURL:      purl(dep),
		Name:      dep.Name,
		Installed: dep.Version,
		Direct:    dep.Direct,
		Scope:     dep.Scope,
	}

	system, ok := resolver.DepsDevPackageSystem(dep.Type)
	if !ok {
		entry.Currency = Unsupported
		resolvestats.AddCurrencyUnsupported()
		return entry
	}
	entry.System = system

	info, err := r.LatestVersion(system, dep.Name)
	switch {
	case err == nil:
		entry.Latest = info.Latest
		entry.IsDeprecated = info.IsDeprecated
		entry.LatestPublishedAt = info.PublishedAt
		entry.Currency = classify(system, dep.Version, info.Latest)
		entry.CheckedAt = now
		entry.Source = "deps.dev"
		resolvestats.AddCurrencyResolved()
	case errors.Is(err, ErrNotFound):
		entry.Currency = Unknown
		entry.CheckedAt = now
		resolvestats.AddCurrencyUnknown()
	default:
		entry.Currency = BucketError
		resolvestats.AddCurrencyError()
	}
	return entry
}

// purl builds a Package URL for a dependency: pkg:{type}/{name}@{version}.
// Maven names are "group:artifact"; the group becomes the PURL namespace.
func purl(dep types.Dependency) string {
	ptype := strings.ToLower(dep.Type)
	if ptype == "gradle" {
		ptype = "maven"
	}
	name := dep.Name
	var b strings.Builder
	b.WriteString("pkg:")
	b.WriteString(ptype)
	b.WriteString("/")
	if (ptype == "maven") && strings.Contains(name, ":") {
		parts := strings.SplitN(name, ":", 2)
		b.WriteString(url.PathEscape(parts[0]))
		b.WriteString("/")
		b.WriteString(url.PathEscape(parts[1]))
	} else {
		b.WriteString(url.PathEscape(name))
	}
	if dep.Version != "" {
		b.WriteString("@")
		b.WriteString(url.PathEscape(dep.Version))
	}
	return b.String()
}
