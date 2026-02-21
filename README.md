# Tech Stack Analyzer

> **Focused on Fast Dependency Discovery**  
> This tool specializes in rapid technology stack detection and dependency discovery. For comprehensive license compliance, security analysis, or deep file scanning, integrate our output with specialized tools in the software supply chain security ecosystem.

A technology stack analyzer written in Go, re-implementing [specfy/stack-analyser](https://github.com/specfy/stack-analyser) with improvements and extended technology support.

## Purpose & Philosophy

**We do one thing exceptionally well: fast, reliable dependency discovery.**

The Tech Stack Analyzer is designed to be the **fastest way to understand what technologies and dependencies your codebase uses**. We focus on speed and accuracy while leaving specialized analysis to dedicated tools:

- **Fast Dependency Detection** - Identify technologies, frameworks, and dependencies in seconds
- **Zero Dependencies** - Single binary deployment, no runtime requirements  
- **Technology Inventory** - Complete overview of your stack for documentation and planning
- **Deep License Analysis** - Use specialized license compliance tools with our output (not covered)
- **Security Scanning** - Use dedicated vulnerability scanners with our dependency list (not covered)
- **File-level Analysis** - Use specialized tools for deep code analysis (not covered)

**Integration Approach:** Our structured output serves as the perfect input for license compliance tools, vulnerability scanners, and software composition analysis (SCA) platforms.

## Use Cases

### Primary Use Cases - What We Excel At:

**Technology Inventory & Documentation**
- Generate comprehensive technology stack documentation
- Create architecture diagrams and dependency maps
- Portfolio analysis across multiple repositories
- M&A due diligence - quick technology assessment

**CI/CD Integration**
- Fast dependency detection in build pipelines
- Technology compliance checks
- Stack drift monitoring
- Automated documentation generation

**Development Planning**
- Technology standardization initiatives
- Migration planning (e.g., cloud migration)
- Skill gap analysis based on detected technologies
- Training needs assessment

### Integration Examples:

**License Compliance Pipeline:**
```bash
# 1. Fast dependency detection
./stack-analyzer scan /project --output deps.json

# 2. License analysis (specialized tool)
license-checker --input deps.json --policy company-policy.json
```

**Security Monitoring:**
```bash
# 1. Dependency discovery
./stack-analyzer scan /project --aggregate dependencies --output deps.json

# 2. Vulnerability scanning
vuln-scanner --dependencies deps.json --database latest
```

**Portfolio Analysis:**
```bash
# Analyze 100 repositories in minutes
for repo in company-projects/*; do
  ./stack-analyzer scan "$repo" --output "results/$(basename $repo).json"
done
```

## What This Project Does

The Tech Stack Analyzer automatically detects technologies, frameworks, databases, and tools used in codebases by analyzing files, dependencies, and configurations. It provides comprehensive insights into:

- **Programming Languages** - Detects source code languages and versions
- **Package Managers** - Identifies npm, pip, cargo, composer, nuget, maven dependencies
- **Frameworks** - Detects .NET, Spring Boot, Angular, React, Django frameworks
- **Databases** - Identifies PostgreSQL, MySQL, MongoDB, Redis, Oracle, SQL Server
- **Infrastructure** - Detects Docker, Kubernetes, Terraform, GitLab configurations
- **DevOps Tools** - Identifies CI/CD pipelines, monitoring, and deployment tools

**Detection Engine:** The analyzer uses 800+ technology rules that can detect technologies through:
- File names and extensions (`.py`, `package.json`, `Dockerfile`)
- Package dependencies across multiple ecosystems  
- Environment variables and configuration files
- Content patterns for precise identification
- Custom detection logic for complex file formats

**Advanced Analysis:** For key technologies, the analyzer extracts detailed metadata:
- **Docker** - Base images, exposed ports, multi-stage builds, stages
- **Terraform** - Providers, resource counts by category, total resources
- **Kubernetes** - Deployments, services, configurations
- **Package Files** - Exact versions from lock files, dependency relationships

**Lock File Support:** The analyzer automatically uses lock files to extract exact resolved versions instead of version ranges:
- **Node.js** - `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock` → falls back to `package.json`
- **Python** - `uv.lock`, `poetry.lock` → falls back to `pyproject.toml`, `requirements.txt`, `setup.py`
- **Rust** - `Cargo.lock` → falls back to `Cargo.toml`
- **Go** - `go.mod` (already contains exact versions)

This ensures accurate dependency versions for security scanning and compliance analysis.

This structured metadata is exposed in the `properties` field of the output, 
enabling security scanning, license compliance, and infrastructure analysis.

See [How to Extend It](#how-to-extend-it) for complete rule documentation.

## Key Features

- **800+ Technology Rules** - Comprehensive detection across 48 technology categories (databases, frameworks, APIs, tools, cloud services)
- **Zero Dependencies** - Single binary deployment without Node.js runtime requirement
- **Automatic .gitignore Support** - Uses project's existing .gitignore files recursively for intelligent exclusions
- **Project Configuration** - `.stack-analyzer.yml` for custom metadata, exclusions, and external dependencies
- **Scan Metadata** - Automatic tracking of scan execution (timestamp, duration, file counts) with git information at top level
- **Glob Pattern Exclusions** - Flexible `--exclude` flag supporting `**`, `*`, `?` patterns for files and directories (overrides .gitignore)
- **Content-Based Detection** - Validates technologies through regex pattern matching in file contents for precise identification
- **Configurable Components** - Override default component classification per rule with `is_component` field
- **Tech-Specific Metadata** - Structured properties for Docker (base images, ports) and Terraform (providers, resource counts)
- **Multi-Technology Components** - Detects hybrid projects with multiple primary technologies in the same directory
- **Professional Logging** - Structured logging with multiple levels (trace/debug/info/warn/error) and JSON/text formats
- **Hierarchical Output** - Component-based analysis with parent-child relationships
- **Aggregated Views** - Rollup summaries for quick technology stack overviews

## How to Use It

### Prerequisites

- **Go 1.19+** - For building from source
- **[Task](https://taskfile.dev)** (optional) - Task runner for build automation (see installation below)
- **Docker** (optional) - For containerized deployment

### Installation

#### Option 1: Build from Source
```bash
# Clone the repository
git clone https://github.com/petrarca/tech-stack-analyzer.git
cd tech-stack-analyzer

# Build stack-analyzer
go build -o bin/stack-analyzer ./cmd/scanner

# Or use Task (recommended)
task build
```

#### Option 2: Install Directly
```bash
go install github.com/petrarca/tech-stack-analyzer/cmd/scanner@latest
```

### Basic Usage

The analyzer uses a command-based interface powered by [Cobra](https://github.com/spf13/cobra):

```bash
# Get help
./bin/stack-analyzer --help
./bin/stack-analyzer scan --help
./bin/stack-analyzer info --help

# Scan current directory (automatically uses .gitignore patterns)
./bin/stack-analyzer scan

# Scan specific directory (automatically uses project's .gitignore files)
./bin/stack-analyzer scan /path/to/project

# Scan multiple directories (merged into one output)
./bin/stack-analyzer scan /path/to/project1 /path/to/project2

# Save results to custom file
./bin/stack-analyzer scan /path/to/project --output results.json

# Override .gitignore exclusions with additional patterns (supports glob patterns)
./bin/stack-analyzer scan /path/to/project --exclude "vendor" --exclude "build-cache" --exclude "*.tmp"

# Scan a single file (useful for quick testing)
./bin/stack-analyzer scan /path/to/pom.xml
./bin/stack-analyzer scan /path/to/package.json
./bin/stack-analyzer scan /path/to/pyproject.toml

# Aggregate output (rollup technologies, languages, licenses, dependencies, git, reasons)
./bin/stack-analyzer scan --aggregate tech,techs,languages,licenses,dependencies,git,reason /path/to/project
./bin/stack-analyzer scan --aggregate all /path/to/project  # Aggregate all fields
./bin/stack-analyzer scan --aggregate reason /path/to/project  # Just reasons

# List all available technologies
./bin/stack-analyzer info techs

# Show rule details for a specific technology
./bin/stack-analyzer info rule postgresql
./bin/stack-analyzer info rule postgresql --format json

# List technology categories
./bin/stack-analyzer info categories

# List component categories only
./bin/stack-analyzer info categories --components
```

### Output Example

The scanner outputs a hierarchical JSON structure showing detected technologies, components, and their relationships:

**Regular Output:**
```json
{
  "id": "root",
  "name": "my-project",
  "path": "/",
  "tech": ["nodejs"],
  "techs": ["nodejs", "react", "postgresql", "docker"],
  "languages": {"JavaScript": 145, "TypeScript": 89},
  "reason": {
    "docker": ["matched file: Dockerfile"],
    "react": ["react matched: ^react$"],
    "_": ["base image: nginx:alpine", "license detected: MIT"]
  },
  "dependencies": [
    ["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "^4.18.2", "prod", true, {"source": "package-lock.json"}]
  ],
  "git": {
    "branch": "main",
    "commit": "a1b2c3d",
    "remote_url": "https://github.com/user/repo.git"
  },
  "children": [
    {
      "id": "backend",
      "name": "backend", 
      "path": "/backend",
      "type": "npm-package",
      "tech": ["nodejs"],
      "techs": ["nodejs", "express", "postgresql"],
      "component_dependencies": [
        ["docker-base-image", "node", "20-alpine", "", {"file": "/backend/Dockerfile"}]
      ],
      "git": {
        "branch": "develop",
        "commit": "def5678",
        "remote_url": "https://github.com/company/backend.git"
      }
    }
  ],
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "duration_ms": 1173,
    "file_count": 523
  }
}
```

**Aggregated Output** (`--aggregate techs,languages,dependencies,git`):
```json
{
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "scan_path": "/path/to/project",
    "specVersion": "0.1",
    "duration_ms": 1173,
    "file_count": 523
  },
  "techs": ["nodejs", "react", "postgresql", "docker", "express", "vite"],
  "languages": {"JavaScript": 145, "TypeScript": 89, "CSS": 12},
  "dependencies": [
    ["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "^4.18.2", "prod", true, {"source": "package-lock.json"}],
    ["npm", "vite", "^5.0.0", "dev", true, {"source": "package-lock.json"}]
  ],
  "git": [
    {
      "branch": "main",
      "commit": "abc1234",
      "remote_url": "https://github.com/user/project.git"
    }
  ]
}
```

**Key Fields:**
- `tech` - Primary technologies (creates components)
- `techs` - All detected technologies (components + tools/libraries)
- `children` - Nested components (sub-projects, services)
- `dependencies` - Package dependencies with versions
- `code_stats` - Code statistics (lines, code, comments, blanks, complexity)
- `git` - Git repository information (branch, commit, dirty status, remote URL)
- `metadata` - Scan execution info (timestamp, duration, file counts)

See [Output Structure](#output-structure) for complete field descriptions.

### Multi-Git Repository Support

The analyzer automatically detects git repositories at both root and component levels, enabling tracking of multiple repositories within a single scan. Each component shows its own git information (branch, commit, dirty status, remote URL), making it ideal for monorepos, workspace scans, and CI/CD pipelines where different sub-projects may be in different git states.

### Code Statistics

The scanner automatically collects code statistics using [SCC](https://github.com/boyter/scc) (Sloc, Cloc and Code). Statistics are enabled by default and can be disabled with `--no-code-stats`.

```bash
# Default: code stats enabled
./bin/stack-analyzer scan /path/to/project

# Disable code stats
./bin/stack-analyzer scan --no-code-stats /path/to/project
```

**Output Structure:**
```json
{
  "code_stats": {
    "total": { "lines": 39212, "code": 32834, "comments": 2027, "blanks": 4351, "complexity": 1960, "files": 858 },
    "by_type": {
      "programming": { 
        "total": { "lines": 22023, "code": 16826, ... }, 
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
      "data": { "total": { "lines": 12575, ... }, "languages": ["YAML", "JSON", "Go Checksums"] },
      "prose": { "total": { "lines": 5003, ... }, "languages": ["Markdown", "Text"] }
    },
    "analyzed": {
      "total": { ... },
      "by_language": [
        {"language": "Go", "lines": 21841, "code": 16679, "comments": 1963, ...},
        {"language": "YAML", "lines": 11385, "code": 11258, ...}
      ]
    },
    "unanalyzed": {
      "total": {"lines": 389, "files": 3},
      "by_language": [{"language": "Go Checksums", "lines": 253, "files": 1}, ...]
    }
  }
}
```

**Fields:**
- **`total`** - Grand total for all analyzed files
- **`by_type`** - Stats grouped by [GitHub Linguist](https://github.com/github-linguist/linguist) language type:
  - `programming` - Go, C++, Java, Python, etc. (includes metrics)
  - `data` - JSON, YAML, CSV, XML, etc.
  - `markup` - HTML, SVG
  - `prose` - Markdown, Text
- **`analyzed`** - Files SCC can fully parse (code/comments/blanks/complexity breakdown)
- **`unanalyzed`** - Files SCC cannot parse (only line counts)

**Stats Fields:**
- `lines` - Total lines in file
- `code` - Lines of code (excluding comments and blanks)
- `comments` - Comment lines
- `blanks` - Blank lines
- `complexity` - Cyclomatic complexity (for supported languages)
- `files` - Number of files

**Derived Metrics** (programming languages only):
```json
{
  "by_type": {
    "programming": {
      "total": { "lines": 6849401, "code": 5298522, ... },
      "metrics": {
        "comment_ratio": 0.14,
        "code_density": 0.77,
        "avg_file_size": 400.6,
        "complexity_per_kloc": 165.08,
        "avg_complexity": 51.16,
        "primary_languages": [
          {"language": "C++", "pct": 0.90},
          {"language": "C", "pct": 0.05},
          {"language": "C#", "pct": 0.02}
        ]
      },
      "languages": ["C++", "C", "C#"]
    }
  }
}
```

| KPI | Formula | Insight |
|-----|---------|---------|
| `comment_ratio` | comments / code | Documentation level (10-20% typical) |
| `code_density` | code / lines | Actual code vs whitespace/comments |
| `avg_file_size` | lines / files | File granularity |
| `complexity_per_kloc` | complexity / (code/1000) | Maintainability indicator |
| `avg_complexity` | complexity / files | Per-file complexity |
| `primary_languages` | primary programming languages (≥1%) | Main programming languages |

All values rounded to 2 decimal places. KPIs are computed from **programming languages only** (excludes data formats like JSON, YAML, CSV).

### Per-Component Code Statistics

Enable per-component code statistics with `--component-code-stats` to get detailed metrics for each detected component (e.g., each Maven module, npm package, or Go module):

```bash
# Enable per-component code stats
./bin/stack-analyzer scan --component-code-stats /path/to/project
```

**Output Structure:**
```json
{
  "code_stats": {
    "total": { "lines": 253628, "code": 200321, "files": 4916 }
  },
  "children": [
    {
      "name": "module-api",
      "tech": ["java"],
      "code_stats": {
        "total": { "lines": 12500, "code": 9800, "files": 45 },
        "analyzed": { "by_language": [{"language": "Java", "lines": 11200, ...}] }
      }
    },
    {
      "name": "module-core", 
      "tech": ["java"],
      "code_stats": {
        "total": { "lines": 48000, "code": 38000, "files": 180 }
      }
    }
  ]
}
```

**Key Points:**
- **Root `code_stats`**: Global statistics for the entire codebase (all files)
- **Component `code_stats`**: Statistics for files directly in that component only
- **Global ≥ Sum of components**: Root-level files not in any component are only in global stats
- **Zero overhead when disabled**: No performance impact when flag is not used

This is useful for:
- Identifying large/complex modules in monorepos
- Tracking code growth per component over time
- Finding components with low comment ratios or high complexity

### Automatic .gitignore Support

The scanner automatically uses your project's existing `.gitignore` files for intelligent exclusions:

#### How It Works
- **Recursive Loading**: Finds and loads ALL `.gitignore` files from root to subdirectories
- **Git-compatible Behavior**: Processes patterns the same way Git does (hierarchical merging)
- **Smart Filtering**: Skips problematic cache directories that contain `*` patterns
- **Pattern Support**: Supports glob patterns (`*`, `?`, `**`) and file extensions

#### What Gets Excluded Automatically
Common patterns that work out of the box:
- **Node.js**: `node_modules`, `dist`, `build`, `.npm`, `.yarn`
- **Python**: `.venv`, `venv`, `__pycache__`, `.pytest_cache`, `.ruff_cache`
- **Build Tools**: `target`, `build`, `dist`, `.next`, `.nuxt`
- **IDE Files**: `.vscode`, `.idea`, `*.swp`, `*.swo`
- **OS Files**: `.DS_Store`, `Thumbs.db`
- **Cache/Temp**: `.cache`, `.tmp`, `*.log`

#### Override .gitignore
Use `--exclude` flags to add additional exclusions or override .gitignore patterns:
```bash
# Add extra exclusions beyond .gitignore
./bin/stack-analyzer scan /path/to/project --exclude "build-cache" --exclude "*.tmp"

# .gitignore patterns are still respected, these are additional
```

#### Performance Benefits
Using .gitignore patterns provides significant performance improvements:
- **Fewer Files**: Skips thousands of unnecessary files (node_modules, .venv, etc.)
- **Faster Scans**: Typical 70-90% reduction in scan time
- **Accurate Results**: Focuses on source code and configuration files

### Project Configuration

#### `.stack-analyzer.yml` Configuration File

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

**Configuration Options:**

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
    - Example: 0.01 shows languages with ≥1% usage, 0.10 shows only languages with ≥10% usage
  - **`use_lock_files`** - Use lock files for dependency resolution (default: true)
    - When enabled, extracts exact versions from lock files (package-lock.json, Cargo.lock, etc.)
    - Set to `false` to use version ranges from manifest files instead

**Benefits:**
- **Version controlled** - Configuration lives with code
- **Team-shared** - Everyone uses same exclusions and metadata
- **Documented** - External dependencies explicitly listed
- **Flexible** - Custom metadata for any use case

See `.stack-analyzer.yml.example` for a complete configuration template.

### Configuration & Logging

The scanner supports configuration through command-line flags and environment variables. Environment variables provide defaults that can be overridden by flags.

#### Environment Variables

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

#### Scan Configuration Files

The `--config` flag supports comprehensive scan configuration through YAML files or inline JSON, enabling multi-path scanning, custom metadata, and unified option management.

**Configuration Precedence** (highest to lowest):
1. **CLI arguments** - Always take precedence over all other sources
2. **Scan config file** - Overrides project config and environment variables  
3. **`.stack-analyzer.yml`** - Project-specific config (merged with scan config)
4. **Environment variables** - Provide defaults for unset values
5. **Built-in defaults** - Used when nothing else is specified

**Usage Examples:**
```bash
# YAML configuration file
stack-analyzer scan --config scan-config.yml

# Inline JSON configuration (ideal for CI/CD pipelines)
stack-analyzer scan --config '{"scan":{"paths":["./src","./tests"],"options":{"debug":true}}}'

# Portfolio analysis with multiple repositories
stack-analyzer scan --config portfolio.yml --output portfolio-analysis.json
```

**Configuration Features:**
- **Multi-path scanning** - Specify multiple directories and files to analyze
- **Custom metadata** - Add project properties (team, environment, version, etc.)
- **Unified options** - All scanner flags configurable in one place
- **External technologies** - Document SaaS services and deployment targets
- **Flexible exclusions** - Project-specific ignore patterns beyond .gitignore
- **Inline JSON support** - Perfect for CI/CD and automation pipelines

See `scan-config.example.yml` for a complete configuration template with all available options and precedence examples.

#### Logging and Output Channels

The scanner separates data output from progress messages following Unix philosophy:

**Output Channels:**
- **stdout** - Structured data (JSON) for piping to tools like `jq`
- **stderr** - Human messages (progress, errors, confirmations)
- **Log file** - Developer debugging (optional)

**Piping Examples:**
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

**Logging Levels:**
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

**Available Log Levels:**
- `trace` - Deep debugging (rule matching, data inspection)
- `debug` - Internal operations (component detection, file processing)
- `error` - Non-fatal errors (default - only errors shown)
- `fatal` - Fatal errors (exit immediately)

**Log Output Examples:**

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

#### Verbose Mode

Show detailed progress information during scanning with the `--verbose` or `-v` flag:

```bash
# Enable verbose mode
./bin/stack-analyzer scan --verbose /path/to/project
./bin/stack-analyzer scan -v /path/to/project

# Combine with other flags
./bin/stack-analyzer scan -v --exclude node_modules --output results.json /path

# Environment variable
STACK_ANALYZER_VERBOSE=true ./bin/stack-analyzer scan /path
```

**Verbose Output Example:**
```
[SCAN] Starting: /path/to/project
[DIR]  Entering: /path/to/project
[COMP] Detected: backend (nodejs) at /path/to/project/backend
[DIR]  Entering: /path/to/project/backend/src
[SKIP] Excluding: /path/to/project/node_modules (excluded)
[COMP] Detected: frontend (nodejs) at /path/to/project/frontend
[DIR]  Entering: /path/to/project/frontend/src
[SCAN] Completed: 3247 files, 412 directories in 2.3s
```

**Event Types:**
- `[SCAN]` - Scan start and completion with statistics
- `[DIR]` - Directory traversal
- `[COMP]` - Component detection (projects, services)
- `[SKIP]` - Excluded directories (node_modules, .git, etc.)

Verbose output is sent to **stderr**, keeping it separate from JSON data output. This allows piping JSON to tools while still seeing progress.

### Commands

#### `scan` - Analyze a project or file

Scans a project directory or single file to detect technologies, frameworks, databases, and services.

**Usage:**
```bash
stack-analyzer scan [path] [flags]
```

**Flags:**
- `--config` - Scan configuration file path or inline JSON (YAML/JSON file path or inline JSON string starting with `{`)
- `--output, -o` - Output file path (default: stack-analysis.json). Use `-o -` or `-o /dev/stdout` for piping
- `--aggregate` - Aggregate fields: `tech,techs,languages,licenses,dependencies,git,all` (use `all` for all aggregated fields)
- `--exclude` - Additional patterns to exclude (combined with .gitignore; supports glob patterns like `**/__tests__/**`, `*.log`; can be specified multiple times)
- `--no-code-stats` - Disable code statistics collection (enabled by default)
- `--pretty` - Pretty print JSON output (default: true)
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

# Verbose mode
stack-analyzer scan -v /path/to/project
stack-analyzer scan --verbose --output results.json /path

# Logging examples
stack-analyzer scan /path --log-level debug --log-format json
stack-analyzer scan /path --log-level trace
```

#### `info` - Display information about rules and categories

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


### Component Classification

The scanner distinguishes between **architectural components** and **tools/libraries**. Technologies like databases, hosting services, and SaaS platforms create components (appear in `tech` field), while development tools, frameworks, and languages are listed only in the `techs` array.

This classification is fully configurable through type definitions and per-rule overrides. See the [Technology Type Configuration](#technology-type-configuration) section for details.

### Content-Based Detection

The scanner validates technology detection through **independent content pattern matching**. This enables precise identification of libraries and frameworks that share common file extensions.

#### Independent Detection Logic

**Extension/File Detection:** Rules with `extensions` or `files` fields detect technologies by file presence alone.

**Content Detection:** Rules with `content` fields detect technologies by matching patterns in file contents. Content patterns must specify which files to check via their own `extensions` or `files` restrictions.

**Key Principle:** Content matching is **independent** of top-level extensions/files. Each content pattern defines its own scope where to look for matches.

#### Rule Examples

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
- `.cpp` file with `#include <afx` → Content pattern matches → **MFC detected**
- `.cpp` file without MFC patterns → No content matches → **MFC not detected**
- Pure C++ project → No MFC patterns → **MFC not detected** (no false positives!)

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

**Behavior:**
- `.pro` file → Extension matches → **Qt detected** (no content check)
- `.cpp` file with `Q_OBJECT` → Content pattern matches → **Qt detected**
- `.cpp` file without Qt patterns → No content matches → **Qt not detected**
- `CMakeLists.txt` with `Qt6::` → Content pattern matches → **Qt detected**

**File-Specific Patterns:**
```yaml
tech: qt
name: Qt Framework
type: ui
content:
  - pattern: 'Qt[0-9]::'
    files: [CMakeLists.txt]    # Only check CMakeLists.txt
  - pattern: 'find_package\s*\(\s*Qt[0-9]'
    files: [CMakeLists.txt]    # Only check CMakeLists.txt
```

**Behavior:**
- `CMakeLists.txt` with `Qt6::` → Content pattern matches → **Qt detected**
- `other_file.txt` with `Qt6::` → Wrong filename → **Qt not detected**
- `CMakeLists.txt` without Qt patterns → No content matches → **Qt not detected**

#### Use Cases

- **Distinguish similar technologies**: MFC vs Qt vs plain C++ in `.h` files
- **Library-specific detection**: Framework-specific patterns in common file types
- **Mixed file types**: Qt `.pro` files (no content check) + `.cpp` files (with content check)
- **Specific file validation**: Only check `package.json`, not all `.json` files
- **Prevent false positives**: Ensure actual usage, not just file presence

### Technology Category Configuration

Technology categories and their component behavior are defined in `internal/config/categories.yaml`. This configuration file determines which technology categories create architectural components versus being classified as tools/libraries.

#### Category Configuration File

```yaml
# internal/config/categories.yaml
types:
  database:
    is_component: true
    description: "Database systems (PostgreSQL, MongoDB, Redis, etc.)"
  
  backend_framework:
    is_component: false
    description: "Backend frameworks (Django, Spring, Express, NestJS, etc.)"
```

**Adding New Technology Categories:**

1. Add the category definition to `internal/config/categories.yaml`:
   ```yaml
   my_new_category:
     is_component: true  # or false
     description: "Description of this category"
   ```

2. Create the category directory and use it in your rules:
   ```bash
   mkdir internal/rules/core/my_new_category
   ```
   ```yaml
   tech: my-tech
   name: My Technology
   # type is derived from folder name automatically
   ```

**Benefits:**
- **No code changes required** - Edit YAML, no recompilation needed
- **Self-documenting** - Descriptions explain each category's purpose
- **Centralized** - All category definitions in one place
- **Discoverable** - Use `stack-analyzer info categories --components` to list all categories

#### Per-Rule Component Override

Individual rules can override the type's default behavior using the `is_component` field:

```yaml
tech: mfc
type: ui_framework  # Default: is_component: false
is_component: true  # Override: create component anyway
```

**Priority Order:**
1. Rule's `is_component` field (highest priority)
2. Category definition in `categories.yaml`
3. Default to `false` if category not defined

**Example Use Cases:**
- **Promote to component**: `desktop_framework` with `is_component: true` creates a component
- **Demote from component**: `database` with `is_component: false` doesn't create a component
- **New categories**: Categories not in `categories.yaml` default to no component creation

This configuration-driven approach allows fine-grained control over which technologies appear as architectural components versus implementation details.

### Output Structure

The scanner outputs a hierarchical JSON structure representing the detected technologies:

- **id**: Unique identifier for each component
- **name**: Component name (e.g., "main", "frontend", "backend")
- **path**: File system path relative to the project root
- **type**: Component type (e.g., "npm-package", "maven-module", "docker-compose-service") - present when the component detector provides it
- **tech**: Array of primary technologies for this component (e.g., `["nodejs", "java"]` for hybrid projects)
- **techs**: Array of all technologies detected in this component (components + tools/libraries)
- **languages**: Object mapping programming languages to file counts
- **licenses**: Array of detected licenses in this component
- **dependencies**: Array of detected dependencies with format `[type, name, version, scope, direct, metadata]` (always 6 elements)
- **component_dependencies**: Array of component-level dependencies (e.g., Docker base images, parent Maven modules) with format `[type, name, version, scope, metadata]` (always 5 elements)
- **children**: Array of nested components (sub-projects, services, etc.)
- **edges**: Array of relationships between components (e.g., service → database connections); created for architectural components like databases, SaaS services, and monitoring tools, but not for hosting/cloud providers
- **reason**: Object mapping technologies to detection reasons, with "_" key for non-tech reasons (licenses, base images, etc.)
- **properties**: Object containing tech-specific metadata (Docker, Terraform, Kubernetes, etc.)
- **code_stats**: Code statistics with analyzed/unanalyzed buckets (only in root payload, see [Code Statistics](#code-statistics))
- **git**: Git repository information (available at root and component levels for multi-repo projects)
- **metadata**: Scan execution metadata (only in root payload)

#### Dependencies vs Component Dependencies

The scanner tracks two types of dependencies:

**Package Dependencies** (`dependencies`):
- Runtime and build-time library dependencies from package managers
- Format: `[type, name, version, scope, direct, metadata]` (6 elements)
- Examples: npm packages, Python packages, Maven artifacts, NuGet packages
- The `direct` field indicates if it's a direct dependency (true) or transitive (false)

```json
"dependencies": [
  ["npm", "react", "18.2.0", "prod", true, {"source": "package-lock.json"}],
  ["npm", "express", "4.18.2", "prod", true, {"source": "package-lock.json"}],
  ["python", "django", "4.2.0", "prod", true, {"source": "requirements.txt"}]
]
```

**Component Dependencies** (`component_dependencies`):
- Structural dependencies between components or infrastructure elements
- Format: `[type, name, version, scope, metadata]` (5 elements, no `direct` field)
- Examples: Docker base images, Maven parent modules, Gradle project dependencies
- Represents architectural relationships rather than code-level dependencies

```json
"component_dependencies": [
  ["docker-base-image", "node", "20-alpine", "", {"file": "/backend/Dockerfile"}],
  ["docker-base-image", "nginx", "alpine", "", {"file": "/frontend/Dockerfile"}],
  ["maven-parent", "spring-boot-starter-parent", "3.2.0", "", {"file": "/pom.xml"}]
]
```

**Key Differences:**
- **Package dependencies** track library/package imports and are versioned with ranges or exact versions
- **Component dependencies** track architectural relationships and infrastructure choices
- **Package dependencies** include the `direct` boolean flag; component dependencies do not
- **Package dependencies** flow through the dependency tree; component dependencies are component-specific

#### Metadata Field

The `metadata` field (present only in the root payload) provides information about the scan execution:

```json
{
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "scan_path": "/absolute/path/to/project",
    "specVersion": "0.1",
    "duration_ms": 1173,
    "file_count": 523,
    "component_count": 87,
    "language_count": 15,
    "tech_count": 3,
    "techs_count": 12,
    "properties": {
      "product": "My Product",
      "team": "Engineering"
    }
  },
  "git": {
    "branch": "main",
    "commit": "a1b2c3d",
    "remote_url": "https://github.com/user/repo.git"
  }
}
```

**Metadata Fields:**
- **timestamp**: ISO 8601 timestamp when scan was performed
- **scan_path**: Absolute path to scanned directory
- **specVersion**: Output format specification version
- **duration_ms**: Scan duration in milliseconds
- **file_count**: Total language-detected files scanned (sum of all language file counts)
- **component_count**: Total components in the payload tree (architectural components, not filesystem directories)
- **language_count**: Number of distinct programming languages detected
- **tech_count**: Number of primary technologies (count of `tech` array)
- **techs_count**: Number of all detected technologies (count of `techs` array)
- **properties**: Custom properties from `.stack-analyzer.yml`

#### Git Field

The `git` field (present only in the root payload) provides git repository information:

- **branch**: Current branch name
- **commit**: Short commit hash (7 characters)
- **remote_url**: Origin remote URL

#### Properties Field

The `properties` field provides structured metadata about specific technologies detected in the project. This field uses an industry-standard format compatible with JSON Schema, OpenAPI, and SBOM tools.

**Supported Technologies:**

**Docker** - Extracts information from Dockerfiles:
```json
"properties": {
  "docker": [
    {
      "file": "/backend/Dockerfile",
      "base_images": ["python:3.13", "python:3.13-slim"],
      "exposed_ports": [8080],
      "multi_stage": true,
      "stages": ["builder"]
    },
    {
      "file": "/frontend/Dockerfile",
      "base_images": ["node:20-alpine", "nginx:alpine"],
      "exposed_ports": [80],
      "multi_stage": true,
      "stages": ["builder"]
    }
  ]
}
```

**Terraform** - Aggregates infrastructure resources:
```json
"properties": {
  "terraform": [
    {
      "file": "/infrastructure/main.tf",
      "providers": ["aws", "google"],
      "resources_by_provider": {
        "aws": 15,
        "google": 3
      },
      "resources_by_category": {
        "compute": 5,
        "storage": 8,
        "database": 3,
        "networking": 2
      },
      "total_resources": 18
    }
  ]
}
```

**Key Features:**
- **Array format**: Supports multiple files (multiple Dockerfiles, .tf files, etc.)
- **File tracking**: Each entry includes the source file path
- **Component-scoped**: Properties can appear at root or in child components
- **Tool-friendly**: Compatible with security scanners, SBOM generators, and CI/CD tools

#### Multi-Technology Components

When multiple technology stacks are detected in the same directory (e.g., a directory with both `package.json` and `pom.xml`), the scanner automatically merges them into a single component with multiple primary technologies. This accurately represents hybrid projects that combine different technology stacks:

```json
{
  "name": "hybrid-service",
  "tech": ["nodejs", "java"],
  "techs": ["nodejs", "java", "maven", "npm", "typescript"],
  "languages": {
    "TypeScript": 150,
    "Java": 45
  }
}
```

This is common in projects with:
- Node.js frontend + Java backend in the same module
- Integration tests (Playwright/TypeScript) alongside Java applications
- Build tools from multiple ecosystems

### Example Full Output

```json
{
  "id": "abc123",
  "name": "main",
  "path": ["/"],
  "tech": ["nodejs"],
  "techs": ["nodejs", "express", "postgresql"],
  "languages": {
    "TypeScript": 45,
    "JavaScript": 12
  },
  "dependencies": [
    ["npm", "express", "^4.18.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "pg", "^8.8.0", "prod", true, {"source": "package-lock.json"}]
  ],
  "children": [
    {
      "id": "def456",
      "name": "frontend",
      "tech": ["nodejs"],
      "dependencies": [["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}]]
    }
  ]
}
```

### Aggregated Output

Use the `--aggregate` flag to get a simplified, rolled-up view of your entire codebase:

```bash
./bin/stack-analyzer scan --aggregate tech,techs,languages,licenses,dependencies,git /path/to/project
./bin/stack-analyzer scan --aggregate git /path/to/project  # Show only git repositories
./bin/stack-analyzer scan --aggregate all /path/to/project  # Aggregate all fields with metadata
```

**Output:**
```json
{
  "tech": ["nodejs", "python", "postgresql", "redis"],
  "techs": ["nodejs", "python", "postgresql", "redis", "react", "typescript", "docker", "eslint", "prettier"],
  "languages": {
    "Python": 130,
    "TypeScript": 89,
    "JavaScript": 45,
    "Go": 12
  },
  "licenses": ["MIT", "Apache-2.0"],
  "dependencies": [
    ["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "^4.18.0", "prod", true, {"source": "package-lock.json"}],
    ["python", "fastapi", "0.118.2", "prod", true, {"source": "requirements.txt"}],
    ["python", "pydantic", "latest", "prod", true, {"source": "requirements.txt"}]
  ],
  "git": [
    {
      "branch": "main",
      "commit": "def5678",
      "remote_url": "https://github.com/company/project.git"
    },
    {
      "branch": "develop", 
      "commit": "abc1234",
      "remote_url": "https://github.com/company/frontend.git"
    }
  ]
}
```

**Available fields:**
- `tech` - Primary technologies
- `techs` - All detected technologies (includes frameworks, tools, libraries)
- `languages` - Programming languages with file counts
- `licenses` - Detected licenses from LICENSE files and package manifests
- `dependencies` - All dependencies as `[type, name, version, scope, direct, metadata]` arrays (always 6 elements)
- `git` - Git repositories (deduplicated) with branch, commit, dirty status, and remote URL
- `all` - Aggregate all available fields (tech, techs, languages, licenses, dependencies, git) with metadata

This is useful for:
- Quick technology stack overview
- Generating technology badges
- Dependency auditing and security scanning
- License compliance checking
- Counting dependencies: `jq '.dependencies | length'`
- Git repository tracking: `jq '.git | length'` for multi-repo projects

## How to Build It

### Using Task (Recommended)

**Task** is a modern task runner that simplifies common development operations. The `Taskfile.yml` defines reusable commands for building, testing, and maintaining the project.

```bash
# Install Task (if not already installed)
# macOS
brew install go-task

# Or install directly with Go
go install github.com/go-task/task/v3/cmd/task@latest

# Build the project
task build

# Run all quality checks (format, check, test)
task fct

# Clean build artifacts
task clean

# Run the scanner (use -- <path>)
task run -- /path/to/project
```

#### Available Tasks

| Task | Description |
|------|-------------|
| `task build` | Compile the stack-analyzer binary |
| `task format` | Format Go code using gofmt |
| `task check` | Run go vet and golangci-lint |
| `task test` | Run all tests |
| `task fct` | Run format, check, and test in sequence |
| `task clean` | Clean up build artifacts and caches |
| `task run` | Run stack-analyzer on a directory |
| `task run:help` | Show stack-analyzer help message |
| `task pre-commit:setup` | Install pre-commit tool |
| `task pre-commit:install` | Install pre-commit git hooks |
| `task pre-commit:run` | Run pre-commit on all files |

### Using Go Commands

```bash
# Build stack-analyzer
go build -o bin/stack-analyzer ./cmd/scanner

# Run tests
go test -v ./...

# Run with race detection
go test -race ./...

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o bin/stack-analyzer-linux ./cmd/scanner
GOOS=windows GOARCH=amd64 go build -o bin/stack-analyzer-windows.exe ./cmd/scanner
```

### Docker Build

```bash
# Build Docker image
docker build -t tech-stack-analyzer .

# Run in container
docker run --rm -v /path/to/project:/app tech-stack-analyzer /app
```

## Architecture Overview

### Project Structure

```
tech-stack-analyzer/
├── cmd/
│   ├── scanner/           # CLI application entry point
│   └── convert-rules/     # Rules conversion utilities
├── internal/
│   ├── aggregator/        # Result aggregation logic
│   ├── cmd/               # CLI command implementations
│   ├── config/            # Configuration management (settings, types)
│   ├── git/               # Git repository information and .gitignore processing
│   ├── metadata/          # Scan metadata (timestamps, file counts, execution info)
│   ├── progress/          # Verbose mode progress reporting
│   ├── provider/          # File system abstraction layer
│   ├── rules/             # Rule loading and validation
│   │   └── core/          # Embedded technology rules (800+ rules in 48 categories)
│   ├── scanner/           # Core scanning engine
│   │   ├── components/    # Component detectors (nodejs, python, java, docker, etc.)
│   │   ├── matchers/      # File and extension matchers
│   │   └── parsers/       # Specialized file parsers (JSON, TOML, XML, HCL)
│   └── types/             # Core data structures
├── docs/                  # Documentation
└── Taskfile.yml           # Task automation
```

### Core Components

#### 1. Scanner Engine (`internal/scanner/`)
- **Main orchestrator** that coordinates all detection phases
- **Sequential processing** with efficient recursive traversal
- **Component detection** through modular detector system
- **Progress reporting** for verbose mode

#### 2. Component Detectors (`internal/scanner/components/`)
Each detector handles specific project types:
- **Node.js** - package.json, npm/yarn detection
- **Python** - pyproject.toml, requirements.txt, setup.py detection  
- **.NET** - .csproj files, NuGet packages
- **Java/Kotlin** - Maven/Gradle detection
- **Docker** - docker-compose.yml services
- **Terraform** - HCL file parsing
- **Ruby** - Gemfile detection
- **Rust** - Cargo.toml detection
- **PHP** - composer.json detection
- **Deno** - deno.json detection
- **Go** - go.mod detection

#### 3. Rule System (`internal/rules/`)
- **800+ technology rules** covering enterprise stacks
- **YAML-based DSL** for easy extension
- **Multi-language support** (npm, pip, cargo, composer, nuget, maven, etc.)
- **Content-based validation** with regex pattern matching

#### 4. Configuration System (`internal/config/`)
- **Settings management** with environment variable support
- **Type definitions** for component classification
- **Validation** and defaults

#### 5. Git Module (`internal/git/`)
- **Repository information** extraction using go-git
- **.gitignore processing** with recursive loading and pattern matching
- **Smart filtering** to avoid problematic cache directory patterns

#### 6. Progress Reporting (`internal/progress/`)
- **Event-based architecture** for verbose mode
- **Pluggable handlers** (SimpleHandler, TreeHandler)
- **Real-time feedback** on scan progress and exclusions

#### 7. Git Integration (`github.com/go-git/go-git/v5`)
- **Pure Go implementation** using go-git library for maximum portability
- **No external dependencies** - doesn't require git command to be installed
- **Repository detection** through `git.PlainOpen()` for reliable git repo identification
- **Branch information** including detached HEAD detection
- **Commit hash extraction** with short 7-character format
- **Dirty status detection** using worktree status analysis
- **Remote URL extraction** from origin remote configuration
- **Cross-platform compatibility** works consistently across Windows, macOS, and Linux

#### 8. Language Detection (`github.com/go-enry/go-enry/v2`)
- **GitHub Linguist integration** for comprehensive language detection
- **1500+ languages** supported through open-source language database
- **Detection** by file extension and filename patterns
- **Handles special files** like Makefile, Dockerfile, etc.

#### 9. Parser System (`internal/scanner/parsers/`)
Specialized parsers for complex file formats:
- **HCL parser** for Terraform files
- **XML parser** for .csproj files
- **JSON parser** for package.json files
- **TOML parser** for pyproject.toml and Cargo.toml files
- **YAML parser** for docker-compose.yml files
- **Dotenv parser** for .env files

### Detection Pipeline

The scanner follows a systematic pipeline to analyze projects:

1. **File Discovery** - Recursive file system scanning
2. **Language Detection** - GitHub Linguist (go-enry) identification by extension and filename
3. **Git Repository Analysis** - Pure Go git integration for repository information
4. **Component Detection** - Project-specific analysis
5. **Dependency Matching** - Pattern matching against rules
6. **Result Assembly** - Hierarchical payload construction

## How to Extend It

### Adding New Technology Rules

#### 1. Create a New Rule File

```yaml
# internal/rules/core/database/newtech.yaml
tech: newtech                    # Required: Unique technology identifier
name: New Technology             # Required: Display name
type: db                         # Required: Technology category
description: Modern database solution with high performance and scalability  # Optional: Technology description
properties:                      # Optional: Arbitrary key/value pairs for custom metadata
  website: https://newtech.com
  founded: 2020
  versions:
    - "1.0"
    - "2.0"
  api_version: v2
  category: "Database"
is_component: true               # Optional: Override component behavior
is_primary_tech: true           # Optional: Override primary tech promotion
dotenv:                          # Optional: Environment variable patterns
  - NEWTECH_
dependencies:                    # Optional: Package dependencies to detect
  - type: npm
    name: newtech-driver         # Can be regex: /^@newtech\/.*/ 
    example: newtech-driver
  - type: python
    name: newtech-client
    example: newtech-client
files:                           # Optional: Specific files to match
  - newtech.conf
  - config/newtech.yml
extensions:                      # Optional: File extensions to match
  - .newtech
  - .nt
content:                         # Optional: Content patterns for validation
  - pattern: 'newtech\s*=\s*['"']'
    extensions: [.conf, .yml]      # Specify where to check
  - pattern: 'import.*newtech'
    extensions: [.js, .py]         # Check JS and Python files
  - type: json-path                # JSON path matching
    path: $.config.provider
    value: newtech
    files: [config.json]
  - type: json-path                # JSON schema validation via $schema field
    path: $.$schema
    value: https://newtech.com/schema.json
    files: [newtech.json]
```

#### Complete Rule Field Reference

**Required Fields:**
- **`tech`** - Unique technology identifier (used in output)
- **`name`** - Human-readable display name
- **`type`** - Technology category (database, framework, language, etc.)

**Optional Fields:**

**`description`** - Technology description for additional context
```yaml
description: AI safety and research company providing Claude AI models and APIs
```
- Used in JSON and YAML outputs of `info techs` command
- Provides additional context about the technology
- Empty string if not specified

**`properties`** - Arbitrary key/value pairs for custom metadata
```yaml
properties:
  website: https://www.anthropic.com
  founded: 2021
  models:
    - claude-3-opus
    - claude-3-sonnet
    - claude-3-haiku
  api_version: v1
  category: "Large Language Models"
```
- Supports any YAML/JSON compatible data types (strings, numbers, arrays, objects)
- Used in JSON and YAML outputs of `info techs` command
- Perfect for storing company info, technical details, documentation links
- Empty map `{}` if not specified (null in JSON, {} in YAML)

**`is_component`** - Override component creation behavior
- `true` - Always create a component
- `false` - Never create a component  
- `null`/omitted - Use type-based default

**`is_primary_tech`** - Override primary technology promotion behavior
- `true` - Always promote to primary tech array (even without component)
- `false` - Never promote to primary tech array (even with component)
- `null`/omitted - Use component-based logic (if component created, promote to primary)

This field provides fine-grained control over the relationship between component creation and primary tech promotion:

| Configuration | Component Created | Primary Tech | Use Case |
|---------------|------------------|--------------|----------|
| `is_component: true` (no `is_primary_tech`) | Yes | Yes | Default behavior (languages, databases) |
| `is_component: true, is_primary_tech: false` | Yes | No | Build tools with organization (CMake, Make) |
| `is_component: false, is_primary_tech: true` | No | Yes | Simple primary tech without components |
| `is_component: false` (no `is_primary_tech`) | No | No | Regular detection (most tools, frameworks) |

**`dotenv`** - Array of environment variable prefixes
```yaml
dotenv:
  - POSTGRES_    # Matches POSTGRES_DB, POSTGRES_HOST, etc.
  - REDIS_URL    # Matches exact env var names
```

**`dependencies`** - Package dependencies to detect
```yaml
dependencies:
  - type: npm
    name: react                  # Exact match
    example: react
  - type: npm
    name: /^@types\/.*$/         # Regex pattern
    example: '@types/node'
  - type: python
    name: django>=3.0            # Version pattern
    example: django
```

**Supported dependency types:**
- `npm`, `python`, `pip`, `cargo`, `composer`, `nuget`, `maven`, `gradle`
- `docker`, `githubAction`, `terraform.resource`

**`files`** - Specific files to match (glob patterns)
```yaml
files:
  - package.json              # Exact filename match
  - requirements.txt          # Exact filename match
  - Dockerfile                # Exact filename match
  - spfile*.ora               # Glob: matches spfile.ora, spfileORCL.ora, etc.
  - *.config.js               # Glob: matches any .config.js file
```
**Pattern syntax:** Glob patterns where `*` matches any characters and `?` matches a single character.

**`extensions`** - File extensions to match
```yaml
extensions:
  - .py
  - .js
  - .ts
  - .go
```

**`content`** - Content patterns for precise detection (regex patterns)

Content patterns use **regex** for matching file contents:

```yaml
content:
  # Regex pattern matching (default type)
  - pattern: 'import\s+.*react'
    extensions: [.js, .jsx, .ts, .tsx]  # Must specify where to check
  - pattern: 'FROM\s+node:'
    files: [Dockerfile]                  # Or specific files
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h, .hpp]         # Check C++ files for Qt

  # JSON Path matching - check values at specific JSON paths
  - type: json-path
    path: $.name                         # Path to check
    files: [package.json]                # Path exists = match
  - type: json-path
    path: $.dependencies.react
    value: /^18\./                       # Optional: regex value match
    files: [package.json]

  # YAML Path matching - check values at specific YAML paths
  - type: yaml-path
    path: $.services.web
    files: [docker-compose.yml]          # Path exists = match
  - type: yaml-path
    path: $.version
    value: "3.8"                         # Optional: exact value match
    files: [docker-compose.yml]
```

**Content Type Reference:**

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `regex` (default) | Regex pattern matching on file content | `pattern`, `extensions` or `files` |
| `json-path` | Checks if JSON path exists or matches value | `path`, `files`, optional `value` |
| `yaml-path` | Checks if YAML path exists or matches value | `path`, `files`, optional `value` |
| `xml-path` | Checks if XML path exists or matches value | `path`, `files`, optional `value` |

**Example: JSON Schema Validation** (using json-path):
```yaml
content:
  - type: json-path
    path: $.$schema
    value: https://ui.shadcn.com/schema.json
    files: [components.json]
```

**Example: XML Generator Detection** (using xml-path):
```yaml
content:
  - type: xml-path
    path: $.Export.generator
    value: /Cache|/IRIS
    files: [package.xml]
```

**Value Matching:**
- Exact string: `value: "3.8"` matches exactly "3.8"
- Regex pattern: `value: /^18\./` matches strings starting with "18."

**Note:** Content patterns must specify `extensions` or `files` to define where to check. They operate independently of top-level `extensions`/`files` fields.

#### 2. Rule Categories

The rules are organized into 30+ categories:

```
internal/rules/core/
├── ai/                   # AI/ML technologies
├── analytics/            # Analytics platforms
├── application/          # Application frameworks
├── automation/           # Automation tools
├── build/                # Build systems
├── ci/                   # CI/CD systems
├── cloud/                # Cloud providers
├── cms/                  # Content management systems
├── collaboration/        # Collaboration tools
├── communication/        # Communication services
├── crm/                  # CRM systems
├── database/             # Database systems
├── etl/                  # ETL tools
├── framework/            # Application frameworks
├── hosting/              # Hosting services
├── infrastructure/       # Infrastructure tools
├── language/             # Programming languages
├── monitoring/           # Monitoring and observability
├── network/              # Network tools
├── notification/         # Notification services
├── payment/              # Payment processors
├── queue/                # Message queues
├── runtime/              # Runtime environments
├── saas/                 # SaaS platforms
├── security/             # Security tools
├── ssg/                  # Static site generators
├── storage/              # Storage services
├── test/                 # Testing frameworks
├── tool/                 # Development tools
├── ui/                   # UI libraries and frameworks
└── validation/           # Validation libraries
```

### Adding New Component Detectors

#### 1. Create Detector Structure

```go
// internal/scanner/components/newtech/detector.go
package newtech

import (
    "github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
    "github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
    return "newtech"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string,
    provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
    // Implementation here
}

func init() {
    components.Register(&Detector{})
}
```

#### 2. Create Parser (if needed)

```go
// internal/scanner/parsers/newtech.go
package parsers

type NewTechParser struct{}

func (p *NewTechParser) ParseConfig(content string) NewTechConfig {
    // Parse configuration files
}
```

#### 3. Register in Scanner

```go
// internal/scanner/scanner.go
import (
    _ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/newtech"
)
```

### Adding New File Matchers

```go
// internal/scanner/matchers/newmatcher.go
func registerNewMatcher() {
    components.RegisterFileMatcher(&matcher.FileMatcher{
        Tech:       "newtech",
        Extensions: []string{".newext"},
        Pattern:    regexp.MustCompile(`newtech\.config`),
    })
}
```

### Custom Rule Directories

> **Note**: External rules support is planned but not yet implemented. Currently, the scanner uses embedded rules only.

## Contributing

We welcome contributions! For detailed guidelines on:
- Code style and formatting
- Pre-commit hooks setup
- Submitting pull requests
- Reporting issues
- Development workflow

Please see [CONTRIBUTING.md](CONTRIBUTING.md)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

### Original Project
This is a Go re-implementation of [specfy/stack-analyser](https://github.com/specfy/stack-analyser) by the original author. The original TypeScript implementation provided the foundation and inspiration for this project.

### Industry Alignment
For specific parser implementations, we reference [Google's deps.dev](https://deps.dev) project when designing our dependency data structures and analysis approaches. This alignment ensures compatibility and consistency with industry standards for open-source dependency analysis, enabling better integration with the broader software supply chain ecosystem.

### Extensions and Enhancements
This Go implementation provides practical improvements focused on deployment simplicity:

- **Zero Dependencies**: Single executable binary with no Node.js runtime or package management required
- **Extended Technology Support**: Added Java/Kotlin and .NET component detectors alongside existing Node.js, Python, Docker, Terraform, Ruby, Rust, PHP, Deno, and Go support
- **Enhanced Database Coverage**: Improved detection for Oracle, MongoDB, Redis, and other enterprise databases
- **Modular Architecture**: Clean component detector system for easier maintenance and extension
- **Comprehensive Rules**: 800+ technology rules across 48 categories covering modern enterprise stacks

### Contributors
Thank you to all contributors who help improve this project.

---

Built with Go - Delivering technology stack analysis for modern development teams.
