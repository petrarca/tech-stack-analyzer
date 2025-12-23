# SPDX 2.1 Expression Enhancement Design Document

## Overview

This document outlines the design for enhancing the Tech Stack Analyzer's license handling to support SPDX 2.1 expressions, bringing our capabilities in line with Google's deps.dev while maintaining our unique advantages in project-level analysis.

## Problem Statement

Our current license handling supports single SPDX identifiers but lacks support for complex license expressions like "MIT OR Apache-2.0" or "GPL-3.0-or-later". This limits our ability to accurately represent real-world licensing scenarios and integrate with compliance tools that expect SPDX 2.1 expressions.

## Current State Analysis

### Our Current License Structure
```go
type License struct {
    LicenseName     string  `json:"license_name"`               // SPDX identifier
    DetectionType   string  `json:"detection_type"`             // Detection method
    SourceFile      string  `json:"source_file"`                // Where detected
    Confidence      float64 `json:"confidence"`                 // Detection confidence
    OriginalLicense string  `json:"original_license,omitempty"` // Raw license
}
```

### deps.dev's Approach
```go
type Version_License struct {
    License string `json:"license,omitempty"`  // Raw license from metadata
    Spdx    string `json:"spdx,omitempty"`    // SPDX 2.1 expression
}
```

### Key Differences
- **Our Strengths**: Rich detection context, source tracking, confidence scoring
- **deps.dev Strengths**: Full SPDX 2.1 expression support, industry standard compliance
- **Gap**: We don't support compound license expressions or SPDX ranges

## Design Goals

1. **Backward Compatibility**: Existing functionality must continue working
2. **SPDX 2.1 Compliance**: Support standard license expressions and operators
3. **Rich Context Preservation**: Maintain our detection metadata advantages
4. **Incremental Adoption**: Allow gradual migration to new features
5. **Industry Interoperability**: Work with compliance tools and SPDX ecosystem

## Proposed Solution

### Enhanced License Structure

```go
type License struct {
    // Existing fields (preserved for backward compatibility)
    LicenseName     string  `json:"license_name"`               // SPDX identifier
    DetectionType   string  `json:"detection_type"`             // Detection method
    SourceFile      string  `json:"source_file"`                // Where detected
    Confidence      float64 `json:"confidence"`                 // Detection confidence
    OriginalLicense string  `json:"original_license,omitempty"` // Raw license
    
    // New SPDX 2.1 expression support
    SPDXExpression  string   `json:"spdx_expression,omitempty"`  // Full SPDX expression
    SPDXComponents  []string `json:"spdx_components,omitempty"`  // Individual licenses
    IsSPDX          bool     `json:"is_spdx"`                    // Mappable to SPDX
    IsNonStandard   bool     `json:"is_non_standard"`            // Unmappable license
}
```

### SPDX Expression Parser

```go
type SPDXExpressionParser struct {
    normalizer *Normalizer
}

type ParsedLicense struct {
    Expression    string   // "MIT OR Apache-2.0"
    Components    []string // ["MIT", "Apache-2.0"]
    Operators     []string // ["OR"]
    IsSPDX        bool     // true if all components are SPDX
    IsNonStandard bool     // true if any component is unmappable
}
```

## Implementation Plan

### Phase 1: Core Infrastructure
1. **Extend License struct** with new SPDX fields
2. **Create SPDXExpressionParser** with expression parsing logic
3. **Enhance spdx_normalizer.go** to handle compound expressions
4. **Add comprehensive unit tests** for parsing logic

### Phase 2: Parser Integration
1. **Update Ruby parser** to use new SPDX expression handling
2. **Enhance other parsers** (npm, Maven, Python) progressively
3. **Update JSON output format** to include new fields
4. **Add integration tests** for end-to-end functionality

### Phase 3: Advanced Features
1. **SPDX validation** against official license list
2. **Expression simplification** (e.g., "MIT AND MIT" → "MIT")
3. **License compatibility checking** (future enhancement)
4. **Compliance tool integration** examples

## Technical Specifications

### Supported SPDX Features

#### Operators
- **AND**: All licenses apply (dual licensing)
- **OR**: Choice of licenses (permissive licensing)
- **WITH**: Exception handling (future enhancement)

#### License Identifiers
- **Standard SPDX**: MIT, Apache-2.0, GPL-3.0, etc.
- **Later versions**: GPL-3.0-or-later, BSD-3-Clause-Clear
- **Non-standard**: Custom license detection with "non-standard" flag

#### Expression Examples
```go
"MIT"                                    // Simple license
"MIT OR Apache-2.0"                      // Dual licensing
"GPL-3.0-or-later"                      // Version range
"MIT AND Apache-2.0"                     // Combined requirements
"CustomLicense"                          // Non-standard (mapped to "non-standard")
```

