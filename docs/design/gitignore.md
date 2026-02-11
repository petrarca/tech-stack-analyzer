# Gitignore and File Exclusion Design

## Overview
The scanner uses a **stack-based gitignore implementation** that properly respects git's exclusion hierarchy, supporting multiple ignore sources with correct precedence.

## Architecture

### Core Components
- **`GitignoreStack`**: Manages pattern sets with push/pop operations
- **`StackBasedLoader`**: Handles loading and exclusion checking
- **`PatternSet`**: Represents patterns from a single ignore file

### Stack-Based Approach
```
Top (highest priority) ──┐
├── .git/info/exclude    │  Local personal excludes
├── Config excludes      │  Configuration file patterns  
├── CLI arguments         │  Command line --exclude patterns
└── Directory .gitignore  │  Pushed/popped during traversal
Bottom (lowest priority) ─┘
```

## Implementation Details

### 1. Pattern Loading
```go
// From .gitignore files
func loadPatternsFromGitignore(path string) ([]string, error)

// From .git/info/exclude (highest priority)
func loadGitInfoExclude(gitDir string) ([]string, error)
```

### 2. Stack Operations
- **`Push()`**: Add patterns when entering directory with .gitignore
- **`Pop()`**: Remove patterns when leaving directory
- **`ShouldExclude()`**: Check file against entire stack in order

### 3. Exclusion Sources
| Source | Priority | Tracked | Use Case |
|--------|----------|---------|----------|
| `.git/info/exclude` | 1 (highest) | No | Personal local ignores |
| Config file | 2 | Yes | Project configuration |
| CLI `--exclude` | 3 | No | Runtime overrides |
| `.gitignore` | 4+ | Yes | Project-wide ignores |

### 4. Pattern Matching
- Uses `doublestar.Match()` for glob patterns
- Supports standard gitignore syntax (excluding negation `!`)
- Matches both relative paths and filenames
- Case-sensitive matching

## Usage Flow

```go
// Initialize
loader := git.NewStackBasedLoaderWithLogger(progress, logger)

// Load top-level excludes
loader.InitializeWithTopLevelExcludes(basePath, cliExcludes, configExcludes)

// During directory traversal
if loader.LoadAndPushGitignore(dirPath) {
    // .gitignore found, patterns active
}

// Check each file
if loader.ShouldExclude(fileName, relativePath) {
    return // Skip file
}

// When leaving directory
loader.PopGitignore()
```

## Key Features

### Proper Hierarchy
- Patterns only apply to their directory and subdirectories
- Maintains git's precedence rules

### Multiple Sources
- CLI arguments
- Configuration files
- `.gitignore` files
- `.git/info/exclude` (personal)

### Performance
- Stack-based O(1) push/pop operations
- Efficient pattern matching with doublestar

### Robustness
- Handles git worktrees and submodules
- Graceful fallback for non-git repos
- Detailed logging for debugging

## Limitations

- No negation pattern support (`!important.log`)
- No `.git/info/exclude` global config support
- Basic glob patterns only (no advanced gitignore syntax)

## Future Enhancements

1. **Negation Patterns**: Support `!` syntax for unignoring files
2. **Global Excludes**: Support `core.excludesfile` configuration
3. **Advanced Syntax**: Full gitignore pattern compatibility
4. **Performance**: Pattern caching for large repositories
