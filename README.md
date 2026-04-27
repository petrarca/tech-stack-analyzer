# Tech Stack Analyzer

A technology stack analyzer written in Go that automatically detects technologies, frameworks, databases, and tools used in codebases. Re-implements [specfy/stack-analyser](https://github.com/specfy/stack-analyser) with improvements and extended technology support.

## What It Does

The Tech Stack Analyzer scans a codebase and produces a structured JSON inventory of its technology stack. It identifies:

- **Programming Languages** - Source code languages and versions
- **Package Dependencies** - npm, pip, cargo, composer, nuget, maven, conan, and more
- **Frameworks** - .NET, Spring Boot, Angular, React, Django, and others
- **Databases** - PostgreSQL, MySQL, MongoDB, Redis, Oracle, SQL Server
- **Infrastructure** - Docker, Kubernetes, Terraform, CI/CD pipelines
- **DevOps Tools** - Monitoring, deployment, and build tools

Detection is powered by **800+ technology rules** across 48 categories, using file names, extensions, package manifests, environment variables, and content pattern matching.

## Key Features

- **Zero Dependencies** - Single binary deployment, no runtime requirements
- **800+ Technology Rules** - Comprehensive detection across 48 categories
- **Lock File Support** - Extracts exact resolved versions from package-lock.json, Cargo.lock, uv.lock, etc.
- **Code Statistics** - Lines of code, complexity metrics, and language breakdown via SCC
- **Automatic .gitignore** - Respects `.gitignore` files with full gitignore semantics (negation `!`, dir-only `/`, last-match-wins)
- **Hierarchical Output** - Component-based analysis with parent-child relationships
- **Aggregated Views** - Rollup summaries for quick technology stack overviews
- **Content-Based Detection** - Validates technologies through regex pattern matching in file contents
- **Language Reclassification** - Override go-enry's language detection per glob pattern to fix misclassified extensions or relabel proprietary file formats

## Quick Start

### Install

```bash
# Build from source
git clone https://github.com/petrarca/tech-stack-analyzer.git
cd tech-stack-analyzer
go build -o bin/stack-analyzer ./cmd/scanner

# Or use Task (recommended)
task build

# Or install directly
go install github.com/petrarca/tech-stack-analyzer/cmd/scanner@latest
```

**Prerequisites:** Go 1.19+

### Scan a Project

```bash
# Scan current directory
./bin/stack-analyzer scan

# Scan a specific directory
./bin/stack-analyzer scan /path/to/project

# Save results to a custom file
./bin/stack-analyzer scan /path/to/project --output results.json

# Get an aggregated overview
./bin/stack-analyzer scan --aggregate all /path/to/project

# Full output + aggregate in one scan pass (e.g. for large codebases)
./bin/stack-analyzer scan /path/to/project --also-aggregate tech,techs,languages,dependencies,git

# Strip fields not needed by downstream consumers
./bin/stack-analyzer scan /path/to/project --omit-fields reason,edges

# Pipe to jq
./bin/stack-analyzer scan -o - /path/to/project | jq '.techs'

# List available technologies
./bin/stack-analyzer info techs
```

### Example Output

```json
{
  "id": "a30339f5ba410aaa588e",
  "name": "my-project",
  "path": ["/"],
  "tech": ["nodejs"],
  "techs": ["nodejs", "react", "postgresql", "docker"],
  "languages": {"TypeScript": 89, "JavaScript": 45},
  "dependencies": [
    ["npm", "react", "18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "4.18.2", "prod", true, {"source": "package-lock.json"}]
  ],
  "children": [
    {
      "id": "f6b220e7ad1a4575fa38",
      "name": "backend",
      "path": ["/backend"],
      "tech": ["nodejs"],
      "techs": ["nodejs", "express", "postgresql"]
    }
  ],
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "duration_ms": 1173,
    "file_count": 523
  }
}
```

## Use Cases

- **Technology Inventory** - Generate comprehensive stack documentation
- **CI/CD Integration** - Fast dependency detection in build pipelines
- **Portfolio Analysis** - Scan hundreds of repositories in minutes
- **Migration Planning** - Understand technology landscape before cloud or framework migrations
- **License Compliance** - Feed dependency output into specialized license tools
- **Security Scanning** - Provide dependency lists to vulnerability scanners

## Documentation

For detailed documentation, see the [docs/](docs/) folder:

| Document | Description |
|----------|-------------|
| [Usage Guide](docs/usage.md) | Commands, flags, verbose mode, code statistics, content-based detection |
| [Configuration](docs/configuration.md) | Project config, environment variables, scan config files, logging |
| [Output Format](docs/output.md) | Output structure, field reference, aggregated output, metadata |
| [Extending](docs/extending.md) | Adding technology rules, component detectors, category configuration |
| [Building](docs/building.md) | Build instructions, project structure, architecture overview |

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on code style, pre-commit hooks, pull requests, and development workflow.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- **Original Project**: Go re-implementation of [specfy/stack-analyser](https://github.com/specfy/stack-analyser) by the original author
- **Industry Alignment**: References [Google's deps.dev](https://deps.dev) for dependency data structure design
- **Language Detection**: Uses [go-enry](https://github.com/go-enry/go-enry) (GitHub Linguist port) for language identification
- **Git Integration**: Uses [go-git](https://github.com/go-git/go-git) for pure Go git operations

---

Built with Go - Single binary, zero dependencies, 800+ technology rules.
