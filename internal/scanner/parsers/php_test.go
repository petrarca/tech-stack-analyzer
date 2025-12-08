package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPHPParser(t *testing.T) {
	parser := NewPHPParser()
	assert.NotNil(t, parser, "Should create a new PHPParser")
	assert.IsType(t, &PHPParser{}, parser, "Should return correct type")
}

func TestParseComposerJSON(t *testing.T) {
	parser := NewPHPParser()

	tests := []struct {
		name                string
		content             string
		expectedProjectName string
		expectedLicense     string
		expectedDeps        []types.Dependency
	}{
		{
			name: "valid composer.json with dependencies",
			content: `{
				"name": "myorg/myapp",
				"license": "MIT",
				"require": {
					"php": "^8.0",
					"symfony/console": "^6.0",
					"doctrine/orm": "^2.14"
				},
				"require-dev": {
					"phpunit/phpunit": "^9.0",
					"symfony/phpunit-bridge": "^6.0"
				}
			}`,
			expectedProjectName: "myorg/myapp",
			expectedLicense:     "MIT",
			expectedDeps: []types.Dependency{
				{Type: "php", Name: "php", Example: "^8.0"},
				{Type: "php", Name: "symfony/console", Example: "^6.0"},
				{Type: "php", Name: "doctrine/orm", Example: "^2.14"},
				{Type: "php", Name: "phpunit/phpunit", Example: "^9.0"},
				{Type: "php", Name: "symfony/phpunit-bridge", Example: "^6.0"},
			},
		},
		{
			name: "composer.json with nil require and require-dev",
			content: `{
				"name": "myorg/myapp",
				"license": "MIT",
				"require": null,
				"require-dev": null
			}`,
			expectedProjectName: "myorg/myapp",
			expectedLicense:     "MIT",
			expectedDeps:        []types.Dependency{}, // Should not panic on nil maps
		},
		{
			name: "composer.json with missing require and require-dev",
			content: `{
				"name": "myorg/myapp",
				"license": "MIT"
			}`,
			expectedProjectName: "myorg/myapp",
			expectedLicense:     "MIT",
			expectedDeps:        []types.Dependency{}, // Should not panic on missing fields
		},
		{
			name: "composer.json with only require",
			content: `{
				"name": "myorg/myapp",
				"require": {
					"php": "^8.0",
					"symfony/console": "^6.0"
				}
			}`,
			expectedProjectName: "myorg/myapp",
			expectedLicense:     "",
			expectedDeps: []types.Dependency{
				{Type: "php", Name: "php", Example: "^8.0"},
				{Type: "php", Name: "symfony/console", Example: "^6.0"},
			},
		},
		{
			name: "composer.json with only require-dev",
			content: `{
				"name": "myorg/myapp",
				"require-dev": {
					"phpunit/phpunit": "^9.0"
				}
			}`,
			expectedProjectName: "myorg/myapp",
			expectedLicense:     "",
			expectedDeps: []types.Dependency{
				{Type: "php", Name: "phpunit/phpunit", Example: "^9.0"},
			},
		},
		{
			name:                "empty composer.json",
			content:             `{}`,
			expectedProjectName: "",
			expectedLicense:     "",
			expectedDeps:        []types.Dependency{},
		},
		{
			name: "invalid JSON",
			content: `{
				"name": "myorg/myapp",
				"require": {
					"php": "^8.0"
				}
				// Invalid JSON comment
			}`,
			expectedProjectName: "",
			expectedLicense:     "",
			expectedDeps:        []types.Dependency{}, // Should handle JSON error gracefully
		},
		{
			name:                "empty content",
			content:             "",
			expectedProjectName: "",
			expectedLicense:     "",
			expectedDeps:        []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectName, license, dependencies := parser.ParseComposerJSON(tt.content)

			assert.Equal(t, tt.expectedProjectName, projectName, "Should return correct project name")
			assert.Equal(t, tt.expectedLicense, license, "Should return correct license")

			require.Len(t, dependencies, len(tt.expectedDeps), "Should return correct number of dependencies")

			// Create dependency maps for order-independent comparison
			expectedDepMap := make(map[string]types.Dependency)
			actualDepMap := make(map[string]types.Dependency)

			for _, dep := range tt.expectedDeps {
				expectedDepMap[dep.Name] = dep
			}

			for _, dep := range dependencies {
				actualDepMap[dep.Name] = dep
			}

			// Verify all expected dependencies are present
			for name, expectedDep := range expectedDepMap {
				actualDep, exists := actualDepMap[name]
				require.True(t, exists, "Expected dependency %s not found", name)
				assert.Equal(t, expectedDep.Type, actualDep.Type, "Should have correct type for %s", name)
				assert.Equal(t, expectedDep.Example, actualDep.Example, "Should have correct version for %s", name)
			}
		})
	}
}

