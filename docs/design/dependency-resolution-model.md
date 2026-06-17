# Dependency Resolution Model

How the scanner models dependency versions, and the distinction between a
*declared* requirement and a *resolved* version. The model borrows concepts
from Google's deps.dev (Open Source Insights) -- adapted to a static,
offline, single-binary scanner.

## Declared vs. Resolved

A dependency has two version notions:

- **Declared**: the requirement as written by the author -- a range
  (`^1.2.0`, `>=2.25.0`), a property reference (`${spring.version}`,
  `$kotlinVersion`), or a placeholder (`latest`).
- **Resolved**: the concrete release that the declaration resolves to
  (`1.2.11`, `5.3.20`). This is what a lockfile records, what property
  interpolation produces, and what a vulnerability scanner needs to match
  advisories accurately.

The scanner keeps the **resolved** value in the dependency's `version` field
(the canonical value consumers use, and the one the SBOM emits as a PURL
version). When the declared form differs from the resolved value, the original
declaration is recorded in `metadata.declared`:

```json
["maven", "org.springframework:spring-core", "5.3.20", "prod", true,
  {"declared": "${spring.version}", "source": "pom.xml"}]
["npm", "react", "18.2.0", "prod", true,
  {"declared": "^18.0.0", "source": "package-lock.json"}]
```

When the declaration is already concrete (`version` == declared), no
`declared` key is written -- it would add noise without information.

### Where declared is recorded

| Ecosystem | Resolved from | Declared form recorded |
|-----------|---------------|------------------------|
| Maven | property/parent interpolation, imported BOM (`scope=import`) | `${...}` references |
| Gradle | `gradle.lockfile`; `platform`/`enforcedPlatform` BOM and Spring Boot plugin BOM (via the Maven chain); `gradle.properties` / inline `ext`/`val`/`def` | `$x` / `${x}` references, or the unversioned coordinate |
| npm / yarn | `package-lock.json` / `yarn.lock` (incl. classic v1, `optionalDependencies`) | range from `package.json` |
| pnpm (v9) | importer resolved version | importer `specifier` (range) |
| PyPI (poetry) | `poetry.lock` | constraint from `pyproject.toml` |
| Cargo | `Cargo.lock` | constraint from `Cargo.toml` |
| NuGet | inline / Central Package Management | n/a (CPM version is the resolved value) |

## Why this model (deps.dev concepts)

deps.dev computes, for every package version, a **resolved dependency graph**
distinct from the **declared requirements**, by reimplementing each
ecosystem's resolution algorithm rather than executing the build tool. Two
concepts from that design carry over here:

1. **The version node is the unit of identity.** A dependency is identified by
   `(ecosystem-type, name, resolved-version)`. Advisories (via PURL/OSV) and
   licenses attach to that node. This is why the SBOM emits one PURL component
   per resolved dependency.

2. **Declared and resolved are separate facts.** Keeping both enables drift
   visibility (what was asked for vs. what is actually used) without losing
   either. It also makes the lockfile-over-manifest preference explicit: the
   resolved value wins, the declared value is retained for reference.

## Fidelity boundary (what this is NOT)

This scanner resolves **only what is statically available** in manifests and
lockfiles. It deliberately does not:

- **Execute build tools** (no `mvn`, `gradle`, `npm install`).
- **Parse build-script ASTs** (Gradle scripts are Turing-complete; the scanner
  uses regex + property interpolation, which covers the real-world cases and
  is validated against the product portfolio). It does, however, resolve the
  *referenced* BOM/platform POMs (`platform`/`enforcedPlatform`, the Spring Boot
  plugin's implicit BOM) through the Maven resolution chain to fill versions
  those BOMs manage -- that is POM resolution, not script execution.
- **Compute the transitive graph from registries.** Transitive resolution
  (npm max-satisfying, Maven nearest-wins, Go MVS, pip backtracking) requires
  each registry's full version list -- inherently an online operation. Where a
  lockfile is present, the scanner reports the lockfile's resolved versions;
  where only a manifest exists, it reports declared ranges (resolved where
  property interpolation applies).

For an inventory/SBOM tool this boundary is correct: downstream consumers
(e.g. Trivy/OSV) perform transitive vulnerability reasoning from the
direct-dependency SBOM. Higher transitive fidelity, if ever needed, is a
deps.dev-style **online enrichment** (query the deps.dev API by PURL to obtain
a precomputed resolved graph) -- an opt-in addition, never the offline default.

## Scope boundary: direct SBOM, not a supply-chain SBOM

The `--sbom` output is a **direct-dependency** SBOM by design -- it lists what a
project *declares*, not the full resolved closure. This is a deliberate
division of responsibility, not a missing feature.

A concrete comparison illustrates the difference. For one product's frontend
(a pnpm workspace), the lockfile contains 86 direct dependencies and 716 total
packages (direct + transitive). stack-analyzer emits the 86; a dedicated SBOM
tool (Syft) emits the full closure (~1000+ after expansion).

Both are correct for their purpose:

| Concern | Tool | Rationale |
|---------|------|-----------|
| Direct/declared inventory, components, architecture | **stack-analyzer** | Its unique value: tech-stack, component hierarchy, coupling, code stats. Direct deps are the architectural signal; transitive entries would drown it. |
| Full transitive closure | Syft / Trivy / deps.dev | They maintain each ecosystem's resolution algorithms; reimplementing them here would duplicate effort with no differentiation. |
| Transitive vulnerability matching | Trivy (OSV/advisory DB) | Bundles and auto-updates the vulnerability database. |
| Resolved graph as a service | deps.dev | Precomputed online, opt-in enrichment. |

**Decision:** stack-analyzer does not, and should not, compute the transitive
closure or perform vulnerability matching. Those belong to Syft/Trivy/deps.dev.
The scanner's job is the direct/declared inventory and the architectural view;
the security pipeline delegates transitive + vuln work to the dedicated tools,
optionally consuming stack-analyzer's SBOM or scanning the artifact directly.

> Note: a security pipeline that needs full transitive vulnerability coverage
> should scan the lockfile/artifact with Syft/Trivy directly, since the
> direct-only SBOM under-reports transitive CVEs by construction.
