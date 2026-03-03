package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"gopkg.in/yaml.v3"
)

//go:embed all:techs
var coreRulesFS embed.FS

// LoadEmbeddedRules loads all rules from the embedded filesystem
func LoadEmbeddedRules() ([]types.Rule, error) {
	var rules []types.Rule

	err := fs.WalkDir(coreRulesFS, "techs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Skip _types.yaml - it's loaded separately
		if strings.HasSuffix(path, "_types.yaml") {
			return nil
		}

		// Only load YAML files
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		content, err := coreRulesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read rule file %s: %w", path, err)
		}

		var rule types.Rule
		if err := yaml.Unmarshal(content, &rule); err != nil {
			return fmt.Errorf("failed to parse rule file %s: %w", path, err)
		}

		// Derive type from folder if not specified
		if rule.Type == "" {
			rule.Type = deriveTypeFromPath(path)
		}

		// Validate rule
		if err := validateRule(&rule); err != nil {
			return fmt.Errorf("invalid rule in %s: %w", path, err)
		}

		rules = append(rules, rule)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded rules: %w", err)
	}

	return rules, nil
}

// LoadExternalRules loads rules from an external directory
func LoadExternalRules(rulesDir string) ([]types.Rule, error) {
	var rules []types.Rule

	err := filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Only load YAML files
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read rule file %s: %w", path, err)
		}

		var rule types.Rule
		if err := yaml.Unmarshal(content, &rule); err != nil {
			return fmt.Errorf("failed to parse rule file %s: %w", path, err)
		}

		// Derive type from folder if not specified
		if rule.Type == "" {
			rule.Type = deriveTypeFromPath(path)
		}

		// Validate rule
		if err := validateRule(&rule); err != nil {
			return fmt.Errorf("invalid rule in %s: %w", path, err)
		}

		rules = append(rules, rule)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk external rules: %w", err)
	}

	return rules, nil
}

// deriveTypeFromPath extracts the type from the folder name in the path
// e.g., "techs/database/postgres.yaml" -> "database"
func deriveTypeFromPath(path string) string {
	dir := filepath.Dir(path)
	return filepath.Base(dir)
}

// validateRule validates a rule definition
func validateRule(rule *types.Rule) error {
	if rule.Tech == "" {
		return fmt.Errorf("tech is required")
	}

	if rule.Name == "" {
		return fmt.Errorf("name is required")
	}

	if rule.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate dependencies
	for i, dep := range rule.Dependencies {
		if dep.Type == "" {
			return fmt.Errorf("dependency %d: type is required", i)
		}
		if dep.Name == "" {
			return fmt.Errorf("dependency %d: name is required", i)
		}
	}

	return nil
}
