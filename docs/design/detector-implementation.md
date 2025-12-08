# Component Detector Design & Implementation Plan

## Overview
This document serves as the comprehensive design specification for component detectors in the tech stack analyzer. It outlines the architecture, implementation patterns, and requirements for 11 component detectors that achieve complete feature parity with the TypeScript implementation while adding enhancements for better ecosystem coverage.

## Architecture Summary
The component detector system follows a modular architecture where each detector is responsible for identifying specific project types, parsing their configuration files, and extracting dependency information. All detectors implement a common interface and are automatically registered through Go's init() system.

## Completed Detectors (13/13)

### Phase 1: Core Languages (High Priority)
1. **Node.js** - Completed (Real Components - package.json detection with npm/yarn package extraction)
2. **Python** - Completed (Real Components - pyproject.toml detection with pip package extraction)
3. **Go** - Completed (Real Components - go.mod and main.go detection with Go module extraction)
4. **Rust** - Completed (Real Components - Cargo.toml detection with crate dependency extraction)
5. **PHP** - Completed (Real Components - composer.json detection with Composer package extraction)

### Phase 2: Modern Runtimes (Medium Priority)
6. **Java/Kotlin** - Completed (Real Components - unified Maven/Gradle detection for both Java and Kotlin)
7. **Ruby** - Completed (Real Components - Gemfile detection with Ruby gem extraction)
8. **Deno** - Completed (Virtual Components - deno.lock detection with Deno package extraction)

### Phase 3: Infrastructure & DevOps Tools
9. **Docker** - Completed (Virtual Components with child components for each Docker service)
10. **Terraform** - Completed (HCL parsing with provider and resource detection for IaC)

### Phase 4: Enhanced Ecosystem Support
11. **.NET** - Completed (Unified detector for modern .NET and .NET Framework with NuGet package extraction)
12. **Oracle Database** - Completed (Comprehensive rule covering all major Oracle drivers and configurations)
13. **Delphi** - Completed (Component detector for .dproj files with VCL/FMX framework detection and package extraction)

### Phase 5: Extension-Based Detection (No Component Detectors Needed)
14. **Zig** - Completed (Handled by extension matcher - .zig files)
15. **C/C++** - Completed (Handled by extension matchers - .c/.cpp/.h/.hpp files)
16. **Other Languages** - Completed (Comprehensive extension-based language detection including AWK, XSLT, Groovy)

---

## 1. Node.js Detector

### Files to Detect
- `package.json` (component - creates named payload)
- `package-lock.json` or `yarn.lock` (optional - for version info)

### Implementation Requirements

#### package.json Detection (Named Component)
- **File**: `package.json`
- **Parsing Logic**:
  - Parse JSON structure using Go's encoding/json
  - Extract `name` field for component naming
  - Extract `dependencies` object for production dependencies
  - Extract `devDependencies` for development dependencies (optional)
- **Dependencies**:
  - Store as: `npm` type with package name and version
  - Match against dependency rules for tech detection
- **Output**: Real Component (named payload)
- **Component Tech**: `"nodejs"`

### TypeScript Reference
- `stack-analyser/src/rules/spec/nodejs/component.ts`

---

## 2. Python Detector

### Files to Detect
- `pyproject.toml` (component - creates named payload)
- `requirements.txt` (optional - for additional dependencies)
- `Pipfile` (optional - Pipenv support)

### Implementation Requirements

#### pyproject.toml Detection (Named Component)
- **File**: `pyproject.toml`
- **Parsing Logic**:
  - Parse TOML format using TOML parser
  - Extract `project.name` for component naming
  - Extract `project.dependencies` array for package list
  - Extract `tool.poetry.dependencies` for Poetry projects
- **Dependencies**:
  - Store as: `pypi` type with package name and version
  - Match against dependency rules for tech detection
- **Output**: Real Component (named payload)
- **Component Tech**: `"python"`

### TypeScript Reference
- `stack-analyser/src/rules/spec/python/lockfile.ts`

---

## 3. Go Detector

### Files to Detect
- `go.mod` (lockfile - creates virtual payload)
- `main.go` (component - creates named payload)

### Implementation Requirements

