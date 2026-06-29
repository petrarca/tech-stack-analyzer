# Dependency Currency (Freshness)

How the scanner reports dependency *currency* -- how far each dependency is
behind its latest available version -- as a separate, time-varying artifact,
backed by a persistent on-disk cache shared across runs and products.

This is **freshness**, not **vulnerability**. "Is this dependency outdated?" is
a maintenance/tech-debt question answered here against package registries
(Google deps.dev). "Is this dependency *known-vulnerable*?" is a security
question answered elsewhere (the CVE scan over the SBOM). The two are kept
strictly separate and have no overlap.

## Goals and non-goals

**Goals**

- For each **direct** dependency, report the latest available version and the
  semver distance (`up_to_date` / `patch` / `minor` / `major`).
- Emit a dedicated, machine-readable artifact (`{out}.currency.json`) joined to
  dependencies by PURL -- without touching the scan output or the SBOM.
- Resolve via Google **deps.dev**, reusing the existing deps.dev integration
  (endpoint already configurable), behind a resolver abstraction.
- Persist results in a **shared, multi-process, per-key-TTL** on-disk cache so
  the same package is looked up once per portfolio run and re-used until stale.
- Be **opt-in** (network access; off by default), consistent with the existing
  `--deps-dev` posture.
- Be re-runnable as a standalone refresh over an existing aggregate file.

**Non-goals (v1)**

- Transitive-dependency currency. The artifact and resolver are designed to
  admit it later; v1 resolves direct only.
- Currency of internal/private packages. deps.dev does not index them; v1
  records them honestly as `unknown` and does not attempt internal-registry
  resolution (designed for later via the resolver chain).
- End-of-life (EOL) dates. deps.dev exposes publish dates, not support windows.
  EOL is a separate concern and out of scope here.
- Modifying the SBOM. The SBOM stays standard CycloneDX. Currency is a separate
  artifact (there is no industry standard schema for freshness; see Rationale).

## Where it sits in the pipeline

Currency is **post-processing**, a sibling of the SBOM and CVE artifacts -- a
derived, time-varying view produced after the scan and aggregation are complete,
never part of tree-walking.

```
scan            -> {out}.json / {out}-agg.json   (scan facts; untouched)
scan --also-sbom-> {out}.cdx.json                (SBOM projection; untouched)
scan:security   -> {out}.cve.cdx.json            (CVE over SBOM; separate pipeline)
enrich-currency -> {out}.currency.json           (currency over deps.dev)   <- THIS
```

It runs over the **aggregate** dependency list (deduped, with reliable
direct/transitive flags after the aggregator merge fix), keyed by
`(system, name)`. The aggregate is the canonical input.
## Architecture: a unified store, with currency as the first consumer

The cache is built **facade-first**. A single SQLite database file is managed by
one store package; consumers create and own their own tables in it. Currency is
the first consumer. A persistent blob cache for dependency *resolution* (today
in-memory only) is the planned second consumer (see "Future: blobcache").

```
internal/store  -- unified SQLite facade (owns the .db file)
  - Open(path), lifecycle, pragmas (WAL, busy_timeout), schema migrations
  - Info() / stats; `cache info|clear|vacuum` subcommands
  - multi-process safe; local-filesystem only
        |
        +-- currency table        CONSUMER #1 (this design): typed columns + per-key TTL
        |
        +-- blobcache (SQLite)    CONSUMER #2 (future): implements the existing
                                  blobcache.Cache interface; permanent entries for
                                  immutable POM/graph bytes; TTL'd negative cache
```

One DB file, one facade, two tables for two consumers. The facade owns
open/close, schema versioning/migrations, the WAL/busy_timeout pragmas, the
path-resolution rules, and introspection. Consumers never open the file
directly.

### Why facade-first

