# Scanner Architecture

## Overview

The scanner uses a **rules-based architecture** with independent detection mechanisms. It features:
- **700+ YAML technology rules** for declarative detection
- **Three independent detection paths**: Extension/File, Content, and Dependency matching
- **Component detector registry** for tech-specific analysis (Node.js, Python, Docker, Terraform, etc.)
- **Content matcher registry** with O(1) hash map lookups for efficient pattern matching
- **go-enry integration** for comprehensive language detection (GitHub Linguist)
- **Implicit component creation** for architectural technologies (databases, SaaS, monitoring)

## Directory Structure

```
internal/
├── scanner/
│   ├── scanner.go                    # Main scanning logic & orchestration
│   ├── components/
│   │   ├── registry.go              # Component detector registry
│   │   ├── nodejs/detector.go       # Node.js package.json analysis
│   │   ├── python/detector.go       # Python pyproject.toml analysis
│   │   ├── dotnet/detector.go       # .NET .csproj analysis
│   │   ├── java/detector.go         # Java Maven/Gradle analysis
│   │   ├── docker/detector.go       # Docker Dockerfile analysis
│   │   ├── terraform/detector.go    # Terraform HCL analysis
│   │   └── [15+ more detectors...]  # Ruby, Rust, PHP, Go, Deno, etc.
│   ├── matchers/
│   │   ├── file.go                  # File name matcher (O(1) hash map)
│   │   ├── extension.go             # Extension matcher (O(1) hash map)
│   │   ├── content.go               # Content pattern matcher (O(1) hash map)
│   │   └── dependency.go            # Dependency matcher (regex-based)
│   └── parsers/
│       ├── json.go                  # JSON parser (package.json, etc.)
│       ├── toml.go                  # TOML parser (pyproject.toml, Cargo.toml)
│       ├── xml.go                   # XML parser (.csproj, pom.xml)
│       ├── hcl.go                   # HCL parser (Terraform .tf files)
│       └── dotenv.go                # Dotenv parser (.env files)
├── rules/
│   ├── loader.go                    # YAML rule loading & validation
│   └── core/                        # 700+ embedded technology rules
│       ├── database/                # PostgreSQL, MongoDB, Redis, etc.
│       ├── framework/               # React, Django, Spring, etc.
│       ├── language/                # Python, JavaScript, Go, etc.
│       ├── ui/                      # Qt, MFC, Tailwind, etc.
│       └── [30+ more categories...]
└── types/
    └── types.go                     # Core data structures (Payload, Rule, etc.)
```

## Scanning Flow

The scanner processes each directory in **three independent phases**:

### Phase 1: Extension & File Matching
```go
// Match by file extensions (.py, .js, .go)
extensionMatches := matchers.MatchExtensions(files)
processTechMatches(extensionMatches)  // Add to payload

// Match by file names (package.json, Dockerfile)
fileMatches := matchers.MatchFiles(files, currentPath, basePath)
processTechMatches(fileMatches)  // Add to payload
```
**Result:** Technologies detected by file presence (e.g., `.py` → python, `package.json` → nodejs)

### Phase 2: Content Matching
```go
// For each file, check if content matchers exist
for file in files {
    if contentMatcher.HasFileMatchers(file.Name) || 
       contentMatcher.HasContentMatchers(file.Extension) {
        content := readFile(file)
        matches := contentMatcher.Match(file, content)
        processContentMatches(matches)  // Add to payload
    }
}
```
**Result:** Technologies detected by code patterns (e.g., `Q_OBJECT` in `.cpp` → qt, `#include <afx` → mfc)

### Phase 3: Component Detection
```go
// Run component detectors for detailed analysis
for detector in componentDetectors {
    components := detector.Detect(files, currentPath)
    for component in components {
        payload.AddChild(component)
        findImplicitComponents(component)  // Create database/SaaS components
    }
}
```
**Result:** Detailed component analysis (e.g., Node.js project with dependencies, Docker images with metadata)

### Full Recursion Flow
```go
func (s *Scanner) recurse(payload *types.Payload, currentPath string) error {
    files := s.provider.ListDir(currentPath)
    
    // Phase 1 & 2: Rule-based detection
    matchedTechs := s.applyRules(payload, files, currentPath)
    
    // Phase 3: Component detection
    s.detectComponents(payload, files, currentPath, matchedTechs)
    
    // Language detection for all files
    for file in files {
        if file.IsFile {
            payload.DetectLanguage(file.Name)
        }
    }
    
    // Recurse into subdirectories
    for dir in directories {
        if !shouldIgnore(dir) {
            s.recurse(payload, filepath.Join(currentPath, dir))
        }
    }
}
```

## How It Works

### 1. Rules-Based Detection

