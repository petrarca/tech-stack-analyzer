# Maven (and Cross-Manifest) Version Resolution

How the scanner should close the **version gap** in SBOMs: dependencies emitted
without a resolved version, which are invisible to advisory matching (Trivy,
OSV). This document records the root-cause analysis behind the gap, compares the
scanner's behaviour with Trivy's, and lays out a staged plan to close it while
preserving the scanner's offline-by-default guarantee.

## Background: the version gap

A component is only useful to a vulnerability scanner when its PURL carries a
**resolved** version (`pkg:maven/group/artifact@1.2.3`). The SBOM emitter
(`internal/sbom/cyclonedx.go`) deliberately omits the version from the PURL when
the value is not a concrete release -- a range (`^1.2.0`), a tag (`latest`), an
unresolved property (`${x}`), or a placeholder. This is correct: a versionless
or range PURL cannot identify a release and breaks advisory matching.

The problem is upstream of the emitter: too many dependencies arrive at the
emitter **without** a resolved version, so they are emitted versionless and are
invisible to scanning.

### Observed gap (reference data set: a large monorepo, 816 components)

| Ecosystem | Total | No version | % missing |
|-----------|-------|-----------|-----------|
| `pkg:maven` | 404 | 261 | 65% |
| `pkg:npm` | 211 | 94 | 45% |
| `pkg:docker` | 173 | 98 | 57% |
| `pkg:pypi` | 28 | 25 | 89% |
| **Total** | **816** | **478** | **59%** |

## Root cause

When a parser cannot determine a concrete version from the source file, it
defaults the version to the literal string `"latest"` (see `maven.go:resolveVersion`,
and the equivalent in the npm/pypi/docker/gradle/gem/cocoapods parsers). The
SBOM emitter then treats `"latest"` (and ranges, and `${...}`) as unresolved and
omits it from the PURL. The emitted gap is therefore **accurate** -- the version
genuinely is not known from the source as currently parsed. The fix is to
**resolve more versions before they reach the emitter**, not to change the
emitter.

There are four distinct causes, one per ecosystem family:

### 1. Maven -- cross-POM `dependencyManagement` not applied (261, biggest)

A versionless `<dependency>` in one POM is **not** matched against a managed
version declared in another POM (a parent POM or an imported BOM). The parser
processes each POM in isolation:

- `maven.go` parses `<dependencyManagement>` only to extract BOM *imports*
  (`scope=import`, `type=pom`); it does **not** build a managed-version table and
  apply it to versionless `<dependency>` entries.
- It resolves `${property}` references only within a **single** POM's property
  scope, so a property defined in a parent/BOM POM stays unresolved.

Illustrative pattern (fictional coordinates):

```
services/example-service/pom.xml   <dependency> com.example/example-lib   (no <version>)
bom/pom.xml                        <example.lib.version>7.6.0</...>
                                   manages com.example/example-lib -> ${example.lib.version}
```

`com.example/example-lib` is emitted versionless today, but `7.6.0` is **fully
determinable offline** from the repo's own POMs by cross-POM `dependencyManagement`
+ parent + property resolution. This is the single largest, deterministic,
network-free win.

A second class within Maven is genuinely unresolvable offline:

- **Private/internal artifacts** (e.g. `com.example.internal/*`) whose versions
  live in a parent/BOM POM that is **not in the repo** (published to a private
  registry). These cannot be resolved offline and cannot be resolved via
  deps.dev (not public). They remain a documented gap.

### 2. npm -- workspace lock not associated with nested `package.json` (94)

The versionless npm components carry **range** versions (`^6.1.20`, `~2.1.2`)
read from `package.json`. The repo is a workspace/monorepo: nested
`packages/*/package.json` declare ranges, and a single hoisted root
`yarn.lock` resolves them. The scanner resolves a `package.json` against a lock
file only when the lock is **adjacent** (same directory), so the nested
manifests never see the parent workspace lock.

Illustrative pattern: a nested package such as `@scope/example-pkg` is emitted
versionless (range `^6.1.20`) but **is present in the workspace-root
`yarn.lock`** with a resolved version. This is fixable offline by associating
nested workspace manifests with the nearest ancestor lock file.

