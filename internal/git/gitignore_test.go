package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadPatternsFromDir is a test helper that loads raw lines from a .gitignore in a directory.
func loadPatternsFromDir(t *testing.T, dir string) []string {
	t.Helper()
	gitignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return nil
	}
	lines, err := LoadPatternsFromFile(gitignorePath)
	require.NoError(t, err)
	return lines
}

func TestLoadPatternsFromFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "basic patterns (trailing slashes preserved)",
			content: `# Comment
.venv/
node_modules
dist/
build/
*.log
`,
			expected: []string{".venv/", "node_modules", "dist/", "build/", "*.log"},
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
			expected: []string{"__pycache__/", "*.pyc", "node_modules", "dist/", "build/"},
		},
		{
			name: "with negation patterns (preserved)",
			content: `# Ignore everything
*
# But not this file
!.gitignore
# And not this config
!config.json
`,
			expected: []string{"*", "!.gitignore", "!config.json"},
		},
		{
			name:     "empty file",
			content:  "",
			expected: nil,
		},
		{
			name: "only comments",
			content: `# This is a comment
# So is this
	# Indented comment
`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gitignorePath := filepath.Join(tmpDir, ".gitignore")

			err := os.WriteFile(gitignorePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			lines, err := LoadPatternsFromFile(gitignorePath)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, lines)
		})
	}
}

func TestLoadPatternsFromFile_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	lines := loadPatternsFromDir(t, tmpDir)
	assert.Empty(t, lines)
}

func TestLoadPatternsFromFile_RealWorld(t *testing.T) {
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
		"__pycache__/", "*.py[cod]", "*$py.class", "*.so", ".Python",
		"build/", "develop-eggs/", "dist/", "downloads/", "eggs/", ".eggs/",
		"lib/", "lib64/", "parts/", "sdist/", "var/", "wheels/", "*.egg-info/",
		".installed.cfg", "*.egg", ".venv", "env/", "venv/", "ENV/",
		"env.bak/", "venv.bak/", ".vscode/", ".idea/", "*.swp", "*.swo",
		".DS_Store", "Thumbs.db",
	}

	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err := os.WriteFile(gitignorePath, []byte(content), 0644)
	require.NoError(t, err)

	lines, err := LoadPatternsFromFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, expected, lines)
}

// --- ParsePatterns tests ---

func TestParsePatterns(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected []Pattern
	}{
		{
			name:  "basic patterns",
			lines: []string{"*.log", "build/", "node_modules"},
			expected: []Pattern{
				{Glob: "*.log", Negate: false, DirOnly: false},
				{Glob: "build", Negate: false, DirOnly: true},
				{Glob: "node_modules", Negate: false, DirOnly: false},
			},
		},
		{
			name:  "negation patterns",
			lines: []string{"*", "!.gitignore", "!src/"},
			expected: []Pattern{
				{Glob: "*", Negate: false, DirOnly: false},
				{Glob: ".gitignore", Negate: true, DirOnly: false},
				{Glob: "src", Negate: true, DirOnly: true},
			},
		},
		{
			name:  "comments and blanks are skipped",
			lines: []string{"# comment", "", "  ", "*.log"},
			expected: []Pattern{
				{Glob: "*.log", Negate: false, DirOnly: false},
			},
		},
		{
			name:     "empty input",
			lines:    []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePatterns(tt.lines)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- ShouldExclude tests ---

func TestShouldExclude_LastMatchWins(t *testing.T) {
	gs := NewGitignoreStack()
	gs.Push("/project", []string{"*", "!.NET/**"})

	assert.False(t, gs.ShouldExclude(".NET", ".NET", true))
	assert.False(t, gs.ShouldExclude("foo.cs", ".NET/foo.cs", false))
	assert.True(t, gs.ShouldExclude("bar.txt", "bar.txt", false))
	assert.True(t, gs.ShouldExclude("src", "src", true))
}

func TestShouldExclude_DirOnlyPattern(t *testing.T) {
	gs := NewGitignoreStack()
	gs.Push("/project", []string{"build/"})

	assert.True(t, gs.ShouldExclude("build", "build", true))
	assert.False(t, gs.ShouldExclude("build", "build", false))
}

func TestShouldExclude_NegationReIncludes(t *testing.T) {
	gs := NewGitignoreStack()
	gs.Push("/project", []string{"vendor/**", "!vendor/important.go"})

	assert.True(t, gs.ShouldExclude("junk.go", "vendor/junk.go", false))
	assert.False(t, gs.ShouldExclude("important.go", "vendor/important.go", false))
}

func TestShouldExclude_StackLayers(t *testing.T) {
	gs := NewGitignoreStack()
	gs.Push("/project", []string{"*.log"})
	gs.Push("/project/sub", []string{"!debug.log"})

	assert.True(t, gs.ShouldExclude("error.log", "sub/error.log", false))
	assert.False(t, gs.ShouldExclude("debug.log", "sub/debug.log", false))
}

func TestShouldExclude_NoPatterns(t *testing.T) {
	gs := NewGitignoreStack()
	assert.False(t, gs.ShouldExclude("anything.txt", "anything.txt", false))
}
