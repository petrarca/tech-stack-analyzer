# Component Detector Reference

## Overview

The scanner uses 15 plugin-based component detectors, each responsible for identifying specific project types, parsing their configuration files, and extracting dependency information. All detectors implement a common interface and auto-register via Go's `init()` mechanism.

## Interface

```go
// internal/scanner/components/detector.go
type Detector interface {
    Name() string
    Detect(files []types.File, currentPath, basePath string,
           provider types.Provider, depDetector DependencyDetector) []*types.Payload
}

type DependencyDetector interface {
    MatchDependencies(dependencies []string, depType string) map[string][]string
    AddPrimaryTechIfNeeded(payload *types.Payload, tech string)
}
```

## Common Patterns

### Named vs Virtual Components

- **Named components** are created from project definition files that have a project name (e.g., `package.json`, `pyproject.toml`, `Cargo.toml`). They appear as separate children in the output tree with their own git info.
- **Virtual components** (name = `"virtual"`) are created for supplementary detection (GitHub Actions, Docker Compose, Deno lock). Their dependencies and techs merge into the parent payload.

### Dependency Matching Flow

All detectors follow the same pattern:
1. Parse file to extract dependency names
2. Call `depDetector.MatchDependencies(depNames, depType)` to match against YAML rules
3. Add matched techs to payload with reasons
4. Call `depDetector.AddPrimaryTechIfNeeded(payload, tech)` for primary tech classification
5. Store dependencies in `payload.Dependencies`

### Lock File Priority

Several detectors support lock file priority: when both a manifest and a lock file exist, the lock file provides exact versions. The manifest defines the project, the lock file provides pinned dependency versions.

### Directory Name Fallback

When a project file exists but doesn't contain a project name, detectors fall back to the directory name. For the root scan path, the fallback name is `"main"`.

## Detector Reference

### Node.js (`nodejs`)

| Field | Value |
|-------|-------|
| **Detection files** | `package.json` |
| **Component type** | Named |
| **Dependency type** | `npm` |
| **Parser** | `parsers.NodeJSParser` |
| **Lock files** | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml` |
| **Extra** | License detection, dev/prod dependency scoping |

Parses `package.json` for project name, dependencies, and devDependencies. Supports npm, yarn, and pnpm lock files for exact version resolution.

---

### Python (`python`)

| Field | Value |
|-------|-------|
| **Detection files** | `pyproject.toml`, `requirements.txt`, `setup.py` |
| **Component type** | Named |
| **Dependency type** | `python` |
| **Parser** | `parsers.PythonParser` (for requirements.txt) |
| **Lock files** | `uv.lock`, `poetry.lock` |
| **Extra** | License detection, PEP 508 compliant parsing |

Priority-based detection:
1. **`pyproject.toml`** - Parses `[project]` or `[tool.poetry]` sections for name and dependencies. Falls back to directory name if no name field found.
2. **`requirements.txt`** - PEP 508 compliant dependency parsing with canonical package name normalization. Uses directory name as component name.
3. **`setup.py`** - Basic detection only (no dependency parsing since setup.py is executable Python). Uses directory name.

If `pyproject.toml` is found and successfully parsed, lower-priority files are skipped.

---

### Go (`golang`)

| Field | Value |
|-------|-------|
| **Detection files** | `go.mod`, `main.go` |
| **Component type** | Named |
| **Dependency type** | `golang` |
| **Parser** | `parsers.GolangParser` |
| **Lock files** | `go.sum` (indirect) |
| **Extra** | Filters indirect dependencies |

Parses `go.mod` for module name and direct dependencies. Skips indirect dependencies (lines with `// indirect` comment). Also detects `main.go` for Go applications.

---

### Java/Kotlin (`java`)

| Field | Value |
|-------|-------|
| **Detection files** | `pom.xml`, `build.gradle`, `build.gradle.kts` |
| **Component type** | Named |
| **Dependency type** | `maven` (pom.xml), `gradle` (build.gradle) |
| **Parser** | `parsers.MavenParser`, `parsers.GradleParser` |
| **Extra** | Property resolution, BOM imports, profile support |

Handles both Java and Kotlin projects (they share Maven/Gradle tooling). Maven parser supports recursive property resolution, dependency management/BOM imports, plugin dependencies, and profile-scoped dependencies.

---

### Rust (`rust`)

| Field | Value |
|-------|-------|
| **Detection files** | `Cargo.toml` |
| **Component type** | Named (package) or Virtual (workspace) |
| **Dependency type** | `rust` |
| **Parser** | `parsers.RustParser` |
| **Lock files** | `Cargo.lock` |
| **Extra** | License detection, workspace support |

Parses `[package]` section for name and license, `[dependencies]` for crate dependencies. Handles different dependency formats: simple string, path, git, object with version. Workspaces produce virtual payloads.

---

### .NET (`dotnet`)

| Field | Value |
|-------|-------|
| **Detection files** | `*.csproj` |
| **Component type** | Named |
| **Dependency type** | `nuget` |
| **Parser** | `parsers.DotNetParser` |
| **Extra** | Framework detection (modern .NET vs legacy .NET Framework) |

