# Output Format

## Overview

The scanner outputs a hierarchical JSON structure showing detected technologies, components, and their relationships.

## Regular Output

```json
{
  "id": "root",
  "name": "my-project",
  "path": "/",
  "tech": ["nodejs", "react", "postgresql"],
  "techs": ["nodejs", "react", "postgresql", "docker", "express", "eslint"],
  "primary_techs": ["nodejs", "react", "postgresql"],
  "languages": {"JavaScript": 145, "TypeScript": 89},
  "reason": {
    "docker": ["matched file: Dockerfile"],
    "react": ["react matched: ^react$"],
    "_": ["base image: nginx:alpine", "license detected: MIT"]
  },
  "dependencies": [
    ["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "^4.18.2", "prod", true, {"source": "package-lock.json"}]
  ],
  "git": {
    "branch": "main",
    "commit": "a1b2c3d",
    "remote_url": "https://github.com/user/repo.git"
  },
  "children": [
    {
      "id": "backend",
      "name": "backend",
      "path": "/backend",
      "type": "npm-package",
      "tech": ["nodejs"],
      "techs": ["nodejs", "express", "postgresql"],
      "component_dependencies": [
        ["docker-base-image", "node", "20-alpine", "", {"file": "/backend/Dockerfile"}]
      ],
      "git": {
        "branch": "develop",
        "commit": "def5678",
        "remote_url": "https://github.com/myorg/backend.git"
      }
    }
  ],
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "duration_ms": 1173,
    "file_count": 523
  }
}
```

## Aggregated Output

Use the `--aggregate` flag to get a simplified, rolled-up view of your entire codebase:

```bash
./bin/stack-analyzer scan --aggregate tech,techs,languages,licenses,dependencies,git /path/to/project
./bin/stack-analyzer scan --aggregate git /path/to/project  # Show only git repositories
./bin/stack-analyzer scan --aggregate all /path/to/project  # Aggregate all fields with metadata
```

**Output:**
```json
{
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "scan_path": "/path/to/project",
    "specVersion": "0.1",
    "duration_ms": 1173,
    "file_count": 523
  },
  "techs": ["nodejs", "react", "postgresql", "docker", "express", "vite"],
  "primary_techs": ["nodejs", "react", "postgresql"],
  "languages": {"JavaScript": 145, "TypeScript": 89, "CSS": 12},
  "dependencies": [
    ["npm", "react", "^18.2.0", "prod", true, {"source": "package-lock.json"}],
    ["npm", "express", "^4.18.2", "prod", true, {"source": "package-lock.json"}],
    ["npm", "vite", "^5.0.0", "dev", true, {"source": "package-lock.json"}]
  ],
  "git": [
    {
      "branch": "main",
      "commit": "abc1234",
      "remote_url": "https://github.com/user/project.git"
    }
  ]
}
```

**Available fields:**
- `tech` - Primary technologies
- `techs` - All detected technologies (includes frameworks, tools, libraries)
- `languages` - Programming languages with file counts
- `licenses` - Detected licenses from LICENSE files and package manifests
- `dependencies` - All dependencies as `[type, name, version, scope, direct, metadata]` arrays (always 6 elements)
- `git` - Git repositories (deduplicated) with branch, commit, dirty status, and remote URL
- `reason` - Detection reasons per technology
- `all` - Aggregate all available fields with metadata

## Field Reference

### Top-Level Fields

