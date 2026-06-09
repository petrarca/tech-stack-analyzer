# Dependency Graph

> Status: implemented (pnpm, npm, yarn, Cargo, poetry, uv, Maven)

Emit the package-to-package dependency graph -- the edges stating which
package depends on which -- in addition to the flat dependency list. Enables
fan-in / blast-radius and dependency-coupling analysis by downstream consumers
(e.g. a graph projection).

## Design

The graph is **read from lockfiles, not resolved**. A lockfile that records
the resolved dependency tree (pnpm snapshots, npm `package-lock` packages,
Cargo.lock, etc.) already contains the edges; the scanner just surfaces them.
It never runs a package manager or computes range resolution -- that boundary
belongs to dedicated tools (Trivy, deps.dev).

Output: an additive, non-breaking `dependency_edges` array on each component
and on the aggregate output. Each edge is `{from, to}` with endpoints in
`name@version` form so they join to the dependency list. Existing consumers
ignore the field.

In the **full component tree** each component carries the edges from its own
lockfile under `dependency_edges`. In the **aggregate output**
(`--also-aggregate dependencies`) all components' edges are flattened into a
single top-level `dependency_edges` array, deduplicated on `from|to` and sorted
by `(from, to)` -- this is the consumable form for a graph projection. The
synthetic root node `.` is the `from` for `direct`-mode edges. Maven nodes use
`groupId:artifactId@version`; all others use `name@version`.

### Modes (`--dependency-graph` / `dependency_graph`)

The full transitive graph is large (tens of edges per dependency), so it is
**off by default** and selected explicitly:

| Mode | Emits |
|------|-------|
| `off` (default) | no edges |
| `direct` | root -> direct dependency edges only (small) |
| `full` | the full transitive package-to-package graph |

The setting flows through the established 3-layer pattern (CLI flag, config
YAML key, registry accessor), mirroring `use_lock_files`. The unset value is
empty and interpreted as `off`, so a `dependency_graph` value in
`scanner-config.yml` merges correctly.

### Contract

Lockfile parsers expose the graph via `parsers.ParseGraphFunc`:

```go
func(input parsers.GraphInput) parsers.LockGraph
```

`GraphInput` carries the lockfile content, the optional component manifest
(package.json, Cargo.toml, pyproject.toml, ...) used for accurate direct-mode
derivation and edge scope, and the mode.

A detector registers an **ordered** `[]components.LockfileGraphProducer`
(`{Lockfile, Manifest, Parse}`) and calls the generic
`components.AttachLockfileGraph`, which builds a resolver chain (local lockfile
first, optional deps.dev online fallback), honors the mode, and is a no-op when
off. The list is ordered so the highest-priority source that exists wins,
matching each ecosystem's flat-extraction priority (npm: `package-lock` > pnpm >
yarn; python: uv > poetry). There is no per-ecosystem special-casing.

Edges carry a `source` (provenance) and `scope` (prod/dev/optional/peer, on
direct edges) field, and producers report `Unresolved` references (lockfile
drift) as the `dependency_graph.unresolved` component property instead of
dropping them.

## Ecosystem coverage

Edges are read from a resolved source. Lockfiles that state the resolved graph
yield it directly; manifest-only ecosystems (Maven, Gradle, Go full) require an
externally generated resolved tree (the analyzer never runs the build tool).

| Ecosystem | Source | Where edges live | Status |
|-----------|--------|------------------|--------|
| pnpm | `pnpm-lock.yaml` (v9) | `snapshots` | implemented |
| npm | `package-lock.json` v3 | `packages[path].dependencies` (+ dev/peer/optional), node_modules nearest-wins | implemented |
| yarn | `yarn.lock` (classic) | each entry's `dependencies:` block, range -> locked entry | implemented |
| Bun | `bun.lock` (JSONC) | `packages[].info.dependencies` (real graph), workspace = direct | implemented |
| Cargo | `Cargo.lock` | `[[package]] dependencies` array (TOML-decoded) | implemented |
| poetry | `poetry.lock` | `[package.dependencies]`, PEP 440 multi-version range match | implemented |
| uv | `uv.lock` | `[[package]] dependencies` / optional-dependencies (TOML-decoded) | implemented |
| Ruby | `Gemfile.lock` | GEM section (4/6-space indent), DEPENDENCIES = direct | implemented |
| PHP | `composer.lock` | `packages[].require` (platform reqs skipped) | implemented |
| NuGet | `packages.lock.json` | per-framework `dependencies`, type=Direct marks direct | implemented |
| Conan (C/C++) | `conan.lock` (v1) | `graph_lock.nodes` (node-id requires), node "0" = root | implemented |
| CocoaPods (Swift/iOS) | `Podfile.lock` | PODS nested deps, DEPENDENCIES = direct (subspecs collapsed) | implemented |
| Elixir | `mix.lock` | per-entry dependency tuples (real graph); direct via heuristic | implemented |
| Dart/Flutter | `pubspec.lock` | resolved packages (root-rooted closure; no package-to-package edges in lock) | implemented |
| Swift (SPM) | `Package.resolved` | resolved pins (root-rooted closure; edges live in Package.swift) | implemented |
| Maven | `dependency-tree.json` (pre-generated) | recursive `children` tree | implemented (read-only ingest) |
| Gradle | `gradle-dependencies.txt` (pre-generated) | ASCII tree, conflict-resolved versions | implemented (read-only ingest) |
| Go | `go.mod` (direct) + `go.mod.graph` (full, pre-generated) | require block / `go mod graph` | implemented |
| CycloneDX | `bom.json` (any SBOM with a graph) | `dependencies` ref/dependsOn section | implemented (ingest) |

