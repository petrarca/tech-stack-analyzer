package matchers

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ExtensionMatcher is a function that matches file extensions and returns the matched tech
type ExtensionMatcher func(extensions []string) (tech string, matchedExt string, matched bool)

// extensionMatcherRegistry holds all registered extension matchers
var extensionMatchers []ExtensionMatcher

// RegisterExtensionMatcher adds an extension matcher to the registry
func RegisterExtensionMatcher(matcher ExtensionMatcher) {
	extensionMatchers = append(extensionMatchers, matcher)
}

// GetExtensionMatchers returns all registered extension matchers
func GetExtensionMatchers() []ExtensionMatcher {
	return extensionMatchers
}

// ClearExtensionMatchers clears all registered extension matchers (useful for testing)
func ClearExtensionMatchers() {
	extensionMatchers = nil
}

// BuildExtensionMatchersFromRules creates extension matchers from rules
func BuildExtensionMatchersFromRules(rules []types.Rule) {
	for _, rule := range rules {
		if len(rule.Extensions) == 0 {
			continue
		}

		// Create matcher for this rule
		RegisterExtensionMatcher(createExtensionMatcherForRule(rule))
	}
}

// createExtensionMatcherForRule creates an extension matcher function for a specific rule
func createExtensionMatcherForRule(rule types.Rule) ExtensionMatcher {
	tech := rule.Tech
	extensions := rule.Extensions

	return func(fileExtensions []string) (string, string, bool) {
		// Check each extension in the rule
		for _, ruleExt := range extensions {
			// Normalize extension (ensure it starts with .)
			if ruleExt[0] != '.' {
				ruleExt = "." + ruleExt
			}

			// Check if this extension exists in the file list
			for _, fileExt := range fileExtensions {
				if fileExt == ruleExt {
					return tech, fileExt, true
				}
			}
		}
		return "", "", false
	}
}

// MatchExtensions runs all extension matchers and returns matched techs
// Returns a map of tech -> reasons
func MatchExtensions(files []types.File) map[string][]string {
	matched := make(map[string][]string)

	// Extract unique extensions from files
	extensionSet := make(map[string]bool)
	for _, file := range files {
		if file.Type == "file" {
			ext := filepath.Ext(file.Name)
			if ext != "" {
				extensionSet[ext] = true
			}
		}
	}

	// Convert set to slice
	var extensions []string
	for ext := range extensionSet {
		extensions = append(extensions, ext)
	}

	// Run all matchers
	for _, matcher := range extensionMatchers {
		if tech, ext, ok := matcher(extensions); ok {
			// Only add if not already matched (like original: if (matched.has(res[0].tech)) { continue; })
			if _, exists := matched[tech]; !exists {
				matched[tech] = []string{"matched extension: " + ext}
			}
		}
	}

	return matched
}
