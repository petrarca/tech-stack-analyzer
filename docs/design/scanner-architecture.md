# Scanner Architecture

## Overview

The scanner uses a **rules-based architecture** with multiple independent detection mechanisms:
- **700+ YAML technology rules** for declarative detection
- **15 plugin-based component detectors** for deep project analysis
- **O(1) hash map matcher registries** for file, extension, and content matching
- **go-enry integration** for language detection (GitHub Linguist)
- **Implicit component creation** for architectural technologies (databases, SaaS, monitoring)

## Directory Structure

```
internal/
├── scanner/
│   ├── scanner.go                    # Main scanning logic & orchestration
│   ├── dependencies.go              # Dependency matching engine
│   ├── component_categories.go      # Component type classification
│   ├── component_registry.go        # Inter-component dependency resolution
│   ├── components/
│   │   ├── detector.go              # Detector interface definition
│   │   ├── registry.go              # Plugin registry (init-based)
│   │   ├── cocoapods/detector.go    # CocoaPods Podfile analysis
│   │   ├── cplusplus/detector.go    # C++ Conan analysis
│   │   ├── delphi/detector.go       # Delphi .dproj analysis
│   │   ├── deno/detector.go         # Deno lock file analysis
│   │   ├── docker/detector.go       # Docker Compose analysis
│   │   ├── dotnet/detector.go       # .NET .csproj analysis
│   │   ├── githubactions/detector.go # GitHub Actions workflow analysis
│   │   ├── golang/detector.go       # Go module analysis
│   │   ├── java/detector.go         # Java Maven/Gradle analysis
│   │   ├── nodejs/detector.go       # Node.js package.json analysis
│   │   ├── php/detector.go          # PHP Composer analysis
│   │   ├── python/detector.go       # Python pyproject.toml/requirements.txt/setup.py
│   │   ├── ruby/detector.go         # Ruby Gemfile analysis
│   │   ├── rust/detector.go         # Rust Cargo.toml analysis
│   │   └── terraform/detector.go    # Terraform HCL analysis
│   ├── matchers/
│   │   ├── file.go                  # File name matcher (O(1) hash map)
│   │   ├── extension.go             # Extension matcher (O(1) hash map)
│   │   └── content.go               # Content pattern matcher (O(1) hash map)
│   ├── parsers/                     # Shared parsing logic (used by detectors)
│   │   ├── nodejs.go                # package.json parsing
│   │   ├── npm_lock.go              # package-lock.json parsing
│   │   ├── yarn_lock.go             # yarn.lock parsing
│   │   ├── pnpm_lock.go            # pnpm-lock.yaml parsing
│   │   ├── python.go                # requirements.txt (PEP 508) parsing
│   │   ├── poetry_lock.go           # poetry.lock parsing
│   │   ├── uv_lock.go              # uv.lock parsing
│   │   ├── maven.go                 # pom.xml parsing
│   │   ├── gradle.go                # build.gradle parsing
│   │   ├── dotnet.go                # .csproj XML parsing
│   │   ├── rust.go                  # Cargo.toml parsing
│   │   ├── cargo_lock.go           # Cargo.lock parsing
│   │   ├── php.go                   # composer.json parsing
│   │   ├── gemfile.go               # Gemfile parsing
│   │   ├── gemfile_lock.go          # Gemfile.lock parsing
│   │   ├── cocoapods.go             # Podfile parsing
│   │   ├── conan.go                 # conanfile.py parsing
│   │   ├── delphi.go                # .dproj parsing
│   │   ├── deno.go                  # deno.lock parsing
│   │   ├── docker_compose.go        # docker-compose.yml parsing
│   │   ├── dockerfile.go            # Dockerfile parsing
│   │   ├── github_actions.go        # GitHub Actions YAML parsing
│   │   ├── golang.go                # go.mod parsing
│   │   ├── terraform.go             # HCL parsing
│   │   ├── dotenv.go                # .env.example parsing
│   │   └── constants.go             # Shared dependency type constants
│   └── semver/                      # Semantic version parsing
├── rules/
│   ├── loader.go                    # YAML rule loading (embedded)
│   └── techs/                       # 700+ embedded technology rules
│       ├── database/                # PostgreSQL, MongoDB, Redis, etc.
│       ├── framework/               # React, Django, Spring, etc.
│       ├── language/                # Python, JavaScript, Go, etc.
│       ├── ui/                      # Qt, MFC, Tailwind, etc.
│       └── [32 categories total]
├── types/                           # Core data structures
│   ├── payload.go                   # Payload (component tree node)
│   ├── rule.go                      # Rule definition
│   └── nanoid.go                    # ID generation
├── provider/                        # File system abstraction
├── license/                         # License detection
├── git/                             # Git repository info
└── config/                          # Configuration & type definitions
```

