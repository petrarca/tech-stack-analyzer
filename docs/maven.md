# Maven Dependency Resolution

Maven projects often declare dependencies **without an explicit version**: the
version comes from a parent POM, a property, or an imported BOM
(`<scope>import</scope>`). A version that the scanner cannot resolve is emitted
without one, and a versionless component is invisible to vulnerability scanners
(Trivy, OSV match on a concrete version).

The scanner resolves these versions offline-first, and -- when you point it at a
Maven repository -- can also resolve versions and **transitive** dependencies
that live only in your private artifacts. This guide shows how.

The same resolution applies to **Gradle** projects (a Gradle platform BOM is a
Maven BOM); see [Gradle dependency versions](#gradle-dependency-versions).

## TL;DR

```bash
# Offline: resolve versions from the repo's own POMs (no flags needed)
stack-analyzer scan /path/to/project --also-sbom

# Use your internal Artifactory/JFrog repo (covers private artifacts)
export STACK_ANALYZER_MAVEN_USER="you@example.com"
export STACK_ANALYZER_MAVEN_TOKEN="<access-token>"
stack-analyzer scan /path/to/project --also-sbom \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo

# Add transitive dependencies, resolved from that same repo (Trivy-style)
stack-analyzer scan /path/to/project --also-sbom \
  --dependency-graph full --maven-graph-source repo \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo
```

## How versions are resolved

The scanner resolves a versionless dependency by looking for the POM that
manages its version, in this order (first hit wins). Each tier degrades
gracefully -- a miss falls through to the next, and a network error or rate
limit never aborts the scan.

1. **Committed resolved files** -- `dependency-list.txt` (`mvn dependency:list`)
   or `dependency-tree.json` (`mvn dependency:tree`), if present. These are
   Maven's own output and the most authoritative.
2. **The repo's own POMs** -- cross-POM `dependencyManagement`, parent POMs, and
   imported BOMs reachable within the scanned tree (offline, no flags).
3. **Cross-module propagation** -- a version resolved for a coordinate in one
   module fills the same coordinate left versionless in another (offline).
4. **Local `~/.m2/repository`** -- previously built/downloaded POMs
   (`--maven-local-repo`).
5. **A configured Maven repository** -- an internal Artifactory/JFrog repo or a
   mirror (`--maven-repo-url`, or from `settings.xml`). Covers **private**
   artifacts.
6. **Maven Central** -- the public fallback (`--maven-central`). Can be combined
   with `--maven-repo-url`, in which case it is consulted last (after the private
   repo).

Steps 1-3 are fully offline and need no configuration. Steps 4-6 are opt-in.

## Using an internal Maven repository (Artifactory / JFrog)

Point `--maven-repo-url` at a repository base -- typically a **virtual repo**
that aggregates public proxies and your private artifacts:

```bash
export STACK_ANALYZER_MAVEN_USER="you@example.com"
export STACK_ANALYZER_MAVEN_TOKEN="<access-token-or-api-key>"
stack-analyzer scan /path/to/project --also-sbom \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo
```

Notes:

- **The URL is the base up to and including the repo key.** The scanner appends
  the coordinate path (`.../group/path/artifact/version/artifact-version.pom`).
- **Credentials come from the environment only**, never config files:
  `STACK_ANALYZER_MAVEN_USER` + `STACK_ANALYZER_MAVEN_TOKEN` are sent as HTTP
  Basic auth (a JFrog reference token is used as the password). With only a
  token and no user, it is sent as a Bearer token.
- **Configuring `--maven-repo-url` is the opt-in to reach it** -- no extra
  online flag is required.
- **Maven Central is not added automatically** when `--maven-repo-url` is set: a
  virtual repo usually proxies public artifacts already. If your repo does *not*
  proxy Central (so some public BOMs/POMs 404), add `--maven-central` -- it is
  then consulted last, after the private repo (see below).

### Reusing your `settings.xml`

If you already have a Maven `settings.xml` (with repository URLs, credentials,
and mirrors), reuse it instead of passing flags:

```bash
stack-analyzer scan /path/to/project --also-sbom \
  --maven-settings ~/.m2/settings.xml
```

The scanner reads `<repositories>` (with their `<server>` credentials by id),
honors `<mirrors>` (including `mirrorOf=*`), and uses `<localRepository>`. The
path defaults to `~/.m2/settings.xml`; override it per scan when different
projects use different settings.

### Local `~/.m2` cache

A developer or CI machine that has built the project usually has most POMs
cached. Reading that cache is offline and needs no credentials:

```bash
stack-analyzer scan /path/to/project --also-sbom --maven-local-repo
# Override the path (otherwise MAVEN_REPO_LOCAL / MAVEN_OPTS / ~/.m2/repository):
stack-analyzer scan /path/to/project --maven-local-repo \
  --maven-local-repo-dir /custom/m2/repository
```

### Public Maven Central only

With no internal repo available, allow the public Maven Central fallback:

```bash
stack-analyzer scan /path/to/project --also-sbom --maven-central
```

## Transitive dependencies

By default the SBOM contains the **declared** dependencies. To include
**transitive** dependencies, enable the dependency graph and choose a Maven
graph source. The graph's resolved nodes are folded into the SBOM as components.

```bash
# Transitive deps resolved from the configured Maven repo (covers private deps)
stack-analyzer scan /path/to/project --also-sbom \
  --dependency-graph full --maven-graph-source repo \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo
```

`--maven-graph-source` selects where Maven transitive edges come from:

| Value | Source | Covers private? | Speed |
|-------|--------|-----------------|-------|
| `repo` | crawls every dependency's POM from the repository chain (in-repo -> `~/.m2` -> `--maven-repo-url`/settings). Never contacts deps.dev. | **yes** | slower (crawls the whole tree, public included) |
| `deps-dev` | **hybrid**: deps.dev for the public tree (whole subtree per request) + a repo crawl for the private artifacts deps.dev cannot resolve | **yes** (when a repo is configured) | faster (skips crawling the large public tree) |
| `none` | offline only (a committed `dependency-tree.json` still applies) | from the file | instant |

A committed `dependency-tree.json` always takes precedence regardless of this
flag. When `--maven-graph-source` is unset, Maven follows the global `--deps-dev`
switch (which also governs non-Maven ecosystems such as npm and PyPI).

The `repo` crawl mirrors Maven's resolution (breadth-first, nearest-wins
conflict mediation, one version per coordinate) and is tolerant of repository
rate limits -- it skips an unreachable POM rather than aborting the scan.