Direct mode is derived from the component manifest where available (accurate
even when a direct dep is also transitive); otherwise from the lockfile's own
root, falling back to a not-referenced heuristic.

Validated on real repos: pnpm 0/86/2442 (cypher-graphdb/explorer), uv 0/29/265
(cypher-graphdb/server), npm 0/7/534 (repomix), poetry multi-version on nicegui
(numpy resolves to both 1.24.4 and 1.26.4), Ruby 101 edges (Rails app,
activesupport highest fan-in), PHP 235 edges (Laravel). Aggregate dedup verified
on the cypher-graphdb monorepo (2752 nested -> 2741 unique, sorted).

## Maven -- read-only ingest of a pre-generated tree

A resolved Maven graph **cannot be derived statically** from `pom.xml`: it
requires Maven's conflict mediation, `dependencyManagement` / BOM overrides,
version ranges and scope rules -- i.e. Maven's own resolver. Research (2024/25)
confirms `mvn dependency:tree` remains the gold standard, and since plugin 3.6.0
it emits machine-readable JSON (`-DoutputType=json`), alongside dot/graphml/tgf.

The analyzer **never runs Maven**. Consistent with every other lockfile, it
reads a file that the operator/CI already generated and committed:

```
mvn dependency:tree -DoutputType=json -DoutputFile=dependency-tree.json
```

`ParseMavenTreeGraph` reads `dependency-tree.json` (recursive `children` tree;
nodes `groupId:artifactId@version`, edges parent -> child). This mirrors
`maven_dependency_list` (which ingests `mvn dependency:list`). If the file is
absent it is a no-op, exactly like a missing lockfile.

The same pre-generated-ingest pattern is now implemented for:

- **Gradle:** `ParseGradleTreeGraph` reads `gradle-dependencies.txt`
  (`gradle dependencies` output) -- an ASCII tree with conflict-resolved
  versions.
- **Go (full):** `ParseGoModGraph` reads `go.mod.graph` (`go mod graph`);
  `go.mod` alone yields the direct graph.
- **CycloneDX:** `ParseCycloneDXGraph` reads a committed SBOM's `dependencies`
  edge section (e.g. `cyclonedx-maven-plugin makeAggregateBom` with
  `dependencyGraph`), registered as a secondary Maven/Gradle source. Standards-
  based and ecosystem-agnostic.

### Remaining future path

1. **Online resolution (deps.dev).** Query deps.dev for a precomputed resolved
   graph, as an opt-in fallback for manifest-only ecosystems. See the appendix
   below.

## Appendix: online resolution via deps.dev (opt-in)

> Status: implemented (opt-in via `--resolve-online`).

Google's deps.dev (Open Source Insights) continuously crawls public registries
and pre-computes the **resolved transitive graph** for every published package
version, using each ecosystem's real resolution algorithm. This is the
"online resolution" path: for manifest-only ecosystems (Maven, Gradle) where no
resolved lockfile/tree-file is committed, deps.dev can supply the edges instead
of requiring the operator to run the build tool.

It is **opt-in and a fallback only** -- it crosses the offline boundary that the
rest of the pipeline deliberately maintains, and it reflects *published*
versions, not *your* repo's resolved state.

### API shape (validated)

