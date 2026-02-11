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
				{Type: "python", Name: "fastapi", Version: "latest"},
				{Type: "python", Name: "requests", Version: ">=2.25.0"},
				{Type: "python", Name: "pydantic", Version: "==1.8.0"},
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
				{Type: "python", Name: "fastapi", Version: ">=0.68.0"},
				{Type: "python", Name: "pytest", Version: ">=6.0.0"},
				{Type: "python", Name: "requests", Version: "latest"},
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
				{Type: "python", Name: "package-name", Version: "latest"},
				{Type: "python", Name: "another-package", Version: ">=1.0.0"},   // Canonical: underscore → hyphen
				{Type: "python", Name: "package-with-dots", Version: "==2.0.0"}, // Canonical: dots → hyphens
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
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
			}
		})
	}
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
	assert.Equal(t, ">=0.68.0", depMap["fastapi"].Version, "FastAPI should have correct version")
	assert.Equal(t, "python", depMap["black"].Type, "Black should be python type")
	assert.Equal(t, "latest", depMap["black"].Version, "Black should have latest version")
}

// Enhanced parser tests for deps.dev features
func TestPythonParser_EnhancedFeatures(t *testing.T) {
	parser := NewPythonParser()

	// Test PEP 508 complex requirements
	tests := []struct {
		name     string
		input    string
		expected []types.Dependency
	}{
		{
			name:  "PEP 508 complex requirements",
			input: "package[extra1,extra2]>=1.0,<2.0; python_version >= '3.8'",
			expected: []types.Dependency{
				{Type: "python", Name: "package", Version: ">=1.0,<2.0", Scope: "prod", Direct: true, SourceFile: "requirements.txt"},
			},
		},
		{
			name:  "Canonical name normalization",
			input: "Django-REST-Framework\nFlask-SQLAlchemy",
			expected: []types.Dependency{
				{Type: "python", Name: "django-rest-framework", Version: "latest", Scope: "prod", Direct: true, SourceFile: "requirements.txt"},
				{Type: "python", Name: "flask-sqlalchemy", Version: "latest", Scope: "prod", Direct: true, SourceFile: "requirements.txt"},
			},
		},
		{
			name:  "Comments and empty lines",
			input: "# This is a comment\n\nrequests>=2.25.0\n# Another comment\nnumpy",
			expected: []types.Dependency{
				{Type: "python", Name: "requests", Version: ">=2.25.0", Scope: "prod", Direct: true, SourceFile: "requirements.txt"},
				{Type: "python", Name: "numpy", Version: "latest", Scope: "prod", Direct: true, SourceFile: "requirements.txt"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseRequirementsTxt(tt.input)
			require.Len(t, result, len(tt.expected), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expected {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
				assert.Equal(t, expectedDep.Scope, result[i].Scope, "Should have correct scope")
				assert.Equal(t, expectedDep.Direct, result[i].Direct, "Should have correct direct flag")
				assert.Equal(t, expectedDep.SourceFile, result[i].Metadata["source"], "Should have correct source file in metadata")
			}
		})
	}
}

func TestPythonParser_CanonicalPackageName(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		input    string
		expected string
	}{
		{"Django", "django"},
		{"Flask-SQLAlchemy", "flask-sqlalchemy"},
		{"requests_oauthlib", "requests-oauthlib"},
		{"Pillow", "pillow"},
		{"django-rest-framework", "django-rest-framework"},
		{"SQLAlchemy", "sqlalchemy"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parser.canonPackageName(tt.input)
			assert.Equal(t, tt.expected, result, "Should normalize package name correctly")
		})
	}
}

func TestPythonParser_PEP508Parsing(t *testing.T) {
	parser := NewPythonParser()

	tests := []struct {
		name        string
		input       string
		expected    PythonDependency
		expectError bool
	}{
		{
			name:  "Simple package",
			input: "requests",
			expected: PythonDependency{
				Name: "requests",
			},
		},
		{
			name:  "Package with version",
			input: "requests>=2.25.0",
			expected: PythonDependency{
				Name:       "requests",
				Constraint: ">=2.25.0",
			},
		},
		{
			name:  "Package with extras",
			input: "django[bcrypt,admin]",
			expected: PythonDependency{
				Name:   "django",
				Extras: "bcrypt,admin",
			},
		},
		{
			name:  "Package with extras and version",
			input: "package[extra1,extra2]>=1.0,<2.0",
			expected: PythonDependency{
				Name:       "package",
				Extras:     "extra1,extra2",
				Constraint: ">=1.0,<2.0",
			},
		},
		{
			name:  "Package with environment marker",
			input: "package>=1.0; python_version >= '3.8'",
			expected: PythonDependency{
				Name:        "package",
				Constraint:  ">=1.0",
				Environment: "python_version >= '3.8'",
			},
		},
		{
			name:  "Complex requirement",
			input: "package[extra1,extra2]>=1.0,<2.0; python_version >= '3.8'",
			expected: PythonDependency{
				Name:        "package",
				Extras:      "extra1,extra2",
				Constraint:  ">=1.0,<2.0",
				Environment: "python_version >= '3.8'",
			},
		},
		{
			name:        "Empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "Invalid - empty name",
			input:       ">=1.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parsePEP508Dependency(tt.input)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid input")
				return
			}

			assert.NoError(t, err, "Should not return error for valid input")
			assert.Equal(t, tt.expected.Name, result.Name, "Should parse name correctly")
			assert.Equal(t, tt.expected.Extras, result.Extras, "Should parse extras correctly")
			assert.Equal(t, tt.expected.Constraint, result.Constraint, "Should parse constraint correctly")
			assert.Equal(t, tt.expected.Environment, result.Environment, "Should parse environment correctly")
		})
	}
}