#### go.mod Detection (Virtual Payload)
- **File**: `go.mod`
- **Parsing Logic**:
  - Parse lines matching pattern: `\t<package-url> v<version>`
  - Split line by spaces after removing first character (tab)
  - Extract: `[url, version, comment, ...rest]`
  - Skip if `rest.length > 0` or `comment` exists (indirect dependencies)
  - Extract package URL and version
- **Dependencies**:
  - Store as: `golang` type with package URL and version
  - Match against dependency rules for tech detection
- **Output**: Real Component (implemented as named component for consistency)

#### main.go Detection (Named Payload)
- **File**: `main.go`
- **Component Name**: Folder name containing main.go
- **Tech Field**: `"golang"`
- **Output**: Real Component (implemented as named component)

### TypeScript Reference
- `stack-analyser/src/rules/spec/golang/lockfile.ts`
- `stack-analyser/src/rules/spec/golang/component.ts`

---

## 4. PHP Detector

### Files to Detect
- `composer.json` (component - creates named payload)
- `composer.lock` (optional - for version info)

### Implementation Requirements

#### composer.json Detection
- **File**: `composer.json`
- **Parsing Logic**:
  - Parse JSON structure
  - Extract `name` field for component name
  - Extract `license` field
  - Parse `require` and `require-dev` dependencies
- **Dependencies**:
  - Store as: `php` type with package name and version
  - Match against dependency rules for tech detection
- **Additional Techs**:
  - Always add `phpcomposer` tech with reason: "matched file: composer.json"
- **License Detection**: Extract and store license information
- **Output**: Completed - Named component with dependencies and techs

### JSON Structure
```json
{
  "name": "vendor/package",
  "license": "MIT",
  "require": {
    "php": "^8.0",
    "vendor/package": "^1.0"
  },
  "require-dev": {
    "phpunit/phpunit": "^9.0"
  }
}
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/php/component.ts`

---

## 5. Ruby Detector

### Files to Detect
- `Gemfile` (lockfile - creates virtual payload)

### Implementation Requirements

#### Gemfile Detection
- **File**: `Gemfile`
- **Parsing Logic**:
  - Parse gem declarations
  - Pattern: `gem "<gem-name>", "<version>"`
  - Regex: `/gem "(.+)",\s+("(.+)")?/`
  - Extract gem names and versions (version is optional)
- **Dependencies**:
  - Store as: `ruby` type with gem name and version (or 'latest')
  - Match against dependency rules for tech detection
- **Additional Techs**:
  - Always add `bundler` tech with reason: "matched file: Gemfile"
- **Output**: Real Component (implemented as named component for consistency with Python)

### Gemfile Format
```ruby
gem "rails", "~> 7.0.0"
gem "pg"
gem "puma", "~> 5.0"
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/ruby/lockfile.ts`

---

## 6. Rust Detector

### Files to Detect
- `Cargo.toml` (component - creates named payload)

### Implementation Requirements

#### Cargo.toml Detection
- **File**: `Cargo.toml`
- **Parsing Logic**:
  - Parse TOML format
  - Extract `[package]` section:
    - `name` field for component name
    - `license` field
  - Parse dependency sections:
    - `[dependencies]`
    - `[dev-dependencies]`
    - `[build-dependencies]`
    - `[workspace.dependencies]`
- **Dependencies**:
  - Store as: `rust` type with crate name
  - Handle different dependency formats:
    - Simple string: `serde = "1.0"` → store as `"1.0"`
    - Path: `{ path = "..." }` → store as `"path:..."`
    - Git: `{ git = "...", branch = "..." }` → store as `"git:...#branch"`
    - Object with version: `{ version = "1.0" }` → store as `"1.0"`
  - Match against dependency rules for tech detection
- **Additional Techs**:
  - Always add `cargo` tech with reason: "matched file: Cargo.toml"
- **License Detection**: Extract and store license information from `[package]` section
- **Output**: Completed - Named component if `[package]` section exists, virtual payload for workspace

### TOML Structure
```toml
[package]
name = "my-project"
version = "0.1.0"
license = "MIT"

[dependencies]
serde = "1.0"
tokio = { version = "1.0", features = ["full"] }

[dev-dependencies]
criterion = "0.3"
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/rust/component.ts`

---

## 7. Deno Detector

### Files to Detect
- `deno.lock` (lockfile - creates virtual payload)

### Implementation Requirements

