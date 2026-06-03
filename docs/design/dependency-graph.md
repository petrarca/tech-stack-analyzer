# Dependency Graph

> Status: experimental (pnpm v9)

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

A detector registers a `lockfile-name -> ParseGraphFunc` map and calls the
generic `components.AttachLockfileGraph`, which honors the mode and is a no-op
when off. There is no per-ecosystem special-casing.

## Ecosystem coverage

Edges are a **lockfile** feature. They can be produced wherever the lockfile
states the resolved graph; they cannot be produced for manifest-only
ecosystems without executing the build tool.

| Ecosystem | Source | Edges possible | Status |
|-----------|--------|----------------|--------|
| pnpm | `pnpm-lock.yaml` (snapshots, v9) | yes | implemented |
| npm | `package-lock.json` v2/v3 | yes | not yet |
| yarn | `yarn.lock` | yes | not yet |
| Cargo | `Cargo.lock` | yes | not yet |
| poetry | `poetry.lock` | yes | not yet |
| uv | `uv.lock` | yes | not yet |
| NuGet | `packages.lock.json` (when present) | conditional | not yet |
| Go | `go.mod` | direct only (full graph needs `go mod graph`) | not yet |
| Maven | `pom.xml` | no static graph | see below |
| Gradle | `build.gradle` | no static graph | see below |

## Manifest-only ecosystems (Maven, Gradle) -- future

Maven and Gradle manifests do not contain a resolved transitive graph. Two
forward paths, both out of scope for the initial feature:

1. **Ingest an externally generated tree file.** This already exists in spirit:
   the `maven_dependency_list` parser ingests `mvn dependency:list` output (a
   flat resolved list of nodes). The graph equivalent would ingest
   `mvn dependency:tree` (nodes + edges) and `gradle dependencies`. This keeps
   the scanner offline and reuses the existing "ingest a generated artifact"
   pattern -- the operator runs the build tool, the scanner reads its output.

2. **Online resolution (deps.dev).** Query the deps.dev API by PURL to obtain a
   precomputed resolved graph. This crosses the offline boundary (a network
   call, data leaving the perimeter) and so would be an opt-in enrichment, not
   the default. Noted as an option, not a recommendation.