### 3. Docker -- unpinned / templated / private-registry tags (98)

`FROM image` without a tag, `FROM ${base_image}` (CI-injected), and private
registry references (e.g. `myregistry.example.com/...`) have no version in
source. Some are an upstream-Dockerfile fix (pin the tag);
the templated/private ones are an architectural pattern (resolved only at
build/deploy time) and are **not** fixable from source. Documented gap.

### 4. PyPI -- unpinned requirements (25)

`requirements*.txt` without `==` pins, or `pyproject.toml` ranges with no
committed `uv.lock` / `poetry.lock` / `requirements.lock`. Fixable when a lock
exists (associate it); otherwise an upstream pinning fix. Mostly minor.

## Pre-generated resolved files (most authoritative, when present)

Maven cannot produce a fully resolved graph statically -- conflict mediation,
version ranges, and cross-POM `dependencyManagement` require Maven itself. The
scanner therefore reads two **pre-generated** files when a user/CI committed
them (the analyzer never runs Maven):

| File | Command | What it carries | Used today for |
|------|---------|-----------------|----------------|
| `dependency-list.txt` | `mvn dependency:list -DoutputFile=...` | resolved flat versions (LATEST/RELEASE/`${...}` already resolved) | flat versions -> SBOM (overrides pom.xml) |
| `dependency-tree.json` | `mvn dependency:tree -DoutputType=json -DoutputFile=...` | resolved graph (post-mediation) | graph **edges only** |

These resolved files are the **most authoritative** source and must take
precedence over any static or online resolution -- they are Maven's own output.
**But in practice they are rarely committed** (they are CI-only build
artifacts), so they cannot be relied on to close the gap on their own. They are
the top of the precedence chain when present, with the offline static resolution
(Stage 1) as the fallback when they are absent.

Current precedence for the **flat** dependency list (what the SBOM emits), from
`internal/scanner/components/java/detector.go`:

1. `dependency-list.txt` if present (overrides pom.xml versions).
2. Otherwise pom.xml only -- with the cross-POM gap described above.

Gap in the current handling: when only `dependency-tree.json` is committed (no
`dependency-list.txt`), its resolved versions are used **only for graph edges**
and are **not** harvested back into the flat dependency list, so the SBOM does
not benefit from them. Stage 1 should also harvest flat resolved versions from
`dependency-tree.json` when present.

## How Trivy resolves Maven (comparison)

Source: `pkg/dependency/parser/java/pom/parse.go` in the Trivy repo.

Trivy resolves Maven versions by **reconstructing Maven's resolution**:

1. Builds a `dependencyManagement` table from the root POM and applies it to
   transitive dependencies (`resolveDepManagement`).
2. Follows the **parent POM** chain and **modules**, inheriting properties,
   `dependencyManagement`, and repositories.
3. **Fetches** parent/BOM/transitive POMs that are not local -- from the local
   `~/.m2/repository` cache and from remote Maven repositories over HTTP.

Steps 1-2 operate on POMs **already available** (the repo's own POMs + parents
that happen to be present). Step 3 crosses the network/offline boundary.

The scanner can replicate **steps 1-2 offline** from the repo's POMs (this is
the 261 Maven win). Step 3 -- fetching parent/BOM POMs that live in a registry
-- is the online piece, and for **public** coordinates it overlaps with what
deps.dev already provides. It will never resolve **private** coordinates.

## Reference implementation (Trivy)

Trivy is the reference implementation for ecosystem-faithful version resolution.
Its source is available locally for direct inspection while implementing each
stage. Consult it for the resolution algorithm and edge cases; do **not** copy
its network-fetching behaviour into the offline stages (that belongs only in the
opt-in online stage). Paths are relative to the Trivy repo root.

