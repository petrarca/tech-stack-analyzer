# Dependency Graph -- Handoff (remaining ecosystems)

> Status: handoff note | Branch: `feature/dependency-graph`

The dependency-graph feature (package-to-package edges) is implemented and
real-repo validated for **pnpm, npm, yarn, Cargo, poetry, uv, and Maven**
(Maven via read-only ingest of a pre-generated `dependency-tree.json`). Read
`docs/design/dependency-graph.md` first for the design, `--dependency-graph`
modes, and the per-ecosystem coverage matrix.

This note tells the next session how to extend it to the **remaining**
ecosystems (Gradle, NuGet, Go) and the CycloneDX edge-section path.

## What is already done (do not redo)

- `types.DependencyEdge` / `Payload.DependencyEdges`, `types.DependencyGraphMode`
  enum + `ParseDependencyGraphMode` (`internal/types/payload.go`).
- The `--dependency-graph` flag + `dependency_graph` config key + validation
  schema entry + registry accessor. Default off.
- The generic wiring: `components.AttachLockfileGraph` taking an **ordered**
  `[]components.LockfileGraphProducer` (`internal/scanner/components/graph.go`)
  and the `parsers.ParseGraphFunc` contract + `parsers.LockGraph`
  (`internal/scanner/parsers/graph.go`). Ordering honors lockfile priority
  (first existing lockfile wins).
- Aggregator collects + dedups edges (`collectDependencyEdges`), sorted.
- Implemented parsers + detector registration:
  - pnpm: `ParsePnpmLockGraph` (nodejs)
  - npm: `ParsePackageLockGraph` (nodejs)
  - yarn: `ParseYarnLockGraph` (nodejs)
  - Cargo: `ParseCargoLockGraph` (rust)
  - uv: `ParseUvLockGraph` (python, registered before poetry)
  - poetry: `ParsePoetryLockGraph` (python)
  - Maven: `ParseMavenTreeGraph` (java; reads `dependency-tree.json`)

## The pattern to follow for each new ecosystem

Each lockfile parser becomes a `ParseGraphFunc` and the owning detector
registers it. Two steps per ecosystem:

### 1. Write a mode-aware `ParseXxxGraph`

Signature (the `ParseGraphFunc` contract):

```go
func ParseXxxLockGraph(content []byte, mode types.DependencyGraphMode) parsers.LockGraph
```

Rules (copy any implemented parser, e.g. `cargo_lock.go` or `uv_lock.go`):

- Set `LockGraph.Dependencies` from the existing flat parser when feasible
  (best-effort; the detector populates `payload.Dependencies` separately).
- `mode == DependencyGraphOff`: return with `Edges == nil`.
- `mode == DependencyGraphDirect`: emit only root -> direct edges, `from` = `"."`.
- `mode == DependencyGraphFull`: emit every package -> dependency edge stated
  by the file.
- Node identity is `"name@version"`. Strip decorations (peer suffixes, registry
  prefixes, source specs).
- Edges are **read, not resolved**. Do not run the package manager or match
  version ranges beyond what the file already states.

### 2. Register it in the detector (ordered)

```go
var lockfileGraphProducers = []components.LockfileGraphProducer{
    {Lockfile: "first-priority.lock", Parse: parsers.ParseFirstGraph},
    {Lockfile: "second.lock",         Parse: parsers.ParseSecondGraph},
}
// ... in the dependency path, once the payload exists:
components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)
```

List producers in the **same priority order** the detector uses for flat
extraction; the first lockfile that exists wins.

## Remaining ecosystems

| Ecosystem | Source | Notes |
|-----------|--------|-------|
| Gradle | pre-generated `gradle dependencies` tree | No standard machine-readable Gradle tree file. Same read-only ingest pattern as Maven: have the operator generate a tree dump, parse it. Decide a filename convention (mirror `MavenTreeFileName`). |
| NuGet | `packages.lock.json` | When present, lists resolved deps with a `dependencies` map per package. Add a `ParsePackagesLockGraph` and register in the dotnet detector. |
| Go | `go.mod` (+ `go mod graph`) | `go.mod` gives direct only. The full graph needs `go mod graph` output -- another read-only ingest (pre-generated file), not static. |
| Maven (alt) | CycloneDX `dependencies` section | `cyclonedx-maven-plugin makeAggregateBom -DschemaVersion=... dependencyGraph` emits CycloneDX with an edge section. The analyzer already emits/consumes CycloneDX (`--sbom`); reading that edge section is a natural second Maven path. |

## Validation (do this for each ecosystem)

1. Unit test mirroring the implemented `*_graph_test.go` files: off=0 edges,
   direct=root edges, full=transitive, with all decorations stripped to clean
   `name@version`.
2. Real-world check: scan a repo using that source with `--dependency-graph
   full`; confirm fan-in looks sane (a common base library has high fan-in) and
   node identities are clean.
3. `task fct` + `go test -race` on touched packages. Default-off must stay off.

## Size / safety reminders

- The full graph is large (pnpm explorer: 88 deps -> 2442 edges, ~28x). Keep
  the feature **off by default**. Never emit edges unless the mode is set.
- For very large monorepos, `full` could be hundreds of thousands of edges.
  `direct` mode is the cheap middle ground -- make sure each ecosystem honors it.

## Out of scope

- A consumer that projects these edges into a graph (Lauro techstack graph) --
  the producing side (this feature) is independent and already lands the
  aggregate `dependency_edges` array as the consumable form.
