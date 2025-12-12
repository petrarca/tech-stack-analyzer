package validation

import (
	"testing"
)

func TestValidateYAML_ValidStackAnalyzerYML(t *testing.T) {
	validYAML := `
properties:
  product: "My Product"
  team: "Engineering"
  version: 1.0
  active: true

exclude:
  - "node_modules"
  - "vendor"
  - "*.log"
  - "**/__tests__/**"

techs:
  - tech: "aws"
    reason: "Cloud hosting"
  - tech: "react"
    reason: "Frontend framework"
  - tech: "postgresql"

root_id: "my-project-2024"
`

	err := ValidateYAML("stack-analyzer-yml.json", []byte(validYAML))
	if err != nil {
		t.Fatalf("Expected valid YAML to pass validation, got error: %v", err)
	}
}

func TestValidateYAML_InvalidStackAnalyzerYML(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		expect string
	}{
		{
			name: "invalid property name",
			yaml: `
properties:
  123_invalid: "value"
`,
			expect: "does not match pattern",
		},
		{
			name: "absolute path in exclude",
			yaml: `
exclude:
  - "/absolute/path"
`,
			expect: "does not match pattern",
		},
		{
			name: "invalid tech name",
			yaml: `
techs:
  - tech: "123_invalid_tech"
    reason: "Invalid name"
`,
			expect: "does not match pattern",
		},
		{
			name: "invalid root_id with spaces",
			yaml: `
root_id: "invalid with spaces"
`,
			expect: "does not match pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYAML("stack-analyzer-yml.json", []byte(tt.yaml))
			if err == nil {
				t.Fatalf("Expected validation to fail for %s", tt.name)
			}
			if !contains(err.Error(), tt.expect) {
				t.Fatalf("Expected error to contain '%s', got: %v", tt.expect, err)
			}
		})
	}
}

func TestValidateJSON_ValidConfig(t *testing.T) {
	// Valid scan configuration
	validConfig := map[string]interface{}{
		"properties": map[string]interface{}{
			"product": "My Product",
			"team":    "Engineering",
			"version": 1.0,
			"active":  true,
		},
		"exclude": []interface{}{
			"node_modules",
			"vendor",
			"*.log",
			"**/__tests__/**",
		},
		"techs": []interface{}{
			map[string]interface{}{"tech": "aws", "reason": "Cloud hosting"},
			map[string]interface{}{"tech": "react", "reason": "Frontend framework"},
			map[string]interface{}{"tech": "postgresql"},
		},
		"scan": map[string]interface{}{
			"output_file": "output.json",
			"pretty":      true,
			"aggregate":   "tech,dependencies",
		},
	}

	err := ValidateJSON("stack-analyzer-config.json", validConfig)
	if err != nil {
		t.Fatalf("Expected valid config to pass validation, got error: %v", err)
	}
}

func TestValidateJSON_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		expect string
	}{
		{
			name: "invalid output file path",
			config: map[string]interface{}{
				"scan": map[string]interface{}{
					"output_file": "/absolute/path.json",
				},
			},
			expect: "does not match pattern",
		},
		{
			name: "invalid aggregate value",
			config: map[string]interface{}{
				"scan": map[string]interface{}{
					"aggregate": "invalid_value",
				},
			},
			expect: "does not match pattern",
		},
		{
			name: "invalid exclude pattern",
			config: map[string]interface{}{
				"exclude": []interface{}{"/absolute/path"},
			},
			expect: "does not match pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSON("stack-analyzer-config.json", tt.config)
			if err == nil {
				t.Fatalf("Expected validation to fail for %s", tt.name)
			}
			if !contains(err.Error(), tt.expect) {
				t.Fatalf("Expected error to contain '%s', got: %v", tt.expect, err)
			}
		})
	}
}

func TestListAvailableSchemas(t *testing.T) {
	schemas, err := ListAvailableSchemas()
	if err != nil {
		t.Fatalf("Failed to list schemas: %v", err)
	}

	expectedSchemas := []string{
		"stack-analyzer-config.json",
		"stack-analyzer-yml.json",
	}

	if len(schemas) < len(expectedSchemas) {
		t.Fatalf("Expected at least %d schemas, got %d", len(expectedSchemas), len(schemas))
	}

	for _, expected := range expectedSchemas {
		found := false
		for _, schema := range schemas {
			if schema == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected to find schema '%s' in list: %v", expected, schemas)
		}
	}
}

func TestValidateJSON_SchemaNotFound(t *testing.T) {
	err := ValidateJSON("nonexistent-schema.json", map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error for nonexistent schema")
	}
	if !contains(err.Error(), "failed to load schema") {
		t.Fatalf("Expected schema loading error, got: %v", err)
	}
}

// Helper functions

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
