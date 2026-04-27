# Configuration

## Project Configuration File

Place a `.stack-analyzer.yml` file in your project root to customize scan behavior, add metadata, and document external dependencies.

```yaml
# .stack-analyzer.yml - Tech Stack Analyzer Configuration

# Custom properties added to metadata.properties in scan output
properties:
  product: "My Product Name"
  team: "Platform Engineering"
  environment: "production"
  owner: "engineering@company.com"

# Files and directories to exclude from scanning
# These patterns use full gitignore semantics (**, *, ?, negation, dir-only)
exclude:
  - "build-cache"          # Exclude build cache directory
  - "*.tmp"                # Exclude temporary files
  - "**/__tests__/**"      # Exclude test directories
  - "vendor/"              # Exclude vendor directory only (not files named "vendor")
  - "!vendor/important/**" # Re-include important vendor subdirectory (negation)
  - "!.NET/**"             # Re-include .NET directory (overrides dot-dir default exclusion)

# Technologies to add to scan results (even if not auto-detected)
techs:
  - tech: "aws"
    reason: "Deployed on AWS ECS"
  - tech: "datadog"
    reason: "Monitoring via Datadog"

# Scan behavior options
scan:
  primary_language_threshold: 0.05 # Minimum percentage for primary languages (default: 0.05 = 5%)
  # debug: false
  # verbose: false
```

### Configuration Options

- **`properties`** - Custom metadata added to `metadata.properties` in output
  - Document product context, ownership, deployment information
  - Any key-value pairs relevant to your project

- **`exclude`** - Additional patterns to exclude from scanning
  - **Combined with .gitignore**: These patterns are added to automatic `.gitignore` exclusions
  - **Full gitignore semantics**: supports the same pattern syntax as `.gitignore` files
    - Glob patterns: `**` (recursive), `*` (single segment), `?` (single char)
    - **Negation**: prefix with `!` to re-include a previously excluded path (e.g. `!.NET/**`)
    - **Dir-only**: trailing `/` matches only directories, not files (e.g. `build/`)
    - **Last-match-wins**: when multiple patterns match, the last one determines the outcome
  - Merged with CLI `--exclude` flags and `.gitignore` files into a single evaluation stack

- **`techs`** - Technologies to force-add to scan results
  - Useful for external dependencies (AWS, SaaS services)
  - Manual documentation of deployment targets or platforms

