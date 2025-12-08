package matchers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ContentMatcherRegistry manages compiled content matchers
type ContentMatcherRegistry struct {
	typeRegistry *ContentTypeRegistry
	matchers     map[string][]CompiledContentMatcher // keyed by extension (e.g., ".cpp", ".h")
	fileMatchers map[string][]CompiledContentMatcher // keyed by filename (e.g., "package.json", "pom.xml")
}

// NewContentMatcherRegistry creates a new content matcher registry
func NewContentMatcherRegistry() *ContentMatcherRegistry {
	return &ContentMatcherRegistry{
		typeRegistry: NewContentTypeRegistry(),
		matchers:     make(map[string][]CompiledContentMatcher),
		fileMatchers: make(map[string][]CompiledContentMatcher),
	}
}

// RegisterContentType adds a custom content type matcher
func (r *ContentMatcherRegistry) RegisterContentType(matcher ContentTypeMatcher) {
	r.typeRegistry.Register(matcher)
}

// BuildFromRules compiles content patterns from rules
func (r *ContentMatcherRegistry) BuildFromRules(rules []types.Rule) error {
	for _, rule := range rules {
		if !r.shouldProcessRule(rule) {
			continue
		}

		for _, contentRule := range rule.Content {
			r.addContentPattern(rule.Tech, contentRule, rule.Extensions)
		}
	}
	return nil
}

func (r *ContentMatcherRegistry) shouldProcessRule(rule types.Rule) bool {
	// Skip rules without content patterns
	if len(rule.Content) == 0 {
		return false
	}

	// Check if any content pattern has extensions or files
	for _, contentRule := range rule.Content {
		if len(contentRule.Extensions) > 0 || len(contentRule.Files) > 0 {
			return true
		}
	}

	// If no content patterns have extensions/files, check if rule has top-level extensions
	// Content patterns can inherit from top-level extensions as fallback
	if len(rule.Extensions) > 0 {
		return true
	}

	// No way to determine which files to check - skip this rule
	return false
}

func (r *ContentMatcherRegistry) addContentPattern(tech string, contentRule types.ContentRule, ruleExtensions []string) {
	// Compile using the type registry (handles regex, json-path, json-schema, yaml-path, etc.)
	compiled, err := r.typeRegistry.Compile(contentRule, tech)
	if err != nil {
		return // Skip invalid patterns
	}

	// If specific files are defined, create file-based matchers
	if len(contentRule.Files) > 0 {
		for _, filename := range contentRule.Files {
			r.fileMatchers[filename] = append(r.fileMatchers[filename], compiled)
		}
		return
	}

	// Otherwise, create extension-based matchers
	targetExtensions := contentRule.Extensions
	if len(targetExtensions) == 0 {
		// If no specific extensions defined, apply to all rule extensions
		targetExtensions = ruleExtensions
	}

	for _, ext := range targetExtensions {
		r.matchers[ext] = append(r.matchers[ext], compiled)
	}
}

// MatchContent checks if content matches any patterns for the given extension
// Returns map of tech -> reasons
// Stops after first match per tech (rule is satisfied with one pattern match)
func (r *ContentMatcherRegistry) MatchContent(extension string, content string) map[string][]string {
	results := make(map[string][]string)

	matchers, exists := r.matchers[extension]
	if !exists {
		return results
	}

	// Check patterns in order - stop after first match per tech
	for _, matcher := range matchers {
		tech := matcher.Tech()
		// Skip if we already matched this tech
		if _, alreadyMatched := results[tech]; alreadyMatched {
			continue
		}

		if matched, reason := matcher.Match(content); matched {
			results[tech] = []string{reason}
		}
	}

	return results
}

// HasContentMatchers checks if there are any content matchers for the given extension
func (r *ContentMatcherRegistry) HasContentMatchers(extension string) bool {
	_, exists := r.matchers[extension]
	return exists
}

// HasFileMatchers checks if there are any content matchers for the given filename
func (r *ContentMatcherRegistry) HasFileMatchers(filename string) bool {
	_, exists := r.fileMatchers[filename]
	return exists
}

// MatchFileContent checks if content matches any patterns for the given filename
// Returns map of tech -> reasons
func (r *ContentMatcherRegistry) MatchFileContent(filename string, content string) map[string][]string {
	results := make(map[string][]string)

	matchers, exists := r.fileMatchers[filename]
	if !exists {
		return results
	}

	// Check patterns in order - stop after first match per tech
	for _, matcher := range matchers {
		tech := matcher.Tech()
		// Skip if we already matched this tech
		if _, alreadyMatched := results[tech]; alreadyMatched {
			continue
		}

		if matched, reason := matcher.Match(content); matched {
			results[tech] = []string{reason}
		}
	}

	return results
}
