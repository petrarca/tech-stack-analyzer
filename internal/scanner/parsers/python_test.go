package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPythonParser(t *testing.T) {
	parser := NewPythonParser()
	assert.NotNil(t, parser, "Should create a new PythonParser")
	assert.IsType(t, &PythonParser{}, parser, "Should return correct type")
}

func TestParsePyprojectTOML(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "standard pyproject.toml with dependencies",
			content: `[build-system]
requires = ["setuptools"]

[project]
name = "test-app"
dependencies = [
    "fastapi",
    "requests>=2.25.0",
    "pydantic==1.8.0",
]

[project.optional-dependencies]
dev = ["pytest", "black"]
`,
			expectedDeps: []types.Dependency{
				{Type: "python", Name: "fastapi", Example: "latest"},
				{Type: "python", Name: "requests", Example: "2.25.0"},
				{Type: "python", Name: "pydantic", Example: "1.8.0"},
			},
		},
		{
			name: "poetry dependencies (not supported by parser)",
			content: `[tool.poetry]
name = "poetry-app"

[tool.poetry.dependencies]
python = "^3.8"
fastapi = "^0.68.0"
requests = ">=2.25.0"
`,
			expectedDeps: []types.Dependency{}, // Parser doesn't handle Poetry dependencies
		},
		{
			name: "empty pyproject.toml",
			content: `[build-system]
requires = ["setuptools"]
`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "pyproject.toml with no dependencies",
			content: `[project]
name = "simple-app"
version = "1.0.0"
`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "complex dependency formats",
			content: `[project]
dependencies = [
    "package-name",
    "another-package>=1.0.0",
    "exact-package==2.0.0",
    "range-package>=1.0.0,<2.0.0",
]
`,
			expectedDeps: []types.Dependency{
				{Type: "python", Name: "package-name", Example: "latest"},
				{Type: "python", Name: "another-package", Example: "1.0.0"},
				{Type: "python", Name: "exact-package", Example: "2.0.0"},
				{Type: "python", Name: "range-package", Example: "1.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParsePyprojectTOML(tt.content)

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Example, result[i].Example, "Should have correct version")
			}
		})
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "basic requirements.txt",
			content: `fastapi
requests>=2.25.0
pydantic==1.8.0
`,
			expectedDeps: []types.Dependency{
				{Type: "python", Name: "fastapi", Example: "latest"},
				{Type: "python", Name: "requests", Example: "2.25.0"},
				{Type: "python", Name: "pydantic", Example: "1.8.0"},
			},
		},
		{
			name: "requirements.txt with comments and empty lines",
			content: `# Production dependencies
fastapi>=0.68.0

# Development dependencies
pytest>=6.0.0

# Empty line above
requests
`,
			expectedDeps: []types.Dependency{
				{Type: "python", Name: "fastapi", Example: "0.68.0"},
				{Type: "python", Name: "pytest", Example: "6.0.0"},
				{Type: "python", Name: "requests", Example: "latest"},
			},
		},
		{
			name:         "empty requirements.txt",
			content:      ``,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "requirements.txt with only comments",
			content: `# This is a comment
# Another comment
`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "complex package names",
			content: `package-name
another_package>=1.0.0
package.with.dots==2.0.0
`,
			expectedDeps: []types.Dependency{
				{Type: "python", Name: "package-name", Example: "latest"},
				{Type: "python", Name: "another_package", Example: "1.0.0"},
				{Type: "python", Name: "package.with.dots", Example: "2.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseRequirementsTxt(tt.content)

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Example, result[i].Example, "Should have correct version")
			}
		})
	}
}

