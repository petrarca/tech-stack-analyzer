package types

import (
	"encoding/json"
	"regexp"
)

// Rule represents a technology detection rule
type Rule struct {
	Tech          string                 `yaml:"tech" json:"tech"`
	Name          string                 `yaml:"name" json:"name"`
	Type          string                 `yaml:"type" json:"type"`
	Description   string                 `yaml:"description,omitempty" json:"description,omitempty"`
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
	Type       string `yaml:"type" json:"type"`
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version,omitempty" json:"version,omitempty"`
	SourceFile string `yaml:"source_file,omitempty" json:"source_file,omitempty"`
}

// MarshalJSON converts Dependency struct to array format [type, name, version, source_file] to match TypeScript
func (d Dependency) MarshalJSON() ([]byte, error) {
	if d.SourceFile != "" {
		// New format with source file
		return json.Marshal([]string{d.Type, d.Name, d.Version, d.SourceFile})
	}
	// Backward compatibility - old format without source file
	return json.Marshal([]string{d.Type, d.Name, d.Version})
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
	IsComponent bool   `yaml:"is_component" json:"is_component"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// CategoriesConfig represents the categories.yaml configuration file
type CategoriesConfig struct {
	Categories map[string]CategoryDefinition `yaml:"categories" json:"categories"`
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
