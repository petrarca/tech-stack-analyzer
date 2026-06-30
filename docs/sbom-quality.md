# SBOM Quality Guide

How to get the highest-fidelity SBOM from the scanner — concretely versioned
components rather than versionless ones. A versionless component (`pkg:npm/express`
without a `@version`) is invisible to vulnerability scanners (Trivy, OSV) and
provides no signal for license or supply-chain analysis.

This guide is organized by ecosystem. Each section covers what the scanner reads,
what causes versionless components, and what to do about it.

## TL;DR

| Ecosystem | Best practice |
|-----------|--------------|
| **Maven** | Commit `dependency-tree.json` or `dependency-list.txt`; or use `--maven-repo-url` / `--maven-central` |
| **Gradle** | Commit `gradle.lockfile` or use `--maven-repo-url --maven-central` |
| **npm / yarn / pnpm** | Commit and keep the lockfile (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`) |
| **Python** | Use `uv.lock` or `poetry.lock`; pin with `==` in `requirements.txt` |
| **NuGet** | Commit `packages.lock.json` or use Central Package Management with pinned versions |
| **Composer (PHP)** | Commit `composer.lock`; never rely on `composer.json` ranges alone |
| **Docker** | Use explicit image tags (`:1.23.3-alpine`, not `:latest`); avoid `FROM $VAR` without an `ARG` default |
| **Cargo / Go / Conan** | Nothing to do -- lockfiles resolve everything automatically |
| **CocoaPods** | Commit `Podfile.lock` |
| **Pub (Flutter/Dart)** | Commit `pubspec.lock` |

---

## Environment coverage

Whether an environment produces SBOM components depends on two things: the
scanner must extract dependencies from its manifests/lockfiles, **and** the
ecosystem must have a [Package URL (PURL)](https://github.com/package-url/purl-spec)
type. Components without a PURL type are not emitted into the SBOM (CycloneDX or
SPDX), because they cannot be matched by downstream vulnerability or license
tooling.

### Fully supported (emitted as SBOM components)

These 17 environments map to a PURL type and are emitted into both CycloneDX 1.7
and SPDX 2.3 output. The quality tier indicates how likely components are to be
concretely versioned out of the box.

| Environment | PURL type | Quality tier | Notes |
|-------------|-----------|--------------|-------|
| Go | `golang` | Excellent | `go.mod` + `go.sum` always committed; 0% versionless expected |
| Rust (Cargo) | `cargo` | Excellent | `Cargo.lock` fully resolves transitive deps |
| C/C++ (Conan) | `conan` | Excellent | `conan.lock` resolves everything |
| npm / yarn / pnpm / Bun | `npm` | Good (needs lockfile) | Lockfile = fully resolved; `package.json` alone = ranges |
| Python | `pypi` | Good (needs lockfile or `==`) | `uv.lock`/`poetry.lock` best; ranges stay versionless |
| Ruby (Bundler) | `gem` | Good (needs lockfile) | `Gemfile.lock` fully resolved |
| PHP (Composer) | `composer` | Good (needs lockfile) | `composer.lock` fully resolved |
| .NET (NuGet) | `nuget` | Good (needs lockfile/CPM) | `packages.lock.json` or pinned CPM |
| CocoaPods | `cocoapods` | Good (needs lockfile) | `Podfile.lock` fully resolved |
| Dart / Flutter (Pub) | `pub` | Good (needs lockfile) | `pubspec.lock` fully resolved |
| Elixir / Erlang (Hex) | `hex` | Good (needs lockfile) | `mix.lock` fully resolved |
| Swift (SPM) | `swift` | Good (needs lockfile) | `Package.resolved` fully resolved |
| Perl (CPAN) | `cpan` | Good (needs snapshot) | `cpanfile.snapshot` fully resolved |
| R (CRAN) | `cran` | Good (needs lockfile) | `renv.lock` fully resolved |
| Maven (JVM) | `maven` | Deepest resolution | 6-tier version resolution + transitive graph; resolves private artifacts via internal repo or deps.dev. See [Maven](#maven) |
| Gradle (JVM) | `maven` | Deepest resolution | Reuses the full Maven chain (a Gradle platform is a Maven BOM); `gradle.lockfile`, BOMs, plugins |
| Docker | `docker` | Variable | Depends on explicit, pinned image tags |

**Maven and Gradle are the most capable, not the weakest.** Unlike the
lockfile-driven ecosystems (which only emit what a committed lockfile already
contains), Maven/Gradle have an active 6-tier resolver: committed
`dependency-tree.json`/`dependency-list.txt`, the repo's own POMs and
`dependencyManagement`, cross-module propagation, the local `~/.m2` cache, an
internal Artifactory/JFrog repo (`--maven-repo-url`, covers **private**
artifacts), and Maven Central (`--maven-central`). Transitive dependencies are
resolved from the same repo chain or via the deps.dev hybrid
(`--dependency-graph full --maven-graph-source repo|deps-dev`). The trade-off is
that out-of-the-box (offline, no flags) a Maven project with externally-managed
BOM versions may show some versionless components until you point it at a repo or
commit a resolved file -- hence "needs configuration for full fidelity" rather
than "low quality." See the [Maven section](#maven) and
[docs/maven.md](maven.md).

### Detected but NOT in the SBOM

These environments are detected by the scanner (and appear in the regular scan
output), but have **no PURL type**, so they are deliberately excluded from the
SBOM. They have no upstream package registry, or represent infrastructure /
runtime rather than installable packages.

| Environment | Why excluded |
|-------------|--------------|
| Deno | Parses `deno.lock`/`deno.json` for tech detection, but `deno` has no PURL type, so its dependencies are not emitted |
| Terraform | Infrastructure-as-Code; no package PURL type |
| GitHub Actions | CI/CD workflow references; no package PURL type |
| Delphi | No public registry / PURL type (`.dproj`) |
| Node (runtime) | A runtime marker, not a package |
| Nx | Reads `project.json` for workspace structure only; extracts no dependencies |

### Not detected for dependencies at all

Any technology detected purely via file extension or config-file presence (most
of the 700+ YAML rules) contributes to the technology inventory but carries no
dependency data, and therefore never appears in the SBOM. Only the environments
in the "Fully supported" table above produce SBOM components.

---

## Maven

### What the scanner reads (in priority order)
1. A committed `dependency-tree.json` (`mvn dependency:tree -DoutputType=json`) or `dependency-list.txt` (`mvn dependency:list`) -- authoritative, always wins.
2. The repository's own POMs -- cross-POM `dependencyManagement`, parent POMs, imported BOMs reachable within the scanned tree (offline, no configuration).
3. Cross-module propagation -- a version resolved in one module fills the same coordinate left versionless in another (offline).
4. Local `~/.m2/repository` -- (`--maven-local-repo`).
5. A configured Maven repository -- internal Artifactory/JFrog (`--maven-repo-url`).
6. Maven Central -- (`--maven-central`).

### What causes versionless
- **BOM-managed or parent-inherited versions.** Maven projects routinely omit `<version>` from dependencies when a parent POM or an imported BOM manages it. The scanner resolves these offline only when the managing POM is inside the scanned tree. External BOMs (e.g. Spring Boot's `spring-boot-dependencies`) require network access.
- **Private artifacts.** Coordinates whose POM lives only in your internal Artifactory cannot be resolved without `--maven-repo-url`.

### Recommendations
1. **Commit resolved files (best).** Run `mvn dependency:tree -DoutputType=json -f pom.xml > dependency-tree.json` and commit. The scanner reads it at priority 1 -- fully offline, 100% resolved.
2. **Use `--maven-central`** for projects using public Spring Boot / Quarkus / Micronaut BOMs that your internal repo does not proxy:
   ```bash
   stack-analyzer scan --also-sbom --maven-central /path/to/project
   ```
3. **Use `--maven-repo-url`** for private artifacts (covers both private and public when the virtual repo proxies Maven Central):
   ```bash
   export STACK_ANALYZER_MAVEN_USER=you@example.com
   export STACK_ANALYZER_MAVEN_TOKEN=<api-key>
   stack-analyzer scan --also-sbom \
     --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo \
     /path/to/project
   ```
4. **`--maven-settings`** reuses your existing `settings.xml` (mirrors, server credentials, local repository):
   ```bash
   stack-analyzer scan --also-sbom --maven-settings ~/.m2/settings.xml /path/to/project
   ```

See [docs/maven.md](maven.md) for the full Maven/Gradle guide.

---

## Gradle

### What the scanner reads (in priority order)
1. A committed `gradle.lockfile` (`gradle dependencies --write-locks`) -- fully resolved versions, authoritative.
2. `platform()` / `enforcedPlatform()` BOM imports -- resolved via the Maven POM source chain (same flags as Maven above).
3. The Spring Boot Gradle plugin (`id("org.springframework.boot") version "X"`) -- the implicit `spring-boot-dependencies:X` BOM is resolved automatically.
4. `gradle.properties` / `ext`/`val`/`def` property references -- resolved offline.

### What causes versionless
- **No `gradle.lockfile` and no Maven repo configured.** A `platform("g:a:v")` BOM manages versions -- without it being resolvable, the declared dependencies have no version.
- **Private platform BOMs.** An internal BOM (e.g. a company-wide platform) needs the repo to fetch its `dependencyManagement`.

### Recommendations
1. **Commit `gradle.lockfile` (best).** Run `gradle dependencies --write-locks` and commit the lock file(s). The scanner reads them at priority 1. This is also the only Gradle resolution path Trivy uses.
2. **Use `--maven-central` + `--maven-repo-url`** when Gradle BOMs live on public repos or a private Artifactory (same flags as Maven, reuses the same source chain).

---

## npm / yarn / pnpm / Bun

### What the scanner reads (in priority order)
1. `package-lock.json` (npm) -- fully resolved.
2. `pnpm-lock.yaml` -- fully resolved.
3. `yarn.lock` (classic v1 and Berry) -- fully resolved, including optionalDependencies.
4. `bun.lock` -- fully resolved.
5. The nearest ancestor lockfile -- for workspace monorepos where member packages rely on a hoisted root lock.
6. `package.json` fallback -- ranges only; results in versionless.

### What causes versionless
- **No lockfile** -- the scanner falls back to `package.json` ranges (`^1.2.0`, `~0.4`), which are not concrete versions.
- **Lockfile not committed** -- common in some WordPress/legacy projects.
- **Internal scoped packages** (`@company/*`) -- these are always resolved from the lockfile; missing lockfile means they stay versionless.

### Recommendations
1. **Always commit the lockfile.** `package-lock.json`, `yarn.lock`, or `pnpm-lock.yaml` must be committed and kept up to date.
2. **For workspaces,** a single root lockfile is sufficient -- the scanner climbs the directory tree to find the nearest ancestor lock.
3. **Transitive dependencies** are included via `--dependency-graph full` + `--deps-dev`, or after the fact via `sbom --resolve-transitive --deps-dev`.

---

## Python

### What the scanner reads (in priority order)
1. `uv.lock` -- fully resolved transitive graph.
2. `poetry.lock` -- fully resolved.
3. `requirements.txt` -- exact (`==`, `~=`, `===`) pins are extracted as concrete versions; ranges (`>=`, `<`, multi-clause) are left unresolved.
4. `pyproject.toml` / `setup.py` / `Pipfile` -- fallback; ranges result in versionless.

### What causes versionless
- **Ranges in `requirements.txt`** (`>=2.25.0`, `!=0.18.3,>=0.18.2`). Only single-clause exact or compatible-release pins (`==x`, `~=x`) resolve to a concrete version.
- **No lockfile.** `pyproject.toml` or `setup.py` carry ranges; without a `uv.lock` or `poetry.lock`, versions are unresolved.

### Recommendations
1. **Use `uv.lock` or `poetry.lock` (best)** -- fully resolved, transitive-aware.
2. **Pin with `==` in `requirements.txt`** -- generates concrete SBOMs even without a lockfile, e.g. `requests==2.32.3`.
3. **Avoid bare ranges** (`>=2.0`) in `requirements.txt` for production dependencies.

---

## NuGet (.NET)

### What the scanner reads

**For the dependency graph (transitive), in priority order:**
1. `<App>.deps.json` -- the .NET SDK's build output, a fully-resolved closure of direct, transitive, and bundled-runtime packages. When present this is the highest-fidelity source: completely offline, no lockfile opt-in, no network. Its filename is project-specific (e.g. `MyApp.deps.json`) and it lives in build output (`bin/.../`).
2. `packages.lock.json` -- fully resolved transitive graph (requires `RestorePackagesWithLockFile=true`).

**For direct dependencies and versions:**
3. `.csproj` / `.vbproj` / `.fsproj` `<PackageReference>` elements -- declared version, including the child `<Version>` element and a CPM per-reference `VersionOverride`.
4. `packages.config` (legacy .NET Framework).

**Version backfill applied to the above:**
- **MSBuild properties.** A version expressed as a property reference -- `<PackageReference Include="Newtonsoft.Json" Version="$(JsonVersion)" />` -- is resolved from the project's `<PropertyGroup>` definitions, including chained properties (`$(FullVer)` -> `$(BaseVer).72`). An undefined property is left intact so it stays detectably unresolved.
- **Central Package Management (CPM).** A versionless `<PackageReference>` is backfilled from the nearest `Directory.Packages.props` (searched up to the scan root). Matching is case-insensitive, since NuGet package ids are case-insensitive (a `.csproj` and `Directory.Packages.props` may differ in casing).

### What causes versionless
- **No `.deps.json` and no `packages.lock.json`**, with a version that is neither declared in the `.csproj` nor resolvable from CPM/properties.
- **Floating versions** (`*`, `1.0.*`) -- a floating range is not a concrete version (there is no resolution of a floating range to a published version offline).
- **Undefined property reference** -- `$(SomeVersion)` with no matching `<PropertyGroup>` entry.

### Recommendations
1. **Scan the build output (best for transitive).** A built project emits `<App>.deps.json`; scanning it (or committing it from CI) yields the complete resolved graph -- direct, transitive, and runtime -- with no further configuration.
2. **Or enable restore lock files:**
   ```xml
   <!-- Directory.Build.props -->
   <RestorePackagesWithLockFile>true</RestorePackagesWithLockFile>
   ```
   Commit the generated `packages.lock.json`.
3. **CPM and property-based versions resolve automatically** as long as the `Directory.Packages.props` and the project's `<PropertyGroup>` definitions are present in the scanned tree -- no extra steps.

### Note on transitive dependencies
Without a `.deps.json` or a `packages.lock.json`, the scanner reports **direct dependencies only** (from `.csproj` / `packages.config`); NuGet transitive resolution is not performed over the network. The two files above are the way to get the transitive graph.

---

## Composer (PHP)

### What the scanner reads (in priority order)
1. `composer.lock` -- fully resolved, transitive-aware.
2. `composer.json` `require`/`require-dev` -- ranges only; PHP platform requirements (`php`, `ext-*`, `lib-*`) are filtered out.

### What causes versionless
- **No `composer.lock`.** `composer.json` declares ranges (`^7.0`, `dev-master`). Without a lock the scanner has no concrete version.
- **`dev-master` / wildcard version constraints** -- unresolvable statically.

### Recommendations
1. **Always commit `composer.lock`** -- run `composer install` and commit the result. This is also required for reproducible builds.
2. **Do not rely on `composer.json` alone** for SBOM purposes; ranges produce no useful version signal.

### Note on PHP platform requirements
`php`, `ext-curl`, `ext-json`, `lib-openssl`, `hhvm`, `php-64bit` etc. are runtime/environment constraints, not installable packages. The scanner filters them out of the SBOM (they would never have a PURL or an advisory match).

---

## Docker

### What the scanner reads
`FROM` statements in Dockerfiles and `image:` fields in Docker Compose files.

### What causes versionless
- **`:latest` tag** -- not a concrete version.
- **Untagged image** (`FROM ubuntu`) -- defaults to latest.
- **Dynamic tags** from environment variables with no ARG default (`FROM $BUILD_IMAGE`).
- **Stage references** (`FROM builder AS final`) -- these are local build stages, not registry images; the scanner correctly skips them.

### Recommendations
1. **Always use explicit, pinned tags:**
   ```dockerfile
   FROM nginx:1.25.3-alpine      # good
   FROM nginx:latest             # versionless in SBOM
   FROM nginx                    # versionless in SBOM
   ```
2. **For ARG-based base images**, provide a default:
   ```dockerfile
   ARG BUILD_IMAGE=node:20-alpine
   FROM $BUILD_IMAGE             # resolves to node:20-alpine
   ```
   Without a default, `FROM $BUILD_IMAGE` is skipped (the actual image is only known at build time).
3. **Consider digest pinning** for critical base images:
   ```dockerfile
   FROM nginx@sha256:abc123...   # immutable, fully resolved
   ```

---

## Cargo (Rust)

**Resolution quality: excellent (0% versionless expected).**

The scanner reads `Cargo.lock`, which contains fully resolved versions for all dependencies including transitive ones. As long as `Cargo.lock` is committed (recommended for applications, optional for libraries), all components are versioned.

---

## Go

**Resolution quality: excellent (0% versionless expected).**

The scanner reads `go.mod` (direct dependencies) and `go.sum` (all dependencies with hashes). Both are always committed as part of the Go module system.

---

## CocoaPods (iOS/macOS)

### What the scanner reads (in priority order)
1. `Podfile.lock` -- fully resolved.
2. `Podfile` -- pod names and version constraints.

### Recommendation
Commit `Podfile.lock`. Xcode projects commit it by default; always include it in the repository.

---

## Pub (Flutter / Dart)

### What the scanner reads (in priority order)
1. `pubspec.lock` -- fully resolved.
2. `pubspec.yaml` -- version constraints only.

### Recommendation
Commit `pubspec.lock`. Flutter projects create it automatically on `flutter pub get`; treat it as a required committed file alongside `pubspec.yaml`.

---

## General principles

1. **Lockfiles are the primary resolution source.** For every ecosystem that supports a lockfile, committing it is the single most effective action to improve SBOM quality.

2. **Ranges ≠ versions.** A version constraint like `^1.2.0` or `>=2.25.0` does not tell you which version is actually installed. Only a lockfile or a pinned exact version does.

3. **Private artifacts need a reachable repository.** Internal packages (Maven/Gradle) that are not in any public registry can only be resolved with `--maven-repo-url` pointing at your internal Artifactory or Nexus.

4. **The `sbom` command can add transitive deps after the fact.** If a scan ran without `--dependency-graph full`, the transitive graph can be added later without re-scanning:
   ```bash
   stack-analyzer sbom results.json --resolve-transitive --deps-dev -o full.cdx.json
   ```

5. **Platform / environment requirements are not packages.** PHP's `php`, `ext-*`; Python's `sys`, `os`; Node's built-in modules -- these are filtered out of the SBOM by design.