- Free, unauthenticated HTTPS -> JSON. Base: `https://api.deps.dev/v3`.
- Resolved graph endpoint (note the gRPC-transcoding `:dependencies` verb, not a
  `/dependencies` path segment; the package-name segment is URL-encoded, so
  Maven's `groupId:artifactId` colon is encoded):

  ```
  GET /v3/systems/{system}/packages/{name}/versions/{version}:dependencies
  ```

  `system` in `maven|npm|go|cargo|pypi|nuget|rubygems`.
- Response is a **deduplicated DAG**: `nodes[]` (each with
  `versionKey {system,name,version}`, `relation` in `SELF|DIRECT|INDIRECT`, a
  per-node `errors[]`, and `bundled`) and `edges[]`
  (`{fromNode, toNode, requirement}` where `from/toNode` are integer indices
  into `nodes` and `requirement` is the declared range).
- Failure modes are clean HTTP: `404` for an unknown version, `429` for rate
  limiting (no published quota -- design for backoff). Caching is expressly
  permitted by the Google API ToS; the BigQuery public dataset is the bulk
  alternative.

### What the shape gives us (vs our lockfile parsers)

- **Same edge model.** Nodes are `name@version` (Maven
  `groupId:artifactId@version`) -- identical to our node identity. The DAG is
  already deduplicated (a diamond dependency is one node with high in-degree),
  matching what our aggregator's `from|to` dedup produces. A deps.dev resolver
  and a lockfile resolver are interchangeable at the edge level. This is
  *cleaner* than the `mvn dependency:tree` JSON, which repeats subtrees per path.
- **Modes for free.** `relation == DIRECT` precomputes our `direct` mode; all
  edges give `full`. No edge-walking needed.
- **Declared range per edge.** `requirement` is the declared constraint
  alongside the resolved version -- a superset of `metadata.declared`.
- **Partial-resolution signal.** Per-node `errors[]` flags an unresolved node
  without failing the whole graph; surface it into metadata rather than dropping.

### Caveats (must be documented at the seam, not glossed over)

- **Published-version, not your lockfile.** deps.dev does not know workspace
  links, private packages, local exclusions, or corporate mirrors. Where a real
  lockfile/tree-file exists it is authoritative; deps.dev is a fallback only.
- **Runtime-scoped.** The graph excludes `test`/`provided` scope (validated:
  no junit nodes in `spring-boot-starter-web`). deps.dev `full` approximates our
  `prod`-scoped graph, not our dev-inclusive one. Correct for blast-radius /
  vulnerability scope, but **not bit-identical** to our lockfile graphs.
- **Crosses the offline boundary.** Off by default; never overrides local
  resolution.

### Integration (resolver seam -- implemented)

The `DependencyResolver` seam is in place (`internal/scanner/resolver`). It
produces `[]types.DependencyEdge` behind a single interface, with a `Chain`
that runs resolvers in precedence order and tags edges with their provenance:

- **`LockfileResolver`** -- the implemented offline path (reflects the repo's
  own resolved state). Wraps the ordered lockfile producers; a present lockfile
  is authoritative even when it yields zero edges (it does not fall through).
- **`DepsDevResolver`** -- online fallback (reflects published versions).
  Gated by `Enabled` and a **pluggable** `OnlineGraphResolver`; with neither it
  falls through, so it is safe to include unconditionally and stays offline by
  default.

`components.AttachLockfileGraph` builds the chain (lockfile first, online
second) and is unchanged for detectors -- they still register an ordered
`[]LockfileGraphProducer`. Edges carry a `source` field (`lockfile` /
`deps.dev`) for downstream trust decisions.

### Online resolution -- implemented

- **Pluggable resolver.** `OnlineGraphResolver` is the interface
  (`ResolveGraph(system, name, version, mode)`); deps.dev is the reference
  implementation (`NewDepsDevFetcher`). A mirror or alternative service exposing
  the same API shape can be supplied without touching the chain or detectors.
- **HTTP fetcher.** Calls the validated `:dependencies` endpoint, maps the
  node/edge DAG to `{from, to}` `name@version` edges (root SELF node -> `.`),
  honors direct/full mode, and **caches per `(system, name, version, mode)`**
  for the run (pinned versions are immutable, so in-run caching is safe).
  `404` -> no edges (not an error); `429` -> error.
- **Configurable endpoint.** `--resolve-online-endpoint` / `resolve_online_endpoint`
  overrides the base URL (default `https://api.deps.dev`) for an
  API-compatible facade or mirror.
- **Coordinate wiring.** Detectors set `payload.GraphCoordinates`
  (`{Ecosystem, Name, Version}`); `AttachLockfileGraph` threads it into the
  resolver request. Wired for Maven and Gradle (the manifest-only cases that
  benefit); any ecosystem can opt in by setting the field.
- **Opt-in switch.** `--resolve-online` / `resolve_online` (default off),
  orthogonal to `--dependency-graph off|direct|full` (which controls *what* to
  emit). Validated end-to-end: a Maven `pom.xml` with no committed tree yields 0
  edges offline and the full 62-edge graph (tagged `source: deps.dev`) with
  `--resolve-online`; a present `dependency-tree.json` still wins (local-first).

Attribution: deps.dev data is CC-BY 4.0 -- consumers should carry "data: Google
Open Source Insights (deps.dev), CC-BY 4.0".
