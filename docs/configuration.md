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
# These patterns are ADDED to .gitignore exclusions
# Supports glob patterns (**, *, ?)
exclude:
  - "build-cache"      # Additional build cache not in .gitignore
  - "*.tmp"            # Temporary files
  - "**/__tests__/**"  # Test directories (if not in .gitignore)
  - "**/*.test.js"     # Test files (if not in .gitignore)

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
  - **Combined with .gitignore**: These patterns are added to automatic .gitignore exclusions
  - Supports glob patterns: `**`, `*`, `?`
  - Matches files and directories
  - Use for patterns not in your .gitignore or project-specific exclusions
  - Merged with CLI `--exclude` flags

- **`techs`** - Technologies to force-add to scan results
  - Useful for external dependencies (AWS, SaaS services)
  - Manual documentation of deployment targets or platforms

- **`scan`** - Scan behavior configuration options
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
export STACK_ANALYZER_USE_LOCK_FILES=false # Disable lock file parsing (default: true)

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
- **Inline JSON support** - Perfect for CI/CD and automation pipelines

See `scan-config.example.yml` for a complete configuration template with all available options and precedence examples.

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
