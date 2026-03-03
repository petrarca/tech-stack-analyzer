package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNodeJSParser(t *testing.T) {
	parser := NewNodeJSParser()
	assert.NotNil(t, parser, "Should create a new NodeJSParser")
	assert.IsType(t, &NodeJSParser{}, parser, "Should return correct type")
}

func TestParsePackageJSON(t *testing.T) {
	parser := NewNodeJSParser()

	tests := []struct {
		name        string
		content     []byte
		expectError bool
		expected    *PackageJSON
	}{
		{
			name: "valid package.json",
			content: []byte(`{
				"name": "test-app",
				"dependencies": {
					"express": "^4.18.0",
					"lodash": "~4.17.21"
				},
				"devDependencies": {
					"jest": "^29.0.0",
					"nodemon": "^2.0.20"
				}
			}`),
			expectError: false,
			expected: &PackageJSON{
				Name: "test-app",
				Dependencies: map[string]string{
					"express": "^4.18.0",
					"lodash":  "~4.17.21",
				},
				DevDependencies: map[string]string{
					"jest":    "^29.0.0",
					"nodemon": "^2.0.20",
				},
			},
		},
		{
			name: "minimal package.json",
			content: []byte(`{
				"name": "minimal-app"
			}`),
			expectError: false,
			expected: &PackageJSON{
				Name:            "minimal-app",
				Dependencies:    nil,
				DevDependencies: nil,
			},
		},
		{
			name: "package.json with only dependencies",
			content: []byte(`{
				"dependencies": {
					"express": "^4.18.0"
				}
			}`),
			expectError: false,
			expected: &PackageJSON{
				Name: "",
				Dependencies: map[string]string{
					"express": "^4.18.0",
				},
				DevDependencies: nil,
			},
		},
		{
			name: "package.json with only devDependencies",
			content: []byte(`{
				"devDependencies": {
					"jest": "^29.0.0"
				}
			}`),
			expectError: false,
			expected: &PackageJSON{
				Name:         "",
				Dependencies: nil,
				DevDependencies: map[string]string{
					"jest": "^29.0.0",
				},
			},
		},
		{
			name:        "invalid JSON",
			content:     []byte(`{ invalid json }`),
			expectError: true,
			expected:    nil,
		},
		{
			name:        "empty content",
			content:     []byte(``),
			expectError: true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePackageJSON(tt.content)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid JSON")
				assert.Nil(t, result, "Should return nil on error")
			} else {
				assert.NoError(t, err, "Should not return error for valid JSON")
				require.NotNil(t, result, "Should return parsed structure")
				assert.Equal(t, tt.expected.Name, result.Name, "Should parse name correctly")
				assert.Equal(t, tt.expected.Dependencies, result.Dependencies, "Should parse dependencies correctly")
				assert.Equal(t, tt.expected.DevDependencies, result.DevDependencies, "Should parse devDependencies correctly")
			}
		})
	}
}

func TestExtractDependencies(t *testing.T) {
	parser := NewNodeJSParser()

	tests := []struct {
		name         string
		packageJSON  *PackageJSON
		expectedDeps []string
	}{
		{
			name: "both dependencies and devDependencies",
			packageJSON: &PackageJSON{
				Dependencies: map[string]string{
					"express": "^4.18.0",
					"lodash":  "~4.17.21",
				},
				DevDependencies: map[string]string{
					"jest":    "^29.0.0",
					"nodemon": "^2.0.20",
				},
			},
			expectedDeps: []string{"express", "lodash", "jest", "nodemon"},
		},
		{
			name: "only dependencies",
			packageJSON: &PackageJSON{
				Dependencies: map[string]string{
					"express": "^4.18.0",
				},
				DevDependencies: nil,
			},
			expectedDeps: []string{"express"},
		},
		{
			name: "only devDependencies",
			packageJSON: &PackageJSON{
				Dependencies: nil,
				DevDependencies: map[string]string{
					"jest": "^29.0.0",
				},
			},
			expectedDeps: []string{"jest"},
		},
		{
			name: "no dependencies",
			packageJSON: &PackageJSON{
				Dependencies:    nil,
				DevDependencies: nil,
			},
			expectedDeps: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractDependencies(tt.packageJSON)

			// Convert to maps for comparison since order isn't guaranteed
			resultMap := make(map[string]bool)
			for _, dep := range result {
				resultMap[dep] = true
			}

			expectedMap := make(map[string]bool)
			for _, dep := range tt.expectedDeps {
				expectedMap[dep] = true
			}

			assert.Equal(t, len(expectedMap), len(resultMap), "Should return correct number of dependencies")
			for dep := range expectedMap {
				assert.True(t, resultMap[dep], "Should include dependency: %s", dep)
			}
		})
	}
}