All technology detection starts with **YAML rules** loaded at startup:

```yaml
# internal/rules/core/ui/qt.yaml
tech: qt
name: Qt Framework
type: ui
extensions: [.pro, .ui, .qrc]  # Simple file presence detection
content:
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h]       # Content validation in C++ files
  - pattern: 'Qt[0-9]::'
    files: [CMakeLists.txt]      # Content validation in specific files
```

**Rule Structure:**
- `tech` - Unique technology identifier
- `name` - Display name
- `type` - Category (database, framework, ui, etc.)
- `extensions` - File extensions for simple detection
- `files` - Specific filenames for simple detection
- `content` - Patterns for content-based validation
- `dependencies` - Package dependencies to match

### 2. Matcher Registries (O(1) Performance)

At startup, rules are compiled into **hash map registries** for O(1) lookups:

#### File Matcher Registry
```go
type FileMatcherRegistry struct {
    matchers map[string][]*FileMatcher  // Key: filename (e.g., "package.json")
}

// Built once at startup
BuildFileMatchersFromRules(rules)

// O(1) lookup during scan
fileMatches := fileRegistry.MatchFiles(files)
// Returns: {"nodejs": ["matched file: package.json"]}
```

#### Extension Matcher Registry
```go
type ExtensionMatcherRegistry struct {
    matchers map[string][]*ExtensionMatcher  // Key: extension (e.g., ".py")
}

// O(1) lookup during scan
extMatches := extRegistry.MatchExtensions(files)
// Returns: {"python": ["matched extension: .py"]}
```

#### Content Matcher Registry
```go
type ContentMatcherRegistry struct {
    matchers     map[string][]*ContentMatcher  // Key: extension (e.g., ".cpp")
    fileMatchers map[string][]*ContentMatcher  // Key: filename (e.g., "CMakeLists.txt")
}

// Built from content patterns with restrictions
for rule in rules {
    for pattern in rule.Content {
        if pattern.Extensions {
            // Register for each extension
            registry.matchers[".cpp"] = append(..., matcher)
        }
        if pattern.Files {
            // Register for specific files
            registry.fileMatchers["CMakeLists.txt"] = append(..., matcher)
        }
    }
}

// O(1) lookup + pattern matching during scan
if contentMatcher.HasContentMatchers(".cpp") {
    content := readFile(file)
    matches := contentMatcher.MatchContent(".cpp", content)
    // Returns: {"qt": ["content matched: Q_OBJECT"]}
}
```

**Performance Benefits:**
- **O(1) lookup** - Hash map access by extension/filename
- **Pre-compiled regex** - Patterns compiled once at startup
- **Filtered checks** - Only check relevant patterns per file
- **Early exit** - Stop after first match per tech

**Example:** For a `.cpp` file:
- Without hash map: Check all 700 rules = 700 iterations
- With hash map: Lookup `.cpp` matchers = ~10 patterns to check
- **70x faster!**

### 3. Independent Detection Paths

The three detection mechanisms are **completely independent** and run in sequence:

```go
// Phase 1: Extension/File Detection (OR logic)
extMatches := MatchExtensions(files)     // .py → python
fileMatches := MatchFiles(files)         // package.json → nodejs

// Phase 2: Content Detection (OR logic, independent)
contentMatches := MatchContent(files)    // Q_OBJECT in .cpp → qt

// Phase 3: Dependency Detection (OR logic, independent)
depMatches := MatchDependencies(deps)    // "react" in package.json → react

// All results are combined (OR)
allTechs := merge(extMatches, fileMatches, contentMatches, depMatches)
```

**Key Principle:** Each detection path is independent. A technology can be detected by:
- Extension alone (`.py` → python)
- Content alone (`Q_OBJECT` in `.cpp` → qt)
- Dependency alone (`"react"` in package.json → react)
- Any combination of the above

### 4. Content Matching Independence

Content patterns are **NOT** additive to extensions - they're independent:

```yaml
# MFC: Content-only detection (no top-level extensions)
tech: mfc
content:
  - pattern: '#include\s+<afx'
    extensions: [.cpp, .h]  # Defines WHERE to check
```

**How it works:**
1. Scanner sees `.cpp` file
2. Checks if content matchers exist for `.cpp` → YES (MFC pattern registered)
3. Reads file content
4. Matches pattern → MFC detected ✓

**Without MFC patterns in `.cpp` file:**
1. Scanner sees `.cpp` file
2. Checks if content matchers exist for `.cpp` → YES
3. Reads file content
4. No pattern matches → MFC NOT detected ✓

**Result:** No false positives! Pure C++ projects don't match MFC.

### 5. Component Detectors

Component detectors provide **detailed analysis** beyond simple detection:

```go
type Detector interface {
    Name() string
    Detect(files []types.File, currentPath string, 
           basePath string, provider types.Provider) *types.Payload
}
```

**Examples:**
- **Node.js**: Parses `package.json`, extracts dependencies, versions
- **Docker**: Parses `Dockerfile`, extracts base images, ports, stages
- **Terraform**: Parses `.tf` files, extracts providers, resource counts
- **Python**: Parses `pyproject.toml`, extracts dependencies

**Auto-registration via `init()`:**
```go
// internal/scanner/components/nodejs/detector.go
func init() {
    components.Register(&Detector{})
}
```

**Detector loop during scan:**
```go
for _, detector := range components.GetDetectors() {
    if component := detector.Detect(files, currentPath, basePath, provider); component != nil {
        payload.AddChild(component)
        
        // Create implicit components for detected techs
        for _, tech := range component.Techs {
            s.findImplicitComponentByTech(component, tech, currentPath)
        }
    }
}
```

## Component Types

### Named Components (User Projects)
- **Purpose**: Represent user-created projects/services
- **Name**: Extracted from config files (`package.json`, `pyproject.toml`, etc.)
- **Tech field**: Contains primary technologies (e.g., `["nodejs"]`, `["python"]`)
- **Examples**: 
  - Node.js: `@myapp/frontend` from `package.json`
  - Python: `mylib` from `pyproject.toml`
  - Docker: Service names from `docker-compose.yml`
- **Hierarchy**: Can contain child components

### Implicit Components (Third-Party Technologies)
- **Purpose**: Represent external dependencies and services
- **Creation**: Automatically created when technology is detected
- **Tech field**: Contains the technology identifier (e.g., `["postgresql"]`, `["redis"]`)
- **Examples**:
  - Databases: PostgreSQL, MongoDB, Redis
  - SaaS: OpenAI, Stripe, Datadog
  - Infrastructure: Nginx, Docker
- **Edges**: Created for architectural components (databases, SaaS, monitoring)
- **Not created for**: Hosting/cloud providers (AWS, GCP, Azure)

### Component Classification

Determined by `type` field in rules and `is_component` override:

```yaml
# Creates component (appears in tech field)
tech: postgresql
type: database
is_component: true  # Optional: type default is true

# No component (only in techs array)
tech: react
type: framework
is_component: false  # Optional: type default is false
```

**Type Configuration** (`internal/config/types.yaml`):
```yaml
types:
  database:
    is_component: true   # Creates components
  framework:
    is_component: false  # No components
```

## Language Detection

The scanner uses **go-enry** (GitHub Linguist Go port) for comprehensive language detection:

```go
import "github.com/go-enry/go-enry/v2"

func (p *Payload) DetectLanguage(filename string) {
    // Try detection by extension first (fast path)
    lang, safe := enry.GetLanguageByExtension(filename)
    
    // Fallback to filename for special files (Makefile, Dockerfile)
    if !safe || lang == "" {
        lang, _ = enry.GetLanguageByFilename(filename)
    }
    
    if lang != "" {
        p.AddLanguage(lang)
    }
}
```

**Features:**
- Detects 1500+ languages from GitHub Linguist database
- Handles special files without extensions (Makefile, Dockerfile)
- Fast extension-based detection with filename fallback
- Language counts stored in `payload.Languages` map

## Supported Component Detectors

### Currently Implemented (20+ Detectors)

| Tech Stack | Detector | Detection File | Analysis |
|------------|----------|----------------|----------|
| Node.js    | `nodejs/` | `package.json` | Dependencies, versions, scripts |
| Python     | `python/` | `pyproject.toml`, `requirements.txt` | Dependencies, versions |
| .NET       | `dotnet/` | `*.csproj` | NuGet packages, target framework |
| Java       | `java/` | `pom.xml`, `build.gradle` | Maven/Gradle dependencies |
| Kotlin     | `kotlin/` | `build.gradle.kts` | Gradle dependencies |
| Go         | `golang/` | `go.mod` | Module dependencies |
| Rust       | `rust/` | `Cargo.toml` | Cargo dependencies |
| Ruby       | `ruby/` | `Gemfile` | Gem dependencies |
| PHP        | `php/` | `composer.json` | Composer dependencies |
| Deno       | `deno/` | `deno.json` | Deno dependencies |
| Docker     | `docker/` | `Dockerfile`, `docker-compose.yml` | Base images, ports, services |
| Terraform  | `terraform/` | `*.tf` | Providers, resources by category |
| Kubernetes | `kubernetes/` | `*.yaml` | Deployments, services |
| GitHub Actions | `githubactions/` | `.github/workflows/*.yml` | Workflows, actions |
| Dotenv     | `dotenv/` | `.env*` | Environment variables |
| License    | `license/` | `LICENSE*` | License detection |
| JSON Schema | `jsonschema/` | `components.json` | Shadcn UI components |