| Stage | Trivy reference | Functions / notes |
|-------|-----------------|-------------------|
| 1 (Maven) | `pkg/dependency/parser/java/pom/parse.go` | `parseRoot` (orchestration), `resolveDepManagement` + `mergeDependencyManagements` (managed-version table), `parseDependencies` (apply managed version to versionless deps), `resolveParent`/`parseParent`/`retrieveParent` + `mergeProperties`/`mergeDependencies` (parent chain + property/dep inheritance). **Offline-relevant** parts: everything that operates on already-available POMs. **Skip for offline**: `tryRepository`, `loadPOMFromLocalRepository` (`~/.m2`), `fetchPOMFromRemoteRepositories` (HTTP). |
| 1 (Maven, properties/ranges) | `pkg/dependency/parser/java/pom/pom.go`, `version.go`/`var.go` | property interpolation and Maven version-range handling. |
| 2 (npm workspace) | `pkg/fanal/analyzer/language/nodejs/{npm,yarn,pnpm}/*.go`, `pkg/dependency/parser/nodejs/{npm,yarn,pnpm,packagejson}/` | how a workspace `package.json` is associated with the root lock and ranges are resolved to locked versions. |
| 2 (PyPI lock) | `pkg/fanal/analyzer/language/python/{poetry,uv,pip,pipenv}/`, `pkg/dependency/parser/python/` | lock-file association and constraint -> resolved version. |

Trivy gates its network access behind an offline option (`WithOffline` in
`parse.go`); the scanner's equivalent boundary is offline-by-default with the
opt-in `--resolve-online` flag (see below). Use Trivy's offline code paths as
the model for Stages 1-2.

## deps.dev (online) -- what it can and cannot do

The scanner already has an opt-in online resolver (`--resolve-online`,
`internal/scanner/resolver/depsdev.go`) backed by deps.dev. Two limits matter
for the version gap:

1. **It resolves the transitive graph of an already-versioned coordinate.** The
   resolver skips any dependency with an empty version
   (`if dep.Name == "" || dep.Version == "" { continue }`) and queries
   `.../versions/{version}:dependencies`. It does **not** backfill a *missing*
   version. To make it close the gap, a separate step must first pick a concrete
   version (e.g. the latest published release for a range) via a
   `GetVersion`/`GetPackage`-style lookup, then feed that into resolution.
2. **It only covers published, public packages** (npm, PyPI, Cargo, Maven on
   deps.dev). Private/internal artifacts (`com.example.internal/*`, private
   registries) return 404 and are skipped. **Online resolution can never close
   the private-package gap** -- by design, those packages are not on deps.dev.

## Offline/online boundary (design constraint)

The scanner is **offline by default**; any network access is opt-in and
explicit. The staged plan respects this:

- **Offline stages** (1, 2) run by default, are deterministic, and need no
  network. They resolve from the repo's own files only.
- **Online stages** (3) run only under `--resolve-online` (and the existing
  `--resolve-online-endpoint` override for a mirror/facade). They resolve only
  public coordinates and degrade gracefully (404 -> skip) for private ones.

This keeps the single-binary, offline guarantee intact while letting operators
opt into broader coverage when a network/VPN is available.

### Resolution precedence (highest to lowest)

1. **Committed resolved files** (`dependency-list.txt`, `dependency-tree.json`)
   -- Maven's own output; authoritative but rarely present. *(Stage 0)*
2. **Offline static cross-POM resolution** -- managed versions + parent +
   property interpolation from the repo's own POMs / workspace locks. *(Stages
   1-2, default)*
3. **Online backfill** (deps.dev / mirror) -- public coordinates only, opt-in.
   *(Stage 3, `--resolve-online`)*
4. **Unresolved** -- left versionless (range/`latest`/`${...}`); emitted without
   a PURL version and documented as a known gap.

## Staged plan

Implement step by step; each stage is independently shippable and testable, and
each online stage is gated behind `--resolve-online`.

### Stage 0 -- Use committed resolved files first (already partly present)

- Keep `dependency-list.txt` as the top-priority flat-version source (already
  done).
- **Add**: when `dependency-tree.json` is present, harvest resolved flat
  versions from it into `payload.Dependencies` (not just edges), so the SBOM
  benefits even when no `dependency-list.txt` exists.
- These files win over Stage 1 static resolution. They are rarely committed, so
  Stage 1 remains necessary as the offline fallback.

