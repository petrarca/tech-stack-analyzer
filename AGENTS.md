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
├── rules/core/        # Embedded YAML rules (700+ files)
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

Create YAML in `internal/rules/core/{category}/`:
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
3. Validate: `yamllint internal/rules/core/{category}/{file}.yaml`
4. Test: `task run -- /test/project`
5. Run: `task fct`

## Adding Component Detectors

Create `internal/scanner/components/{tech}/detector.go`:
```go
func DetectComponent(files []types.File, currentPath, basePath string, provider types.Provider) *types.Payload {
    // Check for key files, parse dependencies
    // Return nil if not detected
}
```

Register in `internal/scanner/components/registry.go`:
```go
func init() {
    registry["tech-name"] = DetectComponent
}
```

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
**Rule not detected:** Validate YAML, check file is in `core/`, test with `task run -- /path`

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

---

**Core Principle:** This project's value is zero-dependency deployment. Focus on deployment simplicity and realistic claims, not marketing language.