func TestExtractProjectName(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name         string
		content      string
		expectedName string
	}{
		{
			name: "project name in [project] section",
			content: `[project]
name = "my-awesome-app"
version = "1.0.0"
`,
			expectedName: "my-awesome-app",
		},
		{
			name: "project name with single quotes",
			content: `[project]
name = 'my-app'
description = "A test app"
`,
			expectedName: "my-app",
		},
		{
			name: "project name with spaces",
			content: `[project]
name =    "spaced-app"   
version = "1.0.0"
`,
			expectedName: "spaced-app",
		},
		{
			name: "no project name",
			content: `[project]
version = "1.0.0"
description = "No name here"
`,
			expectedName: "",
		},
		{
			name: "project name in different section",
			content: `[tool.poetry]
name = "poetry-app"
version = "1.0.0"
`,
			expectedName: "",
		},
		{
			name:         "empty pyproject.toml",
			content:      ``,
			expectedName: "",
		},
		{
			name: "project name after other sections",
			content: `[build-system]
requires = ["setuptools"]

[project]
name = "late-app"
`,
			expectedName: "late-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractProjectName(tt.content)
			assert.Equal(t, tt.expectedName, result, "Should extract correct project name")
		})
	}
}

func TestDetectLicense(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name            string
		content         string
		expectedLicense string
	}{
		{
			name: "MIT license",
			content: `[project]
name = "test-app"
license = "MIT"
`,
			expectedLicense: "MIT",
		},
		{
			name: "Apache license with hyphen",
			content: `[project]
name = "test-app"
license = "apache-2.0"
`,
			expectedLicense: "Apache-2.0",
		},
		{
			name: "Apache license with space",
			content: `[project]
name = "test-app"
license = "apache 2.0"
`,
			expectedLicense: "Apache-2.0",
		},
		{
			name: "GPL license",
			content: `[project]
name = "test-app"
license = "gpl-3.0"
`,
			expectedLicense: "GPL-3.0",
		},
		{
			name: "BSD license",
			content: `[project]
name = "test-app"
license = "bsd"
`,
			expectedLicense: "BSD",
		},
		{
			name: "BSD-3-Clause license",
			content: `[project]
name = "test-app"
license = "bsd-3-clause"
`,
			expectedLicense: "BSD-3-Clause",
		},
		{
			name: "ISC license",
			content: `[project]
name = "test-app"
license = "isc"
`,
			expectedLicense: "ISC",
		},
		{
			name: "license with single quotes",
			content: `[project]
name = "test-app"
license = 'MIT'
`,
			expectedLicense: "MIT",
		},
		{
			name: "no license",
			content: `[project]
name = "test-app"
version = "1.0.0"
`,
			expectedLicense: "",
		},
		{
			name: "license in different section",
			content: `[tool.poetry]
name = "test-app"
license = "MIT"
`,
			expectedLicense: "",
		},
		{
			name: "unknown license",
			content: `[project]
name = "test-app"
license = "Unknown-License"
`,
			expectedLicense: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := types.NewPayload("test", []string{})
			parser.DetectLicense(tt.content, payload)

			if tt.expectedLicense != "" {
				assert.Contains(t, payload.Licenses, tt.expectedLicense, "Should detect license: %s", tt.expectedLicense)
			} else {
				assert.Empty(t, payload.Licenses, "Should not detect any license")
			}
		})
	}
}

func TestExtractLicenseValue(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name          string
		line          string
		expectedValue string
	}{
		{
			name:          "license with double quotes",
			line:          `license = "MIT"`,
			expectedValue: "MIT",
		},
		{
			name:          "license with single quotes",
			line:          `license = 'Apache-2.0'`,
			expectedValue: "Apache-2.0",
		},
		{
			name:          "license without quotes",
			line:          `license = MIT`,
			expectedValue: "MIT",
		},
		{
			name:          "license with spaces",
			line:          `license =    "GPL-3.0"   `,
			expectedValue: "GPL-3.0",
		},
		{
			name:          "no equals sign",
			line:          `license MIT`,
			expectedValue: "",
		},
		{
			name:          "empty license",
			line:          `license = ""`,
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractLicenseValue(tt.line)
			assert.Equal(t, tt.expectedValue, result, "Should extract correct license value")
		})
	}
}

