# Dependency Graph -- Parser Improvement Plan

> Status: plan (one item done) | Informed by a review of Trivy's parsers
> (`pkg/dependency/parser/*`) against our graph producers.

A comparison of our package-to-package edge producers with Trivy's dependency
parsers. Trivy is the reference for resolved-graph fidelity. This note records
where we match, the one correctness bug we found and fixed, and the remaining
improvements ranked by value.

## Model comparison (no change needed)

- Trivy stores edges as `DependsOn []string` adjacency lists keyed by package
  (a `[]Dependency{ID, DependsOn}` slice). We use flat `{from, to}` edges. The
  two are equivalent; our flat form is a better fit for projecting into a graph
  store, and our aggregator already dedups + sorts. Keep ours.
- Trivy sets a `Relationship` enum (`Root/Workspace/Direct/Indirect`) per
  package. We approximate `direct` as "any node not referenced by another." See
  improvement #2.

## Confirmed equivalent (validated against Trivy source)

- **npm v3**: our `npmResolveDep` nearest-wins node_modules walk matches Trivy's
  `findDependsOn` exactly.
- **Cargo**: Cargo.lock self-disambiguates -- a dependency reference is bare
  (`"serde"`) only when the package is locked at a single version, and carries
  the version (`"serde 1.0.2"`) when multiple versions exist. Our `cargoNodeID`
  handles both (and the old 3-field `"name ver (source)"` form). A bare-name to
  single-version map is therefore always safe. No bug.
- **uv**: Trivy also uses a single-version `map[name]Package` for edge targets.
  Our single-version map matches Trivy's behavior. Acceptable.

## Done: poetry multi-version resolution (correctness fix)

**Bug:** poetry.lock can lock the same package at multiple versions (e.g. numpy
`1.24.4` and `1.26.4` for different Python versions). Our old line-based parser
collapsed each name to a single version, so edges pointed at an arbitrary
version and dropped the others.

**Fix (implemented):** `poetry_lock_graph.go` now TOML-decodes the lockfile,
keeps `map[name][]version`, and resolves each dependency range against every
locked version using PEP 440 matching (`aquasecurity/go-pep440-version`),
emitting an edge to each satisfying version. It handles all three poetry
dependency shapes: string range, table (`{version = "..."}`), and array of
tables with environment markers. Unmatched/unparseable ranges fall back to
keeping all versions (over-approximate rather than drop -- safer for
blast-radius). Validated on nicegui: matplotlib now links to BOTH numpy
versions; a strict `<1.25` range correctly excludes `1.26.4`.

## Done since this plan was written

- **TOML decoders for cargo and uv graph parsers** -- both now use
  `BurntSushi/toml` instead of hand-parsing.
- **Manifest-driven `direct` mode** -- the contract now passes the component
  manifest (`GraphInput.Manifest`); cargo/poetry/yarn derive direct edges from
  declared deps (accurate even when a direct dep is also transitive). npm/pnpm/
  uv/maven already used real roots.
- **Scope on edges** -- `DependencyEdge.Scope` (prod/dev/optional/peer) is set
  on direct edges across ecosystems.
- **Edge validation** -- producers report `Unresolved` references (surfaced as
  the `dependency_graph.unresolved` property) instead of dropping silently.
- **New ecosystems** -- Ruby, PHP, NuGet, Go, Gradle, and CycloneDX ingest.

## Remaining improvements (ranked)

### 1. pnpm/yarn workspace relationship (low)

Trivy has a `RelationshipWorkspace` and a TODO to use it for cargo/npm
workspaces. Monorepo workspace links currently appear as ordinary edges. Tagging
workspace edges would let consumers distinguish intra-repo links from external
dependencies.

## Known limitations (documented, by design)

- **Environment markers are not evaluated.** When a poetry dependency has
  marker-gated constraints (array of tables), we emit edges for every locked
  version that satisfies any branch, rather than picking the version for a
  specific Python/OS. This is intentional: the graph captures the full reachable
  surface for blast-radius, not a single resolved environment.
- **No range resolution beyond what the lockfile states.** We read locked
  versions; we never run the package manager. (Maven's resolved graph therefore
  comes from a pre-generated `dependency-tree.json`, and online resolution via
  deps.dev is the opt-in fallback -- see `dependency-graph.md`.)
