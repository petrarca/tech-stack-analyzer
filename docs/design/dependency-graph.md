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
func(content []byte, mode types.DependencyGraphMode) parsers.LockGraph
```

A detector registers an **ordered** `[]components.LockfileGraphProducer`
(`{Lockfile, Parse}`) and calls the generic `components.AttachLockfileGraph`,
which honors the mode and is a no-op when off. The list is ordered so the
highest-priority lockfile that exists wins, matching each ecosystem's existing
flat-extraction priority (npm: `package-lock` > pnpm > yarn; python: uv >
poetry). There is no per-ecosystem special-casing.

## Ecosystem coverage

Edges are a **lockfile** feature. They can be produced wherever the lockfile
states the resolved graph; they cannot be produced for manifest-only
ecosystems without an externally generated resolved tree.

| Ecosystem | Source | Where edges live | Status |
|-----------|--------|------------------|--------|
| pnpm | `pnpm-lock.yaml` (v9) | `snapshots` | implemented |
| npm | `package-lock.json` v3 | `packages[path].dependencies` (+ peer/optional), node_modules nearest-wins resolution | implemented |
| yarn | `yarn.lock` (classic) | each entry's `dependencies:` block, range -> locked entry | implemented |
| Cargo | `Cargo.lock` | `[[package]] dependencies` array | implemented |
| poetry | `poetry.lock` | `[package.dependencies]` sub-tables | implemented |
| uv | `uv.lock` | `[[package]] dependencies` / optional-dependencies | implemented |
| Maven | `dependency-tree.json` (pre-generated) | recursive `children` tree | implemented (read-only ingest) |
| Gradle | `build.gradle` | no static graph | see below |
| NuGet | `packages.lock.json` (when present) | conditional | not yet |
| Go | `go.mod` | direct only (full graph needs `go mod graph`) | not yet |

Validated on real repos: pnpm 0/86/2442 (cypher-graphdb/explorer), uv 0/29/265
(cypher-graphdb/server, fan-in dominated by `typing-extensions`), npm 0/7/534
(repomix). Aggregate dedup verified on the cypher-graphdb monorepo (2752 nested
-> 2741 unique, sorted).

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

### Remaining future paths

1. **Gradle:** the same pattern -- ingest a generated `gradle dependencies`
   tree. Not yet implemented (no standard machine-readable Gradle tree file).
2. **CycloneDX dependency graph.** The `cyclonedx-maven-plugin`
   (`makeAggregateBom` with `dependencyGraph`) emits CycloneDX with a
   `dependencies` edge section. Since the analyzer already emits/consumes
   CycloneDX (`--sbom`), reading that edge section is a natural second Maven
   path.
3. **Online resolution (deps.dev).** Query deps.dev by PURL for a precomputed
   resolved graph. Crosses the offline boundary, so opt-in enrichment only.
   Noted as an option, not a recommendation.
