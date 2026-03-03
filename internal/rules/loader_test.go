package rules

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/require"
)

func TestLoadEmbeddedRules(t *testing.T) {
	// Test that embedded rules can be loaded
	rules, err := LoadEmbeddedRules()
	if err != nil {
		t.Fatalf("Failed to load embedded rules: %v", err)
	}

	// Should have loaded some rules
	if len(rules) == 0 {
		t.Fatal("No rules loaded")
	}

	// Check for cargo rule specifically (since it exists)
	var cargoRule *types.Rule
	for _, rule := range rules {
		if rule.Tech == "cargo" {
			cargoRule = &rule
			break
		}
	}

	require.NotNil(t, cargoRule, "Cargo rule not found")
	require.Equal(t, "package_manager", cargoRule.Type, "Cargo rule type should be 'package_manager'")
	require.NotNil(t, cargoRule.Files, "Cargo rule should have file patterns")

	t.Logf("Cargo rule test passed")
	t.Logf("   - Total rules loaded: %d", len(rules))
	t.Logf("   - Cargo rule type: %s", cargoRule.Type)
}

func TestRuleStructure(t *testing.T) {
	rules, err := LoadEmbeddedRules()
	if err != nil {
		t.Fatalf("Failed to load embedded rules: %v", err)
	}

	// Test that all rules have required fields
	for _, rule := range rules {
		if rule.Tech == "" {
			t.Errorf("Rule missing tech field: %+v", rule)
		}
		if rule.Type == "" {
			t.Errorf("Rule missing type field: %+v", rule)
		}
	}

	t.Logf("Rule structure validation passed for %d rules", len(rules))
}