The hard part -- multi-process SQLite, WAL, migrations, path resolution, the
local-FS guard -- is built and tested **once**, decoupled from any consumer.
Currency then proves the facade as a low-risk, self-contained feature (its own
table; nothing existing depends on it). Only after the facade is proven does the
hot-path resolution cache (blobcache) migrate onto it. This de-risks the SQLite
investment before critical paths depend on it.

### Why SQLite (modernc.org/sqlite)

The cache must be usable by **multiple concurrent scanner processes** on one
machine, be **pure Go** (the binary cross-compiles to darwin/linux/windows on
amd64+arm64 with `CGO_ENABLED=0`), and support **per-key TTL**. Among pure-Go
embedded stores, only a real SQLite satisfies multi-process access; the native-
TTL KV stores (Badger, BuntDB) are single-process and would corrupt under
concurrent processes. `modernc.org/sqlite` is the pure-Go SQLite port
(BSD-3-Clause).

Measured overhead (release build, stripped): **~4.6 MB** added to the binary;
all five target platforms cross-compile with `CGO_ENABLED=0`; two separate OS
processes writing the same DB concurrently passed with `integrity_check = ok`.
Runtime is negligible (microsecond lookups). The cost amortizes across both
consumers (currency now, transitive resolution later), not currency alone.

### Constraints

- **Lazy / on-demand creation.** The DB file is created and opened **only when a
  feature actually needs it** -- in v1, only during currency resolution
  (`--resolve-currency` or `enrich-currency`). A normal scan, an SBOM run, or any
  command without currency MUST NOT create, open, or touch the cache file. The
  facade is opened by its consumer at first use, not at process start; importing
  the store package has no side effects. (`cache info` on a non-existent DB
  reports "no cache yet" rather than creating one.) When the blobcache consumer
  lands it follows the same rule -- the file appears only when transitive
  resolution with `--deps-dev` actually runs.
- **Local filesystem only.** SQLite file locking is unreliable on NFS/SMB. The
  default path is local; a network path is documented as unsupported.
- **WAL mode + `busy_timeout`.** Concurrent readers, serialized writers, brief
  contention waits -- no application-level locking required.
- **Schema-versioned.** The facade records a schema version and migrates
  forward; an unreadable/corrupt cache is treated as empty and rebuilt (it is
  only ever a performance cache, never a source of truth).
- **Tables created on demand.** Each consumer creates its own table lazily on
  first write (`CREATE TABLE IF NOT EXISTS`). The currency table does not exist
  until a currency lookup is cached; the blobcache table does not exist until
  transitive resolution caches a blob. The store facade itself creates no tables.

## Cache location and override

Resolution order (first match wins), consistent with the existing
`STACK_ANALYZER_*` env convention and flag/env/default precedence:

1. `--currency-cache <path>` (per-invocation flag)
2. `STACK_ANALYZER_CURRENCY_CACHE` (environment variable)
3. Default: `os.UserCacheDir()/stack-analyzer/currency.db`
   - macOS: `~/Library/Caches/stack-analyzer/currency.db`
   - Linux: `~/.cache/stack-analyzer/currency.db`
   - Windows: `%LocalAppData%\stack-analyzer\currency.db`

The default is **shared across all products and runs on the machine** -- this is
deliberate. A package's latest version is the same answer for every product on a
given day, so a single shared cache maximizes dedup: the first product warms the
cache, the rest hit it. (When the blobcache consumer lands, it shares the same
DB file -- see "Future".)
## The output artifact

A dedicated file named by the same suffix convention as the SBOM/aggregate
(`{out}.cdx.json`, `{out}-agg.json`): **`{out}.currency.json`**.

