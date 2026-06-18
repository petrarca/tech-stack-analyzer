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
1. `packages.lock.json` -- fully resolved (requires `RestorePackagesWithLockFile=true`).
2. `.csproj` / `.fsproj` `<PackageReference>` elements -- extracts the declared version.
3. Central Package Management (CPM) via `Directory.Packages.props` -- version pinned in the central file.

### What causes versionless
- **No `packages.lock.json`** and CPM-managed versions (version lives in `Directory.Packages.props`, not the `.csproj`). The scanner reads the `.csproj` first; if the version is absent (delegated to CPM) and there is no lock, the dep is versionless.
- **Floating versions** (`*`, `1.0.*`).

### Recommendations
1. **Enable restore lock files:**
   ```xml
   <!-- Directory.Build.props -->
   <RestorePackagesWithLockFile>true</RestorePackagesWithLockFile>
   ```
   Commit the generated `packages.lock.json`.
2. **If using CPM** (`Directory.Packages.props`), the scanner resolves the version from that file -- no extra steps needed as long as the file is committed.

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
