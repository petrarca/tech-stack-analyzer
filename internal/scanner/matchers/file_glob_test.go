package matchers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"*.go", true},
		{"file?.txt", true},
		{"**/*.js", true},
		{"Makefile", false},
		{"pom.xml", false},
		{"requirements.txt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := isGlobPattern(tt.pattern); got != tt.want {
				t.Errorf("isGlobPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob string
		want string
	}{
		{"*.go", `^[^/]*\.go$`},
		{"file?.txt", `^file[^/]\.txt$`},
		{"**/*.js", `^(?:.*/)?[^/]*\.js$`},
		{"pom.xml", `^pom\.xml$`},
		// Regex special chars are escaped
		{"a+b", `^a\+b$`},
		{"a(b)", `^a\(b\)$`},
		{"a[b]", `^a\[b\]$`},
		{"a{b}", `^a\{b\}$`},
		{"a^b", `^a\^b$`},
		{"a$b", `^a\$b$`},
		{"a|b", `^a\|b$`},
		{`a\b`, `^a\\b$`},
	}

	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			if got := globToRegex(tt.glob); got != tt.want {
				t.Errorf("globToRegex(%q) = %q, want %q", tt.glob, got, tt.want)
			}
		})
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern  string
		fileName string
		want     bool
	}{
		// Single * matches within one segment only (no path separators)
		{"*.go", "main.go", true},
		{"*.go", "main.ts", false},
		{"*.go", "main.go.bak", false},
		{"*.go", "src/main.go", false}, // * does not cross /

		// ? matches a single non-separator character
		{"file?.txt", "file1.txt", true},
		{"file?.txt", "file12.txt", false},
		{"file?.txt", "file.txt", false},

		// Exact match (fast path in matchFileName)
		{"pom.xml", "pom.xml", true},
		{"pom.xml", "other.xml", false},

		// ** matches zero or more path segments (including separators)
		{"**/*.js", "app.js", true},     // zero preceding segments
		{"**/*.js", "src/app.js", true}, // one preceding segment
		{"**/*.js", "a/b/app.js", true}, // multiple segments
		{"**/*.js", "app.ts", false},

		// Regex special chars in pattern are treated as literals
		{"a+b.txt", "a+b.txt", true},
		{"a+b.txt", "aab.txt", false},
		{"a.b", "a.b", true},
		{"a.b", "axb", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_vs_"+tt.fileName, func(t *testing.T) {
			if got := matchGlob(tt.pattern, tt.fileName); got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.fileName, got, tt.want)
			}
		})
	}
}

func TestMatchFileName(t *testing.T) {
	tests := []struct {
		pattern  string
		fileName string
		want     bool
	}{
		// Exact match fast path
		{"Makefile", "Makefile", true},
		{"Makefile", "makefile", false},
		// Glob path
		{"*.go", "main.go", true},
		{"*.go", "main.ts", false},
		// No glob, no exact match
		{"requirements.txt", "setup.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := matchFileName(tt.pattern, tt.fileName); got != tt.want {
				t.Errorf("matchFileName(%q, %q) = %v, want %v", tt.pattern, tt.fileName, got, tt.want)
			}
		})
	}
}

func TestMatchFilePattern(t *testing.T) {
	files := []types.File{
		{Name: "pom.xml"},
		{Name: "build.gradle"},
		{Name: "main.go"},
	}

	tests := []struct {
		pattern   string
		wantMatch bool
		wantFile  string
	}{
		{"pom.xml", true, "pom.xml"},
		{"build.gradle", true, "build.gradle"},
		{"*.go", true, "main.go"},
		{"*.ts", false, ""},
		{"Makefile", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			matched, file := matchFilePattern(tt.pattern, files)
			if matched != tt.wantMatch {
				t.Errorf("matchFilePattern(%q) matched = %v, want %v", tt.pattern, matched, tt.wantMatch)
			}
			if file != tt.wantFile {
				t.Errorf("matchFilePattern(%q) file = %q, want %q", tt.pattern, file, tt.wantFile)
			}
		})
	}
}

func TestIsDirectoryPattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"src/main.go", true},
		{"/services/auth", true},
		{"pom.xml", false},
		{"*.go", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := isDirectoryPattern(tt.pattern); got != tt.want {
				t.Errorf("isDirectoryPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchDirectoryPattern(t *testing.T) {
	tests := []struct {
		pattern     string
		currentPath string
		wantMatch   bool
	}{
		{"/services/auth", "/project/services/auth", true},
		{"/services", "/project/services/auth", false},
		{"/services/auth", "/project/services/billing", false},
		{"services/auth", "project/services/auth", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			matched, _ := matchDirectoryPattern(tt.pattern, tt.currentPath)
			if matched != tt.wantMatch {
				t.Errorf("matchDirectoryPattern(%q, %q) = %v, want %v",
					tt.pattern, tt.currentPath, matched, tt.wantMatch)
			}
		})
	}
}
