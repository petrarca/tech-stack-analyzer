package parsers

import (
	"strings"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDotenvProvider for testing
type MockDotenvProvider struct {
	mock.Mock
}

func (m *MockDotenvProvider) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDotenvProvider) ListDir(path string) ([]types.File, error) {
	args := m.Called(path)
	return args.Get(0).([]types.File), args.Error(1)
}

func (m *MockDotenvProvider) Open(path string) (string, error) {
	args := m.Called(path)
	return args.String(0), args.Error(1)
}

func (m *MockDotenvProvider) Exists(path string) (bool, error) {
	args := m.Called(path)
	return args.Bool(0), args.Error(1)
}

func (m *MockDotenvProvider) IsDir(path string) (bool, error) {
	args := m.Called(path)
	return args.Bool(0), args.Error(1)
}

func (m *MockDotenvProvider) GetBasePath() string {
	args := m.Called()
	return args.String(0)
}

func TestNewDotenvDetector(t *testing.T) {
	provider := &MockDotenvProvider{}
	rules := []types.Rule{}

	detector := NewDotenvDetector(provider, rules)

	assert.NotNil(t, detector, "Should create a new DotenvDetector")
	assert.Equal(t, provider, detector.provider, "Should set provider correctly")
	assert.Equal(t, rules, detector.rules, "Should set rules correctly")
}

func TestDotenvDetector_findDotenvFile(t *testing.T) {
	provider := &MockDotenvProvider{}
	detector := NewDotenvDetector(provider, []types.Rule{})

	tests := []struct {
		name     string
		files    []types.File
		expected *types.File
	}{
		{
			name: "finds .env.example file",
			files: []types.File{
				{Name: ".env.example", Path: "/project/.env.example"},
				{Name: "package.json", Path: "/project/package.json"},
				{Name: "README.md", Path: "/project/README.md"},
			},
			expected: &types.File{Name: ".env.example", Path: "/project/.env.example"},
		},
		{
			name: "no .env.example file",
			files: []types.File{
				{Name: ".env", Path: "/project/.env"},
				{Name: "package.json", Path: "/project/package.json"},
				{Name: "README.md", Path: "/project/README.md"},
			},
			expected: nil,
		},
		{
			name:     "empty file list",
			files:    []types.File{},
			expected: nil,
		},
		{
			name: "multiple .env files",
			files: []types.File{
				{Name: ".env", Path: "/project/.env"},
				{Name: ".env.example", Path: "/project/.env.example"},
				{Name: ".env.local", Path: "/project/.env.local"},
			},
			expected: &types.File{Name: ".env.example", Path: "/project/.env.example"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.findDotenvFile(tt.files)

			if tt.expected == nil {
				assert.Nil(t, result, "Should return nil when .env.example not found")
			} else {
				require.NotNil(t, result, "Should return .env.example file")
				assert.Equal(t, tt.expected.Name, result.Name, "Should return correct file name")
				assert.Equal(t, tt.expected.Path, result.Path, "Should return correct file path")
			}
		})
	}
}

func TestDotenvDetector_extractVarName(t *testing.T) {
	provider := &MockDotenvProvider{}
	detector := NewDotenvDetector(provider, []types.Rule{})

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple variable",
			line:     "DATABASE_URL=postgresql://localhost:5432/mydb",
			expected: "DATABASE_URL",
		},
		{
			name:     "variable with spaces",
			line:     "  API_KEY  =  secret123  ",
			expected: "API_KEY",
		},
		{
			name:     "variable with quotes",
			line:     `SECRET="my secret value"`,
			expected: "SECRET",
		},
		{
			name:     "variable with single quotes",
			line:     `TOKEN='auth token'`,
			expected: "TOKEN",
		},
		{
			name:     "empty line",
			line:     "",
			expected: "",
		},
		{
			name:     "comment line",
			line:     "# This is a comment",
			expected: "",
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: "",
		},
		{
			name:     "comment with whitespace",
			line:     "   # Comment with spaces   ",
			expected: "",
		},
		{
			name:     "line without equals",
			line:     "INVALID_LINE",
			expected: "",
		},
		{
			name:     "line with multiple equals",
			line:     "URL=https://example.com/path?param=value&other=test",
			expected: "URL",
		},
		{
			name:     "variable with special characters",
			line:     "REDIS_URL_2=redis://localhost:6379/2",
			expected: "REDIS_URL_2",
		},
		{
			name:     "variable with numbers",
			line:     "PORT_3000=3000",
			expected: "PORT_3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.extractVarName(tt.line)
			assert.Equal(t, tt.expected, result, "Should extract correct variable name")
		})
	}
}

