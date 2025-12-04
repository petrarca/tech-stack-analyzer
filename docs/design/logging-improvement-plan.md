# Logging and Verbose Handling Improvement Plan

## Executive Summary

Improve separation between user output, progress information, and developer logging to enable proper piping (e.g., `stack-analyzer scan /path | jq`) while maintaining debugging capabilities.

## Current State Analysis

### Output Channels

| Channel | Current Usage | Issues |
|---------|--------------|--------|
| **stdout** | JSON results (when no `-o` flag) | ✓ Correct - pipeable |
| **stderr** | Logrus logs + Verbose progress | ✗ Mixed purposes |
| **files** | JSON output (with `-o` flag) | ✓ Works |

### Current Problems

1. **Piping broken when verbose enabled**: `stack-analyzer scan --verbose /path | jq` fails because verbose output goes to stderr
2. **Info logs pollute stderr**: Even without `--verbose`, Info logs appear on stderr
3. **No separation**: User messages and debug logs mixed on stderr
4. **Inconsistent logging**: Three different approaches (logrus, log, fmt)

### Current Code Locations

```go
// scan.go line 173 - CORRECT: JSON to stdout
fmt.Println(string(jsonData))

// scan.go lines 80-83, 127-131, etc. - WRONG: Info logs to stderr
logger.Info("Starting Tech Stack Analyzer")

// progress.go - CORRECT: Verbose to stderr (but needs coordination)
fmt.Fprintf(h.writer, "[SCAN] Starting: %s\n", event.Path)

// info.go lines 122-128, 165, 210 - CORRECT: User output to stdout
fmt.Println(tech)
```

## Design Principles

### The Unix Philosophy

```
┌─────────────────────────────────────────────────────────┐
│                     STDOUT (fd 1)                       │
│  Purpose: Structured data output (JSON, YAML, text)    │
│  Usage: Pipeable results                               │
│  Examples: JSON scan results, tech lists               │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                     STDERR (fd 2)                       │
│  Purpose: Human-readable messages & progress            │
│  Usage: User feedback, errors, warnings                │
│  Examples: Verbose progress, error messages            │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                    LOG FILE (optional)                  │
│  Purpose: Developer debugging information               │
│  Usage: Trace, debug, error logs                       │
│  Examples: Internal operations, deep debugging          │
└─────────────────────────────────────────────────────────┘
```

### Output Strategy

| Output Type | Default | With --verbose | With --log-level=debug |
|-------------|---------|----------------|------------------------|
| JSON results | stdout | stdout | stdout |
| Progress | silent | stderr | stderr |
| Errors | stderr | stderr | stderr |
| Debug logs | silent | silent | stderr or log file |
| Trace logs | silent | silent | log file only |

### Piping Examples

```bash
# ✓ Works: Pipe JSON to jq (no noise)
stack-analyzer scan /path | jq '.techs'

# ✓ Works: Pipe with verbose (progress to stderr, data to stdout)
stack-analyzer scan --verbose /path | jq '.techs'
# stderr shows: [SCAN] Starting: /path...
# stdout pipes: {"techs": [...]}

# ✓ Works: Debug logs to file, data to stdout
stack-analyzer scan --log-level=debug --log-file=debug.log /path | jq

# ✓ Works: Suppress all stderr
stack-analyzer scan --verbose /path 2>/dev/null | jq

# ✓ Works: Capture both streams
stack-analyzer scan --verbose /path 2>progress.log | jq > results.json
```

## Detailed Plan

### Phase 1: Audit Current Usage

**Goal**: Categorize all output statements

#### 1.1 Audit logger.Info() Calls

Location: `internal/cmd/scan.go`

| Line | Current Code | Category | Action |
|------|-------------|----------|--------|
| 83 | `logger.Info("Starting Tech Stack Analyzer")` | User message | Remove (not needed) |
| 131 | `logger.Info("Initializing scanner")` | Debug info | Change to Debug |
| 141 | `logger.Info("Scanning file")` | Verbose progress | Move to progress |
| 144 | `logger.Info("Scanning directory")` | Verbose progress | Move to progress |
| 165 | `logger.Info("Writing results to file")` | User message | Keep on stderr |
| 170 | `logger.Info("Results written successfully")` | User message | Keep on stderr |