- **id**: Unique identifier for each component
- **name**: Component name (e.g., "main", "frontend", "backend")
- **path**: File system path relative to the project root
- **type**: Component type (e.g., "npm-package", "maven-module", "docker-compose-service") - present when the component detector provides it
- **tech**: Array of primary technologies for this component — filtered by `is_primary_tech` category flag (frameworks, runtimes, databases, languages; excludes docker, nginx, CI tools, test frameworks)
- **techs**: Array of all technologies detected in this component (components + tools/libraries)
- **primary_techs**: Weight-filtered subset of `tech[]` identifying the dominant technologies. Uses code-line weighting (≥1% of total typed code) when per-component `code_stats` are available; falls back to component-count threshold otherwise. Present at root level in both full and aggregated formats.
- **languages**: Object mapping programming languages to file counts
- **licenses**: Array of detected licenses in this component
- **dependencies**: Array of detected dependencies with format `[type, name, version, scope, direct, metadata]` (always 6 elements)
- **component_dependencies**: Array of component-level dependencies (e.g., Docker base images, parent Maven modules) with format `[type, name, version, scope, metadata]` (always 5 elements)
- **children**: Array of nested components (sub-projects, services, etc.)
- **edges**: Array of relationships between components (e.g., service -> database connections); created for architectural components like databases, SaaS services, and monitoring tools, but not for hosting/cloud providers
- **reason**: Object mapping technologies to detection reasons, with "_" key for non-tech reasons (licenses, base images, etc.)
- **properties**: Object containing tech-specific metadata (Docker, Terraform, Kubernetes, etc.)
- **code_stats**: Code statistics with analyzed/unanalyzed buckets (see [usage.md](usage.md#code-statistics))
- **subsystem_stats**: Per-subsystem code stats rollup (root node only; present when `--subsystem-depth > 0` or `subsystem-groups` is defined in config). Each entry has `path` (folder prefix or group name), `component_count`, and `code_stats`. See [usage.md](usage.md#subsystem-statistics).
- **git**: Git repository information (available at root and component levels for multi-repo projects)
- **metadata**: Scan execution metadata (only in root payload)

### Dependencies vs Component Dependencies

The scanner tracks two types of dependencies:

**Package Dependencies** (`dependencies`):
- Runtime and build-time library dependencies from package managers
- Format: `[type, name, version, scope, direct, metadata]` (6 elements)
- Examples: npm packages, Python packages, Maven artifacts, NuGet packages
- The `direct` field indicates if it's a direct dependency (true) or transitive (false)

```json
"dependencies": [
  ["npm", "react", "18.2.0", "prod", true, {"source": "package-lock.json"}],
  ["npm", "express", "4.18.2", "prod", true, {"source": "package-lock.json"}],
  ["python", "django", "4.2.0", "prod", true, {"source": "requirements.txt"}]
]
```

**Component Dependencies** (`component_dependencies`):
- Structural dependencies between components or infrastructure elements
- Format: `[type, name, version, scope, metadata]` (5 elements, no `direct` field)
- Examples: Docker base images, Maven parent modules, Gradle project dependencies
- Represents architectural relationships rather than code-level dependencies

```json
"component_dependencies": [
  ["docker-base-image", "node", "20-alpine", "", {"file": "/backend/Dockerfile"}],
  ["docker-base-image", "nginx", "alpine", "", {"file": "/frontend/Dockerfile"}],
  ["maven-parent", "spring-boot-starter-parent", "3.2.0", "", {"file": "/pom.xml"}]
]
```

**Key Differences:**
- **Package dependencies** track library/package imports and are versioned with ranges or exact versions
- **Component dependencies** track architectural relationships and infrastructure choices
- **Package dependencies** include the `direct` boolean flag; component dependencies do not
- **Package dependencies** flow through the dependency tree; component dependencies are component-specific

### Metadata Field

The `metadata` field (present only in the root payload) provides information about the scan execution:

```json
{
  "metadata": {
    "timestamp": "2025-12-01T14:45:35Z",
    "scan_path": "/absolute/path/to/project",
    "specVersion": "0.1",
    "duration_ms": 1173,
    "file_count": 523,
    "component_count": 87,
    "language_count": 15,
    "tech_count": 3,
    "techs_count": 12,
    "properties": {
      "product": "My Product",
      "team": "Engineering"
    }
  }
}
```

**Fields:**
- **timestamp**: ISO 8601 timestamp when scan was performed
- **scan_path**: Absolute path to scanned directory
- **specVersion**: Output format specification version
- **duration_ms**: Scan duration in milliseconds
- **file_count**: Total language-detected files scanned (sum of all language file counts)
- **component_count**: Total components in the payload tree (architectural components, not filesystem directories)
- **language_count**: Number of distinct programming languages detected
- **tech_count**: Number of primary technologies (count of `tech` array)
- **techs_count**: Number of all detected technologies (count of `techs` array)
- **properties**: Custom properties from `.stack-analyzer.yml`

### Git Field

The `git` field provides git repository information:

- **branch**: Current branch name
- **commit**: Short commit hash (7 characters)
- **remote_url**: Origin remote URL

### Properties Field

The `properties` field provides structured metadata about specific technologies detected in the project. This field uses an industry-standard format compatible with JSON Schema, OpenAPI, and SBOM tools.

**Supported Technologies:**

**Docker** - Extracts information from Dockerfiles:
```json
"properties": {
  "docker": [
    {
      "file": "/backend/Dockerfile",
      "base_images": ["python:3.13", "python:3.13-slim"],
      "exposed_ports": [8080],
      "multi_stage": true,
      "stages": ["builder"]
    }
  ]
}
```

**Terraform** - Aggregates infrastructure resources:
```json
"properties": {
  "terraform": [
    {
      "file": "/infrastructure/main.tf",
      "providers": ["aws", "google"],
      "resources_by_provider": {
        "aws": 15,
        "google": 3
      },
      "resources_by_category": {
        "compute": 5,
        "storage": 8,
        "database": 3,
        "networking": 2
      },
      "total_resources": 18
    }
  ]
}
```

**Key Features:**
- **Array format**: Supports multiple files (multiple Dockerfiles, .tf files, etc.)
- **File tracking**: Each entry includes the source file path
- **Component-scoped**: Properties can appear at root or in child components
- **Tool-friendly**: Compatible with security scanners, SBOM generators, and CI/CD tools

### Multi-Technology Components

When multiple technology stacks are detected in the same directory (e.g., a directory with both `package.json` and `pom.xml`), the scanner automatically merges them into a single component with multiple primary technologies:

```json
{
  "name": "hybrid-service",
  "tech": ["nodejs", "java"],
  "techs": ["nodejs", "java", "maven", "npm", "typescript"],
  "languages": {
    "TypeScript": 150,
    "Java": 45
  }
}
```

This is common in projects with:
- Node.js frontend + Java backend in the same module
- Integration tests (Playwright/TypeScript) alongside Java applications
- Build tools from multiple ecosystems

### Lock File Support

The analyzer automatically uses lock files to extract exact resolved versions instead of version ranges:
- **Node.js** - `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock` -> falls back to `package.json`
- **Python** - `uv.lock`, `poetry.lock` -> falls back to `pyproject.toml`, `requirements.txt`, `setup.py`
- **Rust** - `Cargo.lock` -> falls back to `Cargo.toml`
- **Go** - `go.mod` (already contains exact versions)

This ensures accurate dependency versions for security scanning and compliance analysis.
