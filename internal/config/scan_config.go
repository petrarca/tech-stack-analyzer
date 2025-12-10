package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

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
	Scan ScanConfigSection `yaml:"scan" json:"scan"`
}

// ScanConfigSection contains all scan configuration options
type ScanConfigSection struct {
	// What to scan
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"`

	// Output configuration (legacy field names for backward compatibility)
	Output OutputConfig `yaml:"output,omitempty" json:"output,omitempty"`

	// Metadata (identical to .stack-analyzer.yml)
	Properties map[string]interface{} `yaml:"properties,omitempty" json:"properties,omitempty"`

	// Excludes (identical to .stack-analyzer.yml)
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`

	// Additional technologies
	Techs []ConfigTech `yaml:"techs,omitempty" json:"techs,omitempty"`

	// Scanner options (unified structure)
	Options ScanOptions `yaml:"options,omitempty" json:"options,omitempty"`
}

// OutputConfig defines output settings (legacy for backward compatibility)
type OutputConfig struct {
	File      string `yaml:"file,omitempty" json:"file,omitempty"`
	Pretty    bool   `yaml:"pretty,omitempty" json:"pretty,omitempty"`
	Aggregate string `yaml:"aggregate,omitempty" json:"aggregate,omitempty"`
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

	return &config, nil
}

// loadScanConfigFromJSON loads configuration from inline JSON string
func loadScanConfigFromJSON(jsonStr string) (*ScanConfigFile, error) {
	var config ScanConfigFile
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse inline JSON config: %w", err)
	}
	return &config, nil
}

// MergeWithSettings merges scan config with existing settings
// CLI flags take precedence over config file settings
func (c *ScanConfigFile) MergeWithSettings(settings *Settings) {
	if c == nil || settings == nil {
		return
	}

	// Create unified options from config
	configOpts := c.getUnifiedOptions()

	// Merge using reflection to avoid manual field mapping
	mergeOptions(configOpts, settings)
}

// getUnifiedOptions creates unified ScanOptions from config
func (c *ScanConfigFile) getUnifiedOptions() *ScanOptions {
	opts := &ScanOptions{}

	// Copy from options section
	*opts = c.Scan.Options

	// Handle legacy output config for backward compatibility
	if c.Scan.Output.File != "" {
		opts.OutputFile = c.Scan.Output.File
	}
	if c.Scan.Output.Pretty {
		opts.PrettyPrint = c.Scan.Output.Pretty
	}
	if c.Scan.Output.Aggregate != "" {
		opts.Aggregate = c.Scan.Output.Aggregate
	}

	// Copy excludes to options
	if len(c.Scan.Exclude) > 0 {
		opts.ExcludePatterns = c.Scan.Exclude
	}

	return opts
}

// mergeOptions merges config options into settings using reflection
func mergeOptions(configOpts *ScanOptions, settings *Settings) {
	configValue := reflect.ValueOf(configOpts).Elem()
	settingsValue := reflect.ValueOf(settings).Elem()
	configType := configValue.Type()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Field(i)
		fieldType := configType.Field(i)
		settingsField := settingsValue.FieldByName(fieldType.Name)

		if !settingsField.IsValid() || !settingsField.CanSet() {
			continue
		}

		// Only merge if setting is at default value and config has non-default value
		if isDefaultValue(settingsField) && !isDefaultValue(field) {
			settingsField.Set(field)
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
	default:
		return field.IsZero()
	}
}

// GetScanPaths returns the paths to scan, defaulting to ["."] if not specified
func (c *ScanConfigFile) GetScanPaths() []string {
	if c == nil || len(c.Scan.Paths) == 0 {
		return []string{"."}
	}
	return c.Scan.Paths
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

	// Copy from scan config first
	if c.Scan.Properties != nil {
		for k, v := range c.Scan.Properties {
			merged.Properties[k] = v
		}
	}
	if len(c.Scan.Exclude) > 0 {
		merged.Exclude = append(merged.Exclude, c.Scan.Exclude...)
	}
	if len(c.Scan.Techs) > 0 {
		merged.Techs = append(merged.Techs, c.Scan.Techs...)
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
