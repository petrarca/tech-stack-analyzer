package parsers

import (
	"bufio"
	"strings"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestConanParser_ExtractDependencies(t *testing.T) {
	parser := NewConanParser()

	tests := []struct {
		name     string
		content  string
		expected []types.Dependency
	}{
		{
			name: "basic requires",
			content: `
				self.requires("openssl/3.2.6")
				self.requires("sqlite3/3.49.1.0")
			`,
			expected: []types.Dependency{
				{Type: "conan", Name: "openssl", Example: "3.2.6"},
				{Type: "conan", Name: "sqlite3", Example: "3.49.1.0"},
			},
		},
		{
			name: "tool requires",
			content: `
				self.tool_requires("cmake/3.25.0")
				self.tool_requires("ninja/1.11.0")
			`,
			expected: []types.Dependency{
				{Type: "conan", Name: "cmake", Example: "3.25.0"},
				{Type: "conan", Name: "ninja", Example: "1.11.0"},
			},
		},
		{
			name: "mixed requires and tool requires",
			content: `
				self.requires("openssl/3.2.6")
				self.tool_requires("cmake/3.25.0")
				self.requires("boost/1.82.0")
			`,
			expected: []types.Dependency{
				{Type: "conan", Name: "openssl", Example: "3.2.6"},
				{Type: "conan", Name: "cmake", Example: "3.25.0"},
				{Type: "conan", Name: "boost", Example: "1.82.0"},
			},
		},
		{
			name: "complex version strings",
			content: `
				self.requires("cgmassist_dev/2.0.0.26001")
				self.requires("libpq/10.7.1942_2")
			`,
			expected: []types.Dependency{
				{Type: "conan", Name: "cgmassist_dev", Example: "2.0.0.26001"},
				{Type: "conan", Name: "libpq", Example: "10.7.1942_2"},
			},
		},
		{
			name: "no dependencies",
			content: `
				class MyRecipe(ConanFile):
					name = "myproject"
					version = "1.0.0"
			`,
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractDependencies(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d dependencies, got %d", len(tt.expected), len(result))
				return
			}

			// Create a map for easier comparison since order might differ
			expectedMap := make(map[string]types.Dependency)
			for _, dep := range tt.expected {
				key := dep.Name + "/" + dep.Example
				expectedMap[key] = dep
			}

			for _, dep := range result {
				key := dep.Name + "/" + dep.Example
				expected, exists := expectedMap[key]
				if !exists {
					t.Errorf("Unexpected dependency found: %s/%s", dep.Name, dep.Example)
					continue
				}
				if dep.Type != expected.Type {
					t.Errorf("Expected type %s, got %s for %s/%s", expected.Type, dep.Type, dep.Name, dep.Example)
				}
			}
		})
	}
}

func TestConanParser_parsePackagesFile(t *testing.T) {
	parser := NewConanParser()

	// Test with medistar-style packages file
	content := `# Common packages
cbox_dev/25.4.1002.0
cgmassist_dev/2.0.0.26001
chartdirector/5.1.0.1
dcmtk/3.5.4.4
openssl/3.2.6
sqlite3/3.49.1.0

# VS2022 specific packages
iqeasy/0.1.30.76402_2
occi/21.15.0
`

	// Create a temporary file for testing
	// For now, we'll test the parsing logic directly
	scanner := bufio.NewScanner(strings.NewReader(content))
	var dependencies []types.Dependency

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "/") {
			dep := parser.ParseConanDependency(line)
			dependencies = append(dependencies, dep)
		}
	}

	expected := []types.Dependency{
		{Type: "conan", Name: "cbox_dev", Example: "25.4.1002.0"},
		{Type: "conan", Name: "cgmassist_dev", Example: "2.0.0.26001"},
		{Type: "conan", Name: "chartdirector", Example: "5.1.0.1"},
		{Type: "conan", Name: "dcmtk", Example: "3.5.4.4"},
		{Type: "conan", Name: "openssl", Example: "3.2.6"},
		{Type: "conan", Name: "sqlite3", Example: "3.49.1.0"},
		{Type: "conan", Name: "iqeasy", Example: "0.1.30.76402_2"},
		{Type: "conan", Name: "occi", Example: "21.15.0"},
	}

	if len(dependencies) != len(expected) {
		t.Errorf("Expected %d dependencies, got %d", len(expected), len(dependencies))
		return
	}

	for i, expected := range expected {
		if dependencies[i].Name != expected.Name {
			t.Errorf("Expected name %s, got %s", expected.Name, dependencies[i].Name)
		}
		if dependencies[i].Example != expected.Example {
			t.Errorf("Expected version %s, got %s", expected.Example, dependencies[i].Example)
		}
	}
}

func TestConanParser_parseConanDependency(t *testing.T) {
	parser := NewConanParser()

	tests := []struct {
		input    string
		expected types.Dependency
	}{
		{
			input:    "openssl/3.2.6",
			expected: types.Dependency{Name: "openssl", Example: "3.2.6"},
		},
		{
			input:    "cgmassist_dev/2.0.0.26001",
			expected: types.Dependency{Name: "cgmassist_dev", Example: "2.0.0.26001"},
		},
		{
			input:    "libpq/10.7.1942_2",
			expected: types.Dependency{Name: "libpq", Example: "10.7.1942_2"},
		},
		{
			input:    "simplepackage",
			expected: types.Dependency{Name: "simplepackage", Example: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parser.ParseConanDependency(tt.input)
			if result.Name != tt.expected.Name {
				t.Errorf("Expected name %s, got %s", tt.expected.Name, result.Name)
			}
			if result.Example != tt.expected.Example {
				t.Errorf("Expected version %s, got %s", tt.expected.Example, result.Example)
			}
		})
	}
}
