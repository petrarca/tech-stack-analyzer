# Maven (and Cross-Manifest) Version Resolution

Design of how the scanner closes the **version gap** for Maven: dependencies
emitted without a resolved version, which are invisible to advisory matching
(Trivy, OSV match on a concrete version). It covers the root cause, the
resolution model and its sources, transitive resolution, and the offline-first
boundary. For user-facing flags and recipes see the [Maven guide](../maven.md);
for the broader edge model see [Dependency Graph](dependency-graph.md).

## The version gap

A component is useful to a vulnerability scanner only when its PURL carries a
**resolved** version (`pkg:maven/group/artifact@1.2.3`). The SBOM emitter omits
the version when the value is not a concrete release -- a range (`^1.2.0`), a
tag (`latest`), an unresolved property (`${x}`), or a placeholder -- because a
versionless or range PURL cannot identify a release.

The gap is therefore created **upstream** of the emitter: a dependency reaches
it without a resolved version. Maven uniquely produces many such cases because a
`<dependency>` routinely omits its version, inheriting it from:

- a `<dependencyManagement>` entry in the same POM,
- a **parent POM** in the inheritance chain,
- an **imported BOM** (`<scope>import</scope>`, `<type>pom</type>`), or
- a `${property}` resolved elsewhere in that chain.

The scanner's job is to resolve those versions before emission. The emitter is
correct as-is; resolution happens before it.

## Resolution model

Versions are resolved offline-first, by locating the POM that manages a
coordinate's version. Sources are tried in precedence order (first hit wins);
each degrades gracefully, and a network error or rate limit never aborts a scan.

1. **Committed resolved files** -- `dependency-list.txt` (`mvn dependency:list`)
   or `dependency-tree.json` (`mvn dependency:tree`). These are Maven's own
   output and the most authoritative; the analyzer never runs Maven, it only
   reads what was committed. Rarely present, so not relied upon alone.
2. **The repo's own POMs** -- cross-POM `dependencyManagement`, parent chains,
   and imported BOMs reachable within the scanned tree. Fully offline.
3. **Cross-module propagation** -- a coordinate resolved in one module fills the
   same coordinate left versionless in a sibling module. Fully offline.
4. **Local `~/.m2/repository`** -- previously built/downloaded POMs.
5. **A configured Maven repository** -- an internal Artifactory/JFrog repo or
   mirror; covers **private** artifacts.
6. **Maven Central** -- the public fallback.

Steps 1-3 need no configuration; 4-6 are opt-in. A coordinate still unresolved
after all sources is emitted versionless (a documented gap).

### Resolved value and provenance

When a version is resolved, the concrete value goes in the dependency's
`version`; the originally declared form (range / `${...}` / empty) is preserved
in `metadata.declared`, and `metadata.source` records the origin
(`dependency-management`, `cross-module`, `workspace-lock`, ...). This follows
the [Dependency Resolution Model](dependency-resolution-model.md): declared and
resolved are kept as distinct facts.

### Import-scope BOMs are not packages

A `<scope>import</scope>` BOM is a version-management entry, not an artifact.
The parser keeps it as a tech-detection signal, but the SBOM emitter excludes
import-scope entries -- they have no resolvable artifact to scan. (Maven and
Trivy likewise never emit import-scope entries as packages.)

## POM source chain

Resolution needs to *locate* a POM by coordinates. The places a POM can come
from are encapsulated behind a single `PomSource` seam and composed into a
precedence `Chain` (`internal/scanner/mavenresolve`), mirroring the
edge-resolver pattern in `internal/scanner/resolver`:

```
in-repo source index  ->  local ~/.m2  ->  configured repo / settings.xml  ->  Maven Central
```

- **in-repo** -- BOMs/parent POMs committed to the scanned tree, located via a
  generic, ecosystem-agnostic source index (`coordinate -> manifest path`, built
  once per scan; reusable by other ecosystems). Offline.
- **local `~/.m2`** -- the local cache, holding individual POMs. Offline; opt-in
  because it reads outside the scanned tree. Path follows Maven
  (`MAVEN_REPO_LOCAL`, `-Dmaven.repo.local` in `MAVEN_OPTS`, then
  `~/.m2/repository`).
- **configured repo** -- an HTTP Maven repository (internal Artifactory/JFrog
  virtual repo, or a mirror). Resolves private artifacts. Credentials come from
  the environment or `settings.xml` (HTTP Basic, or Bearer with a bare token).
- **Maven Central** -- public fallback, opt-in.

The chain is the single mechanism for *every* POM lookup: the same chain
resolves a versionless dependency's managing BOM, climbs parent POMs (by
relative path in-repo, by coordinate when published), follows nested/imported
BOMs (with a visited-coordinate cycle guard), and -- for the transitive graph --
fetches each dependency's POM. The parser stays free of repository knowledge by
receiving the chain through a small resolver hook.

