package parsers

import (
	"testing"
)

func TestParsePackageLockWithTransitiveDependencies(t *testing.T) {
	content := `{
		"name": "test-project",
		"version": "1.0.0",
		"lockfileVersion": 2,
		"packages": {
			"": {"name": "test-project", "version": "1.0.0"},
			"node_modules/express": {"version": "4.18.2", "resolved": "https://registry.npmjs.org/express/-/express-4.18.2.tgz"},
			"node_modules/express/node_modules/accepts": {"version": "1.3.8", "resolved": "https://registry.npmjs.org/accepts/-/accepts-1.3.8.tgz"},
			"node_modules/express/node_modules/body-parser": {"version": "1.20.2", "resolved": "https://registry.npmjs.org/body-parser/-/body-parser-1.20.2.tgz"}
		}
	}`

	packageJSON := &PackageJSON{
		Name: "test-project",
		Dependencies: map[string]string{
			"express": "^4.18.0",
		},
	}

	// Test 1: Default behavior (backward compatible - only direct dependencies)
	t.Run("default behavior (direct only)", func(t *testing.T) {
		deps := ParsePackageLock([]byte(content), packageJSON)

		if len(deps) != 1 {
			t.Errorf("Expected 1 dependency, got %d", len(deps))
		}

		if deps[0].Name != "express" {
			t.Errorf("Expected 'express', got '%s'", deps[0].Name)
		}

		if deps[0].Version != "4.18.2" {
			t.Errorf("Expected version '4.18.2', got '%s'", deps[0].Version)
		}
	})

	// Test 2: With transitive dependencies enabled
	t.Run("with transitive dependencies", func(t *testing.T) {
		deps := ParsePackageLockWithOptions([]byte(content), packageJSON, nil, ParsePackageLockOptions{
			IncludeTransitive: true,
		})

		if len(deps) != 3 {
			t.Errorf("Expected 3 dependencies, got %d", len(deps))
		}

		// Check that we have all three dependencies
		depMap := make(map[string]string)
		for _, dep := range deps {
			if dep.Version == "" {
				t.Errorf("Dependency %s has empty version", dep.Name)
			}
			depMap[dep.Name] = dep.Version
		}

		expected := map[string]string{
			"express":     "4.18.2",
			"accepts":     "1.3.8",
			"body-parser": "1.20.2",
		}

		for name, version := range expected {
			if depMap[name] != version {
				t.Errorf("Expected %s@%s, got %s@%s", name, version, name, depMap[name])
			}
		}
	})

	// Test 3: Enhanced features demonstration
	t.Run("enhanced features", func(t *testing.T) {
		deps := ParsePackageLock([]byte(content), packageJSON)

		if len(deps) == 0 {
			t.Fatal("Expected at least one dependency")
		}

		dep := deps[0]

		// Verify enhanced features
		if dep.Type != "npm" {
			t.Errorf("Expected type 'npm', got '%s'", dep.Type)
		}

		if dep.Scope != "prod" {
			t.Errorf("Expected scope 'prod', got '%s'", dep.Scope)
		}

		// Semantic version preservation
		if dep.Version != "4.18.2" {
			t.Errorf("Expected version '4.18.2', got '%s'", dep.Version)
		}
	})
}