func TestDotenvDetector_getRelativeFilePath(t *testing.T) {
	provider := &MockDotenvProvider{}
	detector := NewDotenvDetector(provider, []types.Rule{})

	tests := []struct {
		name         string
		basePath     string
		currentPath  string
		fileName     string
		expectedPath string
	}{
		{
			name:         "same directory",
			basePath:     "/project",
			currentPath:  "/project",
			fileName:     ".env.example",
			expectedPath: "/.env.example",
		},
		{
			name:         "subdirectory",
			basePath:     "/project",
			currentPath:  "/project/config",
			fileName:     ".env.example",
			expectedPath: "/config/.env.example",
		},
		{
			name:         "nested subdirectory",
			basePath:     "/project",
			currentPath:  "/project/app/config",
			fileName:     ".env.example",
			expectedPath: "/app/config/.env.example",
		},
		{
			name:         "relative base path",
			basePath:     ".",
			currentPath:  "./config",
			fileName:     ".env.example",
			expectedPath: "/config/.env.example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.getRelativeFilePath(tt.basePath, tt.currentPath, tt.fileName)
			assert.Equal(t, tt.expectedPath, result, "Should return correct relative file path")
		})
	}
}

func TestDotenvDetector_matchesRule(t *testing.T) {
	provider := &MockDotenvProvider{}
	detector := NewDotenvDetector(provider, []types.Rule{})

	tests := []struct {
		name        string
		varName     string
		rule        types.Rule
		shouldMatch bool
	}{
		{
			name:    "matches database rule",
			varName: "DATABASE_URL",
			rule: types.Rule{
				Tech:   "postgresql",
				DotEnv: []string{"DATABASE", "DB"},
			},
			shouldMatch: true,
		},
		{
			name:    "matches redis rule",
			varName: "REDIS_HOST",
			rule: types.Rule{
				Tech:   "redis",
				DotEnv: []string{"REDIS"},
			},
			shouldMatch: true,
		},
		{
			name:    "case insensitive match",
			varName: "database_url",
			rule: types.Rule{
				Tech:   "postgresql",
				DotEnv: []string{"DATABASE", "DB"},
			},
			shouldMatch: true,
		},
		{
			name:    "no match",
			varName: "SOME_OTHER_VAR",
			rule: types.Rule{
				Tech:   "postgresql",
				DotEnv: []string{"DATABASE", "DB"},
			},
			shouldMatch: false,
		},
		{
			name:    "empty dotenv patterns",
			varName: "DATABASE_URL",
			rule: types.Rule{
				Tech:   "some-tech",
				DotEnv: []string{},
			},
			shouldMatch: false,
		},
		{
			name:    "partial match",
			varName: "POSTGRES_DB_NAME",
			rule: types.Rule{
				Tech:   "postgresql",
				DotEnv: []string{"POSTGRES"},
			},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := types.NewPayloadWithPath("test", "/test")
			result := detector.matchesRule(strings.ToLower(tt.varName), tt.varName, tt.rule, payload)

			assert.Equal(t, tt.shouldMatch, result, "Should match rule correctly")

			if tt.shouldMatch {
				assert.NotEmpty(t, payload.Techs, "Should add tech to payload when matched")
				assert.Contains(t, payload.Techs, tt.rule.Tech, "Should add correct tech to payload")
			}
		})
	}
}

func TestDotenvDetector_DetectInDotEnv(t *testing.T) {
	tests := []struct {
		name           string
		files          []types.File
		fileContent    string
		expectedTechs  []string
		shouldFindFile bool
	}{
		{
			name: "detects database from .env.example",
			files: []types.File{
				{Name: ".env.example", Path: "/project/.env.example"},
			},
			fileContent: `DATABASE_URL=postgresql://localhost:5432/mydb
REDIS_URL=redis://localhost:6379
API_KEY=secret123`,
			expectedTechs:  []string{"postgresql", "redis"}, // Assuming rules match these
			shouldFindFile: true,
		},
		{
			name: "no .env.example file",
			files: []types.File{
				{Name: "package.json", Path: "/project/package.json"},
			},
			fileContent:    "",
			expectedTechs:  []string{},
			shouldFindFile: false,
		},
		{
			name: "empty .env.example file",
			files: []types.File{
				{Name: ".env.example", Path: "/project/.env.example"},
			},
			fileContent:    "",
			expectedTechs:  []string{},
			shouldFindFile: true,
		},
		{
			name: ".env.example with only comments",
			files: []types.File{
				{Name: ".env.example", Path: "/project/.env.example"},
			},
			fileContent: `# Database configuration
# API configuration
# Development settings`,
			expectedTechs:  []string{},
			shouldFindFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockDotenvProvider{}

			if tt.shouldFindFile {
				provider.On("ReadFile", "/project/.env.example").Return([]byte(tt.fileContent), nil)
			}

			// Create mock rules for testing
			rules := []types.Rule{
				{Tech: "postgresql", DotEnv: []string{"DATABASE", "DB", "POSTGRES"}},
				{Tech: "redis", DotEnv: []string{"REDIS"}},
				{Tech: "mysql", DotEnv: []string{"MYSQL"}},
			}

			detector := NewDotenvDetector(provider, rules)
			payload := detector.DetectInDotEnv(tt.files, "/project", "/project")

			if tt.shouldFindFile {
				require.NotNil(t, payload, "Should return payload when .env.example is found")
				assert.Equal(t, "/.env.example", payload.Path[0], "Should set correct folder path")

				// Check that expected technologies are detected
				for _, expectedTech := range tt.expectedTechs {
					found := false
					for _, tech := range payload.Techs {
						if strings.Contains(tech, expectedTech) {
							found = true
							break
						}
					}
					if len(tt.expectedTechs) > 0 {
						assert.True(t, found, "Should detect %s technology", expectedTech)
					}
				}
			} else {
				assert.Nil(t, payload, "Should return nil when .env.example not found")
			}

			provider.AssertExpectations(t)
		})
	}
}