It is a self-contained, versioned document, joined to dependencies by **PURL**
(the SBOM's native identifier), carrying its own generation timestamp and the
source/endpoint used. It does **not** mutate the aggregate's dependency tuples
or the SBOM.

```json
{
  "schema": "stack-analyzer.currency/v1",
  "generated_at": "2026-06-29T15:00:00Z",
  "source": "deps.dev",
  "source_endpoint": "https://api.deps.dev",
  "ttl_hours": 24,
  "scope": "direct",
  "summary": {
    "total": 835,
    "resolved": 261,
    "up_to_date": 120, "patch": 40, "minor": 60, "major": 41,
    "unsupported_ecosystem": 540,
    "unknown": 34,
    "deprecated": 3
  },
  "dependencies": [
    {
      "purl": "pkg:npm/react@17.0.2",
      "system": "npm",
      "name": "react",
      "installed": "17.0.2",
      "latest": "19.3.0",
      "currency": "major",
      "direct": true,
      "scope": "prod",
      "is_deprecated": false,
      "latest_published_at": "2026-05-10T00:00:00Z",
      "checked_at": "2026-06-29T15:00:00Z",
      "source": "deps.dev"
    }
  ]
}
```

### The `currency` field

A single enum captures both the resolved distance and the reasons a dependency
could not be resolved -- the unresolved reasons are recorded explicitly, never
silently dropped:

| Value | Meaning |
|-------|---------|
| `up_to_date` | installed == latest stable |
| `patch` | behind by a patch release |
| `minor` | behind by a minor release |
| `major` | behind by a major release |
| `unsupported_ecosystem` | no public registry exists for this ecosystem (e.g. delphi, native libs, project references) -- structurally unanswerable |
| `unknown` | a supported ecosystem was queried, but deps.dev returned no result (yanked, typo, or **internal/private** package) |
| `error` | transient lookup failure (network/5xx); distinct from a definitive not-found |

`unsupported_ecosystem` vs `unknown` is a meaningful distinction: the former is
"we know there is nothing to ask"; the latter is "we asked and got nothing."
Internal packages fall under `unknown` in v1 -- see "Internal dependencies".

"Latest stable" is taken from deps.dev's `isDefault` version, which is defined
as the greatest version ignoring pre-releases. Pre-release filtering is handled
by deps.dev; the scanner does not re-implement it.

### Why a separate artifact (not in the SBOM)

There is **no industry-standard schema** for dependency freshness (confirmed
across OWASP, OpenSSF, CycloneDX, SPDX). CycloneDX can carry custom data only
via namespaced `properties`, which no other tool understands -- "compatible" but
not interoperable, and it pollutes a standard facts document with a time-varying
view. SPDX has no field for it either. The established practice is to keep the
SBOM pure and emit a separate freshness report keyed back to it. We follow that:
the SBOM stays standard CycloneDX; currency is our own small versioned schema,
joined by PURL.

## Scope: direct now, transitive-ready

v1 resolves **direct dependencies only**. This is a value decision, not just a
cost one: transitive currency is not directly actionable (a team cannot bump a
transitively-pinned version without bumping its parent), and the
security-relevant subset of stale transitive deps is already covered by the CVE
scan. Direct deps are what a team controls.

The design is transitive-ready: the artifact has a `scope` field (`direct` in
v1), and resolution is keyed `(system, name)` independent of where the package
appears. Extending to transitive is a matter of feeding the transitive set
through the same resolver -- no schema or resolver change. Direct vs transitive
is read from the aggregate (reliable after the aggregator merge fix).
## Progress / UX (reuse the existing resolution pattern)

Dependency resolution already has a polished progress UX. Currency must use the
**same mechanism and visual pattern** so the experience is consistent -- the
user sees live progress for currency resolution exactly as they do for
dependency resolution today.

The existing pattern (to reuse, not reinvent):

- **Shared atomic counters** in `internal/scanner/resolvestats` -- process-wide
  counters incremented by the fetchers (`AddDepsDevCall`, `AddCacheHit`, ...),
  with a `Snapshot` / `Sub` / `Format` / `Active` API.
- **A `Progress` reporter** (`internal/progress`) with the `ResolveStart` /
  `ResolveProgress(status)` / `ResolveComplete(status, duration)` events
  (`EventResolveStart` / `EventResolveProgress` / `EventResolveComplete`),
  rendered by the simple/tree/summary handlers.
- **A sampling goroutine** (see `internal/cmd/sbom.go` and
  `internal/scanner/scanner.go`): a ticker periodically diffs the counters
  against a baseline; on first activity it emits `ResolveStart`, then periodic
  `ResolveProgress` with the formatted delta, and on completion
  `ResolveComplete` with the totals and elapsed time.

Currency adopts this directly:

- **Extend `resolvestats`** with currency counters in the same style:
  `currencyResolved`, `currencyCacheHits`, `currencyUnknown`,
  `currencyUnsupported`, `currencyErrors` (atomic, with `Add*` helpers, included
  in `Snapshot`/`Sub`/`Format`/`Active`). The deps.dev currency resolver calls
  `AddDepsDevCall` (network) and the new counters; the cache layer calls
  `AddCacheHit`.
- **Reuse the same `Resolve*` progress events.** Currency resolution is a
  resolution phase; it emits `ResolveStart` on first lookup, periodic
  `ResolveProgress` (e.g. `"312/835 resolved, 540 unsupported, 21 cached,
  3 unknown"`), and `ResolveComplete` with totals + duration -- identical
  surface and handlers to dependency resolution.
- **Reuse the sampling-goroutine helper.** The `enrich-currency` command and the
  in-scan `--resolve-currency` path both wrap the resolution loop with the same
  ticker/baseline/diff helper used by `sbom.go`, so behavior (quiet/verbose/tree
  rendering, timing) is consistent for free.

The result: no new progress UI is written. Currency plugs into the existing
counters + events + handlers, and the user sees the same familiar live progress
for currency lookups as for dependency resolution -- including cache-hit rates,
which makes the shared-cache benefit visible.
## Opt-in surface (during scan)

Currency is network-bound and therefore **off by default**, consistent with the
existing `--deps-dev` posture. It mirrors the established deps.dev flags rather
than inventing a new consent concept.

| Flag / env | Default | Meaning |
|------------|---------|---------|
| `--resolve-currency` | false | Enable currency resolution; writes `{out}.currency.json` alongside the scan output. Self-sufficient -- implies the network call. |
| `--deps-dev-endpoint <url>` | public deps.dev | Reused as-is. Points currency at an API-compatible deps.dev mirror/facade. |
| `--currency-cache <path>` | (see location) | Override the cache DB path for this invocation. |
| `STACK_ANALYZER_CURRENCY_CACHE` | (see location) | Environment override for the cache DB path. |
| `--currency-ttl <hours>` | 24 | Per-entry TTL for currency lookups. |

`--resolve-currency` is **self-sufficient**: setting it enables the network
lookup without also requiring `--deps-dev`. It honors `--deps-dev-endpoint` when
set. In-scan, the artifact is named from `--output` using the same suffix rule
as the SBOM/aggregate (`{out}.currency.json`).

## Standalone command (refresh over an aggregate)

Currency drifts on its own -- the installed version is unchanged while upstream
ships new releases -- so it must be refreshable without a rescan. This mirrors
the existing `sbom` subcommand, which reads a saved scan JSON and emits a
derived artifact.

```
stack-analyzer enrich-currency <path-to-agg.json> [--output <file>]
```

- **Input: the aggregate file only.** The aggregate is deduped and (after the
  aggregator merge fix) carries reliable `direct`/`scope` flags -- the correct,
  canonical input. The raw scan JSON is not accepted; this keeps the command's
  contract clear (one input shape, already deduped).
- **Output:** `{stem}.currency.json` (or `--output`).
- **Refresh semantics:** re-running the command **is** the refresh. It always
  computes current values and overwrites the artifact; entries within TTL are
  served from the shared cache, stale/missing ones are re-fetched. A
  `--force` flag bypasses the TTL to re-fetch everything.
- A pure refresh trusts the installed versions recorded in the aggregate. If the
  project's dependencies themselves changed, a fresh **scan** is needed (not a
  currency refresh) -- the two cadences are intentionally decoupled.

