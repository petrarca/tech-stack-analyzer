package matchers

import (
	"fmt"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// RegexContentMatcher handles regex-based content matching (default type)
type RegexContentMatcher struct{}

func (m *RegexContentMatcher) Type() string {
	return "regex"
}

func (m *RegexContentMatcher) Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error) {
	if rule.Pattern == "" {
		return nil, fmt.Errorf("regex content rule requires pattern")
	}

	pattern, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern %q: %w", rule.Pattern, err)
	}

	return &compiledRegexMatcher{
		tech:    tech,
		pattern: pattern,
	}, nil
}

type compiledRegexMatcher struct {
	tech    string
	pattern *regexp.Regexp
}

func (m *compiledRegexMatcher) Match(content string) (bool, string) {
	if m.pattern.MatchString(content) {
		return true, "content matched: " + m.pattern.String()
	}
	return false, ""
}

func (m *compiledRegexMatcher) Tech() string {
	return m.tech
}
