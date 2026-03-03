package matchers

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// XMLPathContentMatcher handles XML XPath-like content matching
type XMLPathContentMatcher struct{}

func (m *XMLPathContentMatcher) Type() string {
	return "xml-path"
}

func (m *XMLPathContentMatcher) Compile(rule types.ContentRule, tech string) (CompiledContentMatcher, error) {
	if rule.Path == "" {
		return nil, fmt.Errorf("xml-path content rule requires path field")
	}
	return &compiledXMLPathMatcher{
		tech:  tech,
		path:  rule.Path,
		value: rule.Value,
	}, nil
}

type compiledXMLPathMatcher struct {
	tech  string
	path  string
	value string
}

func (m *compiledXMLPathMatcher) Tech() string {
	return m.tech
}

func (m *compiledXMLPathMatcher) Match(content string) (bool, string) {
	// Simple XML parsing for basic path matching
	decoder := xml.NewDecoder(strings.NewReader(content))

	// Track current path
	var currentPath []string
	var currentElement string

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			currentElement = se.Name.Local
			currentPath = append(currentPath, currentElement)

			// Check if current path matches our target path
			pathStr := "$." + strings.Join(currentPath, ".")
			if pathStr == m.path {
				// Look for text content or attributes
				text, textErr := m.getElementText(decoder)
				if textErr == nil && m.matchesValue(text) {
					return true, fmt.Sprintf("matched xml-path %q with value %q", m.path, text)
				}
			}

		case xml.EndElement:
			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}
		}
	}

	return false, ""
}

func (m *compiledXMLPathMatcher) getElementText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder

	for {
		token, err := decoder.Token()
		if err != nil {
			return text.String(), err
		}

		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			return text.String(), nil
		}
	}
}

func (m *compiledXMLPathMatcher) matchesValue(text string) bool {
	if m.value == "" {
		return true // Just check for existence
	}

	// Check if value is a regex pattern
	if strings.HasPrefix(m.value, "/") && strings.HasSuffix(m.value, "/") {
		// TODO: Implement regex matching for XML values
		return strings.Contains(text, m.value[1:len(m.value)-1])
	}

	return strings.TrimSpace(text) == strings.TrimSpace(m.value)
}