## Scanning Flow

### High-Level Flow

```
NewScanner(path, options)
  -> Load 700+ YAML rules
  -> Build matcher registries (O(1) hash maps)
  -> Register 15 plugin detectors via init()

Scan()
  -> Create root payload
  -> recurse(rootPayload, basePath)
     -> For each directory:
        1. List files
        2. applyRules(payload, files, currentPath)
        3. Detect languages (go-enry)
        4. Recurse into subdirectories
  -> Resolve inter-component dependencies
  -> Return result tree
```

### The `applyRules` Pipeline

Each directory is processed through 4 detection steps:

```go
func applyRules(payload, files, currentPath) {
    // Step 1: Component detection (15 plugin detectors)
    //   - Creates named components (e.g., Node.js project from package.json)
    //   - Creates virtual components (merged into parent)
    //   - Parses dependencies and matches against rules
    ctx = detectComponents(payload, files, currentPath)

    // Step 2: Dotenv detection
    //   - Reads .env.example files
    //   - Matches variable names against rule dotenv patterns
    //   - E.g., POSTGRES_HOST -> postgresql tech detected
    detectDotenv(ctx, files, currentPath)

    // Step 3: File and extension matching (O(1) hash maps)
    //   - Matches filenames against rules (package.json -> nodejs)
    //   - Matches extensions against rules (.py -> python)
    //   - Matches file content against patterns (Q_OBJECT -> qt)
    matchedTechs = detectByFilesAndExtensions(ctx, files, currentPath)

    // Step 4: Rule-file detection
    //   - Checks rules that define specific file patterns
    //   - Applies remaining rule-based matches
    detectByRuleFiles(ctx, files, matchedTechs)
}
```

### Why This Order Matters

Component detection (step 1) runs first because it may create new child payloads. Steps 2-4 then apply to the correct context (either the parent or a newly created component). This ensures technologies detected by file patterns and dotenv are attributed to the right component.

## Detection Systems

### 1. Plugin-Based Component Detectors

The primary detection system for project-level analysis. All 15 detectors implement a common interface and auto-register via Go's `init()` mechanism.

**Interface:**
```go
type Detector interface {
    Name() string
    Detect(files []types.File, currentPath, basePath string,
           provider types.Provider, depDetector DependencyDetector) []*types.Payload
}
```

**Registration:**
```go
// In each detector's init() function:
func init() {
    components.Register(&Detector{})
}

// In scanner.go, blank imports trigger registration:
import (
    _ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/nodejs"
    _ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/python"
    // ... 13 more
)
```

**Detector loop during scan:**
```go
for _, detector := range components.GetDetectors() {
    components := detector.Detect(files, currentPath, basePath, provider, depDetector)
    // Named components become children, virtual components merge into parent
}
```

**Current detectors (15):**

| Detector | Detection Files | Creates | Analysis |
|----------|----------------|---------|----------|
| `cocoapods` | `Podfile`, `Podfile.lock` | Named | Pod dependencies |
| `cplusplus` | `conanfile.py`, `conanfile.txt` | Named | Conan dependencies |
| `delphi` | `*.dproj` | Named | VCL/FMX framework, packages |
| `deno` | `deno.lock` | Virtual | Deno module dependencies |
| `docker` | `docker-compose.yml` | Virtual | Service images, container deps |
| `dotnet` | `*.csproj` | Named | NuGet packages, target framework |
| `githubactions` | `.github/workflows/*.yml` | Virtual | Action deps, container images |
| `golang` | `go.mod`, `main.go` | Named | Go module dependencies |
| `java` | `pom.xml`, `build.gradle` | Named | Maven/Gradle dependencies |
| `nodejs` | `package.json` | Named | npm/yarn dependencies, license |
| `php` | `composer.json` | Named | Composer dependencies, license |
| `python` | `pyproject.toml`, `requirements.txt`, `setup.py` | Named | pip dependencies, license |
| `ruby` | `Gemfile` | Named | Gem dependencies |
| `rust` | `Cargo.toml` | Named | Cargo dependencies, license |
| `terraform` | `*.tf`, `.terraform.lock.hcl` | Virtual | Providers, resources by category |

### 2. Dotenv Detection

A separate detection step (not a plugin detector) that matches environment variable names from `.env.example` files against `dotenv` patterns in YAML rules.

```yaml
# Example: internal/rules/techs/database/postgresql.yaml
tech: postgresql
dotenv:
  - POSTGRES_
```