#### 1.2 Audit fmt.Print* Calls

Location: `internal/cmd/info.go`

| Line | Current Code | Category | Action |
|------|-------------|----------|--------|
| 80 | `fmt.Fprintf(os.Stderr, "Error...")` | Error | ✓ Correct |
| 122-128 | `fmt.Printf("=== %s ===\n", title)` | User output | ✓ Correct (stdout) |
| 165 | `fmt.Println(tech)` | User output | ✓ Correct (stdout) |
| 210 | `fmt.Println(string(output))` | User output | ✓ Correct (stdout) |

#### 1.3 Audit log.* Calls

Location: `cmd/convert-rules/main.go`

| Usage | Category | Action |
|-------|----------|--------|
| `log.Printf("✓ %s -> %s", ...)` | User progress | Keep (utility script) |
| `log.Fatalf("Conversion failed: %v", err)` | Error | Keep (utility script) |

**Decision**: `convert-rules` is a utility script, not the main CLI. Keep simple logging.

### Phase 2: Logging Level Redesign

**Goal**: Remove Info level, add Error level

#### 2.1 Update Settings

```go
// internal/config/settings.go

type Settings struct {
    // ... existing fields ...
    
    // Logging configuration
    LogLevel  logrus.Level  // trace, debug, error, fatal (NO info, NO warn)
    LogFormat string        // text, json
    LogFile   string        // Optional: write logs to file instead of stderr
}

func DefaultSettings() *Settings {
    return &Settings{
        // ... existing defaults ...
        LogLevel:  logrus.ErrorLevel,  // Changed from InfoLevel
        LogFormat: "text",
        LogFile:   "",  // Empty = stderr
    }
}
```

#### 2.2 Update Logger Configuration

```go
// internal/config/settings.go

func (s *Settings) ConfigureLogger() *logrus.Logger {
    logger := logrus.New()
    logger.SetLevel(s.LogLevel)
    
    // Set log format
    if s.LogFormat == "json" {
        logger.SetFormatter(&logrus.JSONFormatter{
            TimestampFormat: "2006-01-02 15:04:05",
        })
    } else {
        logger.SetFormatter(&logrus.TextFormatter{
            FullTimestamp:   true,
            TimestampFormat: "2006-01-02 15:04:05",
        })
    }
    
    // Set output destination
    if s.LogFile != "" {
        file, err := os.OpenFile(s.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
        if err != nil {
            // Fallback to stderr if file can't be opened
            fmt.Fprintf(os.Stderr, "Warning: Cannot open log file %s: %v\n", s.LogFile, err)
            logger.SetOutput(os.Stderr)
        } else {
            logger.SetOutput(file)
        }
    } else {
        logger.SetOutput(os.Stderr)
    }
    
    return logger
}
```

#### 2.3 Update Flag Definitions

```go
// internal/cmd/scan.go - init()

scanCmd.Flags().String("log-level", logLevel, "Log level: trace, debug, error, fatal")
scanCmd.Flags().String("log-format", logFormat, "Log format: text or json")
scanCmd.Flags().String("log-file", "", "Log file path (default: stderr)")
```

#### 2.4 Valid Log Levels

| Level | Purpose | When to Use |
|-------|---------|-------------|
| `trace` | Deep debugging | Inspecting data structures, rule matching details |
| `debug` | Internal operations | Component detection, file processing decisions |
| `error` | Non-fatal errors | Failed file reads, parsing errors (continue execution) |
| `fatal` | Fatal errors | Invalid config, missing required files (exit) |

**Removed Levels:**
- ~~`info`~~ - Replaced by verbose progress or stdout messages
- ~~`warn`~~ - Use `error` instead (simpler mental model)

### Phase 3: Progress Package Enhancement

**Goal**: Add events for all user-facing messages

