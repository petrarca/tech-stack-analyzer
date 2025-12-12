# Component ID Design

## Overview

The Tech Stack Analyzer uses a hierarchical ID system that ensures unique identification of components while providing deterministic behavior when needed. The system consists of two main types of IDs:

1. **Root Component IDs** - Unique identifiers for the main scan component
2. **Child Component IDs** - Deterministic identifiers for sub-components based on root ID + path

## Design Goals

1. **Uniqueness**: Each scan should have a unique root component ID (or deterministic for reproducible builds)
2. **Determinism**: Child components should have reproducible IDs within the same scan
3. **Flexibility**: Support both deterministic and override-based root ID generation
4. **Git Integration**: Automatic deterministic IDs for version-controlled projects
5. **Path-Based Fallback**: Deterministic IDs for non-git directories using absolute paths
6. **Override Capability**: Allow manual specification of root IDs for CI/CD and testing

## Root Component ID Generation

### Priority System

Root component IDs are generated using the following priority order (highest to lowest):

1. **CLI Flag**: `--root-id <string>` (explicit override)
2. **Config File**: `root_id: <string>` in `.stack-analyzer.yml`
3. **Git Remote**: Deterministic ID from git repository remote URL
4. **Absolute Path**: Deterministic ID from absolute file path (for non-git directories)

### Git-Based ID Generation

When no explicit root ID is provided and the project is a git repository with a remote:

1. **Extract Remote URL**: Get the `origin` remote URL from git configuration
2. **Normalize URL**: Convert various git URL formats to consistent format:
   ```
   https://github.com/user/repo.git    → github.com/user/repo
   git@github.com:user/repo.git        → github.com/user/repo
   git://github.com/user/repo.git      → github.com/user/repo
   ```
3. **Include Relative Path**: For scans in subdirectories, include path from repo root
4. **Generate Hash**: Create SHA256 hash and use first 20 characters:
   ```go
   hash("github.com/user/repo:subdir")[:20]
   ```

### Absolute Path ID Generation

When no explicit root ID is provided and the project is not a git repository:

1. **Normalize Path**: Convert to absolute path and clean path separators:
   ```
   ./project                  → /current/dir/project
   ../sibling/project         → /current/dir/../sibling/project
   /Volumes/Data/Develop/cgm  → /Volumes/Data/Develop/cgm
   ```
2. **Generate Hash**: Create SHA256 hash of absolute path and use first 20 characters:
   ```go
   hash("/Volumes/Data/Develop/cgm/nais-master")[:20]
   ```

This ensures deterministic IDs for non-git directories while maintaining consistency across platforms.

### Examples

```bash
# CLI override (highest priority)
stack-analyzer scan . --root-id "my-project-2024"
# Result: "my-project-2024"

# Config file
# .stack-analyzer.yml
root_id: "config-project-2024"
stack-analyzer scan .
# Result: "config-project-2024"

# Git repository (automatic)
cd /path/to/github.com/user/repo
stack-analyzer scan .
# Result: "a30339f5ba410aaa588e" (deterministic)

# Non-git directory (deterministic path-based)
stack-analyzer scan /Volumes/Data/Develop/cgm/nais-master
# Result: "38e064cf93bfb689cb50" (deterministic from absolute path)

# Same non-git path always gets same ID
stack-analyzer scan /Volumes/Data/Develop/cgm/nais-master
# Result: "38e064cf93bfb689cb50" (same as above)
```

## Child Component ID Generation

### Deterministic Algorithm

Child component IDs are always generated deterministically using the formula:

```
child_id = SHA256(root_id + ":" + name + ":" + relative_path)[:20]
```

The component name is included to ensure that different components at the same path (e.g., language detectors "C" and "C++" at "/") receive unique IDs.

### Components Using This System

- **Language Projects**: Go modules, Node.js packages, Python projects
- **Infrastructure**: Docker services, Terraform providers
- **Configuration**: Kubernetes manifests, CI/CD files
- **Dependencies**: Package managers, build tools

### Path Handling

The `relative_path` is always the path from the scan root to the component's main file:

```
project/
├── go.mod           → relative_path: "/go.mod"
├── services/
│   └── docker.yml  → relative_path: "/services/docker.yml"
└── subdir/
    └── main.go     → relative_path: "/subdir/main.go"
```

### Examples

