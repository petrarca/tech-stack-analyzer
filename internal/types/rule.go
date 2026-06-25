package types

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// Dependency scope constants
const (
	ScopeProd     = "prod"
	ScopeDev      = "dev"
	ScopeTest     = "test"
	ScopeBuild    = "build"
	ScopeOptional = "optional"
	ScopePeer     = "peer"
	// Maven-specific scopes
	ScopeSystem = "system"
	ScopeImport = "import"
)

// NewMetadata creates a new metadata map with the source field set
// This helper eliminates code duplication across parsers
func NewMetadata(source string) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["source"] = source
	return metadata
}

// Rule represents a technology detection rule
type Rule struct {
	Tech          string                 `yaml:"tech" json:"tech"`
	Name          string                 `yaml:"name" json:"name"`
	Type          string                 `yaml:"type" json:"type"`
	Description   string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Aliases       []string               `yaml:"aliases,omitempty" json:"aliases,omitempty"` // Alternative display names for downstream name→key resolution
	Properties    map[string]interface{} `yaml:"properties,omitempty" json:"properties,omitempty"`
	IsComponent   *bool                  `yaml:"is_component,omitempty" json:"is_component,omitempty"`       // nil = auto (use type-based logic)
	IsPrimaryTech *bool                  `yaml:"is_primary_tech,omitempty" json:"is_primary_tech,omitempty"` // nil = use current logic (component = primary tech)
	DotEnv        []string               `yaml:"dotenv,omitempty" json:"dotenv,omitempty"`
	Dependencies  []Dependency           `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Files         []string               `yaml:"files,omitempty" json:"files,omitempty"`
	Extensions    []string               `yaml:"extensions,omitempty" json:"extensions,omitempty"`
	Content       []ContentRule          `yaml:"content,omitempty" json:"content,omitempty"`
}

// Dependency represents a dependency pattern (struct for YAML, but marshals as array for JSON)
type Dependency struct {
	Type       string                 `yaml:"type" json:"type"`
	Name       string                 `yaml:"name" json:"name"`
	Version    string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Scope      string                 `yaml:"scope,omitempty" json:"scope,omitempty"`
	Direct     bool                   `yaml:"direct" json:"direct"`                               // Direct (true) vs transitive (false) dependency
	SourceFile string                 `yaml:"source_file,omitempty" json:"source_file,omitempty"` // Deprecated: use metadata.source instead
	Metadata   map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`       // Package-specific metadata (source, type, classifier, optional, exclusions, peer, etc.)
}

// MarshalJSON converts Dependency struct to array format [type, name, version, scope, direct, {metadata}]
// Format: 6 elements (always consistent)
// - [type, name, version, scope, direct, {metadata}]
// - scope: "prod", "dev", "test", "build", "optional", "peer", etc. (empty string if unknown)
// - direct: true (declared in manifest) or false (transitive)
// - metadata: optional object with source, type, classifier, exclusions, peer, optional, bundled, etc.
func (d Dependency) MarshalJSON() ([]byte, error) {
	// Build a shallow copy of metadata to avoid mutating the original map
	var metadata map[string]interface{}
	if d.SourceFile != "" {
		// Migrate deprecated SourceFile into metadata.source — copy to avoid side effects
		metadata = make(map[string]interface{}, len(d.Metadata)+1)
		for k, v := range d.Metadata {
			metadata[k] = v
		}
		if _, exists := metadata["source"]; !exists {
			metadata["source"] = d.SourceFile
		}
	} else {
		metadata = d.Metadata
	}

	// Always return 6 elements for consistency
	// Use encoder with SetEscapeHTML(false) to avoid escaping >, <, & in version strings
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	var arr []interface{}
	if len(metadata) == 0 {
		arr = []interface{}{d.Type, d.Name, d.Version, d.Scope, d.Direct, struct{}{}}
	} else {
		arr = []interface{}{d.Type, d.Name, d.Version, d.Scope, d.Direct, metadata}
	}

	if err := enc.Encode(arr); err != nil {
		return nil, err
	}
	// Encode appends a newline; trim it for MarshalJSON compatibility
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// UnmarshalJSON reverses MarshalJSON, decoding the array form
// [type, name, version, scope, direct, {metadata}] back into a Dependency. It
// also tolerates the struct (object) form for forward compatibility and YAML
// round-trips. This lets a saved scan output be read back into a Payload (e.g.
// by the "sbom" command) and re-projected into an SBOM.
func (d *Dependency) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		// Object form: decode into an alias to avoid recursing into this method.
		type depAlias Dependency
		var alias depAlias
		if err := json.Unmarshal(data, &alias); err != nil {
			return err
		}
		*d = Dependency(alias)
		return nil
	}

	// Array form: [type, name, version, scope, direct, metadata]
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	dep := Dependency{}
	get := func(i int, dst interface{}) {
		if i < len(arr) {
			_ = json.Unmarshal(arr[i], dst)
		}
	}
	get(0, &dep.Type)
	get(1, &dep.Name)
	get(2, &dep.Version)
	get(3, &dep.Scope)
	get(4, &dep.Direct)
	if len(arr) > 5 {
		var md map[string]interface{}
		if err := json.Unmarshal(arr[5], &md); err == nil && len(md) > 0 {
			dep.Metadata = md
		}
	}
	*d = dep
	return nil
}

