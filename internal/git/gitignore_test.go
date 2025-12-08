package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "basic patterns",
			content: `# Comment
.venv/
node_modules
dist/
build/
*.log
`,
			expected: []string{".venv", "node_modules", "dist", "build", "*.log"},
		},
		{
			name: "with empty lines and comments",
			content: `# Python
__pycache__/
*.pyc

# Node.js
node_modules

# Build outputs
dist/
build/
`,
			expected: []string{"__pycache__", "*.pyc", "node_modules", "dist", "build"},
		},
		{
			name: "with negation patterns (should be skipped)",
			content: `# Ignore everything
*
# But not this file
!.gitignore
# And not this config
!config.json
`,
			expected: []string{"*"},
		},
		{
			name:     "empty file",
			content:  "",
			expected: []string{},
		},
		{
			name: "only comments",
			content: `# This is a comment
# So is this
	# Indented comment
`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and .gitignore file
			tmpDir := t.TempDir()
			gitignorePath := filepath.Join(tmpDir, ".gitignore")

			err := os.WriteFile(gitignorePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Test loading patterns
			loader := NewGitignoreLoader() // No fallback for testing
			patterns, err := loader.LoadPatterns(tmpDir)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, patterns)
		})
	}
}

func TestLoadPatterns_NoGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewGitignoreLoader()

	patterns, err := loader.LoadPatterns(tmpDir)

	// Should not error and should return empty patterns
	assert.NoError(t, err)
	assert.Empty(t, patterns)
}

func TestLoadPatternsFromFile(t *testing.T) {
	content := `# Test gitignore
.venv/
node_modules
dist/
*.log
`
	expected := []string{".venv", "node_modules", "dist", "*.log"}

	// Create temporary .gitignore file
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	err := os.WriteFile(gitignorePath, []byte(content), 0644)
	require.NoError(t, err)

	// Test loading from specific file
	loader := NewGitignoreLoader()
	patterns, err := loader.LoadPatternsFromFile(gitignorePath)

	require.NoError(t, err)
	assert.Equal(t, expected, patterns)
}

func TestLoadPatterns_RealWorld(t *testing.T) {
	// Test with a realistic .gitignore content
	content := `# Byte-compiled / optimized / DLL files
__pycache__/
*.py[cod]
*$py.class

# C extensions
*.so

# Distribution / packaging
.Python
build/
develop-eggs/
dist/
downloads/
eggs/
.eggs/
lib/
lib64/
parts/
sdist/
var/
wheels/
*.egg-info/
.installed.cfg
*.egg

# Virtual environments
.venv
env/
venv/
ENV/
env.bak/
venv.bak/

# IDEs
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
`
	expected := []string{
		"__pycache__", "*.py[cod]", "*$py.class", "*.so", ".Python",
		"build", "develop-eggs", "dist", "downloads", "eggs", ".eggs",
		"lib", "lib64", "parts", "sdist", "var", "wheels", "*.egg-info",
		".installed.cfg", "*.egg", ".venv", "env", "venv", "ENV",
		"env.bak", "venv.bak", ".vscode", ".idea", "*.swp", "*.swo",
		".DS_Store", "Thumbs.db",
	}

	// Create temporary .gitignore file
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	err := os.WriteFile(gitignorePath, []byte(content), 0644)
	require.NoError(t, err)

	// Test loading patterns
	loader := NewGitignoreLoader()
	patterns, err := loader.LoadPatterns(tmpDir)

	require.NoError(t, err)
	assert.Equal(t, expected, patterns)
}