func TestCreateDependencies(t *testing.T) {
	parser := NewNodeJSParser()

	tests := []struct {
		name         string
		packageJSON  *PackageJSON
		depNames     []string
		expectedDeps []types.Dependency
	}{
		{
			name: "mixed dependencies",
			packageJSON: &PackageJSON{
				Dependencies: map[string]string{
					"express": "^4.18.0",
					"lodash":  "~4.17.21",
				},
				DevDependencies: map[string]string{
					"jest":    "^29.0.0",
					"nodemon": "^2.0.20",
				},
			},
			depNames: []string{"express", "jest"},
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "express", Version: "^4.18.0"},
				{Type: "npm", Name: "jest", Version: "^29.0.0"},
			},
		},
		{
			name: "only regular dependencies",
			packageJSON: &PackageJSON{
				Dependencies: map[string]string{
					"express": "^4.18.0",
				},
				DevDependencies: nil,
			},
			depNames: []string{"express"},
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "express", Version: "^4.18.0"},
			},
		},
		{
			name: "only dev dependencies",
			packageJSON: &PackageJSON{
				Dependencies: nil,
				DevDependencies: map[string]string{
					"jest": "^29.0.0",
				},
			},
			depNames: []string{"jest"},
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "jest", Version: "^29.0.0"},
			},
		},
		{
			name: "non-existent dependency",
			packageJSON: &PackageJSON{
				Dependencies:    nil,
				DevDependencies: nil,
			},
			depNames: []string{"non-existent"},
			expectedDeps: []types.Dependency{
				{Type: "npm", Name: "non-existent", Version: ""},
			},
		},
		{
			name: "no dependencies",
			packageJSON: &PackageJSON{
				Dependencies:    nil,
				DevDependencies: nil,
			},
			depNames:     []string{},
			expectedDeps: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CreateDependencies(tt.packageJSON, tt.depNames)

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
			}
		})
	}
}

func TestNodeJSParser_Integration(t *testing.T) {
	parser := NewNodeJSParser()

	// Test complete workflow
	content := []byte(`{
		"name": "integration-test",
		"dependencies": {
			"express": "^4.18.0",
			"cors": "^2.8.5"
		},
		"devDependencies": {
			"jest": "^29.0.0",
			"supertest": "^6.3.0"
		}
	}`)

	// Parse the package.json
	pkg, err := parser.ParsePackageJSON(content)
	require.NoError(t, err, "Should parse package.json successfully")
	require.NotNil(t, pkg, "Should return parsed package")

	// Extract dependencies
	depNames := parser.ExtractDependencies(pkg)
	assert.Len(t, depNames, 4, "Should extract 4 dependencies")

	// Verify package name
	assert.Equal(t, "integration-test", pkg.Name, "Should have correct package name")

	// Create dependency objects
	dependencies := parser.CreateDependencies(pkg, depNames)
	assert.Len(t, dependencies, 4, "Should create 4 dependency objects")

	// Verify dependency objects
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "npm", depMap["express"].Type, "Express should be npm type")
	assert.Equal(t, "^4.18.0", depMap["express"].Version, "Express should have correct version")
	assert.Equal(t, "npm", depMap["jest"].Type, "Jest should be npm type")
	assert.Equal(t, "^29.0.0", depMap["jest"].Version, "Jest should have correct version")
}