## Resolver abstraction (encapsulated; deps.dev is one implementation)

Resolution is behind an interface so the source is swappable and the cache is
independent of it. This mirrors the existing deps.dev integration, which is
already encapsulated (`OnlineGraphResolver` / `DepsDevFetcher`, with an
injectable HTTP client and a configurable base URL).

```go
type CurrencyResolver interface {
    // LatestVersion returns the latest stable version info for a package,
    // or ErrCoordinateNotFound when the source does not know it.
    LatestVersion(system, name string) (LatestInfo, error)
}

type LatestInfo struct {
    Latest        string
    IsDeprecated  bool
    PublishedAt   string
}
```

Layering (each independently swappable):

```
artifact writer
  -> cachedResolver (SQLite, per-key TTL, negative cache)   [decorator]
       -> ChainResolver([]CurrencyResolver)                  [tries each in order]
            -> depsDevCurrencyResolver                        [v1: the only link]
                 -> HTTP (endpoint = --deps-dev-endpoint)
```

- **`depsDevCurrencyResolver`** reuses the existing deps.dev client plumbing
  (HTTP client, endpoint override, 404 -> `ErrCoordinateNotFound`). It calls
  `GetPackage` (`/v3/systems/{system}/packages/{name}`) and reads the
  `isDefault` version + `isDeprecated` + `publishedAt`.