### Stage 1 -- Offline cross-POM Maven `dependencyManagement` + parent + BOM imports (implemented)

- Applies only to dependencies still versionless after Stage 0.
- Build a managed-version table (`groupId:artifactId -> resolved version`) from:
  - the POM's own `<dependencyManagement>` (and active profiles');
  - **parent POMs** reachable through the provider (climbing the chain);
  - **imported BOMs** (`scope=import`, `type=pom`) located in the repo.
- Resolve `${property}` references across the merged parent/property scope.
- Apply managed versions to versionless `<dependency>` entries; never overwrite
  a concrete version. Record the declared form in `metadata.declared` and the
  origin in `metadata.source = "dependency-management"`.
- **BOM-import resolution (the dominant real-world case)**: a child's version
  often lives in a sibling BOM module imported by an ancestor POM, addressed by
  Maven *coordinates*, not a path. To resolve this offline we built a generic,
  ecosystem-agnostic **source index** (`components.SourceIndex`) that maps
  `(ecosystem, coordinate) -> manifest path` by walking the scanned tree once
  (cached per scan). The Maven detector injects a `parsers.BomResolver` backed
  by this index; the parser stays free of any repository/index knowledge.
  Nested and inherited BOMs are followed, with a visited-coordinate set
  guarding against import cycles.
  - The source index is intentionally reusable beyond Maven (e.g. npm/Go/Python
    workspace-member resolution): ecosystems opt in via
    `components.RegisterSourceIndexer`.
- **Import-scope entries are not packages**: the parser keeps `scope=import`
  BOMs as a tech-detection signal, but the SBOM emitter now excludes them
  (they declare no artifact). This matches Maven semantics and Trivy, which
  never emits import-scope entries as packages.
- Private artifacts managed only in a non-repo parent/BOM remain versionless
  (expected; documented gap).
- Reference: Trivy's offline POM resolution (`resolveDepManagement` follows
  `scope=import` BOMs via the same artifact resolver; `parseDependencies` emits
  only `compile`/`runtime` non-optional deps as packages). We mirror the
  algorithm, replacing Trivy's `~/.m2`/remote fetch with the repo source index.

### Stage 2 -- Offline npm workspace lock association (implemented)

- Associate a nested `package.json` with the nearest ancestor lock file
  (`package-lock.json` / `yarn.lock`) when no adjacent lock exists, resolving
  the member's declared ranges to the locked version. The climb is bounded to
  the scan root so a member never picks up an unrelated lock outside the
  workspace. Resolution origin is recorded as `metadata.source =
  "workspace-lock"`.
- Expected impact: the npm workspace/monorepo case (nested members declaring
  ranges while a single hoisted root lock holds the resolved versions).
- Reference: Trivy's nodejs analyzers (see the table above) for workspace/lock
  association.

Python is intentionally **not** handled here: uv/poetry locks sit adjacent to
their `pyproject.toml`, so there is no ancestor-lock pattern to resolve. The
remaining PyPI gap is genuinely unpinned sources (`requirements.txt` without
`==`, `pyproject.toml` ranges with no committed lock) -- addressed by online
backfill or an upstream pinning fix, not by lock association.

### Stage 3 -- Online third-party BOM resolution (opt-in, public packages only, implemented)

- Under `--resolve-online`, when a `scope=import` BOM is **not** in the scanned
  tree (e.g. the Quarkus or Spring BOM), fetch its published POM from a Maven
  repository (Maven Central by default) and read its managed versions. This
  yields the **exact** versions the build uses, not a guessed "latest".
- Implemented as an *online `BomResolver`* composed after the offline source
  index: the same Stage-1 resolution logic handles both, so nested imports
  (internal BOM in repo -> public BOM online) resolve transparently. Verified:
  an in-repo BOM that imports a public platform BOM via a `${...}` version
  property resolves the property, fetches the BOM, and pins the managed
  artifacts to their exact versions.
- Distinct from the deps.dev resolver already in the tree: deps.dev returns a
  resolved **dependency graph** for an *already-versioned* coordinate (it cannot
  supply a missing version); the Maven POM fetch returns the **raw BOM** whose
  `<dependencyManagement>` provides the missing versions. They share only the
  `--resolve-online` gate, not the endpoint (the POM repo is a different API).
