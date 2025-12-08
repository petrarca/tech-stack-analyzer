package rules

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
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

	if cargoRule == nil {
		t.Fatal("Cargo rule not found")
	}

	// Verify cargo rule has expected properties
	if cargoRule.Type != "package_manager" {
		t.Errorf("Expected cargo rule type to be 'package_manager', got '%s'", cargoRule.Type)
	}

	// Check for expected files
	if cargoRule.Files == nil {
		t.Fatal("Cargo rule should have file patterns")
	}

	t.Logf("✅ Cargo rule test passed")
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

	t.Logf("✅ Rule structure validation passed for %d rules", len(rules))
}