#### deno.lock Detection
- **File**: `deno.lock`
- **Parsing Logic**:
  - Parse JSON structure
  - Check for `version` field (must exist)
  - Extract `remote` field (object of URL → hash mappings)
  - Extract dependency URLs from remote keys
- **Dependencies**:
  - Store as: `deno` type with module URL and hash
  - Match against dependency rules for tech detection
- **Output**: Completed - Virtual payload with dependencies and matched techs

### JSON Structure
```json
{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.140.0/path/mod.ts": "abc123...",
    "https://deno.land/x/oak@v10.5.1/mod.ts": "def456..."
  }
}
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/deno/lockfile.ts`

---

## 8. Docker Detector

### Files to Detect
- `docker-compose.yml` or `docker-compose.yaml` (component - creates virtual payload)
- `Dockerfile` (detection via file matcher, not component detector)

### Implementation Requirements

#### docker-compose.yml Detection
- **File**: `docker-compose.yml` or `docker-compose.yaml` (regex: `/^docker-compose(.*)?\.y(a)?ml$/`)
- **Parsing Logic**:
  - Parse YAML structure
  - Extract `services` section
  - For each service:
    - Extract `image` field
    - Parse image name and tag: `<image>:<tag>`
    - Skip images starting with `$` (environment variables)
    - Match image name against dependency rules
    - Create child component for each matched service
- **Child Components**:
  - Name: `container_name` field or service key
  - Tech: Matched tech from dependency rules (or null)
  - Dependencies: `['docker', imageName, imageVersion || 'latest']`
  - Reason: Matched tech reasons or `"matched: <imageName>"`
- **Output**: Completed - Virtual payload with child components for each service

### YAML Structure
```yaml
version: '3'
services:
  web:
    image: nginx:latest
  db:
    image: postgres:14
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/docker/component.ts`

---

## 9. Terraform Detector

### Files to Detect
- `*.tf` files (resource detection)
- `terraform.lock.hcl` (lockfile - creates virtual payload)

### Implementation Requirements

#### terraform.lock.hcl Detection
- **File**: `.terraform.lock.hcl`
- **Parsing Logic**:
  - Parse HCL format using HCL parser
  - Extract `provider` blocks
  - Get provider names and versions from provider blocks
  - Match provider names against dependency rules
  - Create child component for each matched provider
- **Child Components**:
  - Name: Tech name from rules (e.g., "AWS", "Google Cloud")
  - Tech: Matched tech from dependency rules
  - Dependencies: `['terraform', providerName, version || 'latest']`
  - Reason: Matched tech reasons
- **Output**: Completed - Virtual payload with dependencies and child components

#### *.tf Resource Detection
- **Files**: Any `*.tf` file (skip files > 500KB)
- **Parsing Logic**:
  - Parse HCL format using HCL parser
  - Extract `resource` blocks
  - Get resource types as keys (e.g., `aws_instance`, `google_compute_instance`)
  - Match resource types against dependency rules (type: `terraform.resource`)
  - Create child component for each matched resource type
- **Child Components**:
  - Name: Tech name from rules
  - Tech: Matched tech from dependency rules
  - Dependencies: `['terraform.resource', resourceType, 'unknown']`
  - Reason: Matched tech reasons
- **Output**: Completed - Array of virtual payloads (one per .tf file) with child components
- **Note**: Returns array of payloads, not single payload

### HCL Structure (terraform.lock.hcl)
```hcl
provider "registry.terraform.io/hashicorp/aws" {
  version = "4.0.0"
}
```

### HCL Structure (*.tf)
```hcl
resource "aws_instance" "example" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}
```

### TypeScript Reference
- `stack-analyser/src/rules/spec/terraform/lockfile.ts`
- `stack-analyser/src/rules/spec/terraform/resource.ts`

---

## 10. .NET Detector

### Files to Detect
- `*.csproj` (project file - creates named payload)
- `*.sln` (solution file - optional enhancement)
- `global.json` (SDK configuration - optional enhancement)

### Implementation Requirements

#### *.csproj Project Detection
- **File**: Any `*.csproj` file (covers both modern .NET and legacy .NET Framework)
- **Parsing Logic**:
  - Parse XML format using Go's encoding/xml
  - Extract `TargetFramework` element (e.g., net8.0, net6.0, net48, net472)
  - Extract `PackageReference` elements with `Include` and `Version` attributes
  - Extract `ProjectReference` elements for project dependencies
  - Extract project name from file or `AssemblyName` property
