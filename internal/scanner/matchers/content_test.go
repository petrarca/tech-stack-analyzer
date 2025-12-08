package matchers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestContentMatcherRegistry_BuildFromRules(t *testing.T) {
	tests := []struct {
		name           string
		rules          []types.Rule
		expectMatchers bool
	}{
		{
			name: "builds matchers for extension-only rules with content",
			rules: []types.Rule{
				{
					Tech:       "stl",
					Name:       "C++ STL",
					Type:       "library",
					Extensions: []string{".cpp", ".h"},
					Content: []types.ContentRule{
						{Pattern: `#include\s+<vector>`},
						{Pattern: `#include\s+<string>`},
					},
				},
			},
			expectMatchers: true,
		},
		{
			name: "processes rules with both dependencies and content",
			rules: []types.Rule{
				{
					Tech:       "react",
					Name:       "React",
					Type:       "framework",
					Extensions: []string{".js", ".jsx"},
					Dependencies: []types.Dependency{
						{Type: "npm", Name: "react"},
					},
					Content: []types.ContentRule{
						{Pattern: `import.*React`},
					},
				},
			},
			expectMatchers: true, // Content detection works alongside dependency detection
		},
		{
			name: "skips rules without content patterns",
			rules: []types.Rule{
				{
					Tech:       "python",
					Name:       "Python",
					Type:       "language",
					Extensions: []string{".py"},
				},
			},
			expectMatchers: false,
		},
		{
			name: "skips rules without extensions",
			rules: []types.Rule{
				{
					Tech: "custom",
					Name: "Custom",
					Type: "tool",
					Content: []types.ContentRule{
						{Pattern: `custom pattern`},
					},
				},
			},
			expectMatchers: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewContentMatcherRegistry()
			err := registry.BuildFromRules(tt.rules)
			if err != nil {
				t.Fatalf("BuildFromRules() error = %v", err)
			}

			hasMatchers := len(registry.matchers) > 0
			if hasMatchers != tt.expectMatchers {
				t.Errorf("expected matchers = %v, got %v", tt.expectMatchers, hasMatchers)
			}
		})
	}
}

func TestContentMatcherRegistry_MatchContent(t *testing.T) {
	registry := NewContentMatcherRegistry()
	rules := []types.Rule{
		{
			Tech:       "stl",
			Name:       "C++ STL",
			Type:       "library",
			Extensions: []string{".cpp", ".h"},
			Content: []types.ContentRule{
				{Pattern: `#include\s+<vector>`},
				{Pattern: `#include\s+<string>`},
			},
		},
		{
			Tech:       "opengl",
			Name:       "OpenGL",
			Type:       "library",
			Extensions: []string{".cpp", ".c"},
			Content: []types.ContentRule{
				{Pattern: `#include\s+<GL/`},
			},
		},
	}

	err := registry.BuildFromRules(rules)
	if err != nil {
		t.Fatalf("BuildFromRules() error = %v", err)
	}

	tests := []struct {
		name      string
		extension string
		content   string
		wantTechs []string
	}{
		{
			name:      "matches STL vector include",
			extension: ".cpp",
			content:   `#include <vector>\n#include <iostream>`,
			wantTechs: []string{"stl"},
		},
		{
			name:      "matches multiple patterns",
			extension: ".cpp",
			content:   `#include <vector>\n#include <GL/gl.h>`,
			wantTechs: []string{"stl", "opengl"},
		},
		{
			name:      "no match for different extension",
			extension: ".py",
			content:   `#include <vector>`,
			wantTechs: []string{},
		},
		{
			name:      "no match for non-matching content",
			extension: ".cpp",
			content:   `int main() { return 0; }`,
			wantTechs: []string{},
		},
		{
			name:      "matches in .h files",
			extension: ".h",
			content:   `#include <string>\nclass MyClass {};`,
			wantTechs: []string{"stl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.MatchContent(tt.extension, tt.content)

			if len(results) != len(tt.wantTechs) {
				t.Errorf("MatchContent() got %d techs, want %d", len(results), len(tt.wantTechs))
			}

			for _, wantTech := range tt.wantTechs {
				if _, exists := results[wantTech]; !exists {
					t.Errorf("MatchContent() missing expected tech: %s", wantTech)
				}
			}
		})
	}
}

func TestContentMatcherRegistry_HasContentMatchers(t *testing.T) {
	registry := NewContentMatcherRegistry()
	rules := []types.Rule{
		{
			Tech:       "stl",
			Name:       "C++ STL",
			Type:       "library",
			Extensions: []string{".cpp"},
			Content: []types.ContentRule{
				{Pattern: `#include\s+<vector>`},
			},
		},
	}

	err := registry.BuildFromRules(rules)
	if err != nil {
		t.Fatalf("BuildFromRules() error = %v", err)
	}

	tests := []struct {
		name      string
		extension string
		want      bool
	}{
		{
			name:      "has matchers for .cpp",
			extension: ".cpp",
			want:      true,
		},
		{
			name:      "no matchers for .py",
			extension: ".py",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.HasContentMatchers(tt.extension)
			if got != tt.want {
				t.Errorf("HasContentMatchers() = %v, want %v", got, tt.want)
			}
		})
	}
}
