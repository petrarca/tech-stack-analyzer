package matchers

import (
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ContentTypeMatcher is the interface for all content type matchers
// Implement this interface to add new content matching types
type ContentTypeMatcher interface {
	// Type returns the content type identifier (e.g., "regex", "json-path", "json-schema")
	Type() string

	// Compile prepares the matcher from a content rule, returns error if invalid
	Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error)
}

// CompiledContentMatcher is a pre-compiled matcher ready for matching
type CompiledContentMatcher interface {
	// Match checks if the content matches, returns (matched, reason)
	Match(content string) (bool, string)

	// Tech returns the technology this matcher detects
	Tech() string
}

// ContentTypeRegistry manages registered content type matchers
type ContentTypeRegistry struct {
	matchers map[string]ContentTypeMatcher
}

// NewContentTypeRegistry creates a new registry with default matchers
func NewContentTypeRegistry() *ContentTypeRegistry {
	registry := &ContentTypeRegistry{
		matchers: make(map[string]ContentTypeMatcher),
	}

	// Register default matchers
	registry.Register(&RegexContentMatcher{})
	registry.Register(&JSONPathContentMatcher{})
	registry.Register(&YAMLPathContentMatcher{})
	registry.Register(&XMLPathContentMatcher{})

	return registry
}

// Register adds a content type matcher to the registry
func (r *ContentTypeRegistry) Register(matcher ContentTypeMatcher) {
	r.matchers[matcher.Type()] = matcher
}

// Get returns the matcher for a given type, or nil if not found
func (r *ContentTypeRegistry) Get(contentType string) ContentTypeMatcher {
	return r.matchers[contentType]
}

// Compile compiles a content rule using the appropriate matcher
func (r *ContentTypeRegistry) Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error) {
	contentType := rule.GetType()
	matcher := r.Get(contentType)
	if matcher == nil {
		// Fall back to regex for unknown types
		matcher = r.Get("regex")
	}
	return matcher.Compile(rule, tech)
}

// SupportedTypes returns all registered content type names
func (r *ContentTypeRegistry) SupportedTypes() []string {
	types := make([]string, 0, len(r.matchers))
	for t := range r.matchers {
		types = append(types, t)
	}
	return types
}