func TestAddLicenseIfMatch(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name            string
		licenseText     string
		expectedLicense string
		shouldMatch     bool
	}{
		{
			name:            "MIT lowercase",
			licenseText:     "mit",
			expectedLicense: "MIT",
			shouldMatch:     true,
		},
		{
			name:            "MIT uppercase",
			licenseText:     "MIT",
			expectedLicense: "MIT",
			shouldMatch:     true,
		},
		{
			name:            "Apache-2.0",
			licenseText:     "apache-2.0",
			expectedLicense: "Apache-2.0",
			shouldMatch:     true,
		},
		{
			name:            "Apache with space",
			licenseText:     "apache 2.0",
			expectedLicense: "Apache-2.0",
			shouldMatch:     true,
		},
		{
			name:            "GPL-3.0",
			licenseText:     "gpl-3.0",
			expectedLicense: "GPL-3.0",
			shouldMatch:     true,
		},
		{
			name:            "BSD",
			licenseText:     "bsd",
			expectedLicense: "BSD",
			shouldMatch:     true,
		},
		{
			name:            "BSD-3-Clause",
			licenseText:     "bsd-3-clause",
			expectedLicense: "BSD-3-Clause",
			shouldMatch:     true,
		},
		{
			name:            "ISC",
			licenseText:     "isc",
			expectedLicense: "ISC",
			shouldMatch:     true,
		},
		{
			name:            "unknown license",
			licenseText:     "unknown-license",
			expectedLicense: "",
			shouldMatch:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := types.NewPayload("test", []string{})
			result := parser.addLicenseIfMatch(tt.licenseText, payload)

			assert.Equal(t, tt.shouldMatch, result, "Should return correct match result")

			if tt.expectedLicense != "" {
				assert.Contains(t, payload.Licenses, tt.expectedLicense, "Should add license: %s", tt.expectedLicense)
			} else {
				assert.Empty(t, payload.Licenses, "Should not add any license")
			}
		})
	}
}

func TestPythonParser_Integration(t *testing.T) {
	parser := NewPythonParser()

	// Test complete workflow with pyproject.toml
	pyprojectContent := `[project]
name = "integration-test"
version = "1.0.0"
license = "MIT"
dependencies = [
    "fastapi>=0.68.0",
    "requests>=2.25.0",
    "pydantic==1.8.0",
]
`

	// Parse dependencies
	deps := parser.ParsePyprojectTOML(pyprojectContent)
	assert.Len(t, deps, 3, "Should parse 3 dependencies")

	// Extract project name
	name := parser.ExtractProjectName(pyprojectContent)
	assert.Equal(t, "integration-test", name, "Should extract correct project name")

	// Detect license
	payload := types.NewPayload("test", []string{})
	parser.DetectLicense(pyprojectContent, payload)
	assert.Contains(t, payload.Licenses, "MIT", "Should detect MIT license")

	// Verify dependency objects
	depMap := make(map[string]types.Dependency)
	for _, dep := range deps {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "python", depMap["fastapi"].Type, "FastAPI should be python type")
	assert.Equal(t, "0.68.0", depMap["fastapi"].Example, "FastAPI should have correct version")
	assert.Equal(t, "python", depMap["requests"].Type, "Requests should be python type")
	assert.Equal(t, "2.25.0", depMap["requests"].Example, "Requests should have correct version")
}

func TestPythonParser_RequirementsTxtIntegration(t *testing.T) {
	parser := NewPythonParser()

	// Test with requirements.txt
	requirementsContent := `# Production dependencies
fastapi>=0.68.0
requests>=2.25.0

# Development dependencies
pytest>=6.0.0
black
`

	deps := parser.ParseRequirementsTxt(requirementsContent)
	assert.Len(t, deps, 4, "Should parse 4 dependencies")

	// Verify dependency objects
	depMap := make(map[string]types.Dependency)
	for _, dep := range deps {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "python", depMap["fastapi"].Type, "FastAPI should be python type")
	assert.Equal(t, "0.68.0", depMap["fastapi"].Example, "FastAPI should have correct version")
	assert.Equal(t, "python", depMap["black"].Type, "Black should be python type")
	assert.Equal(t, "latest", depMap["black"].Example, "Black should have latest version")
}
