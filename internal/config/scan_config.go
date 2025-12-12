package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/validation"
	"gopkg.in/yaml.v3"
)

// ScanOptions represents all configurable scanner options
// This is the single source of truth for all option fields
type ScanOptions struct {
	// Output settings
	OutputFile  string `yaml:"output_file,omitempty" json:"output_file,omitempty" default:"stack-analysis.json"`
	PrettyPrint bool   `yaml:"pretty,omitempty" json:"pretty,omitempty" default:"true"`
	Aggregate   string `yaml:"aggregate,omitempty" json:"aggregate,omitempty" default:""`

	// Scan behavior
	ExcludePatterns       []string `yaml:"exclude_patterns,omitempty" json:"exclude_patterns,omitempty"`
	Verbose               bool     `yaml:"verbose,omitempty" json:"verbose,omitempty" default:"false"`
	Debug                 bool     `yaml:"debug,omitempty" json:"debug,omitempty" default:"false"`
	TraceTimings          bool     `yaml:"trace_timings,omitempty" json:"trace_timings,omitempty" default:"false"`
	TraceRules            bool     `yaml:"trace_rules,omitempty" json:"trace_rules,omitempty" default:"false"`
	FilterRules           []string `yaml:"filter_rules,omitempty" json:"filter_rules,omitempty"`
	NoCodeStats           bool     `yaml:"no_code_stats,omitempty" json:"no_code_stats,omitempty" default:"false"`
	CodeStatsPerComponent bool     `yaml:"component_code_stats,omitempty" json:"component_code_stats,omitempty" default:"false"`
}

// ScanConfigFile represents the external scan configuration file
type ScanConfigFile struct {
	// Root-level metadata (consistent with .stack-analyzer.yml)
	Properties map[string]interface{} `yaml:"properties,omitempty" json:"properties,omitempty"`

	// Root-level excludes (consistent with .stack-analyzer.yml)
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`

	// Root-level additional technologies (consistent with .stack-analyzer.yml)
	Techs []ConfigTech `yaml:"techs,omitempty" json:"techs,omitempty"`

	// Scan section with flat CLI options (matching CLI arguments)
	Scan ScanOptions `yaml:"scan,omitempty" json:"scan,omitempty"`
}

// LoadScanConfig loads scan configuration from file path or inline JSON
func LoadScanConfig(configPath string) (*ScanConfigFile, error) {
	if configPath == "" {
		return nil, nil
	}

	// Check if it's inline JSON (starts with {)
	if strings.HasPrefix(strings.TrimSpace(configPath), "{") {
		return loadScanConfigFromJSON(configPath)
	}

	// Load from file
	return loadScanConfigFromFile(configPath)
}

// loadScanConfigFromFile loads configuration from a YAML or JSON file
func loadScanConfigFromFile(configPath string) (*ScanConfigFile, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ScanConfigFile

	// Try YAML first (most common)
	if err := yaml.Unmarshal(data, &config); err != nil {
		// Fallback to JSON
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse config as YAML (%v) or JSON (%v)", err, jsonErr)
		}
	}

	// Validate configuration against schema
	if err := validation.ValidateStruct("stack-analyzer-config.json", &config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// loadScanConfigFromJSON loads configuration from inline JSON string
func loadScanConfigFromJSON(jsonStr string) (*ScanConfigFile, error) {
	var config ScanConfigFile
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse inline JSON config: %w", err)
	}

	// Validate configuration against schema
	if err := validation.ValidateStruct("stack-analyzer-config.json", &config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// MergeWithSettings merges scan config with existing settings
// CLI flags take precedence over config file settings
func (c *ScanConfigFile) MergeWithSettings(settings *Settings) {
	if c == nil || settings == nil {
		return
	}

	// Automatically merge scan section options using reflection
	mergeStructFields(c.Scan, settings)

	// Merge root-level excludes manually (different field names)
	if len(c.Exclude) > 0 {
		settings.ExcludePatterns = c.Exclude
	}
}

// GetMergedConfig merges scan config with project config (.stack-analyzer.yml)
// Returns a combined config for the scan operation
func (c *ScanConfigFile) GetMergedConfig(projectConfig *ScanConfig) *ScanConfig {
	if c == nil {
		return projectConfig
	}

	// Start with scan config properties
	merged := &ScanConfig{
		Properties: make(map[string]interface{}),
		Exclude:    make([]string, 0),
		Techs:      make([]ConfigTech, 0),
	}

	// Copy from root-level scan config (new flattened structure)
	if c.Properties != nil {
		for k, v := range c.Properties {
			merged.Properties[k] = v
		}
	}
	if len(c.Exclude) > 0 {
		merged.Exclude = append(merged.Exclude, c.Exclude...)
	}
	if len(c.Techs) > 0 {
		merged.Techs = append(merged.Techs, c.Techs...)
	}

	// Then merge with project config (project config takes precedence)
	if projectConfig != nil {
		if projectConfig.Properties != nil {
			for k, v := range projectConfig.Properties {
				merged.Properties[k] = v
			}
		}
		if len(projectConfig.Exclude) > 0 {
			merged.Exclude = append(merged.Exclude, projectConfig.Exclude...)
		}
		if len(projectConfig.Techs) > 0 {
			merged.Techs = append(merged.Techs, projectConfig.Techs...)
		}
	}

	return merged
}

// mergeStructFields automatically merges fields from source to target using reflection
// Only merges if target field is at default value and source has non-default value
func mergeStructFields(source, target interface{}) {
	sourceValue := reflect.ValueOf(source)
	targetValue := reflect.ValueOf(target)

	if sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}
	if targetValue.Kind() == reflect.Ptr {
		targetValue = targetValue.Elem()
	}

	if sourceValue.Kind() != reflect.Struct || targetValue.Kind() != reflect.Struct {
		return
	}

	sourceType := sourceValue.Type()

	for i := 0; i < sourceValue.NumField(); i++ {
		field := sourceValue.Field(i)
		fieldType := sourceType.Field(i)
		targetField := targetValue.FieldByName(fieldType.Name)

		if !targetField.IsValid() || !targetField.CanSet() {
			continue
		}

		// Only merge if target is at default value and source has non-default value
		if isDefaultValue(targetField) && !isDefaultValue(field) {
			targetField.Set(field)
		}
	}
}

// isDefaultValue checks if a field has its default/zero value
func isDefaultValue(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.String:
		return field.String() == ""
	case reflect.Bool:
		return !field.Bool()
	case reflect.Slice:
		return field.Len() == 0
	case reflect.Interface:
		return field.IsNil()
	default:
		return field.IsZero()
	}
}
