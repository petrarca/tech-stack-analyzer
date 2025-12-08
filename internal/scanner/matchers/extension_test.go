package matchers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestExtensionMatcherRegistry(t *testing.T) {
	// Clear any existing matchers
	ClearExtensionMatchers()

	// Initially should be empty
	assert.Empty(t, GetExtensionMatchers(), "Should start with empty registry")

	// Create a test matcher
	testMatcher := func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".test" {
				return "test-tech", ".test", true
			}
		}
		return "", "", false
	}

	// Register matcher
	RegisterExtensionMatcher(testMatcher)

	// Should have one matcher
	matchers := GetExtensionMatchers()
	assert.Len(t, matchers, 1, "Should have one registered matcher")

	// Test the registered matcher
	tech, ext, matched := matchers[0]([]string{".test", ".other"})
	assert.True(t, matched, "Should match .test extension")
	assert.Equal(t, "test-tech", tech, "Should return correct tech")
	assert.Equal(t, ".test", ext, "Should return matched extension")
}

func TestBuildExtensionMatchersFromRules(t *testing.T) {
	// Clear any existing matchers
	ClearExtensionMatchers()

	// Create test rules
	rules := []types.Rule{
		{
			Tech:       "javascript",
			Extensions: []string{".js", ".mjs"},
		},
		{
			Tech:       "python",
			Extensions: []string{".py"},
		},
		{
			Tech:       "no-extensions",
			Extensions: []string{}, // Should be skipped
		},
	}

	// Build matchers from rules
	BuildExtensionMatchersFromRules(rules)

	// Should have 2 matchers (one for each rule with extensions)
	matchers := GetExtensionMatchers()
	assert.Len(t, matchers, 2, "Should create matchers for rules with extensions")
}

func TestCreateExtensionMatcherForRule(t *testing.T) {
	// Test rule with extensions
	rule := types.Rule{
		Tech:       "test-tech",
		Extensions: []string{".ext1", ".ext2"},
	}

	matcher := createExtensionMatcherForRule(rule)

	// Test matching extension
	tech, ext, matched := matcher([]string{".ext1", ".other"})
	assert.True(t, matched, "Should match .ext1")
	assert.Equal(t, "test-tech", tech, "Should return correct tech")
	assert.Equal(t, ".ext1", ext, "Should return matched extension")

	// Test second matching extension
	tech, ext, matched = matcher([]string{".ext2"})
	assert.True(t, matched, "Should match .ext2")
	assert.Equal(t, "test-tech", tech)
	assert.Equal(t, ".ext2", ext)

	// Test no match
	tech, ext, matched = matcher([]string{".other"})
	assert.False(t, matched, "Should not match .other")
	assert.Equal(t, "", tech, "Should return empty tech when no match")
	assert.Equal(t, "", ext, "Should return empty extension when no match")
}

func TestCreateExtensionMatcherForRule_ExtensionNormalization(t *testing.T) {
	// Test rule with extensions that don't start with .
	rule := types.Rule{
		Tech:       "test-tech",
		Extensions: []string{"ext1", ".ext2"}, // Mixed formats
	}

	matcher := createExtensionMatcherForRule(rule)

	// Should match both (ext1 should be normalized to .ext1)
	tech, ext, matched := matcher([]string{".ext1"})
	assert.True(t, matched, "Should match normalized .ext1")
	assert.Equal(t, "test-tech", tech)
	assert.Equal(t, ".ext1", ext)

	tech, ext, matched = matcher([]string{".ext2"})
	assert.True(t, matched, "Should match .ext2")
	assert.Equal(t, "test-tech", tech)
	assert.Equal(t, ".ext2", ext)
}

func TestMatchExtensions(t *testing.T) {
	// Clear and setup matchers
	ClearExtensionMatchers()

	// Register test matchers
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".js" {
				return "javascript", ".js", true
			}
		}
		return "", "", false
	})

	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".py" {
				return "python", ".py", true
			}
		}
		return "", "", false
	})

	// Test with files having different extensions
	files := []types.File{
		{Name: "app.js", Type: "file"},
		{Name: "script.py", Type: "file"},
		{Name: "config.json", Type: "file"},
		{Name: "README.md", Type: "file"},
	}

	result := MatchExtensions(files)

	// Should match both javascript and python
	assert.Len(t, result, 2, "Should match 2 technologies")
	assert.Contains(t, result, "javascript", "Should match javascript")
	assert.Contains(t, result, "python", "Should match python")
	assert.Equal(t, []string{"matched extension: .js"}, result["javascript"], "Should have correct reason for javascript")
	assert.Equal(t, []string{"matched extension: .py"}, result["python"], "Should have correct reason for python")
}