- **Framework Detection**:
  - Modern .NET: net6.0, net7.0, net8.0, net9.0 (cross-platform)
  - Legacy .NET Framework: net462, net472, net48 ( Windows-only)
  - Store framework as metadata in component
- **Dependencies**:
  - Store as: `nuget` type with package name and version
  - Match against dependency rules for tech detection
- **Output**: Completed - Real Component (named payload)
- **Component Tech**: `"dotnet"` (unified for all .NET projects)
- **Child Components**: None needed (unlike Docker/Terraform)

#### *.sln Solution Detection (Optional)
- **File**: `*.sln` solution files
- **Parsing Logic**:
  - Parse solution format to extract project references
  - Identify all .csproj files in the solution
  - Create virtual payload with child components for each project
- **Output**: Virtual payload with child components

#### global.json SDK Detection (Optional)
- **File**: `global.json`
- **Parsing Logic**:
  - Parse JSON to extract SDK version
  - Add metadata to project detection
- **Output**: Metadata enhancement

### Detection Examples
- **Modern .NET Project**: `MyApp.csproj` (net8.0) → Component "MyApp" with NuGet dependencies
- **Legacy .NET Framework**: `MyLegacyApp.csproj` (net48) → Component "MyLegacyApp" with NuGet dependencies
- **Web Application**: `MyWebApp.csproj` (net7.0) → Component "MyWebApp" with ASP.NET packages
- **Class Library**: `MyLibrary.csproj` (net6.0) → Component "MyLibrary" with library dependencies
- **Solution**: `MySolution.sln` → Virtual payload with multiple project components

### .csproj Structure Examples

#### Modern .NET (SDK-style)
```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <AssemblyName>MyApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.3" />
    <PackageReference Include="Microsoft.Extensions.Hosting" Version="8.0.0" />
  </ItemGroup>
</Project>
```

#### Legacy .NET Framework
```xml
<Project ToolsVersion="15.0" DefaultTargets="Build">
  <PropertyGroup>
    <TargetFramework>net48</TargetFramework>
    <AssemblyName>MyLegacyApp</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.3" />
    <Reference Include="System.Web" />
  </ItemGroup>
</Project>
```

### TypeScript Reference
- None (not implemented in TypeScript)
- This is an **enhancement** for better .NET ecosystem support

---

## 11. Delphi Detector

### Files to Detect
- `*.dproj` (project file - creates named payload)
- `*.dpr` (program source - optional)
- `*.dpk` (package source - optional)

### Implementation Requirements

#### *.dproj Project Detection
- **File**: Any `*.dproj` file (modern Delphi project format, XML-based)
- **Parsing Logic**:
  - Parse XML format using regex (simpler than full XML parsing)
  - Extract `<FrameworkType>` element (VCL or FMX)
  - Extract `<DCC_UsePackage>` element for runtime packages
  - Extract project name from filename
- **Framework Detection**:
  - **VCL**: Visual Component Library (Windows desktop)
  - **FMX**: FireMonkey (cross-platform: Windows, macOS, iOS, Android)
  - Store framework as additional tech in component
- **Dependencies**:
  - Store as: `delphi` type with package name
  - Packages are semicolon-separated in DCC_UsePackage
  - Skip variables like `$(DCC_UsePackage)`
  - Deduplicate packages across multiple DCC_UsePackage elements
- **Output**: Real Component (named payload)
- **Component Tech**: `"delphi"` with framework as additional tech (`"vcl"` or `"fmx"`)

### .dproj Structure Example
```xml
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
    <PropertyGroup>
        <ProjectGuid>{B5EBA39A-276F-40EA-8594-AED10F883351}</ProjectGuid>
        <MainSource>MyApp.dpr</MainSource>
        <FrameworkType>VCL</FrameworkType>
    </PropertyGroup>
    <PropertyGroup Condition="'$(Base_Win32)'!=''">
        <DCC_UsePackage>vcl;rtl;dbrtl;vcldb;FireDAC;$(DCC_UsePackage)</DCC_UsePackage>
    </PropertyGroup>
</Project>
```

### Key Elements
- **FrameworkType**: VCL, FMX, or None
- **DCC_UsePackage**: Semicolon-separated list of runtime packages
- **MainSource**: Reference to .dpr file

