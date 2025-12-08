package matchers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"gopkg.in/yaml.v3"
)

// YAMLPathContentMatcher handles YAML path-based content matching
// Supports simple dot-notation paths like "$.name", "$.services.web"
type YAMLPathContentMatcher struct{}

func (m *YAMLPathContentMatcher) Type() string {
	return "yaml-path"
}

func (m *YAMLPathContentMatcher) Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error) {
	if rule.Path == "" {
		return nil, fmt.Errorf("yaml-path content rule requires path")
	}

	// Determine value matcher
	var valueMatcher func(string) bool
	var valueDesc string

	if rule.Value != "" {
		// Check if value is a regex pattern (starts and ends with /)
		if len(rule.Value) > 2 && rule.Value[0] == '/' && rule.Value[len(rule.Value)-1] == '/' {
			pattern, err := regexp.Compile(rule.Value[1 : len(rule.Value)-1])
			if err != nil {
				return nil, fmt.Errorf("invalid regex value %q: %w", rule.Value, err)
			}
			valueMatcher = func(v string) bool { return pattern.MatchString(v) }
			valueDesc = "matches " + rule.Value
		} else {
			// Exact match
			valueMatcher = func(v string) bool { return v == rule.Value }
			valueDesc = "equals " + rule.Value
		}
	} else {
		// Just check path exists
		valueMatcher = func(v string) bool { return true }
		valueDesc = "exists"
	}

	return &compiledYAMLPathMatcher{
		tech:         tech,
		path:         rule.Path,
		valueMatcher: valueMatcher,
		valueDesc:    valueDesc,
	}, nil
}

type compiledYAMLPathMatcher struct {
	tech         string
	path         string
	valueMatcher func(string) bool
	valueDesc    string
}

func (m *compiledYAMLPathMatcher) Match(content string) (bool, string) {
	var data interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return false, ""
	}

	value, found := getYAMLPath(data, m.path)
	if !found {
		return false, ""
	}

	// Convert value to string for comparison
	strValue := fmt.Sprintf("%v", value)
	if m.valueMatcher(strValue) {
		return true, fmt.Sprintf("yaml-path %s %s", m.path, m.valueDesc)
	}

	return false, ""
}

func (m *compiledYAMLPathMatcher) Tech() string {
	return m.tech
}

// getYAMLPath extracts a value from YAML using simple dot notation
func getYAMLPath(data interface{}, path string) (interface{}, bool) {
	// Remove leading "$" if present
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")

	if path == "" {
		return data, true
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		case map[interface{}]interface{}:
			// YAML often uses interface{} keys
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}
