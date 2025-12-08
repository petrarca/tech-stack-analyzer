package matchers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// JSONPathContentMatcher handles JSON path matching
type JSONPathContentMatcher struct{}

func (m *JSONPathContentMatcher) Type() string {
	return "json-path"
}

func (m *JSONPathContentMatcher) Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error) {
	if rule.Path == "" {
		return nil, fmt.Errorf("json-path requires path field")
	}
	return &compiledJSONPathMatcher{
		tech:  tech,
		path:  rule.Path,
		value: rule.Value,
	}, nil
}

type compiledJSONPathMatcher struct {
	tech  string
	path  string
	value string
}

func (m *compiledJSONPathMatcher) Tech() string {
	return m.tech
}

func (m *compiledJSONPathMatcher) Match(content string) (bool, string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return false, ""
	}

	// Navigate the path
	value, found := navigateJSONPath(data, m.path)
	if !found {
		return false, ""
	}

	// If no value specified, just check path exists
	if m.value == "" {
		return true, "json path exists: " + m.path
	}

	// Convert value to string for comparison
	strValue := valueToString(value)

	// Check if value is a regex pattern (starts and ends with /)
	if len(m.value) > 2 && m.value[0] == '/' && m.value[len(m.value)-1] == '/' {
		pattern := m.value[1 : len(m.value)-1]
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, ""
		}
		if re.MatchString(strValue) {
			return true, "json path " + m.path + " matched pattern: " + m.value
		}
		return false, ""
	}

	// Exact match
	if strValue == m.value {
		return true, "json path " + m.path + " matched: " + m.value
	}

	return false, ""
}

// navigateJSONPath navigates a simple JSON path like "$.name" or "$.$schema" or "$.dependencies.react"
func navigateJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	// Remove leading $. if present
	path = strings.TrimPrefix(path, "$.")
	if path == "" || path == "$" {
		return data, true
	}

	parts := strings.Split(path, ".")
	var current interface{} = data

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
		default:
			return nil, false
		}
	}

	return current, true
}

// valueToString converts a JSON value to string for comparison
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}