func TestParseComposerJSON_EdgeCases(t *testing.T) {
	parser := NewPHPParser()

	// Test that nil require/require-dev don't cause panic
	t.Run("nil dependencies should not panic", func(t *testing.T) {
		content := `{
			"name": "test/app",
			"require": null,
			"require-dev": null
		}`

		// This should not panic
		projectName, license, dependencies := parser.ParseComposerJSON(content)

		assert.Equal(t, "test/app", projectName)
		assert.Equal(t, "", license)
		assert.Empty(t, dependencies)
	})

	// Test that empty dependencies work correctly
	t.Run("empty dependencies", func(t *testing.T) {
		content := `{
			"name": "test/app",
			"require": {},
			"require-dev": {}
		}`

		projectName, license, dependencies := parser.ParseComposerJSON(content)

		assert.Equal(t, "test/app", projectName)
		assert.Equal(t, "", license)
		assert.Empty(t, dependencies)
	})

	// Test malformed JSON doesn't crash
	t.Run("malformed JSON handling", func(t *testing.T) {
		content := `{
			"name": "test/app",
			"require": {
				"php": "^8.0"
			// Missing closing brace
		}`

		// This should not panic, should return empty values
		projectName, license, dependencies := parser.ParseComposerJSON(content)

		assert.Equal(t, "", projectName)
		assert.Equal(t, "", license)
		assert.Empty(t, dependencies)
	})
}

func TestPHPParser_Integration(t *testing.T) {
	parser := NewPHPParser()

	// Test realistic composer.json
	realisticComposer := `{
		"name": "laravel/laravel",
		"type": "project",
		"description": "The Laravel Framework.",
		"keywords": ["framework", "laravel"],
		"license": "MIT",
		"require": {
			"php": "^8.0.2",
			"guzzlehttp/guzzle": "^7.2",
			"laravel/framework": "^9.19",
			"laravel/sanctum": "^3.0",
			"laravel/tinker": "^2.7"
		},
		"require-dev": {
			"fakerphp/faker": "^1.9.1",
			"laravel/pint": "^1.0",
			"laravel/sail": "^1.0.1",
			"mockery/mockery": "^1.4.4",
			"nunomaduro/collision": "^6.0",
			"phpunit/phpunit": "^9.5.10",
			"spatie/laravel-ignition": "^1.0"
		},
		"autoload": {
			"psr-4": {
				"App\\": "app/",
				"Database\\Factories\\": "database/factories/",
				"Database\\Seeders\\": "database/seeders/"
			}
		},
		"minimum-stability": "stable",
		"prefer-stable": true
	}`

	projectName, license, dependencies := parser.ParseComposerJSON(realisticComposer)

	assert.Equal(t, "laravel/laravel", projectName)
	assert.Equal(t, "MIT", license)
	assert.Len(t, dependencies, 12) // 5 require + 7 require-dev

	// Verify some key dependencies
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "php", depMap["php"].Type)
	assert.Equal(t, "^8.0.2", depMap["php"].Example)
	assert.Equal(t, "php", depMap["laravel/framework"].Type)
	assert.Equal(t, "^9.19", depMap["laravel/framework"].Example)
	assert.Equal(t, "php", depMap["phpunit/phpunit"].Type)
	assert.Equal(t, "^9.5.10", depMap["phpunit/phpunit"].Example)
}