If `.env.example` contains `POSTGRES_HOST=localhost`, the scanner detects `postgresql`. About 100 rules define dotenv patterns. This produces a virtual payload merged into the parent.

**Why not a plugin?** Dotenv detection needs access to the full rule set to match variable names against patterns. The plugin `Detector` interface doesn't provide rules. The detection is also fundamentally different -- it detects technology hints from variable names, not components from project files.

### 3. Matcher Registries (O(1) Hash Maps)

At startup, YAML rules are compiled into hash map registries for O(1) lookups:

**File Matcher:** Maps filenames to rules. `"package.json" -> [nodejs rule]`

**Extension Matcher:** Maps extensions to rules. `".py" -> [python rule]`

**Content Matcher:** Maps extensions/filenames to content patterns. When a file has content matchers registered for its extension, the file is read and patterns are checked.

```
Without hash map: Check all 700 rules per file = 700 iterations
With hash map:    Lookup extension matchers = ~10 patterns to check
```

### 4. Implicit Component Creation

When a technology is detected (by any mechanism), the scanner checks if it should create an implicit child component based on the rule's `type` field:

```yaml
# Creates an implicit component (type default: is_component: true)
tech: postgresql
type: database

# Does NOT create an implicit component (type default: is_component: false)
tech: react
type: framework
```

Type defaults are configured in `internal/config/types.yaml`.

## Component Types

### Named Components
- Represent user-created projects/services
- Name extracted from config files (`package.json`, `pyproject.toml`, etc.)
- Appear as separate children in the output tree
- Have their own git info in multi-repo scans
- Example: `my-api-server` from `pyproject.toml`

### Virtual Components
- Represent supplementary detection (dotenv, GitHub Actions, docker-compose)
- Name is always `"virtual"`
- Merged into the parent payload (dependencies and techs are combined)
- Do not appear as separate entries in output

### Implicit Components
- Auto-created when architectural technologies are detected
- Represent third-party services (databases, SaaS, monitoring)
- Created based on `is_component` flag in type configuration
- Can have edges (architectural relationships) to parent components

## Language Detection

Uses **go-enry** (GitHub Linguist Go port) for file-level language detection:

```go
func (p *Payload) DetectLanguage(filename string) {
    lang, safe := enry.GetLanguageByExtension(filename)
    if !safe || lang == "" {
        lang, _ = enry.GetLanguageByFilename(filename)
    }
    if lang != "" {
        p.AddLanguage(lang)
    }
}
```

Detects 1500+ languages. Language counts are stored in `payload.Languages` as a map of language name to file count.

## Adding a New Plugin Detector

### Step 1: Create Detector

```go
// internal/scanner/components/mytech/detector.go
package mytech

import (
    "github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
    "github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string { return "mytech" }

func (d *Detector) Detect(files []types.File, currentPath, basePath string,
    provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
    // 1. Check for relevant files
    // 2. Read and parse config files via provider
    // 3. Extract dependencies
    // 4. Match dependencies: depDetector.MatchDependencies(names, "depType")
    // 5. Return payload(s) or nil
    return nil
}

func init() {
    components.Register(&Detector{})
}
```

### Step 2: Add Blank Import

```go
// internal/scanner/scanner.go
import (
    _ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/mytech"
)
```

### Step 3: Add Tests

Create `internal/scanner/components/mytech/detector_test.go` with:
- Mock provider and dependency detector
- Test cases for file detection, parsing, dependency matching
- Edge cases (malformed files, missing fields, file read errors)

## Architecture Principles

### Separation of Concerns
- **Rules** (YAML): Define WHAT to detect (declarative, no code changes needed)
- **Matchers**: Define HOW to match files/extensions/content
- **Detectors**: Define HOW to analyze project files in depth
- **Parsers**: Shared parsing logic used by detectors
- **Scanner**: Orchestrates WHEN each system runs

### Detection Independence
Each detection mechanism works independently. A technology can be detected by:
- File extension alone (`.py` -> python)
- File content alone (`Q_OBJECT` in `.cpp` -> qt)
- Dependency alone (`"react"` in package.json -> react)
- Dotenv alone (`POSTGRES_HOST` in .env.example -> postgresql)
- Any combination of the above

### Provider Abstraction
All file access goes through the `types.Provider` interface. Detectors never access the filesystem directly. This enables testing with mock providers and potential future support for remote/virtual filesystems.

### Zero-Dependency Deployment
The scanner compiles to a single binary. All 700+ YAML rules are embedded via Go's `embed` package. No external runtime dependencies required.