Parses `.csproj` XML for `AssemblyName`, `TargetFramework`, and `PackageReference` elements. Detects both modern .NET (net6.0+) and legacy .NET Framework (net48, net472). Stores assembly name and package ID in component properties for inter-component dependency resolution.

---

### PHP (`php`)

| Field | Value |
|-------|-------|
| **Detection files** | `composer.json` |
| **Component type** | Named |
| **Dependency type** | `php` |
| **Parser** | `parsers.PHPParser` |
| **Extra** | License detection, dev dependency scoping |

Parses `composer.json` for project name, `require` and `require-dev` dependencies.

---

### Ruby (`ruby`)

| Field | Value |
|-------|-------|
| **Detection files** | `Gemfile` |
| **Component type** | Named |
| **Dependency type** | `ruby` |
| **Parser** | `parsers.RubyParser`, `parsers.GemfileLockParser` |
| **Lock files** | `Gemfile.lock` |
| **Extra** | Group-based scoping, git/path sources |

Parses `Gemfile` for gem declarations with version constraints. Lock file parser extracts exact versions and distinguishes direct from transitive dependencies.

---

### Deno (`deno`)

| Field | Value |
|-------|-------|
| **Detection files** | `deno.lock` |
| **Component type** | Virtual |
| **Dependency type** | `deno` |
| **Parser** | `parsers.DenoParser` |

Parses `deno.lock` JSON to extract remote module URLs and their hashes.

---

### Docker (`docker`)

| Field | Value |
|-------|-------|
| **Detection files** | `docker-compose.yml`, `docker-compose.yaml`, `docker-compose.*.yml` |
| **Component type** | Virtual (with child components per service) |
| **Dependency type** | `docker` |
| **Parser** | `parsers.DockerComposeParser` |
| **Extra** | Service image extraction, container name override |

Parses Docker Compose files to extract service definitions. Each service with an image produces a child component. Skips images starting with `$` (environment variables).

---

### Terraform (`terraform`)

| Field | Value |
|-------|-------|
| **Detection files** | `*.tf`, `.terraform.lock.hcl` |
| **Component type** | Virtual |
| **Dependency type** | `terraform` (providers), `terraform.resource` (resources) |
| **Parser** | `parsers.TerraformParser` |
| **Extra** | HCL parsing, resource categorization |

Parses `.terraform.lock.hcl` for provider dependencies and `*.tf` files for resource declarations. Resources are matched against rules for technology detection (e.g., `aws_instance` -> AWS).

---

### GitHub Actions (`githubactions`)

| Field | Value |
|-------|-------|
| **Detection files** | `.github/workflows/*.yml` |
| **Component type** | Virtual |
| **Dependency type** | `githubAction` (actions), `docker` (container/service images) |
| **Parser** | `parsers.GitHubActionsParser` |

Parses workflow YAML files to extract action references (`uses: actions/checkout@v4`), container images, and service images. Action names are matched against rules for technology detection.

---

### CocoaPods (`cocoapods`)

| Field | Value |
|-------|-------|
| **Detection files** | `Podfile`, `Podfile.lock` |
| **Component type** | Named |
| **Dependency type** | `cocoapods` |
| **Parser** | `parsers.CocoaPodsParser` |

Parses `Podfile` for pod declarations and `Podfile.lock` for resolved versions.

---

### C++ / Conan (`cplusplus`)

| Field | Value |
|-------|-------|
| **Detection files** | `conanfile.py`, `conanfile.txt` |
| **Component type** | Named |
| **Dependency type** | `conan` |
| **Parser** | `parsers.ConanParser` |

Parses Conan package manager files for C++ dependencies. Extracts project name from class definition in `conanfile.py`.

---

### Delphi (`delphi`)

| Field | Value |
|-------|-------|
| **Detection files** | `*.dproj` |
| **Component type** | Named |
| **Dependency type** | `delphi` |
| **Parser** | `parsers.DelphiParser` |
| **Extra** | VCL/FMX framework detection |

Parses `.dproj` XML for framework type (VCL or FMX), runtime packages from `DCC_UsePackage` elements, and project name from filename.

## Adding a New Detector

1. Create `internal/scanner/components/{name}/detector.go`
2. Implement the `Detector` interface (Name + Detect)
3. Register in `init()`: `components.Register(&Detector{})`
4. Add blank import in `scanner.go`: `_ "github.com/petrarca/.../components/{name}"`
5. Create parser in `parsers/` if needed (keep parsing separate from detection)
6. Write tests in `detector_test.go` with mock provider and dependency detector
7. Run `task fct` to verify

## Testing Strategy

Each detector should have tests covering:
- **Happy path**: Correct files trigger detection with expected output
- **Parsing**: Dependencies and metadata extracted correctly
- **Dependency matching**: Technologies detected from dependencies
- **Missing files**: Returns nil when detection files are absent
- **Malformed input**: Handles invalid file content gracefully
- **File read errors**: Provider returning errors
- **Priority logic**: When multiple files exist (e.g., Python: pyproject.toml > requirements.txt > setup.py)
- **Relative path handling**: Correct path computation for different directory depths