### Configuration collapses to one internal view

Repository URLs, credentials, and the local-repo path may come from a Maven
`settings.xml` (repositories with `<server>` credentials by id, `<mirrors>`
including `mirrorOf` semantics, `<localRepository>`), from scanner flags/config,
or from the environment (secrets only). These **merge into one internal
configuration** the chain reads; the resolver never sees the individual inputs.
The merged remote order is: settings.xml repos -> configured repo URL -> Maven
Central. A repository that lacks a coordinate (e.g. a virtual repo returning 404
for a public BOM) simply falls through to the next.

### Online sources are gated independently

The two public online sources are separate concerns with separate switches (no
single umbrella):

- **Maven Central** -- public fallback for version (BOM/parent) resolution.
- **deps.dev** -- transitive dependency-graph edges.

An **explicitly configured** Maven repository (repo URL or settings.xml repos)
is always used -- configuring it *is* the opt-in, no separate online flag. When
one is set, Maven Central is **not** also appended: an internal virtual repo
already proxies public artifacts, so reaching Central too would be redundant and
an egress surprise.

## Transitive dependencies

By default the SBOM lists **declared** dependencies. When a dependency graph is
resolved (`--dependency-graph full`), the emitter folds the graph's transitive
nodes in as components, deduplicated against the declared set (edge nodes carry
resolved `name@version`; the owning component's ecosystem supplies the PURL
type).

Maven transitive edges can come from two sources, selected per ecosystem (a
committed `dependency-tree.json` always takes precedence over both):

- **Repository crawl** -- recursively fetch each dependency's POM through the
  POM source chain, read its `<dependencies>`, and walk. This is the only source
  that covers **private** artifacts (they are in the internal repo / `~/.m2`,
  never on deps.dev), and it needs no Maven Central. It reproduces Maven's
  resolution: breadth-first with **nearest-wins** conflict mediation (one
  version per coordinate per module, edges rewritten to the mediated version),
  transitive-scope filtering (`test`/`provided`/`optional`/`import` excluded),
  and a visited-set cycle guard. An unreachable POM yields no children and never
  aborts the scan.
- **deps.dev** -- the pre-computed resolved graph for a public, already-versioned
  coordinate (one request per declared dependency, not a per-artifact crawl).
  Covers public coordinates only.

The two are not combined within one tree (different walk models -> inconsistent
mediation). The selector is `--maven-graph-source=repo|deps-dev|none`, defaulting
to the global `--deps-dev` toggle; the same per-ecosystem-override pattern can
extend to other ecosystems later.

### Why a crawl is acceptable here (vs. Trivy)

Trivy resolves Maven by recursively fetching every dependency's POM from
repositories. That is faithful but brittle against the public Maven Central
rate limiter -- on a large monorepo it can abort the whole scan on a 429. The
scanner's crawl rides the same offline-first source chain: it prefers in-repo
and `~/.m2`, targets an internal repository (no public rate limit) when
configured, and treats a 429/5xx as a fall-through rather than a fatal error.
For public-only environments without a configured repo, deps.dev's pre-computed
graph avoids the crawl entirely. The transitive sets produced match Trivy's on
the same inputs.

## Offline-by-default boundary

The scanner is offline by default. Reading outside the scanned tree (local
`~/.m2`) and any network access (configured repo, Maven Central, deps.dev) are
explicit opt-ins. With no Maven flags, only the repo's own POMs, committed
resolved files, and cross-module propagation are used -- deterministic and
network-free.

## Other ecosystems (brief)

The same "resolve before emit" principle applies elsewhere, by ecosystem-native
means:

- **npm** -- a nested workspace `package.json` (declaring ranges) is associated
  with the nearest ancestor lock file (`package-lock.json`/`yarn.lock`) up to
  the scan root, resolving ranges to the locked version.
- **PyPI, Cargo, etc.** -- resolved from committed lockfiles adjacent to the
  manifest; transitive edges via deps.dev under `--deps-dev`.

## Out of scope (gaps not closable from source)

- Private Maven artifacts whose versions live only in a repository not reachable
  by any configured source.
- Docker templated/private tags (`${base_image}`, private registries) -- bound
  at build/deploy time, not present in source.
- Unpinned PyPI/Docker with no committed lock and no upstream pin.

## Verification

- Re-scan and count versionless PURLs per ecosystem before/after; confirm the
  reduction.
- Spot-check known cases: a BOM-managed artifact resolving to its managed
  version; a nested workspace package resolving from the workspace-root lock; a
  transitive subtree matching the dependency's published POM.
- Confirm offline mode requires no network.
- Compare the SBOM against `trivy sbom` / `trivy fs` output where a fair
  baseline is available (same resolved transitive sets; the scanner additionally
  resolves private artifacts when pointed at the internal repository).
