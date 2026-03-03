# Documentation

Detailed documentation for the Tech Stack Analyzer. For a quick introduction and getting started, see the [project README](../README.md).

## Guides

| Document | Description |
|----------|-------------|
| [Usage Guide](usage.md) | Commands, flags, verbose mode, code statistics, .gitignore support, content-based detection |
| [Configuration](configuration.md) | Project config file, environment variables, scan config files, logging |
| [Output Format](output.md) | Output structure, field reference, aggregated output, metadata, properties |
| [Extending the Analyzer](extending.md) | Adding technology rules, component detectors, file matchers, category configuration |
| [Building and Architecture](building.md) | Build instructions, project structure, core components, detection pipeline |

## Design Documents

| Document | Description |
|----------|-------------|
| [Scanner Architecture](design/scanner-architecture.md) | Scanning flow, detection systems, component types, plugin architecture |
| [Detector Reference](design/detector-implementation.md) | All 15 component detectors with detection files, parsers, and patterns |

## Quick Reference

### Common Commands

```bash
# Scan a project
stack-analyzer scan /path/to/project

# Aggregated overview
stack-analyzer scan --aggregate all /path/to/project

# Pipe to jq
stack-analyzer scan -o - /path | jq '.techs'

# List available technologies
stack-analyzer info techs

# Show rule details
stack-analyzer info rule postgresql
```

### Key Concepts

- **tech** - Primary technologies that define a component (e.g., nodejs, java, postgresql)
- **techs** - All detected technologies including tools and libraries (superset of tech)
- **Component** - An architectural unit detected by the scanner (e.g., a Node.js package, a Docker service)
- **Rule** - A YAML definition that describes how to detect a technology
- **Detector** - Go code that handles complex detection logic for specific project types
