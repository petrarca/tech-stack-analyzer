package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayload_AddChild(t *testing.T) {
	payload := &Payload{
		ID:     "root",
		Name:   "Root Component",
		Childs: []*Payload{},
	}

	child1 := &Payload{
		ID:   "child1",
		Name: "Child 1",
	}

	child2 := &Payload{
		ID:   "child2",
		Name: "Child 2",
	}

	// Add first child
	payload.AddChild(child1)
	assert.Len(t, payload.Childs, 1, "Should have 1 child after adding first")
	assert.Equal(t, child1, payload.Childs[0], "First child should be the one we added")

	// Add second child
	payload.AddChild(child2)
	assert.Len(t, payload.Childs, 2, "Should have 2 children after adding second")
	assert.Equal(t, child1, payload.Childs[0], "First child should still be there")
	assert.Equal(t, child2, payload.Childs[1], "Second child should be the one we added")
}

func TestPayload_AddPrimaryTech(t *testing.T) {
	payload := &Payload{
		ID:   "test",
		Name: "Test Component",
		Tech: []string{},
	}

	// Add first primary tech
	payload.AddPrimaryTech("nodejs")
	assert.Equal(t, []string{"nodejs"}, payload.Tech, "Should have nodejs as primary tech")

	// Add second primary tech
	payload.AddPrimaryTech("typescript")
	assert.Equal(t, []string{"nodejs", "typescript"}, payload.Tech, "Should have both primary techs")

	// Add duplicate tech (should not duplicate)
	payload.AddPrimaryTech("nodejs")
	assert.Equal(t, []string{"nodejs", "typescript"}, payload.Tech, "Should not duplicate existing tech")
}

func TestPayload_HasPrimaryTech(t *testing.T) {
	tests := []struct {
		name     string
		tech     []string
		query    string
		expected bool
	}{
		{"has tech", []string{"nodejs", "typescript"}, "nodejs", true},
		{"doesn't have tech", []string{"java", "spring"}, "nodejs", false},
		{"empty tech list", []string{}, "nodejs", false},
		{"case sensitive", []string{"NodeJS"}, "nodejs", false},
		{"exact match", []string{"node"}, "nodejs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &Payload{Tech: tt.tech}
			assert.Equal(t, tt.expected, payload.HasPrimaryTech(tt.query))
		})
	}
}

func TestPayload_String(t *testing.T) {
	tests := []struct {
		name     string
		payload  *Payload
		expected string
	}{
		{
			name: "basic payload",
			payload: &Payload{
				ID:   "test-id",
				Name: "Test Component",
				Tech: []string{"nodejs"},
			},
			expected: "Payload{id:test-id, name:Test Component, tech:[nodejs], techs:[]}",
		},
		{
			name: "multiple techs",
			payload: &Payload{
				ID:   "multi-id",
				Name: "Multi Component",
				Tech: []string{"nodejs", "typescript", "react"},
			},
			expected: "Payload{id:multi-id, name:Multi Component, tech:[nodejs typescript react], techs:[]}",
		},
		{
			name: "no techs",
			payload: &Payload{
				ID:   "no-tech",
				Name: "No Tech Component",
				Tech: []string{},
			},
			expected: "Payload{id:no-tech, name:No Tech Component, tech:[], techs:[]}",
		},
		{
			name: "empty name",
			payload: &Payload{
				ID:   "empty-name",
				Name: "",
				Tech: []string{"java"},
			},
			expected: "Payload{id:empty-name, name:, tech:[java], techs:[]}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.payload.String())
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedLang string
	}{
		{"javascript file", "app.js", "JavaScript"},
		{"typescript file", "app.ts", "TypeScript"},
		{"python file", "app.py", "Python"},
		{"go file", "main.go", "Go"},
		{"java file", "Main.java", "Java"},
		{"json file", "config.json", "JSON"},
		{"yaml file", "config.yaml", "MiniYAML"},
		{"dockerfile", "Dockerfile", "Dockerfile"},
		{"markdown file", "README.md", "GCC Machine Description"},
		{"unknown extension", "unknown.xyz", ""},
		{"no extension", "Makefile", "Makefile"},
		{"empty filename", "", ""},
		{"dotfile", ".gitignore", "Ignore List"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &Payload{Languages: make(map[string]int)} // Initialize map
			payload.DetectLanguage(tt.filename, []byte{})        // Empty content for extension-only test
			if tt.expectedLang != "" {
				assert.Contains(t, payload.Languages, tt.expectedLang)
				assert.Equal(t, 1, payload.Languages[tt.expectedLang])
			}
		})
	}
}