// MetadataKeyDeclared is the metadata key recording the originally declared
// version form (a range, property reference, or specifier) when it differs
// from the resolved Version. This mirrors the deps.dev model of keeping the
// declared requirement separate from the resolved version.
const MetadataKeyDeclared = "declared"

// SetDeclaredVersion records the originally declared version form in metadata
// when it differs from the resolved Version. No-op when declared is empty or
// equal to the resolved version, so concrete declarations add no noise.
func (d *Dependency) SetDeclaredVersion(declared string) {
	if declared == "" || declared == d.Version {
		return
	}
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata[MetadataKeyDeclared] = declared
}

// CompiledDependency is a pre-compiled dependency for performance
type CompiledDependency struct {
	Regex *regexp.Regexp
	Tech  string
	Name  string
	Type  string
}

// ContentRule represents a content-based detection pattern
type ContentRule struct {
	Type       string   `yaml:"type,omitempty" json:"type,omitempty"`             // Match type: "regex" (default), "json-path", "json-schema", "yaml-path"
	Pattern    string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`       // Regex pattern (for type=regex) or expected value (for path types)
	Path       string   `yaml:"path,omitempty" json:"path,omitempty"`             // JSON/YAML path (e.g., "$.$schema", "$.name")
	Value      string   `yaml:"value,omitempty" json:"value,omitempty"`           // Expected value for path matching (exact match or regex if starts/ends with /)
	Extensions []string `yaml:"extensions,omitempty" json:"extensions,omitempty"` // Optional: limit pattern to specific extensions
	Files      []string `yaml:"files,omitempty" json:"files,omitempty"`           // Optional: limit pattern to specific filenames
}

// GetType returns the content rule type, defaulting to "regex" if not specified
func (c *ContentRule) GetType() string {
	if c.Type == "" {
		return "regex"
	}
	return c.Type
}

// CategoryDefinition represents a technology category configuration
type CategoryDefinition struct {
	IsComponent   bool   `yaml:"is_component" json:"is_component"`
	IsPrimaryTech *bool  `yaml:"is_primary_tech,omitempty" json:"is_primary_tech,omitempty"`
	CreateEdges   *bool  `yaml:"create_edges,omitempty" json:"create_edges,omitempty"`
	Description   string `yaml:"description,omitempty" json:"description,omitempty"`
}

// CategoriesConfig represents the categories.yaml configuration file
type CategoriesConfig struct {
	Categories map[string]CategoryDefinition `yaml:"categories" json:"categories"`
}

// EcosystemDefinition represents a technology ecosystem from ecosystems.yaml
type EcosystemDefinition struct {
	Name           string   `yaml:"name" json:"name"`
	Description    string   `yaml:"description,omitempty" json:"description,omitempty"`
	ComponentTypes []string `yaml:"component_types" json:"component_types"`
	Techs          []string `yaml:"techs" json:"techs"`
	Languages      []string `yaml:"languages" json:"languages"`
}

// EcosystemsConfig represents the ecosystems.yaml configuration file
type EcosystemsConfig struct {
	Ecosystems []EcosystemDefinition `yaml:"ecosystems" json:"ecosystems"`
}

// EcosystemEntry represents a detected ecosystem in the aggregated output
type EcosystemEntry struct {
	Ecosystem  string `json:"ecosystem"`
	Components int    `json:"components"`
}

// Compile compiles a dependency pattern to regex for performance
func (d *Dependency) Compile() (*CompiledDependency, error) {
	pattern := d.Name

	// Check if it's a regex pattern
	if len(pattern) > 2 && pattern[0] == '/' && pattern[len(pattern)-1] == '/' {
		regex, err := regexp.Compile(pattern[1 : len(pattern)-1])
		if err != nil {
			return nil, err
		}
		return &CompiledDependency{
			Regex: regex,
			Tech:  "", // Will be set by rule
			Name:  d.Name,
			Type:  d.Type,
		}, nil
	}

	// Simple string match - compile to exact regex
	regex := regexp.MustCompile("^" + regexp.QuoteMeta(pattern) + "$")
	return &CompiledDependency{
		Regex: regex,
		Tech:  "", // Will be set by rule
		Name:  d.Name,
		Type:  d.Type,
	}, nil
}

// Match checks if the dependency matches the given string
func (cd *CompiledDependency) Match(s string) bool {
	return cd.Regex.MatchString(s)
}