func TestDotenvDetector_Integration(t *testing.T) {
	provider := &MockDotenvProvider{}

	// Mock realistic .env.example content
	envContent := `# Database Configuration
DATABASE_URL=postgresql://localhost:5432/myapp
DB_HOST=localhost
DB_PORT=5432
DB_NAME=myapp
DB_USER=postgres
DB_PASSWORD=password

# Redis Configuration
REDIS_URL=redis://localhost:6379/0
REDIS_HOST=localhost
REDIS_PORT=6379

# API Configuration
API_BASE_URL=https://api.example.com
API_KEY=your-api-key-here
API_SECRET=your-api-secret

# JWT Configuration
JWT_SECRET=your-jwt-secret-here
JWT_EXPIRES_IN=24h

# Email Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password

# File Upload Configuration
UPLOAD_PATH=./uploads
MAX_FILE_SIZE=10MB

# Application Configuration
APP_NAME=MyApp
APP_ENV=development
APP_DEBUG=true
APP_URL=http://localhost:3000

# Third-party Services
STRIPE_API_KEY=sk_test_...
GOOGLE_CLIENT_ID=your-google-client-id
FACEBOOK_APP_ID=your-facebook-app-id

# Logging
LOG_LEVEL=info
LOG_FILE=./logs/app.log
`

	provider.On("ReadFile", "/project/.env.example").Return([]byte(envContent), nil)

	rules := []types.Rule{
		{Tech: "postgresql", DotEnv: []string{"DATABASE", "DB", "POSTGRES"}},
		{Tech: "redis", DotEnv: []string{"REDIS"}},
		{Tech: "mysql", DotEnv: []string{"MYSQL"}},
		{Tech: "jwt", DotEnv: []string{"JWT"}},
		{Tech: "stripe", DotEnv: []string{"STRIPE"}},
		{Tech: "google", DotEnv: []string{"GOOGLE"}},
		{Tech: "facebook", DotEnv: []string{"FACEBOOK"}},
		{Tech: "smtp", DotEnv: []string{"SMTP", "MAIL"}},
	}

	detector := NewDotenvDetector(provider, rules)
	payload := detector.DetectInDotEnv([]types.File{
		{Name: ".env.example", Path: "/project/.env.example"},
	}, "/project", "/project")

	require.NotNil(t, payload, "Should return payload")
	assert.Equal(t, "/.env.example", payload.Path[0], "Should set correct folder path")

	// Should detect multiple technologies
	assert.NotEmpty(t, payload.Techs, "Should detect technologies")

	// Check for specific expected technologies
	expectedTechs := []string{"postgresql", "redis", "jwt", "stripe", "google", "facebook", "smtp"}

	for _, expectedTech := range expectedTechs {
		found := false
		for _, tech := range payload.Techs {
			if strings.Contains(tech, expectedTech) {
				found = true
				break
			}
		}
		assert.True(t, found, "Should detect %s technology", expectedTech)
	}

	provider.AssertExpectations(t)
}

func TestDotenvDetector_ErrorHandling(t *testing.T) {
	provider := &MockDotenvProvider{}

	// Test file read error
	provider.On("ReadFile", "/project/.env.example").Return([]byte{}, assert.AnError)

	detector := NewDotenvDetector(provider, []types.Rule{})
	payload := detector.DetectInDotEnv([]types.File{
		{Name: ".env.example", Path: "/project/.env.example"},
	}, "/project", "/project")

	assert.Nil(t, payload, "Should return nil when file read fails")
	provider.AssertExpectations(t)
}
