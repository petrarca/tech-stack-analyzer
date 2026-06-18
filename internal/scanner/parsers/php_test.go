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
				// php is a platform requirement and must be filtered out
				{Type: "composer", Name: "symfony/console", Version: "^6.0"},
				{Type: "composer", Name: "doctrine/orm", Version: "^2.14"},
				{Type: "composer", Name: "phpunit/phpunit", Version: "^9.0"},
				{Type: "composer", Name: "symfony/phpunit-bridge", Version: "^6.0"},
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
				// php is a platform requirement and must be filtered out
				{Type: "composer", Name: "symfony/console", Version: "^6.0"},
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
				{Type: "composer", Name: "phpunit/phpunit", Version: "^9.0"},
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
				assert.Equal(t, expectedDep.Version, actualDep.Version, "Should have correct version for %s", name)
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
	assert.Len(t, dependencies, 11) // 4 require (php filtered) + 7 require-dev

	// Verify some key dependencies
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// php is a platform requirement and must not appear
	assert.Empty(t, depMap["php"].Name, "php platform requirement must be filtered")
	assert.Equal(t, "composer", depMap["laravel/framework"].Type)
	assert.Equal(t, "^9.19", depMap["laravel/framework"].Version)
	assert.Equal(t, "composer", depMap["phpunit/phpunit"].Type)
	assert.Equal(t, "^9.5.10", depMap["phpunit/phpunit"].Version)
}

// TestParseComposerJSON_PlatformRequirementsFiltered verifies that PHP platform
// requirements (php, hhvm, ext-*, lib-*, php-*) are excluded from the
// dependency list. They are runtime/environment constraints, not installable
// packages, and must not appear as SBOM components. Matches Trivy's behaviour.
func TestParseComposerJSON_PlatformRequirementsFiltered(t *testing.T) {
	content := `{
		"name": "myorg/myapp",
		"require": {
			"php": "^8.1",
			"ext-curl": "*",
			"ext-json": "*",
			"lib-openssl": "*",
			"php-64bit": "*",
			"hhvm": ">=3.0",
			"guzzlehttp/guzzle": "^7.0",
			"laravel/framework": "^10.0"
		},
		"require-dev": {
			"php": ">=7.4",
			"ext-xdebug": "*",
			"phpunit/phpunit": "^10.0"
		}
	}`
	p := NewPHPParser()
	_, _, deps := p.ParseComposerJSON(content)

	got := map[string]bool{}
	for _, d := range deps {
		got[d.Name] = true
	}

	// Real packages must be present.
	for _, want := range []string{"guzzlehttp/guzzle", "laravel/framework", "phpunit/phpunit"} {
		if !got[want] {
			t.Errorf("expected package %q not found in deps: %v", want, got)
		}
	}

	// Platform requirements must be absent.
	for _, banned := range []string{"php", "hhvm", "ext-curl", "ext-json", "ext-xdebug", "lib-openssl", "php-64bit"} {
		if got[banned] {
			t.Errorf("platform requirement %q must be filtered out, but was emitted as a dependency", banned)
		}
	}

	// Sanity: only the 3 real packages should remain.
	if len(deps) != 3 {
		t.Errorf("expected 3 real deps, got %d: %v", len(deps), got)
	}
}
