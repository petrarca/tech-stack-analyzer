# SPDX 2.1 Expression Enhancement

## Status

**Phase 1 and Phase 3 complete.** All priority stacks have manifest-based license detection with SPDX expression support. Shared helper eliminates code duplication across detectors.

### What exists

- `Normalizer` with ~35 SPDX license string mappings (`internal/license/spdx_normalizer.go`)
- `ParseLicenseExpression()` splits on OR/AND operators, returns normalized components
- `ProcessLicenseExpression()` shared helper for all detectors (`internal/license/license_helper.go`)
- `IsSPDXValid()` validates whether a string is a known SPDX identifier
- `ParseTOMLLicense()` handles TOML-specific license formats (Python)
- File-based license detection via `go-license-detector` (`internal/license/license_detector.go`)

### Detector coverage (12 of 15)

| Detector | Manifest license | Source field | Expression support |
|----------|-----------------|--------------|-------------------|
| Node.js | Yes | `package.json` -> `license` | Full |
| PHP | Yes | `composer.json` -> `license` | Full |
| Rust | Yes | `Cargo.toml` -> `license` | Full |
| Python | Yes | `pyproject.toml` -> `license` (PEP 639) | Full |
| C++ | Yes | `conanfile.py` -> `license` | Full |
| .NET | Yes | `.csproj` -> `PackageLicenseExpression`, fallback `PackageLicenseUrl` | Full |
| Java | Yes | `pom.xml` -> `<licenses>/<license>/<name>` | Full |
| Ruby | Yes | `.gemspec` -> `license`/`licenses` | Full |
| Deno | Yes | `deno.json` -> `license` | Full |
| CocoaPods | Yes | `.podspec` -> `license` | Full |
| Go | No | `go.mod` has no license field (file-based only) | N/A |
| Gradle | No | No standard field in `build.gradle` (file-based only) | N/A |
| Docker | No | No standard license field | N/A |
| Terraform | No | No standard license field | N/A |
| GitHub Actions | No | No standard license field | N/A |

All 15 detectors benefit from file-based license detection (LICENSE/COPYING files).

### What is missing

- `IsSPDXValid()` is never called from any detector
- No `WITH` exception operator support (`"Apache-2.0 WITH LLVM-exception"`)
- No parenthesized/mixed operator expressions (`"(MIT OR BSD-2-Clause) AND Apache-2.0"`)
- Operator type (OR vs AND) is lost in return value -- callers cannot distinguish choice from requirement
- Enhanced License struct fields not yet implemented

## Problem Statement

Current license handling supports single SPDX identifiers and simple compound expressions (OR/AND) but lacks support for complex expressions with exceptions or parenthesized grouping. This limits accurate representation of real-world licensing and integration with compliance tools.

## Current License Structure

```go
type License struct {
    LicenseName     string  `json:"license_name"`               // SPDX identifier
    DetectionType   string  `json:"detection_type"`             // Detection method
    SourceFile      string  `json:"source_file"`                // Where detected
    Confidence      float64 `json:"confidence"`                 // Detection confidence
    OriginalLicense string  `json:"original_license,omitempty"` // Raw license
}
```

## Proposed Enhancement

### Enhanced License Structure

```go
type License struct {
    // Existing fields (preserved for backward compatibility)
    LicenseName     string  `json:"license_name"`
    DetectionType   string  `json:"detection_type"`
    SourceFile      string  `json:"source_file"`
    Confidence      float64 `json:"confidence"`
    OriginalLicense string  `json:"original_license,omitempty"`

    // New SPDX 2.1 expression support
    SPDXExpression  string   `json:"spdx_expression,omitempty"`  // Full SPDX expression
    SPDXComponents  []string `json:"spdx_components,omitempty"`  // Individual licenses
    IsSPDX          bool     `json:"is_spdx"`                    // Mappable to SPDX
    IsNonStandard   bool     `json:"is_non_standard"`            // Unmappable license
}
```

### Supported SPDX Features

**Operators:**
- **AND**: All licenses apply (combined requirements)
- **OR**: Choice of licenses (dual licensing)
- **WITH**: Exception handling (e.g., `"Apache-2.0 WITH LLVM-exception"`)

**Expression examples:**
```
"MIT"                        // Simple license
"MIT OR Apache-2.0"          // Dual licensing (choice)
"GPL-3.0-or-later"          // Version range
"MIT AND Apache-2.0"         // Combined requirements
```

## Implementation Plan

### Phase 1: Quick wins (no struct changes) -- DONE
- [x] Wire `ParseLicenseExpression` into Rust detector (Cargo.toml uses SPDX natively)
- [x] Wire `ParseLicenseExpression` into PHP detector (composer.json supports expressions)
- [x] Expand SPDX mapping table (added AGPL, EPL, CDDL, Artistic, Zlib, 0BSD, BSL, AFL, EUPL, PostgreSQL)
- [x] Wire `ParseLicenseExpression` into Python detector (PEP 639 compound expressions)
- [x] Add license detection to Java detector (pom.xml `<licenses>` element)
- [x] Add license detection to .NET detector (`PackageLicenseExpression` + `PackageLicenseUrl` fallback)
- [x] Add license detection to C++ detector (conanfile.py `license` attribute)
- [x] Add license detection to Ruby detector (.gemspec `license`/`licenses` fields)
- [x] Add license detection to Deno detector (deno.json `license` field)
- [x] Add license detection to CocoaPods detector (.podspec `license` field)
- [x] Extract shared `ProcessLicenseExpression()` helper (`internal/license/license_helper.go`)

### Phase 2: Enhanced struct and parser
- [ ] Extend License struct with `SPDXExpression`, `SPDXComponents`, `IsSPDX`, `IsNonStandard`
- [ ] Preserve operator type in `ParseLicenseExpression` return value
- [ ] Call `IsSPDXValid()` in detectors to populate `IsSPDX` flag
- [ ] Update JSON schema
- [ ] Add comprehensive unit tests

### Phase 3: Advanced expression support
- [ ] Support `WITH` exception operator
- [ ] Support parenthesized expressions
- [ ] Support mixed operators (`"MIT OR Apache-2.0 AND GPL-3.0"`)
- [ ] SPDX validation against official license list

## Design Principles

1. **Backward Compatibility**: Existing fields preserved, new fields use `omitempty`
2. **Incremental Adoption**: Each phase delivers value independently
3. **No output schema breaks**: New fields are additive only
