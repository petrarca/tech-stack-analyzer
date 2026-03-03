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

### Override .gitignore

Use `--exclude` flags to add additional exclusions or override .gitignore patterns:
```bash
# Add extra exclusions beyond .gitignore
./bin/stack-analyzer scan /path/to/project --exclude "build-cache" --exclude "*.tmp"

# .gitignore patterns are still respected, these are additional
```

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

Enable per-component code statistics with `--component-code-stats` to get detailed metrics for each detected component (e.g., each Maven module, npm package, or Go module):

```bash
# Enable per-component code stats
./bin/stack-analyzer scan --component-code-stats /path/to/project
```

Key points:
- **Root `code_stats`**: Global statistics for the entire codebase (all files)
- **Component `code_stats`**: Statistics for files directly in that component only
- **Global >= Sum of components**: Root-level files not in any component are only in global stats
- **Zero overhead when disabled**: No performance impact when flag is not used

This is useful for:
- Identifying large/complex modules in monorepos
- Tracking code growth per component over time
- Finding components with low comment ratios or high complexity

## Multi-Git Repository Support

The analyzer automatically detects git repositories at both root and component levels, enabling tracking of multiple repositories within a single scan. Each component shows its own git information (branch, commit, dirty status, remote URL), making it ideal for monorepos, workspace scans, and CI/CD pipelines where different sub-projects may be in different git states.

## Verbose Mode

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
