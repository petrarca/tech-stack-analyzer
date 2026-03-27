# Extending the Analyzer

## Adding New Technology Rules

### 1. Create a New Rule File

```yaml
# internal/rules/core/database/newtech.yaml
tech: newtech                    # Required: Unique technology identifier
name: New Technology             # Required: Display name
type: db                         # Required: Technology category
description: Modern database solution with high performance and scalability  # Optional: Technology description
properties:                      # Optional: Arbitrary key/value pairs for custom metadata
  website: https://newtech.example.com
  founded: 2020
  versions:
    - "1.0"
    - "2.0"
  api_version: v2
  category: "Database"
is_component: true               # Optional: Override component behavior
is_primary_tech: true           # Optional: Override primary tech promotion
dotenv:                          # Optional: Environment variable patterns
  - NEWTECH_
dependencies:                    # Optional: Package dependencies to detect
  - type: npm
    name: newtech-driver         # Can be regex: /^@newtech\/.*/ 
    example: newtech-driver
  - type: python
    name: newtech-client
    example: newtech-client
files:                           # Optional: Specific files to match
  - newtech.conf
  - config/newtech.yml
extensions:                      # Optional: File extensions to match
  - .newtech
  - .nt
content:                         # Optional: Content patterns for validation
  - pattern: 'newtech\s*=\s*['"']'
    extensions: [.conf, .yml]      # Specify where to check
  - pattern: 'import.*newtech'
    extensions: [.js, .py]         # Check JS and Python files
  - type: json-path                # JSON path matching
    path: $.config.provider
    value: newtech
    files: [config.json]
  - type: json-path                # JSON schema validation via $schema field
    path: $.$schema
    value: https://newtech.example.com/schema.json
    files: [newtech.json]
```

### Complete Rule Field Reference

**Required Fields:**
- **`tech`** - Unique technology identifier (used in output)
- **`name`** - Human-readable display name
- **`type`** - Technology category (database, framework, language, etc.)

**Optional Fields:**

**`description`** - Technology description for additional context
```yaml
description: AI safety and research company providing Claude AI models and APIs
```
- Used in JSON and YAML outputs of `info techs` command
- Provides additional context about the technology
- Empty string if not specified

**`properties`** - Arbitrary key/value pairs for custom metadata
```yaml
properties:
  website: https://example.com
  founded: 2021
  models:
    - model-large
    - model-medium
    - model-small
  api_version: v1
  category: "Large Language Models"
```
- Supports any YAML/JSON compatible data types (strings, numbers, arrays, objects)
- Used in JSON and YAML outputs of `info techs` command
- Perfect for storing technical details, documentation links
- Empty map `{}` if not specified (null in JSON, {} in YAML)

**`is_component`** - Override component creation behavior
- `true` - Always create a component
- `false` - Never create a component
- `null`/omitted - Use type-based default

**`is_primary_tech`** - Override primary technology promotion behavior
- `true` - Always promote to `tech[]` array (even without component)
- `false` - Never promote to `tech[]` array (even with component)
- `null`/omitted - Check category-level `is_primary_tech` (from `categories.yaml`), then fall back to `is_component`

Priority chain: **rule `is_primary_tech`** → **category `is_primary_tech`** → **`is_component`**

Technologies in `tech[]` are then weight-filtered into `primary_techs[]` at the product level (code-line weighting or component-count threshold).

| Configuration | Component Created | In `tech[]` | Use Case |
|---------------|------------------|-------------|----------|
| `is_component: true` (no `is_primary_tech`) | Yes | Category decides | databases, messaging — promoted by category |
| `is_component: true, is_primary_tech: false` | Yes | No | docker, nginx — infrastructure, not app stack |
| `is_component: false, is_primary_tech: true` | No | Yes | angular, fastapi — framework, no own graph node |
| `is_component: false` (no `is_primary_tech`) | No | Category decides | most tools and frameworks |

**`dotenv`** - Array of environment variable prefixes
```yaml
dotenv:
  - POSTGRES_    # Matches POSTGRES_DB, POSTGRES_HOST, etc.
  - REDIS_URL    # Matches exact env var names
```

**`dependencies`** - Package dependencies to detect
```yaml
dependencies:
  - type: npm
    name: react                  # Exact match
    example: react
  - type: npm
    name: /^@types\/.*$/         # Regex pattern
    example: '@types/node'
  - type: python
    name: django>=3.0            # Version pattern
    example: django
```

**Supported dependency types:**
- `npm`, `python`, `pip`, `cargo`, `composer`, `nuget`, `maven`, `gradle`
- `docker`, `githubAction`, `terraform.resource`

**`files`** - Specific files to match (glob patterns)
```yaml
files:
  - package.json              # Exact filename match
  - requirements.txt          # Exact filename match
  - Dockerfile                # Exact filename match
  - spfile*.ora               # Glob: matches spfile.ora, spfileORCL.ora, etc.
  - *.config.js               # Glob: matches any .config.js file
```
**Pattern syntax:** Glob patterns where `*` matches any characters and `?` matches a single character.

**`extensions`** - File extensions to match
```yaml
extensions:
  - .py
  - .js
  - .ts
  - .go
```

**`content`** - Content patterns for precise detection (regex patterns)

Content patterns use **regex** for matching file contents:

