# Building and Architecture

## Building

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

### Available Tasks

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