### Common Delphi Packages
- **Built-in**: rtl, vcl, vcldb, fmx, FireDAC, dbrtl, inet, xmlrtl
- **DevExpress**: dx*, cx* prefixes (e.g., dxCoreRS28, cxGridRS28)
- **TMS**: TMS* prefix
- **FastReport**: frx*, fs* prefixes
- **FlexCel**: FlexCel_* prefix
- **JVCL/JCL**: Jv*, Jcl* prefixes
- **Indy**: Indy* prefix

### TypeScript Reference
- None (not implemented in TypeScript)
- This is an **enhancement** for Delphi/Pascal ecosystem support

---

## 12. Java/Kotlin Detector

### Files to Detect
- `pom.xml` (Maven - component - creates named payload)
- `build.gradle` or `build.gradle.kts` (Gradle - component - creates named payload)

### Implementation Requirements

**NOTE**: TypeScript implementation does NOT have Java component detection (only extension matching). However, for consistency with other languages (Python, Go, Rust, PHP, Ruby), we should add it.

**IMPORTANT**: This detector handles BOTH Java and Kotlin projects since they use the same build tools:
- Java files: `.java` (detected by extension matcher)
- Kotlin files: `.kt`, `.kts` (detected by extension matcher)
- Both use Maven (`pom.xml`) or Gradle (`build.gradle`)
- No separate Kotlin detector needed

#### Maven (pom.xml) Detection
- **File**: `pom.xml`
- **Parsing Logic**:
  - Parse XML structure
  - Extract `<project>` root element
  - Extract `<groupId>` and `<artifactId>` for component name
  - Extract `<dependencies>` section
  - Parse each `<dependency>` with `<groupId>:<artifactId>`
- **Component Name**: `groupId:artifactId` or just `artifactId`
- **Dependencies**:
  - Store as: `maven` type with `groupId:artifactId` format
  - Match against dependency rules for tech detection
- **Additional Techs**:
  - Always add `maven` tech with reason: "matched file: pom.xml"
- **Output**: Real Component with dependencies and techs

#### Gradle (build.gradle) Detection
- **File**: `build.gradle` or `build.gradle.kts` (Kotlin DSL)
- **Parsing Logic**:
  - Parse Groovy/Kotlin DSL (text-based, not full parsing)
  - Look for `rootProject.name` or `project.name` for component name
  - Extract dependencies from `dependencies {}` block
  - Pattern: `implementation 'group:artifact:version'` or `implementation("group:artifact:version")`
- **Component Name**: Project name from settings or folder name
- **Dependencies**:
  - Store as: `gradle` type with `group:artifact` format
  - Match against dependency rules for tech detection
- **Additional Techs**:
  - Always add `gradle` tech with reason: "matched file: build.gradle"
- **Output**: Real Component with dependencies and techs

### XML Structure (pom.xml)
```xml
<project>
  <groupId>com.example</groupId>
  <artifactId>my-app</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
      <version>3.0.0</version>
    </dependency>
  </dependencies>
</project>
```

### Groovy Structure (build.gradle)
```groovy
plugins {
    id 'java'
}

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web:3.0.0'
    testImplementation 'junit:junit:4.13.2'
}
```

### TypeScript Reference
- None (not implemented in TypeScript)
- This is an **enhancement** for better Java ecosystem support

---

## 13. Zig Detector

### Files to Detect
- `.zig` extension files (via extension matcher, not component detector)

### Implementation Requirements

**NO COMPONENT DETECTOR NEEDED**

Zig detection is handled entirely through:
1. **Extension matcher**: `.zig` files
2. **Dependency matcher**: GitHub Action `goto-bus-stop/setup-zig`

The TypeScript implementation only has a rule registration, no component detector:
```typescript
register({
  tech: 'zig',
  name: 'Zig',
  type: 'language',
  extensions: ['.zig'],
  dependencies: [{ type: 'githubAction', name: 'goto-bus-stop/setup-zig' }],
});
```

**No implementation needed** - already handled by existing matchers.

### TypeScript Reference
- `stack-analyser/src/rules/spec/zig/index.ts` (rule only, no detector)

---

## Implementation Order (Priority)

