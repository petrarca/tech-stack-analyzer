package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScanConfigFile represents the external scan configuration file
type ScanConfigFile struct {
	Scan ScanConfigSection `yaml:"scan" json:"scan"`
}

// ScanConfigSection contains all scan configuration options
type ScanConfigSection struct {
	// What to scan
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"`

	// Output configuration
	Output OutputConfig `yaml:"output,omitempty" json:"output,omitempty"`

	// Metadata (identical to .stack-analyzer.yml)
	Properties map[string]interface{} `yaml:"properties,omitempty" json:"properties,omitempty"`

	// Excludes (identical to .stack-analyzer.yml)
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`

	// Additional technologies
	Techs []ConfigTech `yaml:"techs,omitempty" json:"techs,omitempty"`

	// Scanner options
	Options ScannerOptions `yaml:"options,omitempty" json:"options,omitempty"`
}

// OutputConfig defines output settings
type OutputConfig struct {
	File      string `yaml:"file,omitempty" json:"file,omitempty"`
	Pretty    bool   `yaml:"pretty,omitempty" json:"pretty,omitempty"`
	Aggregate string `yaml:"aggregate,omitempty" json:"aggregate,omitempty"`
}

// ScannerOptions defines scanner behavior options
type ScannerOptions struct {
	Verbose               bool     `yaml:"verbose,omitempty" json:"verbose,omitempty"`
	Debug                 bool     `yaml:"debug,omitempty" json:"debug,omitempty"`
	NoCodeStats           bool     `yaml:"no_code_stats,omitempty" json:"no_code_stats,omitempty"`
	CodeStatsPerComponent bool     `yaml:"component_code_stats,omitempty" json:"component_code_stats,omitempty"`
	TraceTimings          bool     `yaml:"trace_timings,omitempty" json:"trace_timings,omitempty"`
	TraceRules            bool     `yaml:"trace_rules,omitempty" json:"trace_rules,omitempty"`
	FilterRules           []string `yaml:"filter_rules,omitempty" json:"filter_rules,omitempty"`
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

	// Only merge if settings haven't been explicitly set via CLI flags
	// (we assume non-default values come from CLI)

	// Output settings
	if c.Scan.Output.File != "" && settings.OutputFile == "stack-analysis.json" {
		settings.OutputFile = c.Scan.Output.File
	}
	if !settings.PrettyPrint && c.Scan.Output.Pretty {
		settings.PrettyPrint = c.Scan.Output.Pretty
	}
	if c.Scan.Output.Aggregate != "" && settings.Aggregate == "" {
		settings.Aggregate = c.Scan.Output.Aggregate
	}

	// Scanner options
	if !settings.Verbose && c.Scan.Options.Verbose {
		settings.Verbose = c.Scan.Options.Verbose
	}
	if !settings.Debug && c.Scan.Options.Debug {
		settings.Debug = c.Scan.Options.Debug
	}
	if !settings.NoCodeStats && c.Scan.Options.NoCodeStats {
		settings.NoCodeStats = c.Scan.Options.NoCodeStats
	}
	if !settings.CodeStatsPerComponent && c.Scan.Options.CodeStatsPerComponent {
		settings.CodeStatsPerComponent = c.Scan.Options.CodeStatsPerComponent
	}
	if !settings.TraceTimings && c.Scan.Options.TraceTimings {
		settings.TraceTimings = c.Scan.Options.TraceTimings
	}
	if !settings.TraceRules && c.Scan.Options.TraceRules {
		settings.TraceRules = c.Scan.Options.TraceRules
	}
	if len(settings.FilterRules) == 0 && len(c.Scan.Options.FilterRules) > 0 {
		settings.FilterRules = c.Scan.Options.FilterRules
	}

	// Exclude patterns will be merged separately with project config
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