func TestMatchExtensions_EmptyFiles(t *testing.T) {
	ClearExtensionMatchers()

	// Register a matcher
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		return "", "", false
	})

	// Test with empty files
	result := MatchExtensions([]types.File{})
	assert.Empty(t, result, "Should return empty result for no files")
}

func TestMatchExtensions_NoFileExtensions(t *testing.T) {
	ClearExtensionMatchers()

	// Register a matcher
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		return "", "", false
	})

	// Test with files that have no extensions
	files := []types.File{
		{Name: "Makefile", Type: "file"},
		{Name: "Dockerfile", Type: "file"},
		{Name: "README", Type: "file"},
	}

	result := MatchExtensions(files)
	assert.Empty(t, result, "Should return empty result for files without extensions")
}

func TestMatchExtensions_DirectoriesIgnored(t *testing.T) {
	ClearExtensionMatchers()

	// Register matchers for both extensions
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".js" {
				return "javascript", ".js", true
			}
		}
		return "", "", false
	})

	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".py" {
				return "python", ".py", true
			}
		}
		return "", "", false
	})

	// Test with mix of files and directories
	files := []types.File{
		{Name: "app.js", Type: "file"},
		{Name: "src", Type: "directory"},
		{Name: "script.py", Type: "file"},
		{Name: "node_modules", Type: "directory"},
	}

	result := MatchExtensions(files)

	// Should only match files, ignore directories
	assert.Len(t, result, 2, "Should match 2 technologies from files only")
	assert.Contains(t, result, "javascript", "Should match javascript from file")
	assert.Contains(t, result, "python", "Should match python from file")
}

func TestMatchExtensions_NoDuplicateMatches(t *testing.T) {
	ClearExtensionMatchers()

	// Register two matchers that could match the same tech
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".js" {
				return "javascript", ".js", true
			}
		}
		return "", "", false
	})

	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		for _, ext := range extensions {
			if ext == ".mjs" {
				return "javascript", ".mjs", true // Same tech, different extension
			}
		}
		return "", "", false
	})

	// Test with files having both extensions
	files := []types.File{
		{Name: "app.js", Type: "file"},
		{Name: "module.mjs", Type: "file"},
	}

	result := MatchExtensions(files)

	// Should only have one entry for javascript (first match wins)
	assert.Len(t, result, 1, "Should not have duplicate matches for same tech")
	assert.Contains(t, result, "javascript", "Should match javascript")
	// Should be the first match (.js)
	assert.Equal(t, []string{"matched extension: .js"}, result["javascript"], "Should use first match")
}

func TestMatchExtensions_ComplexScenario(t *testing.T) {
	ClearExtensionMatchers()

	// Build matchers from realistic rules
	rules := []types.Rule{
		{Tech: "javascript", Extensions: []string{".js", ".mjs", ".cjs"}},
		{Tech: "typescript", Extensions: []string{".ts", ".tsx"}},
		{Tech: "python", Extensions: []string{".py"}},
		{Tech: "go", Extensions: []string{".go"}},
		{Tech: "rust", Extensions: []string{".rs"}},
	}

	BuildExtensionMatchersFromRules(rules)

	// Test with realistic file set
	files := []types.File{
		{Name: "index.js", Type: "file"},
		{Name: "app.ts", Type: "file"},
		{Name: "main.py", Type: "file"},
		{Name: "server.go", Type: "file"},
		{Name: "lib.rs", Type: "file"},
		{Name: "config.json", Type: "file"},
		{Name: "README.md", Type: "file"},
		{Name: "src", Type: "directory"}, // Should be ignored
	}

	result := MatchExtensions(files)

	// Should match all 5 technologies
	assert.Len(t, result, 5, "Should match 5 technologies")
	assert.Contains(t, result, "javascript", "Should match javascript")
	assert.Contains(t, result, "typescript", "Should match typescript")
	assert.Contains(t, result, "python", "Should match python")
	assert.Contains(t, result, "go", "Should match go")
	assert.Contains(t, result, "rust", "Should match rust")
}

func TestClearExtensionMatchers(t *testing.T) {
	// Save current state
	originalCount := len(GetExtensionMatchers())

	// Register a matcher
	RegisterExtensionMatcher(func(extensions []string) (string, string, bool) {
		return "", "", false
	})

	// Should have one more matcher
	assert.Len(t, GetExtensionMatchers(), originalCount+1, "Should have one additional matcher")

	// Clear matchers
	ClearExtensionMatchers()

	// Should be empty now
	assert.Empty(t, GetExtensionMatchers(), "Should be empty after clearing")
}
