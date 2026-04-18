package scanner

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
)

// TestDepTypeAliases_GradleMatchesMavenRules verifies that a dependency
// queried with type "gradle" also matches rules declared as type "maven",
// and vice versa. This covers the common case where JVM rules only declare
// maven coordinates but the project uses Gradle.
func TestDepTypeAliases_GradleMatchesMavenRules(t *testing.T) {
	rules := []types.Rule{
		{
			Tech: "springboot",
			Dependencies: []types.Dependency{
				{Type: "maven", Name: "/^org\\.springframework\\.boot:.*/"},
			},
		},
	}
	detector := NewDependencyDetector(rules)

	// Gradle query must hit the maven rule via alias
	matched := detector.MatchDependencies(
		[]string{"org.springframework.boot:spring-boot-starter-web"},
		"gradle",
	)
	assert.Contains(t, matched, "springboot", "gradle query should match maven-typed springboot rule")

	// maven query still works directly
	matched = detector.MatchDependencies(
		[]string{"org.springframework.boot:spring-boot-starter-web"},
		"maven",
	)
	assert.Contains(t, matched, "springboot", "maven query should still match maven-typed rule")
}

func TestDepTypeAliases_MavenMatchesGradleRules(t *testing.T) {
	rules := []types.Rule{
		{
			Tech: "h2",
			Dependencies: []types.Dependency{
				{Type: "gradle", Name: "com.h2database:h2"},
			},
		},
	}
	detector := NewDependencyDetector(rules)

	// maven query must hit the gradle rule via alias
	matched := detector.MatchDependencies([]string{"com.h2database:h2"}, "maven")
	assert.Contains(t, matched, "h2", "maven query should match gradle-typed h2 rule")
}

func TestDepTypeAliases_NoCrossMatchForUnrelatedTypes(t *testing.T) {
	rules := []types.Rule{
		{
			Tech: "react",
			Dependencies: []types.Dependency{
				{Type: "npm", Name: "react"},
			},
		},
	}
	detector := NewDependencyDetector(rules)

	// gradle must not accidentally match npm rules
	matched := detector.MatchDependencies([]string{"react"}, "gradle")
	assert.Empty(t, matched, "gradle query must not cross-match npm rules")
}

func TestDepTypeAliases_ExplicitBothTypesNoDuplicate(t *testing.T) {
	// Rules that already declare both maven and gradle (like h2.yaml) match
	// twice after aliasing — once via the canonical type and once via the
	// alias. MatchDependencies returns both reasons, but when applied to a
	// payload the deduplication in AddTech ensures the tech appears once.
	// Verify both: the raw match has two reasons, and ApplyMatchesToPayload
	// produces a payload with a single tech entry.
	rules := []types.Rule{
		{
			Tech: "h2",
			Dependencies: []types.Dependency{
				{Type: "maven", Name: "com.h2database:h2"},
				{Type: "gradle", Name: "com.h2database:h2"},
			},
		},
	}
	detector := NewDependencyDetector(rules)

	matched := detector.MatchDependencies([]string{"com.h2database:h2"}, "gradle")
	assert.Contains(t, matched, "h2")
	// With a rule declaring both maven and gradle, a gradle query hits two
	// matchers (the gradle one directly + the maven one via the alias).
	// Both reasons happen to be identical because the pattern is identical;
	// that's the current behaviour and worth pinning.
	assert.Len(t, matched["h2"], 2, "gradle query should hit both matchers when rule declares both types")

	payload := types.NewPayload("test", []string{"/"})
	detector.ApplyMatchesToPayload(payload, matched)
	// AddTech dedupes, so the tech appears once even with duplicate reasons.
	h2Count := 0
	for _, t := range payload.Techs {
		if t == "h2" {
			h2Count++
		}
	}
	assert.Equal(t, 1, h2Count, "h2 must appear exactly once in payload.Techs")
}
