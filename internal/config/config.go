package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/petrarca/tech-stack-analyzer/internal/validation"
	"gopkg.in/yaml.v3"
)

//go:embed categories.yaml
var categoriesConfigData []byte

//go:embed ecosystems.yaml
var ecosystemsConfigData []byte

// ScanConfig represents the .stack-analyzer.yml configuration file
type ScanConfig struct {
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	Exclude    []string               `yaml:"exclude,omitempty"`
	Techs      []ConfigTech           `yaml:"techs,omitempty"`
	Reclassify []ReclassifyRule       `yaml:"reclassify,omitempty"`
	RootID     string                 `yaml:"root_id,omitempty"` // Override random root ID for deterministic scans
}

// ConfigTech represents a technology to add to the scan
type ConfigTech struct {
	Tech   string `yaml:"tech"`
	Reason string `yaml:"reason,omitempty"`
}

// ReclassifyRule overrides go-enry's language detection for files matching a glob pattern.
// At least one of Language or Type must be set.
//   - Language: override the detected language label (e.g. "CSV", "C++")
//   - Type: override the language type category ("programming", "data", "markup", "prose")
type ReclassifyRule struct {
	Match    string `yaml:"match"`              // Glob pattern matched relative to scan root (supports **)
	Language string `yaml:"language,omitempty"` // Override language label (must be known to go-enry for automatic type resolution)
	Type     string `yaml:"type,omitempty"`     // Override language type: programming, data, markup, prose
}

// LoadConfig attempts to load .stack-analyzer.yml from the scan root
// Returns nil if file doesn't exist (not an error)
func LoadConfig(scanPath string) (*ScanConfig, error) {
	configPath := filepath.Join(scanPath, ".stack-analyzer.yml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No config file - return empty config (not an error)
		return &ScanConfig{}, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var config ScanConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Validate configuration against schema
	if err := validation.ValidateYAML("stack-analyzer-yml.json", data); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// MergeExcludes merges config excludes with CLI excludes
// CLI excludes take precedence
func (c *ScanConfig) MergeExcludes(cliExcludes []string) []string {
	if c == nil {
		return cliExcludes
	}

	// Create a map to deduplicate
	excludeMap := make(map[string]bool)

	// Add config excludes first
	for _, exclude := range c.Exclude {
		excludeMap[exclude] = true
	}

	// Add CLI excludes (will override if duplicate)
	for _, exclude := range cliExcludes {
		excludeMap[exclude] = true
	}

	// Convert back to slice
	result := make([]string, 0, len(excludeMap))
	for exclude := range excludeMap {
		result = append(result, exclude)
	}

	return result
}

// LoadCategoriesConfig loads the categories configuration from categories.yaml
func LoadCategoriesConfig() (*types.CategoriesConfig, error) {
	var config types.CategoriesConfig
	if err := yaml.Unmarshal(categoriesConfigData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse categories.yaml: %w", err)
	}

	return &config, nil
}

// LoadEcosystemsConfig loads the ecosystem definitions from ecosystems.yaml
func LoadEcosystemsConfig() (*types.EcosystemsConfig, error) {
	var config types.EcosystemsConfig
	if err := yaml.Unmarshal(ecosystemsConfigData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse ecosystems.yaml: %w", err)
	}

	return &config, nil
}