### Adding New Detectors

Easy to add following the same pattern:
1. Create `internal/scanner/components/newtech/detector.go`
2. Implement `Detector` interface
3. Add `init()` for auto-registration
4. Import in `scanner.go`

## Adding a New Detector

### Step 1: Create Detector File

```go
// internal/scanner/components/golang/detector.go
package golang

import (
    "github.com/stack-analyser/scanner/internal/scanner/components"
    "github.com/stack-analyser/scanner/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
    return "golang"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, 
                          provider types.Provider, depDetector DependencyDetector) []*types.Payload {
    // Detection logic here
    return nil
}

func init() {
    components.Register(&Detector{})
}
```

### Step 2: Import in Scanner

```go
// internal/scanner/scanner.go
import (
    _ "github.com/stack-analyser/scanner/internal/scanner/components/golang"
)
```

That's it! The detector is now active.

## Architecture Benefits

### Performance

**O(1) Hash Map Lookups:**
- File matcher: `O(1)` lookup by filename
- Extension matcher: `O(1)` lookup by extension
- Content matcher: `O(1)` lookup by extension/filename
- **Result**: 70x faster than iterating all 700 rules

**Pre-compiled Regex:**
- Patterns compiled once at startup
- No runtime compilation overhead
- Efficient pattern matching

**Efficient File Reading:**
- Only read files that have content matchers
- Early exit after first match per tech
- Skip ignored directories (`.venv`, `node_modules`)

### Modularity

**Separation of Concerns:**
- **Rules**: Declarative YAML (no code changes needed)
- **Matchers**: Generic pattern matching logic
- **Detectors**: Tech-specific analysis
- **Scanner**: Orchestration and recursion

**Independent Detection Paths:**
- Extension/File detection
- Content detection
- Dependency detection
- Each can work independently

### Extensibility

**Adding Technologies:**
1. Create YAML rule file
2. No code changes required
3. Automatic registration in matchers

**Adding Detectors:**
1. Create detector file
2. Implement interface
3. Auto-registration via `init()`
4. Import in scanner

**Adding Matcher Types:**
1. Create new matcher registry
2. Build from rules at startup
3. Call during scan phase

### Maintainability

**Clear Architecture:**
- Rules define WHAT to detect
- Matchers define HOW to match
- Detectors define HOW to analyze
- Scanner defines WHEN to run

**Testability:**
- Each component independently testable
- Mock providers for file system
- Isolated matcher testing
- Detector unit tests

**Configuration-Driven:**
- Type definitions in YAML
- Component behavior configurable
- No code changes for new types

## Implementation Status

### ✅ Fully Implemented

**Core Scanning:**
- ✅ Recursive directory traversal with ignore patterns
- ✅ Three-phase detection (extension/file, content, component)
- ✅ Language detection with go-enry (1500+ languages)
- ✅ Hierarchical component tree
- ✅ Implicit component creation with edges

**Matcher Registries:**
- ✅ File matcher registry (O(1) hash map)
- ✅ Extension matcher registry (O(1) hash map)
- ✅ Content matcher registry (O(1) hash map)
- ✅ Dependency matcher (regex-based)

**Rules System:**
- ✅ 700+ embedded YAML technology rules
- ✅ 30+ rule categories
- ✅ Independent detection paths (extension, content, dependency)
- ✅ Content patterns with extension/file restrictions
- ✅ Type-based component classification

**Component Detectors (20+):**
- ✅ Node.js, Python, .NET, Java, Kotlin, Go, Rust, Ruby, PHP, Deno
- ✅ Docker, Terraform, Kubernetes
- ✅ GitHub Actions, Dotenv, License, JSON Schema

**Advanced Features:**
- ✅ Docker metadata extraction (base images, ports, stages)
- ✅ Terraform resource analysis (providers, categories)
- ✅ Progress reporting (verbose, debug, trace modes)
- ✅ Rule tracing for debugging
- ✅ Configurable component types
- ✅ Project configuration (`.stack-analyzer.yml`)

### Thread Safety

The scanner is designed for thread-safe operation:
- Component registry uses `sync.RWMutex`
- Matcher registries are read-only after initialization
- Provider interface abstracts file system access
- Ready for concurrent scanning (future feature)

## Summary

The Tech Stack Analyzer uses a **rules-based architecture** with:
- **700+ YAML rules** for declarative detection
- **O(1) hash map lookups** for performance
- **Independent detection paths** for flexibility
- **20+ component detectors** for detailed analysis
- **Content matching** with no false positives
- **Modular design** for easy extension

This architecture provides a solid foundation for comprehensive technology stack analysis with excellent performance and maintainability. 🎉