```yaml
content:
  # Regex pattern matching (default type)
  - pattern: 'import\s+.*react'
    extensions: [.js, .jsx, .ts, .tsx]  # Must specify where to check
  - pattern: 'FROM\s+node:'
    files: [Dockerfile]                  # Or specific files
  - pattern: 'Q_OBJECT'
    extensions: [.cpp, .h, .hpp]         # Check C++ files for Qt

  # JSON Path matching - check values at specific JSON paths
  - type: json-path
    path: $.name                         # Path to check
    files: [package.json]                # Path exists = match
  - type: json-path
    path: $.dependencies.react
    value: /^18\./                       # Optional: regex value match
    files: [package.json]

  # YAML Path matching - check values at specific YAML paths
  - type: yaml-path
    path: $.services.web
    files: [docker-compose.yml]          # Path exists = match
  - type: yaml-path
    path: $.version
    value: "3.8"                         # Optional: exact value match
    files: [docker-compose.yml]
```

**Content Type Reference:**

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `regex` (default) | Regex pattern matching on file content | `pattern`, `extensions` or `files` |
| `json-path` | Checks if JSON path exists or matches value | `path`, `files`, optional `value` |
| `yaml-path` | Checks if YAML path exists or matches value | `path`, `files`, optional `value` |
| `xml-path` | Checks if XML path exists or matches value | `path`, `files`, optional `value` |

**Value Matching:**
- Exact string: `value: "3.8"` matches exactly "3.8"
- Regex pattern: `value: /^18\./` matches strings starting with "18."

**Note:** Content patterns must specify `extensions` or `files` to define where to check. They operate independently of top-level `extensions`/`files` fields.

### 2. Rule Categories

The rules are organized into 30+ categories:

```
internal/rules/core/
├── ai/                   # AI/ML technologies
├── analytics/            # Analytics platforms
├── application/          # Application frameworks
├── automation/           # Automation tools
├── build/                # Build systems
├── ci/                   # CI/CD systems
├── cloud/                # Cloud providers
├── cms/                  # Content management systems
├── collaboration/        # Collaboration tools
├── communication/        # Communication services
├── crm/                  # CRM systems
├── database/             # Database systems
├── etl/                  # ETL tools
├── framework/            # Application frameworks
├── hosting/              # Hosting services
├── infrastructure/       # Infrastructure tools
├── language/             # Programming languages
├── monitoring/           # Monitoring and observability
├── network/              # Network tools
├── notification/         # Notification services
├── payment/              # Payment processors
├── queue/                # Message queues
├── runtime/              # Runtime environments
├── saas/                 # SaaS platforms
├── security/             # Security tools
├── ssg/                  # Static site generators
├── storage/              # Storage services
├── test/                 # Testing frameworks
├── tool/                 # Development tools
├── ui/                   # UI libraries and frameworks
└── validation/           # Validation libraries
```

## Adding New Component Detectors

### 1. Create Detector Structure

```go
// internal/scanner/components/newtech/detector.go
package newtech

import (
    "github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
    "github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
    return "newtech"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string,
    provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
    // Implementation here
}

func init() {
    components.Register(&Detector{})
}
```

### 2. Create Parser (if needed)

```go
// internal/scanner/parsers/newtech.go
package parsers

type NewTechParser struct{}

func (p *NewTechParser) ParseConfig(content string) NewTechConfig {
    // Parse configuration files
}
```

### 3. Register in Scanner

```go
// internal/scanner/scanner.go
import (
    _ "github.com/petrarca/tech-stack-analyzer/internal/scanner/components/newtech"
)
```

## Adding New File Matchers

```go
// internal/scanner/matchers/newmatcher.go
func registerNewMatcher() {
    components.RegisterFileMatcher(&matcher.FileMatcher{
        Tech:       "newtech",
        Extensions: []string{".newext"},
        Pattern:    regexp.MustCompile(`newtech\.config`),
    })
}
```

## Technology Category Configuration

Technology categories and their component behavior are defined in `internal/config/categories.yaml`. This configuration file determines which technology categories create architectural components versus being classified as tools/libraries.

### Category Configuration File

```yaml
# internal/config/categories.yaml
types:
  database:
    is_component: true
    description: "Database systems (PostgreSQL, MongoDB, Redis, etc.)"

  backend_framework:
    is_component: false
    description: "Backend frameworks (Django, Spring, Express, NestJS, etc.)"
```

### Adding New Technology Categories

1. Add the category definition to `internal/config/categories.yaml`:
   ```yaml
   my_new_category:
     is_component: true  # or false
     description: "Description of this category"
   ```

2. Create the category directory and use it in your rules:
   ```bash
   mkdir internal/rules/core/my_new_category
   ```
   ```yaml
   tech: my-tech
   name: My Technology
   # type is derived from folder name automatically
   ```

**Benefits:**
- **No code changes required** - Edit YAML, no recompilation needed
- **Self-documenting** - Descriptions explain each category's purpose
- **Centralized** - All category definitions in one place
- **Discoverable** - Use `stack-analyzer info categories --components` to list all categories

### Per-Rule Component Override

Individual rules can override the type's default behavior using the `is_component` field:

```yaml
tech: mfc
type: ui_framework  # Default: is_component: false
is_component: true  # Override: create component anyway
```

**Priority Order:**
1. Rule's `is_component` field (highest priority)
2. Category definition in `categories.yaml`
3. Default to `false` if category not defined

**Example Use Cases:**
- **Promote to component**: `desktop_framework` with `is_component: true` creates a component
- **Demote from component**: `database` with `is_component: false` doesn't create a component
- **New categories**: Categories not in `categories.yaml` default to no component creation

## Custom Rule Directories

> **Note**: External rules support is planned but not yet implemented. Currently, the scanner uses embedded rules only.