- Strictly public-only: a 404 leaves the dependency versionless. Private BOMs
  (not on a public repository) are never resolvable this way and remain a
  documented gap.
- Network access is build-tag-free in production (off unless `--resolve-online`)
  with a mocked-HTTP unit test and a `//go:build online` live test against
  Maven Central (`task test:online`).

### POM source chain (`internal/scanner/mavenresolve`)

BOM-import and parent-POM resolution needs to *locate* a POM by coordinates.
The places a POM can come from are encapsulated behind a `PomSource` seam
(mirroring `internal/scanner/resolver`, which applies the same precedence-chain
pattern to graph edges), composed into a `Chain` tried in order:

1. `RepoSource` -- BOMs committed to the scanned tree, via the components source
   index (offline, no network).
2. `LocalRepoSource` -- the local `~/.m2/repository` cache (offline, no
   credentials). Opt-in (`--maven-local-repo`) since it reads outside the
   scanned tree. The repo path follows Maven: `MAVEN_REPO_LOCAL`, then
   `-Dmaven.repo.local` in `MAVEN_OPTS`, then `~/.m2/repository`
   (`--maven-local-repo-dir` overrides).
3. `RemoteSource` -- a Maven repository over HTTP. Opt-in (`--resolve-online`);
   defaults to **Maven Central**, overridable to a mirror or internal
   Artifactory/JFrog virtual repo via `--maven-repo-url`. Optional bearer-token
   auth (`STACK_ANALYZER_MAVEN_TOKEN`, env-only) resolves private artifacts. A
   JFrog URL is the base up to and including the repo key, e.g.
   `https://host/artifactory/<repo>`; the coordinate path is appended.

Each tier degrades gracefully (a miss or a transient 429/5xx falls through; only
a definitive 404 is cached) and never aborts the scan. The chain is adapted to
the parser's `BomResolver` hook, so the parser stays free of repository/index
knowledge.

### Transitive components in the SBOM (implemented)

By default the SBOM contains the **declared** dependencies. When the scan
resolved a dependency graph (`--dependency-graph full`), the SBOM emitter also
folds the graph's **transitive nodes** in as components, deduplicated against
the declared set. Edge nodes carry resolved `name@version` identities; the
owning component's ecosystem supplies the PURL type.

This gives transitive breadth **without replicating a package manager's
recursive POM/registry crawl**. The transitive graph itself comes from:
- committed lockfiles / `dependency-tree.json` (offline, full and private), or
- deps.dev under `--resolve-online`, which returns the pre-computed resolved
  transitive graph for a public, already-versioned coordinate (one call per
  declared dep, not a per-artifact crawl).

Why this is sufficient (vs. Trivy's approach): Trivy reconstructs the transitive
tree by fetching every dependency's POM, which is broad but brittle (on a large
monorepo it hit a Maven Central 429 rate-limit and aborted the whole scan).
deps.dev already provides the resolved graph for public coordinates, so we get
the transitive set with far fewer requests and never abort on rate-limiting.
Measured on a large monorepo: declared-only ~360 Maven components (~58%
versioned) vs. online + full graph ~1560 components (~90% versioned). The
residual gap to a full POM crawl is dominated by private subtrees that no public
source can resolve.

### Out of scope (documented gaps, not closable from source)

- Docker templated/private tags (`${base_image}`, private registries).
- Private Maven artifacts whose versions are not in any repo POM.
- Unpinned Docker/PyPI with no lock and no upstream pin (upstream fix).

## Verification

For each stage, on the reference data set:

- Re-scan and re-count versionless PURLs per ecosystem (the breakdown table
  above); record the reduction.
- Spot-check known cases (a BOM-managed artifact resolving to its managed
  version; a nested workspace package resolving from the workspace-root lock).
- Confirm no regression in offline mode (stages 1-2 must not require network).
- Run `trivy sbom` before/after and compare the count of components that Trivy
  is able to evaluate (versionless components are skipped by Trivy).
- Standard `task fct` and `go test -race ./...`.
