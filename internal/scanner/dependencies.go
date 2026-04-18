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

// depTypeAliases maps a dependency type to additional types whose matchers
// should also be consulted when matching that type. This avoids requiring
// every rule to declare redundant entries for coordinate-equivalent ecosystems.
//
// Maven and Gradle both use the group:artifact coordinate format, so a rule
// that declares `type: maven` is automatically available when matching Gradle
// dependencies — and vice versa. Rules that explicitly declare both types
// (e.g. h2.yaml) continue to work correctly; the cross-registration below
// just fills the gaps where only one type is declared.
//
// Treat this map as read-only. It is consulted at detector construction
// time only and must not be mutated at runtime.
var depTypeAliases = map[string][]string{
	"gradle": {"maven"},
	"maven":  {"gradle"},
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
			regex, err := compileDependencyPattern(dep.Name)
			if err != nil {
				continue // Skip invalid regex
			}
			matcher := &DependencyMatcher{
				Regex: regex,
				Tech:  rule.Tech,
				Type:  dep.Type,
			}

			// Register under the canonical type
			detector.matchers[dep.Type] = append(detector.matchers[dep.Type], matcher)

			// Also register under alias types so callers querying e.g.
			// "gradle" automatically hit "maven" rules and vice versa,
			// without needing to duplicate entries in every rule YAML.
			for _, alias := range depTypeAliases[dep.Type] {
				detector.matchers[alias] = append(detector.matchers[alias], matcher)
			}
		}
	}

	return detector
}

// compileDependencyPattern compiles a dependency name to a regex. Names
// wrapped in forward slashes (/pattern/) are treated as raw regex patterns;
// anything else is compiled as an exact match.
func compileDependencyPattern(name string) (*regexp.Regexp, error) {
	if strings.HasPrefix(name, "/") && strings.HasSuffix(name, "/") {
		return regexp.Compile(name[1 : len(name)-1])
	}
	return regexp.Compile("^" + regexp.QuoteMeta(name) + "$")
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

// ApplyMatchesToPayload applies a map of matched techs (as returned by
// MatchDependencies) to a payload: each tech is added with its reasons, and
// promoted to primary if the corresponding rule is marked as a primary tech.
//
// This consolidates the "for each tech, add reasons, maybe promote" loop that
// would otherwise be repeated in every component detector.
func (d *DependencyDetector) ApplyMatchesToPayload(payload *types.Payload, matches map[string][]string) {
	for tech, reasons := range matches {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		d.AddPrimaryTechIfNeeded(payload, tech)
	}
}
