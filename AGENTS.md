# AGENTS.md

AI coding agents: Follow these rules strictly when working on Tech Stack Analyzer.

## Project Overview

**Tech Stack Analyzer** is a Go implementation focusing on **zero-dependency deployment** - a single binary without Node.js runtime. This is the main value proposition, not performance.

### Key Principles

- **Zero Dependencies**: Single binary deployment is the primary benefit
- **Realistic Claims**: Never exaggerate performance
- **Modular Architecture**: Component-based detector system
- **Comprehensive Coverage**: 700+ rules across 32 categories

## Architecture

```
cmd/scanner/           # CLI entry point
internal/
├── provider/          # File system abstraction
├── rules/techs/       # Embedded YAML rules (700+ files)
├── scanner/           # Core scanning logic
│   ├── components/    # Technology-specific detectors
│   ├── matchers/      # Pattern matching
│   └── parsers/       # File parsers
└── types/             # Data structures
```

## Essential Commands

```bash
task build              # Build project
task fct                # format + check + test (run before commit)
task pre-commit:run     # Run pre-commit hooks
task run -- /path       # Test scanner on directory
```

## Running the Scanner

After building with `task build`, the scanner binary is available at `./bin/stack-analyzer`:

```bash
# Basic scan (outputs to stack-analysis.json)
./bin/stack-analyzer scan /path/to/project

# Scan multiple directories (merged into one output)
./bin/stack-analyzer scan /path/to/project1 /path/to/project2

# Scan with custom output file
./bin/stack-analyzer scan -o /tmp/output.json /path/to/project

# Scan with verbose output
./bin/stack-analyzer scan -v /path/to/project

# Scan with debug output (shows directory tree)
./bin/stack-analyzer scan -d /path/to/project

# Scan with exclusions (multiple --exclude options are supported, .gitignore syntax, .gitignore is used when available)
./bin/stack-analyzer scan --exclude "node_modules" --exclude "vendor" /path/to/project

# View available options
./bin/stack-analyzer scan --help
```

**Common use cases:**

- Test on local project: `./bin/stack-analyzer scan -o /tmp/scan.json /path/to/project`
- Quick test during development: `task run -- /path/to/project`
- Production scan: `./bin/stack-analyzer scan -o results.json --exclude "node_modules" /project`

## Configuration

The scanner supports configuration via `.stack-analyzer.yml` in project root:

```yaml
# Custom properties added to metadata
properties:
  product: "My Product Name"
  team: "Platform Engineering"
  owner: "engineering@company.com"

# Files and directories to exclude
exclude:
  - "node_modules"
  - "vendor"
  - "dist"
  - "build"
  - ".git"
```

## Testing Guidelines

- **Test projects shall be created in a temp directory like "/tmp"**
- Use `/tmp/test-project` or similar paths for temporary test files
- Clean up test files after verification
- Never create test files in the main project directory

## Before Making Repository Public

### Never Commit

- `.env*`, `*.key`, `*.pem` - Secrets and credentials
- `.DS_Store`, `Thumbs.db` - OS files
- `bin/`, `dist/` - Build artifacts
- Personal notes with sensitive context

### Pre-Publication Checklist

- [ ] Review `.gitignore` completeness
- [ ] Scan for secrets: `git grep -i "password\|api_key\|secret\|token"`
- [ ] Verify LICENSE file
- [ ] Test README installation instructions
- [ ] Update repository URLs

## Adding Technology Rules

Create YAML in `internal/rules/techs/{category}/`:

```yaml
tech: "technology-name"
name: "Display Name"
type: "database|framework|language|tool"
files: ["config.file"]
extensions: [".ext"]
dependencies:
  - type: "npm"
    name: "package-name"
```

**Process:**

1. Choose category (32 available: `ai/`, `analytics/`, `application/`, `automation/`, `build/`, `ci/`, `cloud/`, `cms/`, `collaboration/`, `communication/`, `crm/`, `database/`, `etl/`, `framework/`, `hosting/`, `infrastructure/`, `language/`, `misc/`, `monitoring/`, `network/`, `notification/`, `payment/`, `queue/`, `runtime/`, `saas/`, `security/`, `ssg/`, `storage/`, `test/`, `tool/`, `ui/`, `validation/`)
2. Create YAML file
3. Validate: `yamllint internal/rules/techs/{category}/{file}.yaml`
4. Test: `task run -- /test/project`
5. Generate taxonomy files: `task build:taxonomies`
6. Run: `task fct`

