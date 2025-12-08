package matchers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestRegexContentMatcher(t *testing.T) {
	matcher := &RegexContentMatcher{}

	tests := []struct {
		name        string
		rule        types.ContentRule
		content     string
		shouldMatch bool
	}{
		{
			name:        "matches simple pattern",
			rule:        types.ContentRule{Pattern: `Q_OBJECT`},
			content:     "class MyWidget : public QWidget { Q_OBJECT };",
			shouldMatch: true,
		},
		{
			name:        "no match",
			rule:        types.ContentRule{Pattern: `Q_OBJECT`},
			content:     "class MyWidget : public QWidget {};",
			shouldMatch: false,
		},
		{
			name:        "matches regex pattern",
			rule:        types.ContentRule{Pattern: `#include\s+<Qt[A-Z]`},
			content:     "#include <QtWidgets>",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := matcher.Compile(tt.rule, "test-tech")
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			matched, _ := compiled.Match(tt.content)
			if matched != tt.shouldMatch {
				t.Errorf("Match() = %v, want %v", matched, tt.shouldMatch)
			}
		})
	}
}

func TestJSONPathContentMatcher(t *testing.T) {
	matcher := &JSONPathContentMatcher{}

	tests := []struct {
		name        string
		rule        types.ContentRule
		content     string
		shouldMatch bool
	}{
		{
			name:        "matches path exists",
			rule:        types.ContentRule{Path: "$.name"},
			content:     `{"name": "my-project"}`,
			shouldMatch: true,
		},
		{
			name:        "matches path with exact value",
			rule:        types.ContentRule{Path: "$.name", Value: "my-project"},
			content:     `{"name": "my-project"}`,
			shouldMatch: true,
		},
		{
			name:        "no match for wrong value",
			rule:        types.ContentRule{Path: "$.name", Value: "other-project"},
			content:     `{"name": "my-project"}`,
			shouldMatch: false,
		},
		{
			name:        "matches nested path",
			rule:        types.ContentRule{Path: "$.dependencies.react"},
			content:     `{"dependencies": {"react": "^18.0.0"}}`,
			shouldMatch: true,
		},
		{
			name:        "matches with regex value",
			rule:        types.ContentRule{Path: "$.version", Value: "/^1\\./"},
			content:     `{"version": "1.2.3"}`,
			shouldMatch: true,
		},
		{
			name:        "no match for missing path",
			rule:        types.ContentRule{Path: "$.missing"},
			content:     `{"name": "test"}`,
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := matcher.Compile(tt.rule, "test-tech")
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			matched, _ := compiled.Match(tt.content)
			if matched != tt.shouldMatch {
				t.Errorf("Match() = %v, want %v", matched, tt.shouldMatch)
			}
		})
	}
}

func TestYAMLPathContentMatcher(t *testing.T) {
	matcher := &YAMLPathContentMatcher{}

	tests := []struct {
		name        string
		rule        types.ContentRule
		content     string
		shouldMatch bool
	}{
		{
			name:        "matches path exists",
			rule:        types.ContentRule{Path: "$.name"},
			content:     "name: my-project\nversion: 1.0.0",
			shouldMatch: true,
		},
		{
			name:        "matches path with exact value",
			rule:        types.ContentRule{Path: "$.name", Value: "my-project"},
			content:     "name: my-project",
			shouldMatch: true,
		},
		{
			name:        "matches nested path",
			rule:        types.ContentRule{Path: "$.services.web"},
			content:     "services:\n  web:\n    image: nginx",
			shouldMatch: true,
		},
		{
			name:        "no match for missing path",
			rule:        types.ContentRule{Path: "$.missing"},
			content:     "name: test",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := matcher.Compile(tt.rule, "test-tech")
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			matched, _ := compiled.Match(tt.content)
			if matched != tt.shouldMatch {
				t.Errorf("Match() = %v, want %v", matched, tt.shouldMatch)
			}
		})
	}
}

func TestContentTypeRegistry(t *testing.T) {
	registry := NewContentTypeRegistry()

	// Check all default types are registered
	expectedTypes := []string{"regex", "json-path", "yaml-path", "xml-path"}
	for _, typeName := range expectedTypes {
		if registry.Get(typeName) == nil {
			t.Errorf("Expected type %q to be registered", typeName)
		}
	}

	// Test compilation through registry
	rule := types.ContentRule{
		Type:  "json-path",
		Path:  "$.$schema",
		Value: "https://example.com/schema.json",
	}

	compiled, err := registry.Compile(rule, "test-tech")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if compiled.Tech() != "test-tech" {
		t.Errorf("Tech() = %q, want %q", compiled.Tech(), "test-tech")
	}
}

func TestContentMatcherRegistry_WithJSONPath(t *testing.T) {
	registry := NewContentMatcherRegistry()

	rules := []types.Rule{
		{
			Tech: "shadcn",
			Name: "Shadcn",
			Type: "ui",
			Content: []types.ContentRule{
				{
					Type:  "json-path",
					Path:  "$.$schema",
					Value: "https://ui.shadcn.com/schema.json",
					Files: []string{"components.json"},
				},
			},
		},
	}

	err := registry.BuildFromRules(rules)
	if err != nil {
		t.Fatalf("BuildFromRules failed: %v", err)
	}

	// Test matching
	content := `{"$schema": "https://ui.shadcn.com/schema.json", "style": "default"}`
	results := registry.MatchFileContent("components.json", content)

	if len(results) == 0 {
		t.Error("Expected to match shadcn, got no results")
	}

	if _, ok := results["shadcn"]; !ok {
		t.Errorf("Expected shadcn in results, got %v", results)
	}
}