- **`subsystem-groups`** *(optional)* - Named subsystem groups for `subsystem_stats[]` rollup. Only needed for large monorepos with 10+ top-level folders where depth-based splitting (via `--subsystem-depth`) produces too many entries. When defined, overrides `--subsystem-depth`. See [Subsystem Groups](#subsystem-groups) below.

- **`reclassify`** - Override language detection for specific file patterns. See [Reclassify](#reclassify) below.

- **`scan`** - Scan behavior configuration options
  - **`component_stats_depth`** - Include `code_stats` on components up to this tree depth in output (default: 0 = none). Matches `--component-stats-depth` flag.
  - **`subsystem_depth`** - Produce `subsystem_stats[]` rolled up per depth-N path prefix (default: 0 = none). Ignored when `subsystem-groups` is defined. Matches `--subsystem-depth` flag.
  - **`primary_language_threshold`** - Minimum percentage (0.001-1.0) for a programming language to be considered primary
    - Default: 0.05 (5%)
    - Lower values show more languages, higher values show only dominant languages
    - Example: 0.01 shows languages with >=1% usage, 0.10 shows only languages with >=10% usage
  - **`use_lock_files`** - Use lock files for dependency resolution (default: true)
    - When enabled, extracts exact versions from lock files (package-lock.json, Cargo.lock, etc.)
    - Set to `false` to use version ranges from manifest files instead

### Benefits

- **Version controlled** - Configuration lives with code
- **Team-shared** - Everyone uses same exclusions and metadata
- **Documented** - External dependencies explicitly listed
- **Flexible** - Custom metadata for any use case

See `.stack-analyzer.yml.example` for a complete configuration template.

## Environment Variables

```bash
# Output configuration (default: stack-analysis.json in current directory)
export STACK_ANALYZER_OUTPUT=/tmp/scan-results.json
export STACK_ANALYZER_PRETTY=false

# Scan behavior
export STACK_ANALYZER_EXCLUDE_DIRS=vendor,node_modules,build
export STACK_ANALYZER_AGGREGATE=tech,techs,languages,git
export STACK_ANALYZER_VERBOSE=true         # Show detailed progress information
export STACK_ANALYZER_USE_LOCK_FILES=false        # Disable lock file parsing (default: true)
export STACK_ANALYZER_COMPONENT_STATS_DEPTH=1    # Include code_stats on depth-1 components
export STACK_ANALYZER_SUBSYSTEM_DEPTH=1          # Produce subsystem_stats per depth-1 folder

# Logging
export STACK_ANALYZER_LOG_LEVEL=debug      # trace, debug, error, fatal (default: error)
export STACK_ANALYZER_LOG_FORMAT=json      # text or json
export STACK_ANALYZER_LOG_FILE=debug.log   # Optional: write logs to file
```

## Scan Configuration Files

The `--config` flag supports comprehensive scan configuration through YAML files or inline JSON, enabling multi-path scanning, custom metadata, and unified option management.

### Configuration Precedence

(highest to lowest):
1. **CLI arguments** - Always take precedence over all other sources
2. **Scan config file** - Overrides project config and environment variables
3. **`.stack-analyzer.yml`** - Project-specific config (merged with scan config)
4. **Environment variables** - Provide defaults for unset values
5. **Built-in defaults** - Used when nothing else is specified

### Usage Examples

```bash
# YAML configuration file
stack-analyzer scan --config scan-config.yml

# Inline JSON configuration (ideal for CI/CD pipelines)
stack-analyzer scan --config '{"scan":{"paths":["./src","./tests"],"options":{"debug":true}}}'

# Portfolio analysis with multiple repositories
stack-analyzer scan --config portfolio.yml --output portfolio-analysis.json
```

### Configuration Features

- **Multi-path scanning** - Specify multiple directories and files to analyze
- **Custom metadata** - Add project properties (team, environment, version, etc.)
- **Unified options** - All scanner flags configurable in one place
- **External technologies** - Document SaaS services and deployment targets
- **Flexible exclusions** - Project-specific ignore patterns beyond .gitignore
- **Language reclassification** - Override go-enry's language detection per glob pattern (see [Reclassify](#reclassify))
- **Inline JSON support** - Perfect for CI/CD and automation pipelines

See `stack-analyzer-config.example.yml` for a complete configuration template with all available options and precedence examples.

### Reclassify

The `reclassify` option overrides go-enry's (GitHub Linguist) language detection for files matching glob patterns. This is useful when file extensions are ambiguous or misclassified — for example, proprietary data files that share extensions with known programming languages.

Each rule requires a `match` glob pattern and at least one of `language` or `type`:

```yaml
reclassify:
  # Relabel AND recategorize — both language label and type bucket are overridden
  - match: "**/*.e"
    language: CSV      # Override the language label (must be a go-enry known language for automatic type resolution)
    type: data         # Override the type category: programming, data, markup, prose

  # Language-only — type is resolved automatically from go-enry's knowledge of the language
  - match: "**/*.h"
    language: "C++"    # .h files in this project are always C++, not C

  # Type-only — language label is left empty; file still contributes to by-type stats
  - match: "**/generated/**"
    type: data         # Force all generated files into the data bucket regardless of extension
```

**Rule fields:**

| Field | Required | Description |
|---|---|---|
| `match` | Yes | Glob pattern matched relative to the scan root. Supports `**` for recursive matching. |
| `language` | One of `language`/`type` | Override the detected language label. Use a language name known to GitHub Linguist (e.g. `CSV`, `C++`, `JavaScript`) for automatic type resolution. Unknown names produce `unknown` type unless `type` is also set. |
| `type` | One of `language`/`type` | Override the language type category. One of: `programming`, `data`, `markup`, `prose`. When omitted, type is derived from `language` via go-enry. |

**Precedence:** Rules are evaluated in order — the first match wins. When both `.stack-analyzer.yml` and `--config` define rules, project config rules (`.stack-analyzer.yml`) take precedence and are checked first.

**Behaviour summary:**

| Config | Language in output | Type bucket |
|---|---|---|
| `language: CSV` | `CSV` | `data` (go-enry knows CSV is data) |
| `language: CSV` + `type: data` | `CSV` | `data` (explicit, redundant but safe) |
| `type: data` only | *(absent)* | `data` |
| `language: "MyFormat"` only | `MyFormat` | `unknown` (go-enry doesn't know it) |
| `language: "MyFormat"` + `type: data` | `MyFormat` | `data` |

### Subsystem Groups

The `subsystem-groups` config option lets you define named logical groups that aggregate multiple depth-1 folders into a single `subsystem_stats` entry. This is useful for large monorepos (10+ top-level folders) where depth-based folder splitting produces too many entries to be useful.

```yaml
# stack-analyzer-config.yml
subsystem-groups:
  core:
    paths: [/core, /platform, /shared]
    description: "Core framework and shared platform libraries"
  services:
    paths: [/svc-auth, /svc-billing, /svc-notifications]
    description: "Business services"
  integrations:
    paths: [/integration, /adapters, /connectors]
    description: "External system integrations"
```

Or equivalently as inline JSON for CI/CD:

```bash
stack-analyzer scan \
  --config '{"subsystem-groups":{"core":{"paths":["/core","/platform"]},"services":{"paths":["/svc-auth","/svc-billing"]}}}' \
  /path/to/project
```

Key behaviour:
- When `subsystem-groups` is defined, `--subsystem-depth` is ignored
- Folders not listed in any group are excluded from `subsystem_stats` (but still counted in global `code_stats`)
- Each group's `code_stats` aggregates all files under all listed paths
- Group names become the `path` field in each `subsystem_stats` entry

## Logging

The scanner separates data output from progress messages following Unix philosophy.

### Output Channels

- **stdout** - Structured data (JSON) for piping to tools like `jq`
- **stderr** - Human messages (progress, errors, confirmations)
- **Log file** - Developer debugging (optional)

### Piping Examples

```bash
# Pipe JSON to jq (use -o - for stdout)
./bin/stack-analyzer scan -o - /path | jq '.techs'

# Alternative: use /dev/stdout
./bin/stack-analyzer scan -o /dev/stdout /path | jq '.metadata.file_count'

# Show progress while piping (progress on stderr, data pipes)
./bin/stack-analyzer scan --verbose -o - /path | jq '.languages'

# Suppress stderr if needed
./bin/stack-analyzer scan --verbose -o - /path 2>/dev/null | jq
```

### Logging Levels

```bash
# Debug logging (internal operations)
./bin/stack-analyzer scan /path --log-level debug

# Trace logging (deep debugging with data inspection)
./bin/stack-analyzer scan /path --log-level trace

# Write logs to file (keeps stderr clean)
./bin/stack-analyzer scan /path --log-level debug --log-file debug.log

# JSON format for automated processing
./bin/stack-analyzer scan /path --log-level debug --log-format json

# Combine verbose progress + debug logs to file
./bin/stack-analyzer scan --verbose --log-level debug --log-file debug.log /path
```

### Available Log Levels

- `trace` - Deep debugging (rule matching, data inspection)
- `debug` - Internal operations (component detection, file processing)
- `error` - Non-fatal errors (default - only errors shown)
- `fatal` - Fatal errors (exit immediately)

### Log Output Examples

Text format:
```
time="2025-12-02 15:30:26" level=debug msg="Initializing scanner" path=/path exclude_dirs="[]"
time="2025-12-02 15:30:26" level=debug msg="Scanning directory" directory=/path
time="2025-12-02 15:30:27" level=debug msg="Generating output" aggregate= pretty_print=true
```

JSON format:
```json
{"level":"debug","msg":"Initializing scanner","path":"/path","time":"2025-12-02 15:30:26"}
{"directory":"/path","level":"debug","msg":"Scanning directory","time":"2025-12-02 15:30:26"}
{"aggregate":"","level":"debug","msg":"Generating output","pretty_print":true,"time":"2025-12-02 15:30:27"}
```