## Adding Component Detectors

All detectors are plugins under `internal/scanner/components/{tech}/`. See `docs/design/scanner-architecture.md` for the full architecture and `docs/design/detector-implementation.md` for the detector reference.

Create `internal/scanner/components/{tech}/detector.go`:

```go
package mytech

import (
    "github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
    "github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string { return "mytech" }

func (d *Detector) Detect(files []types.File, currentPath, basePath string,
    provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
    // Check for key files, read via provider, parse dependencies
    // Match deps: depDetector.MatchDependencies(names, "depType")
    // Return nil if not detected
    return nil
}

func init() {
    components.Register(&Detector{})
}
```

Add blank import in `internal/scanner/scanner.go`:

```go
_ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/mytech"
```

**Process:**

1. Create detector package with `detector.go` and `detector_test.go`
2. Implement `Detector` interface (Name + Detect)
3. Register via `init()` with `components.Register()`
4. Add blank import in `scanner.go`
5. Create parser in `parsers/` if needed (keep parsing separate from detection)
6. Run: `task fct`

## Testing

```bash
task test                           # Run all tests
go test -v ./...                    # Verbose output
go test -race ./...                 # Race detection
go test -cover ./...                # Coverage report
go test -run TestName ./path        # Specific test
```

Write table-driven tests:

```go
func TestDetector(t *testing.T) {
    tests := []struct {
        name     string
        files    []types.File
        expected *types.Payload
    }{
        {"detects config", []types.File{{Path: "config.yml"}}, &types.Payload{...}},
        {"returns nil when missing", []types.File{}, nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := DetectComponent(tt.files, "", "", mockProvider)
            // assertions
        })
    }
}
```

## Critical Rules

### NEVER Claim

- "Parallel processing" - implementation is sequential
- "Faster than original" - without benchmarks
- "Production-ready"/"Enterprise-grade" - marketing fluff

### ALWAYS

- Use provider interface for file operations (never direct file access)
- Handle errors gracefully
- Run `task fct` before committing
- Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`
- Update README for user-facing changes
- NEVER push to remote without explicit user permission
- If we change structure of the output, `schemas/stack-analyzer-output.json` needs to be updated
- After updating `schemas/stack-analyzer-output.json` schema, the example outputs shall be re-created using `task build:examples`
- Add package-level documentation comment to all Go packages (required for godoc and code clarity)
- When technology rules or categories are added or modified, generate taxonomy files with `task build:taxonomies`
- No emojis in code, comments, test output, or documentation

### Security

- No path traversal, no `exec.Command`, no hardcoded secrets
- Validate all inputs, sanitize user data
- Use provider interface exclusively

## Performance Claims

Only claim improvements with:

```bash
go test -bench=. -benchmem ./internal/scanner/
go test -cpuprofile=cpu.out -bench=.
time task run -- /large/project  # Run 10+ times for significance
```

Must have: reproducible benchmarks, >10% improvement, real-world validation.

## Dependencies

**Current:** `github.com/go-enry/go-enry/v2` (language detection), `gopkg.in/yaml.v3` (YAML parsing)

**Adding new:** `go get dependency@version` → update go.mod → test with `task test`

## Common Issues

**Build fails:** `task clean && go mod tidy && task build`  
**Tests fail:** `go test -v ./path/to/package`  
**Lint fails:** `task format && golangci-lint run --fix`  
**Rule not detected:** Validate YAML, check file is in `techs/`, test with `task run -- /path`

## Git Workflow

```bash
# Commit messages (one sentence max)
feat: Add Ruby detector
fix: Resolve path handling bug
docs: Update installation guide

# Branches
feature/detector-name
fix/issue-description
```

## Pre-Commit Checklist

- [ ] `task fct` passes
- [ ] Tests pass with `-race`
- [ ] No exaggerated claims
- [ ] Docs updated if needed
- [ ] No secrets or hardcoded paths
- [ ] Conventional commit message
- [ ] If technology rules or categories were modified, taxonomy files generated with `task build:taxonomies`

---

**Core Principle:** This project's value is zero-dependency deployment. Focus on deployment simplicity and realistic claims, not marketing language.
