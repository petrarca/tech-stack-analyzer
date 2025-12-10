package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"gopkg.in/yaml.v3"
)

//go:embed categories.yaml
var categoriesConfigData []byte

// ScanConfig represents the .stack-analyzer.yml configuration file
type ScanConfig struct {
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	Exclude    []string               `yaml:"exclude,omitempty"`
	Techs      []ConfigTech           `yaml:"techs,omitempty"`
	RootID     string                 `yaml:"root_id,omitempty"` // Override random root ID for deterministic scans
}

// ConfigTech represents a technology to add to the scan
type ConfigTech struct {
	Tech   string `yaml:"tech"`
	Reason string `yaml:"reason,omitempty"`
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
