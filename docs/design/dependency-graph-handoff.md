# Dependency Graph -- Handoff (extending to more lockfiles)

> Status: handoff note | Branch: `feature/dep-processing-trivy-patterns`

The dependency-graph feature (package-to-package edges) is implemented and
working end-to-end for **pnpm v9**. This note tells the next session exactly
how to extend it to the remaining lockfile ecosystems. Read
`docs/design/dependency-graph.md` first for the design and `--dependency-graph`
modes.

## What is already done (do not redo)

- `types.DependencyEdge` (`{from, to}`) and `Payload.DependencyEdges`
  (`internal/types/payload.go`).
- `types.DependencyGraphMode` enum (`off`/`direct`/`full`) +
  `types.ParseDependencyGraphMode` (`internal/types/payload.go`).
- The `--dependency-graph` flag + `dependency_graph` config key + validation
  schema entry + registry accessor
  (`components.SetDependencyGraphMode` / `DependencyGraphMode`). Default off.
- The generic wiring: `components.AttachLockfileGraph`
  (`internal/scanner/components/graph.go`) and the `parsers.ParseGraphFunc`
  contract + `parsers.LockGraph` (`internal/scanner/parsers/graph.go`).
- Aggregator collects + dedups edges (`collectDependencyEdges`).
- pnpm implementation: `parsers.ParsePnpmLockGraph` (mode-aware) and the nodejs
  detector registers it via `lockfileGraphProducers`.

## The pattern to follow for each new ecosystem

Each lockfile parser becomes a `ParseGraphFunc` and the owning detector
registers it. Two steps per ecosystem:

### 1. Write a mode-aware `ParseXxxGraph`

Signature (the `ParseGraphFunc` contract):

```go
func ParseXxxLockGraph(content []byte, mode types.DependencyGraphMode) parsers.LockGraph
```

Rules (copy pnpm's structure in `internal/scanner/parsers/pnpm_lock.go`):

- Always set `LockGraph.Dependencies` from the existing flat parser (reuse it).
- `mode == DependencyGraphOff`: return with `Edges == nil`.
- `mode == DependencyGraphDirect`: emit only root -> direct edges. Use the
  manifest's declared deps as the `to` set; `from` is the synthetic root `"."`.
- `mode == DependencyGraphFull`: emit every package -> dependency edge stated
  by the lockfile.
- Node identity is `"name@version"`. Normalize away any decorations (peer-dep
  suffixes, registry prefixes) -- see pnpm's `pnpmNodeID`.
- Edges are **read, not resolved**. Do not run the package manager or match
  version ranges.

### 2. Register it in the detector

In the owning detector's dependency-processing path, add the lockfile to its
producers map and call the generic helper (already done for nodejs):

```go
var lockfileGraphProducers = map[string]parsers.ParseGraphFunc{
    "package-lock.json": parsers.ParsePackageLockGraph,
    "pnpm-lock.yaml":    parsers.ParsePnpmLockGraph,
    "yarn.lock":         parsers.ParseYarnLockGraph,
}
// ...
components.AttachLockfileGraph(payload, currentPath, provider, producers)
```

For detectors that do not yet call `AttachLockfileGraph` (python, rust,
java/maven, dotnet, etc.), add the call once in their dependency path, with a
producers map for that ecosystem's lockfiles.

> Note: `AttachLockfileGraph` tries producers in map order and stops at the
> first lockfile found. For ecosystems with a lockfile **priority** (npm:
> package-lock > pnpm > yarn), pass them so the highest-priority lockfile that
> exists wins -- align with how `extractDependenciesFromLockFiles` already
> prioritizes. If strict ordering matters, extend the helper to take an ordered
> slice instead of a map (small change; do it when the first multi-lockfile
> ecosystem needs it).

## Per-ecosystem implementation notes

| Lockfile | Where edges live | Notes |
|----------|------------------|-------|
| `package-lock.json` (npm) | v3 `packages[path].dependencies` (+ `peer/optional`) | We already parse `packages` in `npm_lock.go` (`parsePackagesV3`). The dep map per package is the edge set. Node id = `name@version` from the resolved entry. v2 uses nested `dependencies` -- prefer v3. |
| `yarn.lock` | each entry's `dependencies:` block | Classic yarn (v1) is a custom format already parsed in `yarn_lock.go`; each resolved entry lists its deps with ranges -- resolve the range to the locked entry to get `name@version`. |
| `Cargo.lock` | `[[package]] dependencies = ["name version", ...]` | Already parsed in `cargo_lock.go` (`parseCargoLockPackages`). The `dependencies` array is the edge list; entries are `"name"` or `"name version"`. |
| `poetry.lock` | `[package.dependencies]` sub-tables | `poetry_lock.go` currently parses name->version only; needs sub-table parsing. Match each dep range to the locked version (see `matchVersion`-style logic) to form `name@version`. This is the most parser work. |
| `uv.lock` | `[[package]] dependencies` | `uv_lock.go` parses packages; add the per-package `dependencies` list as edges. |
| `pnpm-lock.yaml` | v9 `snapshots` | DONE -- reference implementation. |

## Validation (do this for each ecosystem)

1. Unit test mirroring `TestParsePnpmLockGraph_Modes` and `_V9Edges`:
   off=0 edges, direct=root edges, full=transitive, with peer/range
   decorations stripped to clean `name@version`.
2. Real-world check: scan a product that uses that lockfile with
   `--dependency-graph full`, confirm fan-in looks sane (e.g. a common base
   library has high fan-in). Real repos under `~/Develop/petrarca` and
   `/Volumes/Data/Develop/cgm`:
   - npm: `cypher-graphdb/explorer` (pnpm) -- for npm proper, pick a repo with
     `package-lock.json`.
   - Cargo: `cgm-intelligence-platform` / `mdoc-gen7` had cargo deps.
   - poetry/uv: most Python services (`lauro/server`, `sonnet-server`) use uv.
3. `task fct` + `go test -race` on touched packages. Default-off must stay off.

## Size / safety reminders

- The full graph is large (pnpm explorer: 88 deps -> 2442 edges, ~28x). Keep
  the feature **off by default**. Never emit edges unless the mode is set.
- For very large monorepos, `full` could be hundreds of thousands of edges.
  `direct` mode exists as the cheap middle ground -- make sure each ecosystem
  honors it.

## Out of scope (separate future work)

- Maven / Gradle: no static resolved graph. Future path is ingesting
  `mvn dependency:tree` / `gradle dependencies` output (mirrors the existing
  `maven_dependency_list` parser which ingests `mvn dependency:list`), or
  online deps.dev resolution. See `docs/design/dependency-graph.md`.
- A consumer that projects these edges into a graph (Lauro techstack graph) --
  the producing side (this feature) is independent and can land first.

## Branch state at handoff

All on `feature/dep-processing-trivy-patterns`. `task fct` passes; all three
input paths (default/flag/config) validated. New files: `parsers/graph.go`,
`components/graph.go`, `docs/design/dependency-graph.md`, this handoff. Modified:
types, registry, config (settings + scan_config + validation schema), nodejs
detector, pnpm parser (+ test), scan/scan_enhance wiring, usage docs, examples.