#### 3.1 New Event Types

```go
// internal/progress/progress.go

const (
    // Existing events
    EventScanStart EventType = iota
    EventScanComplete
    EventEnterDirectory
    EventLeaveDirectory
    EventComponentDetected
    EventFileProcessing
    EventSkipped
    EventProgress
    
    // New events
    EventScanInitializing
    EventFileWriting
    EventFileWritten
    EventError
)
```

#### 3.2 New Event Methods

```go
// internal/progress/progress.go

func (p *Progress) ScanInitializing(path string, excludePatterns []string) {
    p.Report(Event{
        Type: EventScanInitializing,
        Path: path,
        Info: strings.Join(excludePatterns, ", "),
    })
}

func (p *Progress) FileWriting(path string) {
    p.Report(Event{
        Type: EventFileWriting,
        Path: path,
    })
}

func (p *Progress) FileWritten(path string) {
    p.Report(Event{
        Type: EventFileWritten,
        Path: path,
    })
}

func (p *Progress) Error(message string, err error) {
    // ALWAYS show errors to stderr, even if verbose is disabled
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %s: %v\n", message, err)
    } else {
        fmt.Fprintf(os.Stderr, "Error: %s\n", message)
    }
}
```

#### 3.3 Handler Updates

```go
// internal/progress/progress.go - SimpleHandler

func (h *SimpleHandler) Handle(event Event) {
    switch event.Type {
    // ... existing cases ...
    
    case EventScanInitializing:
        fmt.Fprintf(h.writer, "[INIT] Initializing scanner: %s\n", event.Path)
        if event.Info != "" {
            fmt.Fprintf(h.writer, "[INIT] Excluding: %s\n", event.Info)
        }
    
    case EventFileWriting:
        fmt.Fprintf(h.writer, "[OUT]  Writing results to: %s\n", event.Path)
    
    case EventFileWritten:
        fmt.Fprintf(h.writer, "[OUT]  Results written: %s\n", event.Path)
    }
}
```

### Phase 4: Migration Strategy

**Goal**: Move all Info logs to appropriate channels

#### 4.1 Scan Command Migration

```go
// internal/cmd/scan.go

// BEFORE (line 80-83)
logger.WithFields(logrus.Fields{
    "version": "1.0.0",
    "command": "scan",
}).Info("Starting Tech Stack Analyzer")

// AFTER - Remove entirely (not needed)
// User knows they ran the command

// ---

// BEFORE (line 127-131)
logger.WithFields(logrus.Fields{
    "path":         scannerPath,
    "exclude_dirs": settings.ExcludeDirs,
    "verbose":      settings.Verbose,
}).Info("Initializing scanner")

// AFTER - Split into debug log and verbose progress
logger.Debug("Initializing scanner",
    "path", scannerPath,
    "exclude_patterns", settings.ExcludePatterns)
s.progress.ScanInitializing(scannerPath, settings.ExcludePatterns)

// ---

// BEFORE (line 141)
logger.WithField("file", absPath).Info("Scanning file")

// AFTER
logger.Debug("Scanning file", "file", absPath)
s.progress.FileProcessing(absPath, "scanning")

// ---

// BEFORE (line 144)
logger.WithField("directory", absPath).Info("Scanning directory")

// AFTER
logger.Debug("Scanning directory", "directory", absPath)
// Progress already handled by ScanStart event

// ---

// BEFORE (line 165)
logger.WithField("output_file", settings.OutputFile).Info("Writing results to file")

// AFTER - Keep as user message on stderr
fmt.Fprintf(os.Stderr, "Writing results to %s\n", settings.OutputFile)
// OR use progress
s.progress.FileWriting(settings.OutputFile)

// ---

// BEFORE (line 170)
logger.WithField("file", settings.OutputFile).Info("Results written successfully")

// AFTER
fmt.Fprintf(os.Stderr, "Results written to %s\n", settings.OutputFile)
// OR use progress
s.progress.FileWritten(settings.OutputFile)
```

#### 4.2 Error Handling Migration