### Parsing Logic

```go
func (p *SPDXExpressionParser) Parse(expression string) ParsedLicense {
    // 1. Clean and normalize input
    // 2. Split by operators (AND/OR)
    // 3. Normalize each component using existing mappings
    // 4. Validate SPDX identifiers
    // 5. Set flags for SPDX/non-standard detection
    // 6. Return structured result
}
```

### Error Handling

- **Malformed expressions**: Return empty components, set IsSPDX=false
- **Unknown licenses**: Mark as IsNonStandard=true, preserve original
- **Invalid syntax**: Log warning, return simple normalized result
- **Empty input**: Return empty ParsedLicense

## Migration Strategy

### Backward Compatibility

1. **Existing fields preserved**: All current License fields remain unchanged
2. **Optional new fields**: New SPDX fields are optional (`omitempty`)
3. **Graceful degradation**: Old parsers continue working with basic fields
4. **Progressive enhancement**: New features activated when available

### Data Migration

```go
// Migration helper for existing licenses
func (l *License) EnhanceWithSPDX() {
    if l.SPDXExpression == "" && l.LicenseName != "" {
        l.SPDXExpression = l.LicenseName
        l.SPDXComponents = []string{l.LicenseName}
        l.IsSPDX = true
    }
}
```

## Testing Strategy

### Unit Tests
- **Expression parsing**: All operator combinations and edge cases
- **License normalization**: Verify SPDX mapping accuracy
- **Error handling**: Malformed input and unknown licenses
- **Backward compatibility**: Existing functionality unchanged

### Integration Tests
- **Ruby parser**: Gemfile license detection with expressions
- **JSON output**: Verify new fields in scan results
- **Real projects**: Test against complex licensing scenarios
- **Compatibility**: Ensure old output format still valid

### Performance Tests
- **Large projects**: License parsing performance impact
- **Memory usage**: Additional field overhead
- **Parsing speed**: Complex expression evaluation

## Output Format Examples

### Simple License (Current)
```json
{
  "license_name": "MIT",
  "detection_type": "normalized",
  "source_file": "package.json",
  "confidence": 0.95,
  "original_license": "MIT"
}
```

### Enhanced License (New)
```json
{
  "license_name": "MIT",
  "detection_type": "normalized", 
  "source_file": "package.json",
  "confidence": 0.95,
  "original_license": "MIT OR Apache-2.0",
  "spdx_expression": "MIT OR Apache-2.0",
  "spdx_components": ["MIT", "Apache-2.0"],
  "is_spdx": true,
  "is_non_standard": false
}
```

### Non-Standard License
```json
{
  "license_name": "Proprietary",
  "detection_type": "direct",
  "source_file": "LICENSE.txt",
  "confidence": 0.80,
  "original_license": "Custom License 1.0",
  "spdx_expression": "non-standard",
  "spdx_components": ["Custom License 1.0"],
  "is_spdx": false,
  "is_non_standard": true
}
```

## Benefits and Impact

### Immediate Benefits
- **Industry Compliance**: Align with SPDX 2.1 standard
- **Tool Integration**: Work with compliance and security tools
- **Accuracy**: Better representation of complex licensing
- **Future-Proof**: Foundation for advanced license analysis

### Long-term Impact
- **Compliance Workflows**: Enable automated license compliance
- **Security Integration**: Combine with vulnerability scanning
- **Legal Analysis**: Foundation for license compatibility checking
- **Ecosystem Alignment**: Match industry standard capabilities

## Risks and Mitigations

### Technical Risks
- **Performance Impact**: Parsing overhead for large projects
  - *Mitigation*: Lazy evaluation, caching, optimized parsing
- **Backward Compatibility**: Breaking changes to existing integrations
  - *Mitigation*: Optional fields, extensive testing, gradual rollout

### Business Risks
- **Complexity**: Increased system complexity
  - *Mitigation*: Clear documentation, phased implementation
- **Maintenance**: Additional code to maintain
  - *Mitigation*: Comprehensive tests, reusable components

## Success Metrics

### Technical Metrics
- **Test Coverage**: >95% for new SPDX functionality
- **Performance**: <5% impact on scan performance
- **Compatibility**: 100% backward compatibility maintained

### Functional Metrics
- **Expression Support**: Handle all common SPDX 2.1 expressions
- **Accuracy**: >99% correct SPDX mapping for known licenses
- **Integration**: Work with 3+ compliance tools

## Timeline

### Phase 1 (Week 1-2): Core Infrastructure
- [ ] Extend License struct
- [ ] Implement SPDXExpressionParser
- [ ] Add comprehensive unit tests
- [ ] Update spdx_normalizer.go