- **The cache is a decorator** wrapping the resolver -- so the cache is
  independent of the source. Swapping deps.dev for a mirror, or adding a second
  source, never touches the cache or the artifact code.
- **`ChainResolver`** tries each resolver in order until one resolves. v1's chain
  is `[depsDev]`. This is the same multi-source pattern the scanner already uses
  for transitive Maven graphs (`MavenGraphSource: deps-dev | repo | none`).

### Reuse existing deps.dev plumbing -- do not duplicate it

`depsDevCurrencyResolver` is a **thin call against the existing deps.dev client**,
not a second HTTP client. The current deps.dev integration
(`internal/scanner/resolver/depsdev_fetch.go`) already provides everything except
the one new endpoint, and the currency resolver MUST reuse these rather than
re-implement them:

| Existing primitive | Where | Reuse for currency |
|--------------------|-------|--------------------|
| `HTTPDoer` interface + default `http.Client{Timeout: 30s}` | `depsdev_fetch.go` | same client; inject for tests |
| `DefaultDepsDevBaseURL` + base-URL override | `depsdev_fetch.go` | same constant + the `--deps-dev-endpoint` value from settings |
| `ErrCoordinateNotFound` (404 sentinel) | `depsdev_fetch.go` | currency maps 404 -> `ErrCoordinateNotFound`, identical semantics |
| 429 / non-200 handling | `depsdev_fetch.go` `request()` | same status-code handling |
| `DepsDevEndpoint` settings field + URL validation | `internal/config/settings.go` | reused as-is; no new endpoint flag/field |

The **only** genuinely new code is the `GetPackage` request and response decode:
the existing `request()` targets the graph endpoint
(`.../versions/{version}:dependencies`); currency needs
`.../packages/{name}` and reads `versions[].isDefault` + `isDeprecated` +
`publishedAt`.

To avoid duplicating the HTTP/endpoint/status-handling logic, **refactor the
shared transport out of the graph-specific `request()`** into a small internal
helper on the deps.dev client (e.g. `getJSON(path) (body, notFound, err)`), then
have **both** the existing graph fetch and the new currency `GetPackage` call it.
No copy-pasted client, no second timeout/endpoint/sentinel implementation. The
graph path keeps its exact current behavior; currency is a second caller of the
same transport.