```go
// BEFORE - Fatal on every error
if err != nil {
    logger.WithError(err).Fatal("Failed to read file")
}

// AFTER - Use Error for non-fatal, Fatal for fatal
if err != nil {
    logger.Error("Failed to read file", "path", filePath, "error", err)
    s.progress.Error("Failed to read file", err)
    continue // Don't terminate
}

// For truly fatal errors
if err != nil {
    logger.Fatal("Invalid configuration", "error", err)
    // Also show to user
    fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
    os.Exit(1)
}
```

### Phase 5: Log File Support

**Goal**: Enable logging to file for debugging

#### 5.1 Implementation

Already covered in Phase 2.2 - `ConfigureLogger()` method.

#### 5.2 Usage Examples

```bash
# Debug to stderr (default)
stack-analyzer scan --log-level=debug /path

# Debug to file (keeps stderr clean)
stack-analyzer scan --log-level=debug --log-file=debug.log /path

# Trace to file (very verbose)
stack-analyzer scan --log-level=trace --log-file=trace.log /path

# Combine: verbose progress + debug logs to file
stack-analyzer scan --verbose --log-level=debug --log-file=debug.log /path
```

### Phase 6: Standardization

**Goal**: Consistent logging across all commands

#### 6.1 Info Command

**Current**: Uses `log` package and `fmt.Fprintf(os.Stderr, ...)`

**Action**: Keep as-is. Info command is for user output, not debugging.

Rationale:
- Simple read-only command
- No complex operations to debug
- stdout output is correct
- stderr errors are correct

#### 6.2 Convert Rules Utility

**Current**: Uses `log` package

**Action**: Keep as-is. It's a utility script, not part of main CLI.

Rationale:
- Developer-only tool
- Simple logging needs
- Not worth the complexity

### Phase 7: Documentation

**Goal**: Document the new logging strategy

#### 7.1 Update AGENTS.md

```markdown
## Logging Guidelines

### Output Channels

- **stdout**: Structured data only (JSON, YAML, text lists)
  - Scan results
  - Info command output
  - Always pipeable

- **stderr**: Human messages only
  - Verbose progress (with --verbose)
  - Error messages (always shown)
  - User notifications (file written, etc.)

- **Log file**: Developer debugging (with --log-file)
  - Trace: Deep inspection
  - Debug: Internal operations
  - Error: Non-fatal errors

### Log Levels

- **trace**: Deep debugging (data inspection, rule matching)
- **debug**: Internal operations (component detection, decisions)
- **error**: Non-fatal errors (failed file reads, continue execution)
- **fatal**: Fatal errors (invalid config, exit immediately)

### Rules

1. NEVER use Info level - it's removed
2. User messages go to stderr with fmt.Fprintf
3. Progress goes through progress package
4. Debug/Trace for developers only
5. Keep stdout clean for piping

### Examples

```go
// ✓ Good: Debug log
logger.Debug("Detected component", "name", name, "tech", tech)

// ✓ Good: User message
fmt.Fprintf(os.Stderr, "Writing results to %s\n", outputFile)

// ✓ Good: Progress
s.progress.ComponentDetected(name, tech, path)

// ✓ Good: Non-fatal error
logger.Error("Failed to read file", "path", path, "error", err)
s.progress.Error("Failed to read file", err)

// ✗ Bad: Info level
logger.Info("Starting scan")  // DON'T USE

// ✗ Bad: User message to stdout (breaks piping)
fmt.Println("Starting scan...")  // Use stderr or progress

// ✗ Bad: Fatal for non-fatal error
logger.Fatal("Failed to read file")  // Use Error instead
```
```

#### 7.2 Update README.md

Add section on logging and output:

```markdown
## Output and Logging

### Output Channels

Stack Analyzer separates data output from progress messages:

- **stdout**: Structured data (JSON) - safe for piping
- **stderr**: Progress and error messages

### Piping Examples

```bash
# Pipe JSON results to jq
stack-analyzer scan /path | jq '.techs'