### Phase 2 (Week 3-4): Parser Integration
- [ ] Update Ruby parser
- [ ] Enhance other parsers
- [ ] Update JSON output format
- [ ] Add integration tests

### Phase 3 (Week 5-6): Advanced Features & Polish
- [ ] SPDX validation
- [ ] Expression simplification
- [ ] Documentation updates
- [ ] Performance optimization

## Knowledge Graph Integration

### Current Knowledge Graph Model

Our existing knowledge graph uses a simple `HAS_LICENSE` relationship between projects and license nodes:

```
Project --HAS_LICENSE--> License Node
```

### Challenge with SPDX Expressions

SPDX expressions like "MIT OR Apache-2.0" create complexity for our current model:
- **Granularity Loss**: Can't distinguish individual license usage
- **Query Complexity**: How to find all projects using MIT?
- **Relationship Loss**: AND/OR semantics not captured

### Enhanced Knowledge Graph Design

We'll extend our existing model to support expressions while preserving the `HAS_LICENSE` relationship:

```
Project --HAS_LICENSE--> Expression Node --CONTAINS--> Atomic License Nodes
```

#### Node Types

**Atomic License Node** (Type: "atomic")
```json
{
  "id": "license-mit",
  "name": "MIT License", 
  "spdx_id": "MIT",
  "type": "atomic",
  "properties": {
    "category": "permissive"
  }
}
```

**Expression License Node** (Type: "expression")
```json
{
  "id": "license-mit-or-apache",
  "name": "MIT OR Apache-2.0",
  "spdx_id": "MIT OR Apache-2.0", 
  "type": "expression",
  "expression": "MIT OR Apache-2.0",
  "operator": "OR",
  "components": ["MIT", "Apache-2.0"],
  "properties": {
    "is_dual_licensing": true
  }
}
```

#### Relationships

- **`HAS_LICENSE`**: Project → License (atomic OR expression) - *existing relationship*
- **`CONTAINS`**: Expression → Atomic licenses - *new relationship for expressions*

#### Processing Logic

```go
func (kg *KnowledgeGraph) AddLicense(projectID, expression string) {
    parsed := spdxParser.Parse(expression)
    
    if len(parsed.Components) == 1 {
        // Simple license - existing behavior
        atomicNode := kg.GetOrCreateLicenseNode(parsed.Components[0], "atomic")
        kg.AddRelationship(projectID, atomicNode.ID, "HAS_LICENSE")
    } else {
        // Complex expression - create expression node + CONTAINS links
        exprNode := kg.GetOrCreateLicenseNode(expression, "expression")
        exprNode.Expression = expression
        exprNode.Operator = parsed.Operator
        exprNode.Components = parsed.Components
        
        // Project -> Expression (uses existing HAS_LICENSE)
        kg.AddRelationship(projectID, exprNode.ID, "HAS_LICENSE")
        
        // Expression -> Atomic licenses (new CONTAINS relationships)
        for _, component := range parsed.Components {
            atomicNode := kg.GetOrCreateLicenseNode(component, "atomic")
            kg.AddRelationship(exprNode.ID, atomicNode.ID, "CONTAINS")
        }
    }
}
```

#### Query Examples

```cypher
// Find all projects using MIT (direct or in expressions)
MATCH (p:Project)-[:HAS_LICENSE]->(l:LicenseNode)
WHERE l.spdx_id = "MIT" OR "MIT" IN l.components
RETURN p

// Get complete license breakdown for a project
MATCH (p:Project {id: $projectId})-[:HAS_LICENSE]->(l:LicenseNode)
OPTIONAL MATCH (l)-[:CONTAINS]->(atomic:LicenseNode)
RETURN p, l, atomic

// Find dual-licensed projects
MATCH (p:Project)-[:HAS_LICENSE]->(l:LicenseNode)
WHERE l.type = "expression" AND l.operator = "OR"
RETURN p, l.expression
```

#### Benefits of This Design

- **Atomic tracking**: Can query individual license usage across all projects
- **Expression preservation**: Complex licensing relationships maintained  
- **Traversal power**: Can follow expressions to their components
- **Backward compatibility**: Simple licenses work exactly as before
- **Minimal disruption**: Reuses existing `HAS_LICENSE` relationship
- **Query flexibility**: Supports both simple and complex license analysis

## Conclusion

This enhancement brings our Tech Stack Analyzer to full SPDX 2.1 compliance while preserving our unique advantages in project-level analysis and detection context. The phased approach ensures minimal risk while delivering significant value for license compliance and tool integration.

The design maintains backward compatibility while providing a clear migration path to advanced license expression support, positioning our scanner as a comprehensive solution for modern software license management.