Equally, **do not re-implement dependency-management concepts that already
exist**: ecosystem/PURL handling, the `(system, name, version)` model, the
declared-vs-resolved distinction, and the deduped direct/transitive flags all
come from the aggregate and the existing types -- currency consumes them, it does
not re-derive them. Currency adds exactly one new fact per package (its latest
version) and nothing else.

### Internal / private dependencies

deps.dev only indexes public registries. An internal package (e.g. an internal
Maven coordinate or scoped npm package) returns 404 from deps.dev.

- **v1 behavior:** a 404 on a supported ecosystem is recorded as `unknown`
  (queried, not found). v1 does **not** attempt to classify *why* (internal vs
  yanked vs typo) -- no prefix lists, no heuristic internal-detection. The
  artifact reports these honestly and counts them; it never claims they are
  current.
- **Designed-for-later:** internal currency is a second link in the chain. An
  internal-registry resolver (querying an Artifactory/JFrog or enterprise npm
  registry) would be added to the chain, reusing the existing `--maven-repo-url`
  and `STACK_ANALYZER_MAVEN_USER`/`STACK_ANALYZER_MAVEN_TOKEN` machinery. When it
  lands, `unknown` entries that resolve against the internal registry simply
  upgrade to `patch`/`minor`/`major` -- **no schema or artifact change**, because
  `unknown` always meant "not resolved *yet*".

So both extensions -- transitive coverage and internal-registry resolution --
ride the same resolver-chain abstraction. Neither is built in v1; both are
recorded as `unknown`/out-of-scope and slot in without redesign.

## Future: blobcache as the second store consumer

The scanner already has a `blobcache.Cache` interface backing dependency
resolution (Maven POM bytes and deps.dev graph responses). Today it has only an
in-memory implementation, and its own doc comment states the intent to replace
it "later by a persistent one (file/SQLite) for cross-run caching, since
published POMs and resolved-graph responses are immutable."

That persistent implementation is the **second consumer** of the unified store:

- A `blobcache` table in the same DB file, implementing the existing
  `blobcache.Cache` interface -- a drop-in replacement for `NewMemory()` at the
  current injection points (`internal/scanner/components/graph_cache.go`,
  `internal/scanner/mavenresolve`).
- **Permanent (no-TTL) positive entries**: a pinned `(system, name, version)`
  graph or a published POM is immutable, so it is cached forever -- a near-100%
  hit rate after the first portfolio run, and far fewer deps.dev/Maven fetches
  on re-scans.
- **TTL'd negative entries**: per the interface comment, a 404 today can become a
  published artifact later, so negative cache entries carry a TTL even though
  positive ones do not. The store's per-row TTL handles both uniformly.
- **Consolidation benefit**: today the in-memory blobcache is fragmented across
  per-run instances; one shared SQLite store unifies them into a single
  machine-wide cache.

Why it is the **second** consumer, not the first: transitive resolution is a hot
path that critical scans depend on. Proving the store facade with the
low-risk, self-contained currency feature first de-risks the SQLite/concurrency
investment before the resolver hot path migrates onto it. The blobcache
migration is then a small, additive change (a second table, an interface
implementation) touching code that already accepts the `blobcache.Cache`
interface.

## The `cache` command

A top-level `cache` command manages the shared store, following the existing
parent/subcommand pattern (like `info` with its subcommands; each subcommand in
its own file under `internal/cmd`). It resolves the DB path with the same
precedence as resolution (`--currency-cache` / `STACK_ANALYZER_CURRENCY_CACHE` /
default) and **never creates the file** -- if the cache does not exist, the
commands report that rather than initializing it.

```
stack-analyzer cache info      # where it is, size, number of records
stack-analyzer cache clear     # drop the cache (all, or --expired-only)
stack-analyzer cache vacuum    # reclaim space after deletions
```

### `cache info`

Reports, without creating anything:

- **Location** -- the resolved DB file path (and which source set it: flag, env,
  or default).
- **Size** -- the file size on disk (0 / "no cache yet" if it does not exist).
- **Records** -- total row count, and per-table counts: currency entries (with
  how many are expired vs fresh), and -- once the blobcache consumer lands --
  blob entries.
- Schema version.

If the file does not exist, `cache info` prints a clear "no cache yet" line and
exits 0 (it is not an error to have never run currency).

### `cache clear`

Drops cached data so the next run re-resolves from source.

- Default: remove all entries (all tables) -- the file may be deleted outright or
  truncated.
- `--expired-only`: remove only currency entries past their TTL, keeping fresh
  ones (and the immutable blobcache entries) intact.

### `cache vacuum`

Runs SQLite `VACUUM` to reclaim space after large deletions. Optional
convenience; `clear` already shrinks via row removal.

All three operate on the single shared store, so they cover both the currency
consumer and (later) the blobcache consumer.

## Rationale summary

- **Currency != vulnerability.** Freshness (this) is a maintenance signal from
  registries; vulnerability is a security signal from the CVE scan over the
  SBOM. Separate artifacts, separate pipelines, no overlap.
- **Separate artifact, not in the SBOM.** No standard schema for freshness
  exists; SBOM stays pure CycloneDX; currency is our own versioned doc joined by
  PURL.
- **deps.dev as the single source**, behind a resolver interface + chain, reusing
  the existing (already endpoint-configurable) integration. One adapter covers
  npm/maven/pypi/nuget/cargo/go/rubygems; unsupported ecosystems and
  not-found/internal packages are recorded honestly, never guessed.
- **Reuse, do not duplicate.** The currency resolver shares the existing deps.dev
  client transport (HTTP client, endpoint override, 404/429 handling,
  `ErrCoordinateNotFound`); only the `GetPackage` call is new. Ecosystem/PURL,
  the `(system,name,version)` model, and direct/transitive flags come from the
  aggregate and existing types -- currency adds exactly one new fact per package.
- **Persistent, shared, multi-process, per-key-TTL SQLite cache** behind a
  facade -- currency is consumer #1; the existing in-memory blobcache becomes
  consumer #2, fulfilling an intent already stated in the code. The cache file is
  created **lazily, only when a feature needs it** -- a plain scan never touches it.
- **Opt-in, self-sufficient `--resolve-currency`**; standalone `enrich-currency`
  over the aggregate doubles as the refresh path.
- **Consistent UX**: currency reuses the existing `resolvestats` counters and
  `Resolve*` progress events -- same live progress as dependency resolution.

## Implementation order

1. `internal/store` -- the unified SQLite facade (lazy open: no file unless a
   consumer asks; pragmas/migrations/path-resolution/local-FS guard,
   `cache info|clear|vacuum`). No side effects on import.
2. Refactor the deps.dev client transport: extract a shared `getJSON(path)`
   helper from the graph-specific `request()` so currency reuses it. Then
   `CurrencyResolver` interface + `depsDevCurrencyResolver` (the new
   `GetPackage` call) + `ChainResolver`; the SQLite-backed cache decorator
   (currency table created on first write, per-key TTL).
3. `resolvestats` currency counters + wire the `Resolve*` progress sampling
   goroutine into the resolution loop.
4. The `{out}.currency.json` writer + schema; semver-distance bucketing.
5. `--resolve-currency` in-scan path and the `enrich-currency` subcommand
   (aggregate-only input; `--force` refresh).
6. The `cache` command (`info` / `clear` / `vacuum`), parent/subcommand pattern,
   never creates the file.
7. License compliance: re-run `task licenses` (adds modernc.org/sqlite's
   BSD-3/MIT deps; keeps the tree clean).
8. (Later) `blobcache.SQLite` as the second store consumer; migrate the
   `NewMemory()` injection points.
