# Configuration Settings

The Tech Stack Analyzer supports configuration through command-line flags and environment variables.

## Settings Structure

```go
type Settings struct {
    // Output settings
    OutputFile  string
    PrettyPrint bool

    // Scan behavior
    ExcludePatterns []string
    Aggregate       string

    // Logging
    LogLevel logrus.Level
    LogFormat string // "text" or "json"
}
```

## Command Line Flags

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--output`, `-o` | `STACK_ANALYZER_OUTPUT` | stdout | Output file path |
| `--pretty` | `STACK_ANALYZER_PRETTY` | true | Pretty print JSON output |
| `--exclude-dir` | `STACK_ANALYZER_EXCLUDE_DIRS` | (none) | Comma-separated directories to exclude |
| `--aggregate` | `STACK_ANALYZER_AGGREGATE` | (none) | Aggregate fields to include |
| `--log-level` | `STACK_ANALYZER_LOG_LEVEL` | error | Log level: trace, debug, error, fatal |
| `--log-format` | `STACK_ANALYZER_LOG_FORMAT` | text | Log format: text or json |

## Examples

### Using Environment Variables
```bash
export STACK_ANALYZER_PRETTY=false
export STACK_ANALYZER_EXCLUDE_DIRS=vendor,node_modules,build
export STACK_ANALYZER_OUTPUT=/tmp/scan-results.json
export STACK_ANALYZER_LOG_LEVEL=debug
export STACK_ANALYZER_LOG_FORMAT=json

./bin/stack-analyzer scan /path/to/project
```

### Mixing Flags and Environment Variables
```bash
export STACK_ANALYZER_EXCLUDE_DIRS=vendor,node_modules
export STACK_ANALYZER_LOG_LEVEL=trace

./bin/stack-analyzer scan /path/to/project --pretty --output results.json
```

### Command Line Only
```bash
./bin/stack-analyzer scan /path/to/project \
  --exclude-dir vendor,node_modules,build \
  --log-level debug \
  --log-format json \
  --output results.json
```

### Logging Examples
```bash
# Debug logging with structured fields
./bin/stack-analyzer scan /path/to/project --log-level debug

# JSON logging for automated processing
./bin/stack-analyzer scan /path/to/project --log-format json

# Trace level for maximum detail
./bin/stack-analyzer scan /path/to/project --log-level trace

# Environment variables for logging
STACK_ANALYZER_LOG_LEVEL=debug STACK_ANALYZER_LOG_FORMAT=json \
  ./bin/stack-analyzer scan /path/to/project
```

## Precedence

1. **Command line flags** take highest precedence
2. **Environment variables** are used as defaults
3. **Built-in defaults** are used when neither is provided

## Implementation Notes

- Settings are initialized once during command startup
- Environment variables are loaded automatically
- Settings validation happens before scanning begins
- The settings object is passed to scanner initialization

## Future Enhancements

- Profile-based settings
- Global user configuration in `~/.config/stack-analyzer/`