### Phase 1: Core Languages (High Priority)
1. **Node.js** - Completed (Real Components - package.json detection with npm/yarn package extraction)
2. **Python** - Completed (Real Components - pyproject.toml detection with pip package extraction)
3. **Go** - Completed (Real Components - named payloads)
4. **Rust** - Completed (Real Components - Cargo.toml detection)
5. **PHP** - Completed (Real Components - composer.json detection)

### Phase 2: Modern Runtimes (Medium Priority)
6. **Java/Kotlin** - Completed (Real Components - unified Java/Kotlin detector)
7. **Ruby** - Completed (Real Components - named payloads for consistency with Python)
8. **Deno** - Completed (Virtual Components - deno.lock detection)

### Phase 3: Infrastructure (DevOps Tools)
9. **Docker** - Completed (Virtual Components with child components for each service)
10. **Terraform** - Completed (HCL parsing with provider and resource detection)

### Phase 4: Additional Enhancements
11. **.NET** - Completed (Unified detector for modern .NET and .NET Framework with NuGet package extraction)
12. **Oracle Database** - Completed (Comprehensive rule covering all major Oracle drivers and configurations)
13. **Delphi** - Completed (Component detector for .dproj files with VCL/FMX framework and package extraction)

### Phase 5: No Implementation Needed
14. **Zig** - Already handled by extension matcher

**Total implemented: 13 detectors** (9 from TypeScript + 4 enhancements)

---

## Architectural Decision: Real Components vs Virtual Payloads

### Original TypeScript Pattern
- **Go**: Virtual payloads from go.mod, named components from main.go
- **Python**: Virtual payloads from pyproject.toml  
- **Ruby**: Virtual payloads from Gemfile

### Go Implementation Decision
For **consistency across all language detectors**, the Go implementation uses **real components ( named payloads)**:
- **Go**: Named components for both go.mod and main.go
- **Python**: Named components for pyproject.toml (existing)
- **Ruby**: Named components for Gemfile (updated)

### Benefits of This Approach
1. **Consistency**: All language detectors follow the same pattern
2. **Multi-project Support**: Each project file becomes a separate component
3. **Better Organization**: Clear hierarchy with parent-child relationships
4. **Dependency Isolation**: Dependencies are scoped to individual components
5. **Tech Detection**: More accurate tech detection per component

---

## Common Patterns

### Virtual Payloads
- Used for lockfiles that don't define a project name
- Name is always `"virtual"`
- Dependencies and techs are extracted
- Children are merged into parent

### Named Payloads
- Used for project definition files (composer.json, Cargo.toml, etc.)
- Name extracted from file content
- Tech field set to language identifier
- Becomes a separate component in output

### Dependency Matching
All detectors follow this pattern:
1. Parse file to extract dependency names
2. Store dependencies in payload
3. Call `depDetector.MatchDependencies(depNames, type)`
4. Add matched techs to payload with reasons

### License Detection
Some detectors (PHP, Rust) extract license information:
- Look for `license` field in config files
- Store in payload for output

---

## Required Go Libraries

### JSON Parsing
- Standard library: `encoding/json`
- Usage: PHP composer.json, Deno deno.lock

### XML Parsing
- Standard library: `encoding/xml`
- Usage: Java Maven pom.xml

### YAML Parsing
- Library: `gopkg.in/yaml.v3`
- Usage: Docker Compose, GitHub Actions

### TOML Parsing
- Library: `github.com/BurntSushi/toml`
- Usage: Rust Cargo.toml, Python pyproject.toml (already used)

### HCL Parsing
- Library: `github.com/hashicorp/hcl/v2`
- Usage: Terraform files

---

## Testing Strategy

### Unit Tests
Each detector should have tests for:
1. File detection (correct files trigger detector)
2. Parsing logic (extract correct data)
3. Dependency matching (techs are detected)
4. Edge cases (malformed files, missing fields)

### Integration Tests
Test with real project examples:
- Go: A project with go.mod and main.go
- Rust: A project with Cargo.toml
- PHP: A project with composer.json
- Ruby: A project with Gemfile.lock
- Deno: A project with deno.json
- Docker: A project with docker-compose.yml
- Terraform: A project with .tf files
- Zig: A project with build.zig
- Delphi: A project with .dproj files (VCL or FMX)

---

## Next Steps

1. Review and approve this design document
2. Implement detectors in priority order
3. Add required dependencies to go.mod
4. Write unit tests for each detector
5. Test with real-world projects
6. Update documentation
7. Update scanner.go with new imports