# Show progress while piping
stack-analyzer scan --verbose /path | jq '.techs'
# Progress appears on stderr, data pipes to jq

# Suppress progress
stack-analyzer scan --verbose /path 2>/dev/null | jq
```

### Logging Options

```bash
# Enable debug logs (stderr)
stack-analyzer scan --log-level=debug /path

# Write logs to file (keeps stderr clean)
stack-analyzer scan --log-level=debug --log-file=debug.log /path

# Deep debugging with trace level
stack-analyzer scan --log-level=trace --log-file=trace.log /path
```

### Log Levels

- `trace`: Deep debugging (rule matching, data inspection)
- `debug`: Internal operations (component detection)
- `error`: Non-fatal errors (default)
- `fatal`: Fatal errors only
```

## Implementation Checklist

### Phase 1: Audit ✅
- [x] Audit logger.Info() calls
- [x] Audit fmt.Print* calls
- [x] Audit log.* calls
- [x] Categorize each output statement

### Phase 2: Settings ✅
- [x] Remove Info from valid log levels
- [x] Add Error level support
- [x] Add LogFile field to Settings
- [x] Update ConfigureLogger() for file output
- [x] Add --log-file flag
- [x] Update default log level to Error

### Phase 3: Progress ✅
- [x] Add EventScanInitializing
- [x] Add EventFileWriting
- [x] Add EventFileWritten
- [x] Implement new event methods
- [x] Update SimpleHandler
- [x] Update TreeHandler

### Phase 4: Migration ✅
- [x] Remove line 80-83 (Starting message)
- [x] Migrate line 127-131 (Initializing)
- [x] Migrate line 141 (Scanning file)
- [x] Migrate line 144 (Scanning directory)
- [x] Migrate line 165-170 (File output messages)
- [x] Add support for `-o -` as stdout shorthand

### Phase 5: Testing ✅
- [x] Test piping: `stack-analyzer scan -o - /path | jq`
- [x] Test verbose piping: `stack-analyzer scan --verbose -o - /path | jq`
- [x] Test log file: `--log-level=debug --log-file=debug.log`
- [x] Test stderr suppression: `2>/dev/null`
- [x] Test combined: `--verbose --log-level=debug --log-file=debug.log`
- [x] Test default behavior (writes to file)

### Phase 6: Documentation ✅
- [x] Update README.md with output/logging section
- [x] Add piping examples to README
- [x] Update flag descriptions
- [x] Update environment variables
- [x] Add output channels explanation

## Success Criteria

1. ✅ `stack-analyzer scan /path | jq` works without noise
2. ✅ `stack-analyzer scan --verbose /path | jq` shows progress on stderr, pipes data
3. ✅ `--log-level=debug` enables debug logs
4. ✅ `--log-file=debug.log` writes logs to file
5. ✅ No Info level logs exist
6. ✅ All user messages go to stderr or stdout appropriately
7. ✅ Progress package handles all verbose output
8. ✅ Documentation is clear and complete

## Migration Timeline

- **Phase 1-2**: 1 hour (audit + settings)
- **Phase 3**: 1 hour (progress package)
- **Phase 4**: 2 hours (migration + testing)
- **Phase 5**: 1 hour (documentation)

**Total**: ~5 hours

## Questions to Resolve

1. **Should we show "Writing results to X" without --verbose?**
   - ✅ **DECISION**: Option C - Show only final confirmation (like curl)
   - Implementation: `fmt.Fprintf(os.Stderr, "Results written to %s\n", settings.OutputFile)`
   - Rationale: User feedback without being verbose

2. **Should errors always go to stderr even without --verbose?**
   - ✅ **DECISION**: Yes, always show errors
   - Rationale: Users need to see errors

3. **Default log level?**
   - ✅ **DECISION**: `error` (only errors and fatal)
   - Rationale: Clean output by default, debug on demand

4. **Should we add --quiet flag to suppress stderr?**
   - ✅ **DECISION**: No, use `2>/dev/null` instead
   - Rationale: Unix philosophy - don't reinvent shell features
