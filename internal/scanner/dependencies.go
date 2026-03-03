package scanner

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DependencyMatcher represents a compiled dependency pattern
type DependencyMatcher struct {
	Regex *regexp.Regexp
	Tech  string
	Type  string
}

// DependencyDetector handles dependency-based technology detection
type DependencyDetector struct {
	matchers map[string][]*DependencyMatcher // keyed by dependency type (npm, python, etc.)
	rules    []types.Rule                    // Store rules for primary tech checking
}

// NewDependencyDetector creates a new dependency detector
func NewDependencyDetector(rules []types.Rule) *DependencyDetector {
	detector := &DependencyDetector{
		matchers: make(map[string][]*DependencyMatcher),
		rules:    rules,
	}

	// Compile all dependency patterns from rules
	for _, rule := range rules {
		for _, dep := range rule.Dependencies {
			matcher := &DependencyMatcher{
				Tech: rule.Tech,
				Type: dep.Type,
			}

			// Compile the dependency name to regex
			if strings.HasPrefix(dep.Name, "/") && strings.HasSuffix(dep.Name, "/") {
				// It's already a regex pattern
				regex, err := regexp.Compile(dep.Name[1 : len(dep.Name)-1])
				if err != nil {
					continue // Skip invalid regex
				}
				matcher.Regex = regex
			} else {
				// Convert to exact match regex
				regex, err := regexp.Compile("^" + regexp.QuoteMeta(dep.Name) + "$")
				if err != nil {
					continue // Skip invalid regex
				}
				matcher.Regex = regex
			}

			detector.matchers[dep.Type] = append(detector.matchers[dep.Type], matcher)
		}
	}

	return detector
}

// MatchDependencies matches a list of package names against dependency patterns
func (d *DependencyDetector) MatchDependencies(packages []string, depType string) map[string][]string {
	matched := make(map[string][]string)

	matchers, exists := d.matchers[depType]
	if !exists {
		return matched
	}

	for _, pkg := range packages {
		for _, matcher := range matchers {
			if matcher.Regex.MatchString(pkg) {
				if _, exists := matched[matcher.Tech]; !exists {
					matched[matcher.Tech] = []string{}
				}
				matched[matcher.Tech] = append(matched[matcher.Tech],
					matcher.Tech+" matched: "+matcher.Regex.String())
			}
		}
	}

	return matched
}

// AddPrimaryTechIfNeeded checks if a tech should be primary and adds it if needed
func (d *DependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Find the rule for this tech
	for i := range d.rules {
		if d.rules[i].Tech == tech {
			if ShouldAddPrimaryTech(d.rules[i]) {
				payload.AddPrimaryTech(tech)
			}
			return
		}
	}
}
