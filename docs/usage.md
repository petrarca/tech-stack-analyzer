# Usage Guide

## Commands

### `scan` - Analyze a project or file

Scans a project directory or single file to detect technologies, frameworks, databases, and services.

**Usage:**
```bash
stack-analyzer scan [path] [flags]
```

**Flags:**
- `--config` - Scan configuration file path or inline JSON (YAML/JSON file path or inline JSON string starting with `{`)
- `--output, -o` - Output file path (default: stack-analysis.json). Use `-o -` or `-o /dev/stdout` for piping
- `--aggregate` - Aggregate fields: `tech,techs,languages,licenses,dependencies,git,components,all` (use `all` for all aggregated fields). The `components` field produces a flat list of all components with `id`, `name`, `type`, `tech`, `techs`, `path`.
- `--also-aggregate` - Produce both full and aggregate output in one scan pass. The aggregate file gets a `-agg` suffix (e.g. `output.json` → `output-agg.json`). Cannot be combined with `--aggregate`. Useful for large codebases where scanning twice would be too slow.
- `--sbom` - Emit an SBOM (with Package URLs) as the primary output instead of the scan tree. Consumable directly by vulnerability scanners such as Trivy (`trivy sbom ...`). Only dependencies with a PURL-mappable ecosystem are included; non-package types (terraform, docker images as build steps, etc.) are skipped.
- `--also-sbom` - Produce both the scan output and an SBOM in one scan pass. The SBOM file gets a format-specific suffix (e.g. `output.json` → `output.cdx.json` for CycloneDX, `output.spdx.json` for SPDX).
- `--sbom-format` - SBOM format for `--sbom`/`--also-sbom`: `cyclonedx` (CycloneDX 1.7 JSON, default) or `spdx` (SPDX 2.3 JSON). Both carry the same package set with PURLs and are read by Trivy.
- `--omit-fields` - Strip fields from the full output tree before writing (e.g. `reason,edges`). Applied recursively to all components. Useful to reduce file size when downstream consumers don't need certain fields.
- `--exclude` - Additional patterns to exclude (combined with `.gitignore`; full gitignore semantics including `**` globs, `!` negation, trailing `/` for dir-only; can be specified multiple times)
- `--dependency-graph` - Emit package-to-package dependency edges read from lockfiles: `off` (default), `direct` (root-to-direct edges only), or `full` (the full transitive graph). The full graph can be very large in big projects, so it is off by default. Produced directly from lockfiles for: JS (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`), Python (`uv.lock`, `poetry.lock`), Rust (`Cargo.lock`), Go (`go.mod` for direct; full graph from a pre-generated `go.mod.graph`), Ruby (`Gemfile.lock`), PHP (`composer.lock`), .NET (`packages.lock.json`), C/C++ (`conan.lock`), Swift/iOS (`Podfile.lock`, `Package.resolved`), Dart (`pubspec.lock`), Elixir (`mix.lock`), Perl (`cpanfile.snapshot`), and R (`renv.lock`). For Maven and Gradle the scanner ingests a pre-generated resolved tree it never produces -- `dependency-tree.json` (`mvn dependency:tree -DoutputType=json`) or `gradle-dependencies.txt` (`gradle dependencies`) -- or a CycloneDX `bom.json` dependency-graph section. Each edge carries `source` (provenance: `lockfile` or `deps.dev`) and, on direct edges, `scope` (`prod`/`dev`/`build`/`optional`/`peer`). Edges appear per component in the full tree and as a single deduplicated, sorted top-level `dependency_edges` array in the aggregate output.
- `--deps-dev` - Allow online dependency-graph resolution via deps.dev as a fallback for ecosystems without a committed resolved tree (all ecosystems; default off). When enabled the scanner fans out over each component's declared dependencies, queries deps.dev for each, and unions the results. Private or unknown deps are silently skipped (404). Edges are tagged `source: deps.dev`. A present local lockfile/tree always wins (local-first). Per deps.dev API docs, graph data is available for **npm, Cargo, Maven, and PyPI** only; others fall through gracefully.
- `--deps-dev-endpoint` - Base URL for deps.dev (default: public `https://api.deps.dev`). Override with a deps.dev-API-compatible facade or mirror. Also used by `--resolve-currency` and the `currency` command.
- `--resolve-currency` - Resolve dependency currency (how far each direct dependency is behind its latest release) via deps.dev and write a `{out}.currency.json` companion. Opt-in; sends public package coordinates over the network. Results are cached across runs in a shared SQLite store with a per-entry TTL. For force-refresh or concurrency tuning, use the standalone `currency` command. See the [`currency` command](#currency---resolve-dependency-currency-freshness).
- `--currency-cache` - Override the currency cache DB path (default: `STACK_ANALYZER_CURRENCY_CACHE` env var, else the OS cache dir).
- `--currency-ttl` - Per-entry currency cache TTL in hours (default: 24).
- `--maven-central` - Enable the public Maven Central fallback for resolving Maven/Gradle BOM/parent POM versions (default off). May be combined with `--maven-repo-url` (consulted last, after the private repo) so public BOMs resolve when the private repo does not proxy Central.
- `--maven-repo-url`, `--maven-graph-source`, `--maven-local-repo`, `--maven-settings` - Maven/Gradle resolution against an internal/JFrog repository, including transitive resolution and Gradle `platform`/`enforcedPlatform` and Spring Boot plugin BOMs. See the [Maven guide](maven.md).
- `--harvest-licenses` - Also harvest per-dependency declared licenses from out-of-tree global package caches (default off). Currently supported: NuGet (the global packages folder, respecting `NUGET_PACKAGES`). In-tree sources — a `node_modules/` directory present under the scan root — are **always** harvested regardless of this flag. Harvested licenses appear in the `metadata.license` field of each dependency and as `licenses[].license.id` on CycloneDX SBOM components. This flag mirrors the `--maven-local-repo` opt-in for the Maven `~/.m2` cache: it reads outside the scanned tree, so it is off by default to keep scans deterministic across machines.
- `--no-code-stats` - Disable code statistics collection (enabled by default)
- `--component-stats-depth N` - Include `code_stats` on components up to depth N in output (default: 0 = none)
- `--subsystem-depth N` - Produce `subsystem_stats[]` rolled up per depth-N path prefix (default: 0 = none)
- `--pretty` - Pretty print JSON output (default: true)
- `--quiet, -q` - Suppress all progress output (default: false)
- `--verbose, -v` - Show detailed progress information on stderr (default: false)
- `--log-level` - Log level: trace, debug, error, fatal (default: error)
- `--log-format` - Log format: text or json (default: text)
- `--log-file` - Log file path (default: stderr)

**Examples:**
```bash
# Basic usage (automatic .gitignore exclusions)
stack-analyzer scan /path
stack-analyzer scan --aggregate all /path  # Aggregate all fields with metadata

# Scan configuration file
stack-analyzer scan --config scan-config.yml
stack-analyzer scan --config portfolio-config.yml --output portfolio-analysis.json

# Inline JSON configuration (useful for CI/CD)
stack-analyzer scan --config '{"scan":{"paths":["./project"],"output":{"file":"results.json"},"properties":{"build":"123"}}}'

# Add additional exclusions beyond .gitignore
stack-analyzer scan /path --exclude build-cache --exclude "*.tmp"
stack-analyzer scan /path --exclude "**/__tests__/**" --exclude "*.log"

# Produce full output AND aggregate in one scan pass
# Generates: results.json (full) + results-agg.json (aggregate)
stack-analyzer scan /path --output results.json --also-aggregate tech,techs,languages,dependencies,git

# Strip unused fields to reduce output size (applied recursively to all components)
stack-analyzer scan /path --omit-fields reason,edges
stack-analyzer scan /path --omit-fields reason,edges --also-aggregate tech,techs,languages,dependencies,git

# Emit a CycloneDX SBOM for vulnerability scanning, then scan it with Trivy
stack-analyzer scan /path --sbom -o sbom.cdx.json
trivy sbom sbom.cdx.json

# Emit an SPDX 2.3 SBOM instead
stack-analyzer scan /path --sbom --sbom-format spdx -o sbom.spdx.json

# Produce the scan output AND an SBOM in one pass (results.json + results.cdx.json)
stack-analyzer scan /path --output results.json --also-sbom

# Verbose mode
stack-analyzer scan -v /path/to/project
stack-analyzer scan --verbose --output results.json /path

# Logging examples
stack-analyzer scan /path --log-level debug --log-format json
stack-analyzer scan /path --log-level trace
```

### `sbom` - Generate an SBOM from a saved scan output

Re-projects a previously written scan output JSON into an SBOM, without
re-scanning. The scan's resolved dependencies are already in the output file, so
this is a pure transformation -- letting a single (potentially long) scan produce
multiple SBOM formats, or a direct-only variant, on demand.

**Usage:**
```bash
stack-analyzer sbom <scan-output.json> [flags]
```

**Flags:**
- `--format` - `cyclonedx` (CycloneDX 1.7 JSON, default) or `spdx` (SPDX 2.3 JSON).
- `-o, --output` - Output file path (default: stdout).
- `--pretty` - Pretty-print the JSON (default: true).
- `--direct-only` - Emit only the project's direct dependencies, excluding transitive (dependency-of-dependency) graph nodes. This is the direct/transitive axis (same as `--dependency-graph direct`); the emitted versions are still the resolved ones. Has no effect on a scan that captured no transitive graph.
- `--resolve-transitive` - Resolve the transitive dependency graph **online from the direct-dependency coordinates** and fold it into the SBOM. The original source files (lockfiles, POMs) are gone by this point, so resolution is coordinate-based: deps.dev for public packages (all ecosystems), and, for Maven/Gradle, the configured Maven repository (repo crawl / deps.dev hybrid) for private artifacts. Private non-Maven packages that deps.dev cannot resolve stay direct. Mutually exclusive with `--direct-only`. Progress is reported as a resolution phase, like a scan.
  - Online sources are opt-in via the same flags as `scan`: `--deps-dev`, `--deps-dev-endpoint`, `--maven-graph-source`, `--maven-repo-url`, `--maven-central`, `--maven-settings`, `--maven-local-repo[-dir]`, `--dependency-graph direct|full`. Maven repository credentials come from `STACK_ANALYZER_MAVEN_USER` / `STACK_ANALYZER_MAVEN_TOKEN` (environment only).

This lets a fast, default (direct-only) scan be enriched with the transitive
graph later, on demand, without re-scanning -- the transitive resolution that a
`--dependency-graph full` scan would have done, run from the saved direct
dependencies instead.

The input must be a full scan output that still contains the `dependencies`
field (i.e. produced without `--omit-fields dependencies` and without an
`--aggregate` that strips them). Whether the result includes transitive
dependencies depends on what the original scan captured: a scan run with
`--dependency-graph full` carries the transitive graph in its output, so the
`sbom` command can emit it (or exclude it with `--direct-only`); a default scan
(`--dependency-graph off`) has only direct dependencies to begin with.

**Examples:**
```bash
# CycloneDX from a saved scan (to stdout)
stack-analyzer sbom results.json

# SPDX 2.3 to a file
stack-analyzer sbom results.json --format spdx -o results.spdx.json

# Direct dependencies only (drop transitive graph nodes)
stack-analyzer sbom results.json --direct-only -o results-direct.cdx.json

# Resolve the transitive graph online from a direct-only scan (public packages)
stack-analyzer sbom results.json --resolve-transitive --deps-dev -o results-full.cdx.json

# Resolve transitive incl. private Maven artifacts from an internal repo
stack-analyzer sbom results.json --resolve-transitive \
  --maven-graph-source deps-dev \
  --maven-repo-url https://artifactory.example.com/artifactory/my-virtual-repo \
  -o results-full.cdx.json
```

### `currency` - Resolve dependency currency (freshness)

Resolves how far each **direct** dependency is behind its latest available
release from a previously written **aggregate** (`-agg.json`) file, and writes a
`{stem}.currency.json` artifact. Latest versions come from
[Google deps.dev](https://deps.dev) (opt-in network access). This is **freshness**,
not vulnerability — "is this dependency outdated?" is a maintenance question,
separate from the CVE/security concern.

```bash
stack-analyzer currency <agg.json> [flags]
```

Re-running the command **is** the refresh: results are cached across runs and
products in a shared SQLite store with a per-entry TTL, so only stale (or new)
entries are re-fetched. The same package is looked up once and reused everywhere.

**Flags:**

- `-o, --output` - Output file (default: `<agg-stem>.currency.json`).
- `--currency-cache` - Override the cache DB path (default: `STACK_ANALYZER_CURRENCY_CACHE` env var, else the OS cache dir).
- `--currency-ttl` - Per-entry cache TTL in hours (default: 24).
- `--currency-concurrency` - Parallel deps.dev lookups (default: 10).
- `--force` - Ignore the cache TTL and re-fetch every package.
- `--deps-dev-endpoint` - Base URL for deps.dev (default: public; override for a mirror).
- `-q, --quiet` - Suppress progress output.

**The `{out}.currency.json` artifact** is a dedicated, time-varying view joined
to dependencies by PURL (the scan output and SBOM are never modified). Each entry
carries the installed and latest versions and a `currency` classification:

| `currency` | Meaning |
|------------|---------|
| `up_to_date` | installed is the latest stable |
| `patch` / `minor` / `major` | behind by that semver level |
| `unsupported` | no public registry exists for the ecosystem (e.g. native libs, project references) |
| `unpinned` | the installed version is not pinned (`latest`, `RELEASE`, a range, or a property) — currency cannot be assessed |
| `unknown` | a concrete version was queried but deps.dev did not know it (likely an internal/private package) |
| `error` | a transient lookup failure |

The `summary` block totals each classification, plus `deprecated` (packages whose
latest version is marked deprecated upstream).

```bash
# Resolve currency from an aggregate (writes app-agg.currency.json)
stack-analyzer currency app-agg.json

# Force a full refresh, ignoring the cache TTL
stack-analyzer currency app-agg.json --force

# Point at a deps.dev-compatible mirror
stack-analyzer currency app-agg.json --deps-dev-endpoint https://deps.example.com
```

### `cache` - Inspect and manage the shared currency cache

Manages the shared SQLite cache used by currency resolution. These commands
**never create** the cache file: if it does not exist, they report "no cache yet".

```bash
stack-analyzer cache info     # location, size, schema version, record counts
stack-analyzer cache clear    # remove cached entries (all, or --expired-only)
stack-analyzer cache vacuum   # reclaim space after deletions (SQLite VACUUM)
```

All three honor `--currency-cache` (and `STACK_ANALYZER_CURRENCY_CACHE`) to target
a specific cache file.

### `summary` - Human-readable codebase summary

Runs the same scanner pipeline as `scan` but outputs a concise text report
instead of JSON. Useful for quick codebase introspection, onboarding, and
as input for LLM-based analysis.

**Usage:**
```bash
stack-analyzer summary [path] [flags]
```

**Flags:**
- `--config` - Scan configuration file path or inline JSON
- `--exclude` - Patterns to exclude (same semantics as `scan`)
- `--subsystem-depth N` - Produce subsystem stats per depth-N path prefix
- `--component-stats-depth N` - Per-component code stats depth (default: 1 for summary)
- `--no-code-stats` - Disable code statistics
- `--quiet, -q` - Suppress scan progress output
- `--verbose, -v` - Show scan progress with simple output
- `--debug, -d` - Show scan progress with tree structure
- `--log-level` / `--log-format` / `--log-file` - Logging options

**Examples:**
```bash
# Quick overview of a project
stack-analyzer summary /path/to/project

# With a scan config (for excludes, subsystem groups, etc.)
stack-analyzer summary --config scan-config.yml

# Quiet mode (no scan progress, just the report)
stack-analyzer summary --quiet /path/to/project

# With subsystem breakdown
stack-analyzer summary --subsystem-depth 1 /path/to/project
```

**Report sections:**

| Section | Content |
|---------|---------|
| Metadata | Scan path, component/language/tech counts, scan time |
| Code Statistics | Files and Code LoC by type (programming, markup, data, prose) |
| Languages | Top 15 languages by Code LoC, primary language breakdown |
| Technologies | Primary and secondary techs, ecosystems |
| Component Tree | Top-level directories with component counts, files, Code LoC |
| Subsystems | Per-subsystem stats (when `--subsystem-depth` or named groups configured) |
| Observations | Generated/vendored files (go-enry), large programming files, encoding issues, niche languages, complexity, duplicated components |

The Observations section includes actionable exclude pattern suggestions
where applicable. These should be reviewed before applying -- false positives
are possible (e.g. go-enry may flag IDE config directories as vendored).

### `info` - Display information about rules and categories

**Subcommands:**

**`info categories`** - List all technology categories
```bash
stack-analyzer info categories                    # List all categories with descriptions
stack-analyzer info categories --components       # Show component vs non-component categories
stack-analyzer info categories --format json      # JSON format with descriptions
```
Shows which technology categories create components (appear in `tech` field) vs those that don't (only in `techs` array).

**`info techs`** - List all available technologies
```bash
stack-analyzer info techs                    # Text format (simple list with categories)
stack-analyzer info techs --format json      # JSON with name, category, description, properties
stack-analyzer info techs --format yaml      # YAML with name, category, description, properties
stack-analyzer info techs | grep postgres    # Filter technologies
```
Lists all technology names from the embedded rules. JSON and YAML formats include detailed information (tech key, name, category, description, and custom properties).

**`info rule [tech-name]`** - Show rule details
```bash
stack-analyzer info rule postgresql
stack-analyzer info rule postgresql --format json
```
Displays the complete rule definition for a given technology.

**Flags:**
- `--format, -f` - Output format: `text`, `yaml`, or `json` (default varies by command)
- `--components` - Show only component categories (for `info categories` command)

### Global Flags

- `--help, -h` - Help for any command
- `--version, -v` - Show version information

## Automatic .gitignore Support

The scanner automatically uses your project's existing `.gitignore` files for intelligent exclusions.

### How It Works

- **Recursive Loading**: Finds and loads ALL `.gitignore` files from root to subdirectories
- **Git-compatible Behavior**: Processes patterns the same way Git does (hierarchical merging)
- **Smart Filtering**: Skips problematic cache directories that contain `*` patterns
- **Pattern Support**: Supports glob patterns (`*`, `?`, `**`) and file extensions

### What Gets Excluded Automatically

Common patterns that work out of the box:
- **Node.js**: `node_modules`, `dist`, `build`, `.npm`, `.yarn`
- **Python**: `.venv`, `venv`, `__pycache__`, `.pytest_cache`, `.ruff_cache`
- **Build Tools**: `target`, `build`, `dist`, `.next`, `.nuxt`
- **IDE Files**: `.vscode`, `.idea`, `*.swp`, `*.swo`
- **OS Files**: `.DS_Store`, `Thumbs.db`
- **Cache/Temp**: `.cache`, `.tmp`, `*.log`

### Exclude Patterns

Use `--exclude` flags to add additional exclusions. These support full gitignore semantics:
```bash
# Add extra exclusions beyond .gitignore
./bin/stack-analyzer scan /path/to/project --exclude "build-cache" --exclude "*.tmp"

# Dir-only pattern: exclude directory named "build", but not a file named "build"
./bin/stack-analyzer scan /path/to/project --exclude "build/"

# Negation: exclude vendor but re-include a specific subdirectory
./bin/stack-analyzer scan /path/to/project --exclude "vendor/**" --exclude "!vendor/important/**"

# Negation: re-include .NET directory (overrides dot-dir default exclusion)
./bin/stack-analyzer scan /path/to/project --exclude "!.NET/**"
```

All `--exclude` patterns, config `exclude:` patterns, and `.gitignore` files are evaluated
together using **last-match-wins** semantics — the last matching pattern determines whether
a path is excluded or included.

### Performance Benefits

Using .gitignore patterns provides significant performance improvements:
- **Fewer Files**: Skips thousands of unnecessary files (node_modules, .venv, etc.)
- **Faster Scans**: Typical 70-90% reduction in scan time
- **Accurate Results**: Focuses on source code and configuration files

## Code Statistics

The scanner automatically collects code statistics using [SCC](https://github.com/boyter/scc) (Sloc, Cloc and Code). Statistics are enabled by default and can be disabled with `--no-code-stats`.

```bash
# Default: code stats enabled
./bin/stack-analyzer scan /path/to/project

# Disable code stats
./bin/stack-analyzer scan --no-code-stats /path/to/project
```

### Output Structure

```json
{
  "code_stats": {
    "total": { "lines": 39212, "code": 32834, "comments": 2027, "blanks": 4351, "complexity": 1960, "files": 858 },
    "by_type": {
      "programming": {
        "total": { "lines": 22023, "code": 16826 },
        "metrics": {
          "comment_ratio": 0.12,
          "code_density": 0.76,
          "avg_file_size": 236.81,
          "complexity_per_kloc": 116.49,
          "avg_complexity": 21.08,
          "primary_languages": [{"language": "Go", "pct": 1}]
        },
        "languages": ["Go"]
      },
      "data": { "total": { "lines": 12575 }, "languages": ["YAML", "JSON", "Go Checksums"] },
      "prose": { "total": { "lines": 5003 }, "languages": ["Markdown", "Text"] }
    },
    "analyzed": {
      "total": {},
      "by_language": [
        {"language": "Go", "lines": 21841, "code": 16679, "comments": 1963},
        {"language": "YAML", "lines": 11385, "code": 11258}
      ]
    },
    "unanalyzed": {
      "total": {"lines": 389, "files": 3},
      "by_language": [{"language": "Go Checksums", "lines": 253, "files": 1}]
    }
  }
}
```

### Fields

- **`total`** - Grand total for all analyzed files
- **`by_type`** - Stats grouped by [GitHub Linguist](https://github.com/github-linguist/linguist) language type:
  - `programming` - Go, C++, Java, Python, etc. (includes metrics)
  - `data` - JSON, YAML, CSV, XML, etc.
  - `markup` - HTML, SVG
  - `prose` - Markdown, Text
  - Type classification can be overridden per glob pattern via `reclassify` in the project config — see [configuration.md](configuration.md#reclassify)
- **`analyzed`** - Files SCC can fully parse (code/comments/blanks/complexity breakdown)
- **`unanalyzed`** - Files SCC cannot parse (only line counts)

### Stats Fields

- `lines` - Total lines in file
- `code` - Lines of code (excluding comments and blanks)
- `comments` - Comment lines
- `blanks` - Blank lines
- `complexity` - Cyclomatic complexity (for supported languages)
- `files` - Number of files

### Derived Metrics

Programming languages only:

| KPI | Formula | Insight |
|-----|---------|---------|
| `comment_ratio` | comments / code | Documentation level (10-20% typical) |
| `code_density` | code / lines | Actual code vs whitespace/comments |
| `avg_file_size` | lines / files | File granularity |
| `complexity_per_kloc` | complexity / (code/1000) | Maintainability indicator |
| `avg_complexity` | complexity / files | Per-file complexity |
| `primary_languages` | primary programming languages (>=1%) | Main programming languages |

All values rounded to 2 decimal places. KPIs are computed from programming languages only (excludes data formats like JSON, YAML, CSV).

### Per-Component Code Statistics

Enable per-component code statistics with `--component-stats-depth N` to get detailed metrics for each detected component up to depth N in the component tree (e.g., each Maven module, npm package, or Go module):

```bash
# Include code_stats on top-level components only (depth 1)
./bin/stack-analyzer scan --component-stats-depth 1 /path/to/project

# Include code_stats on first two levels of the component tree
./bin/stack-analyzer scan --component-stats-depth 2 /path/to/project
```

Key points:
- **Root `code_stats`**: Global statistics for the entire codebase (all files) — always present
- **Component `code_stats`**: Statistics for all files under that component's subtree
- **Depth 0 (default)**: No per-component stats — global stats only, no overhead
- **Global >= Sum of components**: Root-level files not inside any component are only in global stats

This is useful for:
- Identifying large/complex top-level modules in monorepos
- Tracking code growth per component over time
- Finding components with low comment ratios or high complexity

### Subsystem Statistics

For large monorepos with many top-level folders, `--subsystem-depth N` produces a `subsystem_stats[]` array at the root level with one rolled-up entry per depth-N path prefix:

```bash
# One stat entry per depth-1 folder (/server, /frontend, /libs, etc.)
./bin/stack-analyzer scan --subsystem-depth 1 /path/to/project
```

Each entry aggregates all files under that folder — regardless of how many individual components are inside. This gives a clean per-subsystem breakdown without polluting the flat `components[]` list.

For named groups (aggregating multiple folders into a single logical subsystem), define `subsystem-groups` in a config file:

```bash
./bin/stack-analyzer scan \
  --config '{"subsystem-groups":{"core":{"paths":["/core","/platform"],"description":"Core platform"},"services":{"paths":["/svc-a","/svc-b"],"description":"Business services"}}}' \
  /path/to/project
```

When `subsystem-groups` is defined, `--subsystem-depth` is ignored — the named groups take precedence. See [configuration.md](configuration.md#subsystem-groups) for details.

Key points:
- **`subsystem_stats[]`** appears on the root node of both full and aggregated outputs
- **`path`**: the depth-N prefix (e.g. `/server`) or group name (e.g. `core-platform`)
- **`paths`**: source folder prefixes from config (only present in named-group mode; absent in depth mode)
- **`description`**: group description from config (only present in named-group mode)
- **`component_count`**: number of components in that subsystem
- **`techs`**: deduplicated union of all component `techs` in this subsystem, sorted alphabetically
- **`languages`**: merged language file counts from all components in this subsystem (language name to file count)
- **`code_stats`**: full stats structure identical to root `code_stats`
- **Independent of `--component-stats-depth`**: both flags can be used together or separately

#### Example output (named groups)

When using `subsystem-groups` in the config, each entry includes the group name, paths, description, techs, and languages:

```json
{
  "subsystem_stats": [
    {
      "path": "core",
      "paths": ["/core", "/platform", "/shared"],
      "description": "Core framework and shared platform libraries",
      "component_count": 8,
      "techs": ["java", "kotlin", "maven", "spring-boot"],
      "languages": { "Java": 280, "Kotlin": 32 },
      "code_stats": {
        "total": { "lines": 45200, "code": 38100, "comments": 3200, "blanks": 3900, "complexity": 2100, "files": 312 },
        "by_type": { "programming": { "total": { "lines": 38000, "code": 32000 }, "languages": ["Java", "Kotlin"] } }
      }
    },
    {
      "path": "services",
      "paths": ["/svc-auth", "/svc-billing", "/svc-notifications"],
      "description": "Business services",
      "component_count": 12,
      "techs": ["java", "maven", "postgresql", "redis", "spring-boot"],
      "languages": { "Java": 489 },
      "code_stats": {
        "total": { "lines": 67800, "code": 55400, "comments": 5600, "blanks": 6800, "complexity": 3400, "files": 489 },
        "by_type": { "programming": { "total": { "lines": 58000, "code": 48000 }, "languages": ["Java"] } }
      }
    }
  ]
}
```

In depth mode (`--subsystem-depth 1`), entries also include `techs` and `languages` but have no `paths` or `description` fields:

```json
{
  "subsystem_stats": [
    { "path": "/server", "component_count": 5, "techs": ["java", "spring-boot"], "languages": { "Java": 120 }, "code_stats": { "..." : "..." } },
    { "path": "/frontend", "component_count": 3, "techs": ["nodejs", "react"], "languages": { "TypeScript": 85 }, "code_stats": { "..." : "..." } }
  ]
}
```

## Multi-Git Repository Support

The analyzer automatically detects git repositories at both root and component levels, enabling tracking of multiple repositories within a single scan. Each component shows its own git information (branch, commit, dirty status, remote URL), making it ideal for monorepos, workspace scans, and CI/CD pipelines where different sub-projects may be in different git states.

## Progress Output

By default the scanner shows a single updating progress line on stderr with a spinner, file/directory/component counts, and elapsed time. When the scan completes it resolves to a one-line summary:

```
  ✓  852 files, 195 dirs, 4 components  (3s)
```

Progress is suppressed automatically when stderr is not a TTY (piped output).

**Flags:**
- `--quiet, -q` — suppress all progress output
- `--verbose, -v` — show per-directory and per-component events (detailed)
- `--debug, -d` — show tree-structured output

```bash
# Default (summary progress line)
./bin/stack-analyzer scan /path/to/project

# Quiet (no output to stderr)
./bin/stack-analyzer scan --quiet --output results.json /path

# Verbose (per-event detail)
./bin/stack-analyzer scan --verbose /path/to/project

# Environment variable
STACK_ANALYZER_VERBOSE=true ./bin/stack-analyzer scan /path
```

Progress output is always sent to **stderr**, keeping it separate from JSON output. This allows piping JSON to tools while still seeing progress.

## Component Classification

The scanner distinguishes between **architectural components** and **tools/libraries**. Technologies like databases, hosting services, and SaaS platforms create components (appear in `tech` field), while development tools, frameworks, and languages are listed only in the `techs` array.

This classification is fully configurable through type definitions and per-rule overrides. See [extending.md](extending.md) for details on the Technology Type Configuration.

## Content-Based Detection

The scanner validates technology detection through **independent content pattern matching**. This enables precise identification of libraries and frameworks that share common file extensions.

### Independent Detection Logic

**Extension/File Detection:** Rules with `extensions` or `files` fields detect technologies by file presence alone.

**Content Detection:** Rules with `content` fields detect technologies by matching patterns in file contents. Content patterns must specify which files to check via their own `extensions` or `files` restrictions.

**Key Principle:** Content matching is **independent** of top-level extensions/files. Each content pattern defines its own scope where to look for matches.

### Rule Examples

**Content-Only Detection:**
```yaml
tech: mfc
name: Microsoft Foundation Class Library
type: ui_framework
content:
  - pattern: '#include\s+<afx'
    extensions: [.cpp, .h, .hpp]
  - pattern: 'class\s+\w+\s*:\s*public\s+C(Wnd|FrameWnd|CDialog)'
    extensions: [.cpp, .h, .hpp]
  - pattern: '(BEGIN_MESSAGE_MAP|END_MESSAGE_MAP|DECLARE_MESSAGE_MAP)'
    extensions: [.cpp, .h, .hpp]
```

**Behavior:**
- `.cpp` file with `#include <afx` -> Content pattern matches -> **MFC detected**
- `.cpp` file without MFC patterns -> No content matches -> **MFC not detected**
- Pure C++ project -> No MFC patterns -> **MFC not detected** (no false positives)

**Hybrid Detection** (Extension + Content):
```yaml
tech: qt
name: Qt Framework
type: ui
extensions: [.pro, .ui, .qrc]  # Qt-specific files
content:
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h, .hpp, .c]  # Check C++ files for Qt code
  - pattern: 'Qt[0-9]::'
    files: [CMakeLists.txt]           # Check CMake files
  - pattern: '<ui\s+version='
    extensions: [.ui]                 # Check UI files
```

### Use Cases

- **Distinguish similar technologies**: MFC vs Qt vs plain C++ in `.h` files
- **Library-specific detection**: Framework-specific patterns in common file types
- **Mixed file types**: Qt `.pro` files (no content check) + `.cpp` files (with content check)
- **Specific file validation**: Only check `package.json`, not all `.json` files
- **Prevent false positives**: Ensure actual usage, not just file presence