```bash
# Same root ID, same path, different names → Different child IDs
root_id = "test-123", path = "/test"
child_id("C", "/test")    = "f6b220e7ad1a4575fa38"
child_id("C++", "/test")  = "d4233ec7e4b8df375c03"
child_id("Go", "/test")   = "d2ba3bf8f3e788e7d390"

# Different root IDs, same name + path → Different child IDs
root_id = "project-2024"  → child_id("app", "/go.mod") = "1a2b3c4d5e6f7890abcd"
root_id = "project-2025"  → child_id("app", "/go.mod") = "9f8e7d6c5b4a3210fedc"

# Same root ID + same name + same path → Same child ID (deterministic)
root_id = "test-123"      → child_id("app", "/go.mod") = "694c605b68b590b7f272" (always)
```

## Implementation Details

### Post-Processing Architecture

The ID system uses post-processing to assign IDs after the entire component tree is built:

1. **Component Detection**: All components created with temporary IDs
2. **Tree Construction**: Complete hierarchy built with relationships
3. **ID Assignment**: `AssignIDs(root_id_override)` called on root
4. **Recursive Assignment**: All children receive deterministic IDs

### Edge Handling

Edges reference components by object pointers, not ID strings. When IDs are updated in post-processing:

```go
type Edge struct {
    Target *Payload  // Pointer to actual payload object
    Read   bool
    Write  bool
}
```

During JSON serialization, the current ID is read:
```go
func (e Edge) MarshalJSON() ([]byte, error) {
    targetID := e.Target.ID  // Reads current ID after post-processing
    // ...
}
```

### API Functions

```go
// Generate random root ID (12 characters)
func GenerateRootID() string

// Generate deterministic child ID (20 characters)
func GenerateComponentID(rootID, name, relativePath string) string

// Post-process entire tree with optional root override
func (p *Payload) AssignIDs(rootID string)
```

## Use Cases

### Development Workflow
```bash
# Local development - deterministic IDs for non-git directories
stack-analyzer scan .
# Root: "hash(/current/dir/.)" (deterministic from absolute path)
# Child: "hash(root_id:/go.mod)"

# Git repository - deterministic IDs
cd my-git-project
stack-analyzer scan .
# Root: "a30339f5ba410aaa588e" (from git remote)
# Child: "hash(a30339f5ba410aaa588e:/go.mod)"

# Non-git directory - reproducible across scans
stack-analyzer scan /Volumes/Data/Develop/cgm/nais-master
# Root: "38e064cf93bfb689cb50" (always same for this path)
# Child: "hash(38e064cf93bfb689cb50:/api/Dockerfile)"
```

### CI/CD Pipeline
```bash
# Reproducible builds with explicit root ID
stack-analyzer scan . --root-id "build-${BUILD_NUMBER}"
# Root: "build-1234"
# Child: "hash(build-1234:/go.mod)"
```

### Testing
```bash
# Deterministic test results
stack-analyzer scan test-project --root-id "test-fixtures"
# Root: "test-fixtures"
# Child: "hash(test-fixtures:/go.mod)"
```


## Configuration

### CLI Flags
```bash
--root-id <string>    # Override root component ID
```

### Config File
```yaml
# .stack-analyzer.yml
root_id: "my-project-2024"  # Override root component ID
```

### Environment Variables
```bash
# No direct env var for root ID (use config file or CLI)
```

## Security Considerations

1. **No Path Traversal**: Relative paths are validated and normalized
2. **Hash Collision**: SHA256 provides extremely low collision probability
3. **Git URL Sanitization**: Remote URLs are normalized before hashing
4. **No Secrets in IDs**: Only repository structure, not credentials, affects IDs

## Performance Impact

- **Minimal Overhead**: Post-processing adds O(n) time where n = number of components
- **Memory Efficient**: In-place ID updates, no additional data structures
- **Git Operations**: Remote URL extraction is cached and reused
- **Hash Computation**: SHA256 is fast and parallelizable

## Future Enhancements

1. **Custom Hash Functions**: Allow specification of hash algorithms
2. **Multiple Remotes**: Consider multiple git remotes for ID generation
3. **Branch/Tag Inclusion**: Optional inclusion of git branch/tag in ID
4. **Metadata Integration**: Include project metadata in ID generation
5. **ID Versioning**: Support for different ID generation schemes
