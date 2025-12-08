# Tech Stack Analyzer JSON Schemas

This directory contains JSON schemas for validating Tech Stack Analyzer output files.

## Files

- `stack-analyzer-output.json` - Main schema supporting both full hierarchical and aggregated output formats
- `examples/` - **Real output examples from our tech-stack-analyzer project**
  - `full.json` - Actual full hierarchical scan of our project (971 files, 8 technologies)
  - `aggregated.json` - Actual aggregated scan of our project showing Go, Git, CI/CD tools

## Schema Usage

### Generate Examples from Project
Use the Taskfile to regenerate examples from our actual project:
```bash
task build:examples
```
This creates real examples showing our project's actual tech stack and structure, with user-specific paths sanitized for privacy.

### Full Output
Complete scan result with component hierarchy:
```bash
stack-analyzer scan /path/to/project --output results.json
```

### Aggregated Output  
Flattened scan result with aggregated data:
```bash
stack-analyzer scan /path/to/project --aggregate all --output results.json
```

## Validation

Use any JSON Schema validator to validate output files:

```bash
# Using ajv (Node.js)
ajv validate -s schemas/stack-analyzer-output.json -d results.json

# Using python-jsonschema (Python)
python -m jsonschema -i results.json schemas/stack-analyzer-output.json
```

## Schema Structure

The schema uses `oneOf` to support two output formats:

- **Full Output** - Complete hierarchical structure with `childs`, `edges`, and component relationships
- **Aggregated Output** - Flattened structure with aggregated `tech`, `techs`, and `reason` arrays

Both formats share common definitions for:
- `metadata` - Scan execution information (matches ScanMetadata struct)
- `code_stats` - Code analysis metrics  
- `dependencies` - Dependency tuples
- `reason` - Detection reason mappings
- `properties` - Technology-specific properties (NEW!)

### Metadata Structure

The `metadata` field contains comprehensive scan execution information matching the Go `ScanMetadata` struct:

#### Core Required Fields
```json
"metadata": {
  "timestamp": "2025-12-04T14:47:23Z",
  "scan_path": "/absolute/path/to/scanned/project", 
  "specVersion": "1.0"
}
```

#### Optional Fields (omitempty in Go)
```json
"metadata": {
  "timestamp": "2025-12-04T14:47:23Z",
  "scan_path": "/absolute/path/to/scanned/project",
  "specVersion": "1.0",
  "duration_ms": 1250,
  "file_count": 150,
  "component_count": 12,
  "language_count": 3,
  "tech_count": 5,
  "techs_count": 18,
  "properties": {
    "custom_field": "custom_value"
  }
}
```

**Field Descriptions:**
- `timestamp` - ISO 8601/RFC3339 timestamp when scan started
- `scan_path` - Absolute path that was scanned
- `specVersion` - Output format specification version (e.g., "1.0")
- `duration_ms` - Total scan duration in milliseconds
- `file_count` - Number of files processed
- `component_count` - Number of components detected
- `language_count` - Number of distinct programming languages
- `tech_count` - Number of primary technologies (architectural)
- `techs_count` - Number of all detected technologies
- `properties` - Additional scan-level properties (not technology-specific)

### Technology-Specific Properties

*Note: This is different from `metadata.properties` - these are detailed analysis results for specific technologies.*

The `properties` field contains detailed analysis results for specific technologies:

#### Docker Properties
```json
"properties": {
  "docker": [
    {
      "file": "/Dockerfile",
      "base_images": ["alpine:latest"],
      "exposed_ports": [80, 443],
      "multi_stage": true,
      "stages": ["builder", "runtime"]
    }
  ]
}
```

#### Terraform Properties  
```json
"properties": {
  "terraform": [
    {
      "file": "/main.tf",
      "providers": ["aws", "google"],
      "resources_by_provider": {"aws": 5, "google": 2},
      "resources_by_category": {"compute": 3, "storage": 4},
      "total_resources": 7
    }
  ]
}
```

*Properties are optional and technology-specific. The schema validates known structures (Docker, Terraform) while allowing extensibility for new technologies.*

## External Tool Integration

The schema is designed for external tool integration:
- Clear field descriptions and validation rules
- Supports IDE autocompletion and validation
- Compatible with API documentation generators
- Ready for automated testing frameworks

Reference: `https://github.com/petrarca/tech-stack-analyzer/schemas/stack-analyzer-output.json`