**Which to use:** prefer `deps-dev` (hybrid) when deps.dev is acceptable -- it
resolves the public tree far faster while still crawling private artifacts from
your repo. Use `repo` for a privacy-strict scan that must never send coordinates
to deps.dev, accepting that it crawls the full tree.

### Performance and progress

Transitive resolution over a large private monorepo fetches many POMs and can
take minutes; the version-only scan (omit `--dependency-graph`) is much faster
and already gives ~99% versioned components for vulnerability matching. A
scan-wide cache and a per-coordinate subtree memo avoid re-fetching and
re-resolving shared subtrees across modules.

Resolution is reported as a phase with a one-line status broken down by source,
e.g. `dependencies resolved — 2294 POMs, 393 deps.dev, 4630 cached`. The
`deps.dev` count appears for any ecosystem using online resolution; `POMs` and
`cached` are Maven repository fetches (other ecosystems read local lockfiles,
so they have nothing to fetch). A `401/403` from a configured repository prints
a warning that credentials are missing and private artifacts went unresolved.

## Gradle dependency versions

Gradle projects (`build.gradle`, `build.gradle.kts`) have the same versionless
problem as Maven: dependencies are routinely declared without a version because
a BOM or a plugin manages it. The scanner resolves these by reusing the Maven
resolution chain above -- a Gradle "platform" is a `pom`-packaged BOM identical
to a Maven imported BOM -- so the same flags (`--maven-repo-url`,
`--maven-central`, `settings.xml`, `~/.m2`) apply.

