package matchers

import (
	"regexp"
	"strings"
	"sync"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// regexCache holds compiled regex patterns
var regexCache = sync.Map{}

// FileMatcher is a function that matches files and returns the matched tech and file
type FileMatcher func(files []types.File, currentPath, basePath string) (tech string, matchedFile string, matched bool)

// fileMatcherRegistry holds all registered file matchers
var fileMatchers []FileMatcher

// RegisterFileMatcher adds a file matcher to the registry
func RegisterFileMatcher(matcher FileMatcher) {
	fileMatchers = append(fileMatchers, matcher)
}

// GetFileMatchers returns all registered file matchers
func GetFileMatchers() []FileMatcher {
	return fileMatchers
}

// ClearFileMatchers clears all registered file matchers (useful for testing)
func ClearFileMatchers() {
	fileMatchers = nil
	regexCache.Range(func(key, value interface{}) bool {
		regexCache.Delete(key)
		return true
	})
}

// BuildFileMatchersFromRules creates file matchers from rules
func BuildFileMatchersFromRules(rules []types.Rule) {
	for _, rule := range rules {
		if len(rule.Files) == 0 {
			continue
		}

		// Skip package managers - they should not be promoted to main techs
		if rule.Type == "package_manager" {
			continue
		}

		// Create matcher for this rule
		RegisterFileMatcher(createFileMatcherForRule(rule))
	}
}

// createFileMatcherForRule creates a file matcher function for a specific rule
func createFileMatcherForRule(rule types.Rule) FileMatcher {
	tech := rule.Tech
	patterns := rule.Files

	return func(fileList []types.File, currentPath, basePath string) (string, string, bool) {
		for _, pattern := range patterns {
			if matched, matchedPath := matchPattern(pattern, fileList, currentPath); matched {
				return tech, matchedPath, true
			}
		}
		return "", "", false
	}
}

func matchPattern(pattern string, fileList []types.File, currentPath string) (bool, string) {
	if isDirectoryPattern(pattern) {
		return matchDirectoryPattern(pattern, currentPath)
	}
	return matchFilePattern(pattern, fileList)
}

func isDirectoryPattern(pattern string) bool {
	return strings.Contains(pattern, "/")
}

func matchDirectoryPattern(pattern, currentPath string) (bool, string) {
	if strings.HasSuffix(currentPath, pattern) {
		return true, pattern
	}
	return false, ""
}

func matchFilePattern(pattern string, fileList []types.File) (bool, string) {
	for _, file := range fileList {
		if matched := matchFileName(pattern, file.Name); matched {
			return true, file.Name
		}
	}
	return false, ""
}

func matchFileName(pattern, fileName string) bool {
	// Fast path: exact match (most common case)
	if pattern == fileName {
		return true
	}

	// Check if pattern contains glob characters
	if isGlobPattern(pattern) {
		return matchGlob(pattern, fileName)
	}

	return false
}

// isGlobPattern checks if a string contains glob special characters
func isGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "?")
}

// matchGlob converts a glob pattern to regex and matches
func matchGlob(pattern, fileName string) bool {
	// Convert glob to regex:
	// - Escape regex special chars (except * and ?)
	// - Convert * to .*
	// - Convert ? to .
	regexPattern := globToRegex(pattern)

	// Try to get from cache first
	if cached, ok := regexCache.Load(regexPattern); ok {
		re := cached.(*regexp.Regexp)
		return re.MatchString(fileName)
	}

	// Compile and cache for future use
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}
	regexCache.Store(regexPattern, re)
	return re.MatchString(fileName)
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	var result strings.Builder
	result.WriteString("^")

	for _, char := range glob {
		switch char {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			// Escape regex special characters
			result.WriteString("\\")
			result.WriteRune(char)
		default:
			result.WriteRune(char)
		}
	}

	result.WriteString("$")
	return result.String()
}

// MatchFiles runs all file matchers and returns matched techs
// Returns a map of tech -> reasons
func MatchFiles(files []types.File, currentPath, basePath string) map[string][]string {
	matched := make(map[string][]string)

	for _, matcher := range fileMatchers {
		if tech, file, ok := matcher(files, currentPath, basePath); ok {
			// Only add if not already matched (like original: if (matched.has(res[0].tech)) { continue; })
			if _, exists := matched[tech]; !exists {
				matched[tech] = []string{"matched file: " + file}
			}
		}
	}

	return matched
}