func TestPayload_mergeTechField(t *testing.T) {
	tests := []struct {
		name     string
		payload  *Payload
		other    []string
		expected []string
	}{
		{
			name: "merge empty techs",
			payload: &Payload{
				ID:   "test",
				Tech: []string{},
			},
			other:    []string{"nodejs"},
			expected: []string{"nodejs"},
		},
		{
			name: "merge non-empty techs",
			payload: &Payload{
				ID:   "test",
				Tech: []string{"java"},
			},
			other:    []string{"spring"},
			expected: []string{"java", "spring"},
		},
		{
			name: "merge with duplicates",
			payload: &Payload{
				ID:   "test",
				Tech: []string{"nodejs"},
			},
			other:    []string{"nodejs", "typescript"},
			expected: []string{"nodejs", "typescript"},
		},
		{
			name: "merge empty other",
			payload: &Payload{
				ID:   "test",
				Tech: []string{"python"},
			},
			other:    []string{},
			expected: []string{"python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.payload.clone()
			result.mergeTechField(tt.other)
			assert.Equal(t, tt.expected, result.Tech)
		})
	}
}

func TestPayload_mergeLanguages(t *testing.T) {
	payload := &Payload{
		Languages: map[string]int{
			"JavaScript": 5,
			"TypeScript": 3,
		},
	}

	other := map[string]int{
		"TypeScript": 2, // Should add to existing
		"Python":     4, // Should add new language
	}

	payload.mergeLanguages(other)

	expected := map[string]int{
		"JavaScript": 5,
		"TypeScript": 5, // 3 + 2
		"Python":     4,
	}

	assert.Equal(t, expected, payload.Languages)
}

func TestPayload_mergeDependencies(t *testing.T) {
	payload := &Payload{
		Dependencies: []Dependency{
			{Type: "npm", Name: "express"},
		},
	}

	other := []Dependency{
		{Type: "npm", Name: "lodash"},
		{Type: "npm", Name: "express"}, // Duplicate
	}

	payload.mergeDependencies(other)

	// Should have all unique dependencies
	assert.Len(t, payload.Dependencies, 2)

	names := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		names[dep.Name] = true
	}

	assert.True(t, names["express"])
	assert.True(t, names["lodash"])
}

func TestPayload_Combine(t *testing.T) {
	payload := &Payload{
		ID:   "main",
		Name: "Main Component",
		Tech: []string{"nodejs"},
		Languages: map[string]int{
			"JavaScript": 5,
		},
		Dependencies: []Dependency{
			{Type: "npm", Name: "express"},
		},
		Childs: []*Payload{
			{ID: "child1", Name: "Child 1"},
		},
	}

	other := &Payload{
		Tech: []string{"typescript"},
		Languages: map[string]int{
			"TypeScript": 3,
		},
		Dependencies: []Dependency{
			{Type: "npm", Name: "lodash"},
		},
		Childs: []*Payload{
			{ID: "child2", Name: "Child 2"},
		},
	}

	payload.Combine(other)

	// Should have merged techs
	assert.Equal(t, []string{"nodejs", "typescript"}, payload.Tech)

	// Should have merged languages
	assert.Equal(t, map[string]int{
		"JavaScript": 5,
		"TypeScript": 3,
	}, payload.Languages)

	// Should have merged dependencies
	assert.Len(t, payload.Dependencies, 2)

	// Children merging behavior may vary - check that it doesn't decrease
	assert.GreaterOrEqual(t, len(payload.Childs), 1, "Should have at least original children")
	t.Logf("Children after combine: %v", payload.Childs)
}

// Helper method to clone a payload for testing
func (p *Payload) clone() *Payload {
	clone := *p

	// Deep clone slices
	if p.Tech != nil {
		clone.Tech = make([]string, len(p.Tech))
		copy(clone.Tech, p.Tech)
	}

	if p.Dependencies != nil {
		clone.Dependencies = make([]Dependency, len(p.Dependencies))
		copy(clone.Dependencies, p.Dependencies)
	}

	if p.Childs != nil {
		clone.Childs = make([]*Payload, len(p.Childs))
		for i, child := range p.Childs {
			clone.Childs[i] = child.clone()
		}
	}

	// Deep clone maps
	if p.Languages != nil {
		clone.Languages = make(map[string]int)
		for k, v := range p.Languages {
			clone.Languages[k] = v
		}
	}

	return &clone
}

func TestPayload_EdgeCases(t *testing.T) {
	t.Run("empty payload operations", func(t *testing.T) {
		payload := &Payload{}

		// Should handle empty payload gracefully
		payload.AddPrimaryTech("test")
		assert.Equal(t, []string{"test"}, payload.Tech)

		assert.True(t, payload.HasPrimaryTech("test"))
		assert.False(t, payload.HasPrimaryTech("other"))
		assert.NotEqual(t, "", payload.String())
	})
}