Three Gradle-specific sources are resolved, in order of precedence:

1. **`gradle.lockfile`** -- if a committed dependency-lock file is present
   (`gradle dependencies --write-locks`), its fully resolved versions are
   authoritative and supersede the build-script analysis. This is the most
   reliable source and is also what Trivy uses.
2. **`platform(...)` / `enforcedPlatform(...)` BOM imports** -- the wrapped
   coordinate (e.g. `enforcedPlatform("io.quarkus.platform:quarkus-bom:3.11.0")`)
   is resolved as a BOM, and its managed versions backfill any sibling
   dependency declared without one.
3. **Version-managing plugins** -- the Spring Boot Gradle plugin
   (`id("org.springframework.boot") version "3.3.0"`, with
   `io.spring.dependency-management`) implicitly imports
   `org.springframework.boot:spring-boot-dependencies` at the plugin's version,
   supplying versions for `spring-boot-starter-*` and the libraries it manages.

Versions declared inline (`implementation("group:artifact:1.2.3")`) or via a
`gradle.properties` / `ext`/`val`/`def` property reference are still used as
before; BOM resolution only fills the ones left without a version.

```bash
# Quarkus/Spring Boot Gradle project: resolve BOM-managed versions from Central
stack-analyzer scan /path/to/project --also-sbom --maven-central

# Or from an internal repo (covers private platform BOMs); add --maven-central
# if that repo does not proxy Central for public BOMs
stack-analyzer scan /path/to/project --also-sbom \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo \
  --maven-central
```

As with Maven, the BOM/platform coordinate itself is a version-management entry,
not a runtime dependency, so it is not emitted as an SBOM component.

## Configuration file

All flags have config-file equivalents (in the `--config` scan config under
`scan:`). Credentials always stay in the environment.

```yaml
scan:
  dependency_graph: full
  maven_graph_source: repo
  maven_repo_url: https://artifactory.example.com/artifactory/my-virtual-repo
  maven_local_repo: true
  # maven_settings: /path/to/settings.xml
  # maven_central: true        # public fallback (consulted after maven_repo_url)
  # deps_dev: true             # deps.dev for non-Maven ecosystems
```

## Flags reference

| Flag | Config key | Purpose |
|------|-----------|---------|
| `--maven-repo-url` | `maven_repo_url` | Remote Maven repo base (internal/JFrog or mirror). Always used when set. |
| `--maven-settings` | `maven_settings` | Path to a Maven `settings.xml` (repos, credentials, mirrors, local repo). Default `~/.m2/settings.xml`. |
| `--maven-local-repo` | `maven_local_repo` | Read the local `~/.m2/repository` cache (offline). |
| `--maven-local-repo-dir` | `maven_local_repo_dir` | Override the local repo path. |
| `--maven-central` | `maven_central` | Enable the public Maven Central fallback. Can be combined with `--maven-repo-url`: Central is consulted last, after the private repo, so public BOMs/POMs resolve even when the private repo does not proxy Central. |
| `--maven-graph-source` | `maven_graph_source` | Maven transitive graph source: `repo` \| `deps-dev` \| `none`. |
| `--dependency-graph` | `dependency_graph` | Graph depth: `off` \| `direct` \| `full` (transitive folds into the SBOM). |

Environment variables (credentials, never in config):

| Variable | Purpose |
|----------|---------|
| `STACK_ANALYZER_MAVEN_USER` | Username for Basic auth against the remote Maven repo. |
| `STACK_ANALYZER_MAVEN_TOKEN` | Access token / API key (Basic password with a user, else Bearer). |

## Offline guarantee

The scanner is offline by default. Reading outside the scanned tree
(`--maven-local-repo`) and any network access (`--maven-repo-url`,
`--maven-central`, `--deps-dev`) are explicit opt-ins. With no Maven flags, only
the repo's own POMs and committed resolved files are used.

For the internals (resolution algorithm, conflict mediation, source chain, and
the comparison with Trivy), see the design doc:
[Maven (and Cross-Manifest) Version Resolution](design/maven-version-resolution.md).
