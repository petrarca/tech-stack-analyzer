package aggregator

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// findEco returns the components count for an ecosystem in the result, or -1.
func findEco(entries []types.EcosystemEntry, name string) int {
	for _, e := range entries {
		if e.Ecosystem == name {
			return e.Components
		}
	}
	return -1
}

// TestComputeEcosystems characterizes the three detection signals (component
// type, tech, language fallback) and the result ordering, using real values
// from the embedded ecosystems.yaml. Written before refactoring.
func TestComputeEcosystems(t *testing.T) {
	t.Run("component type is the strongest signal", func(t *testing.T) {
		comps := []ComponentEntry{
			{Type: "maven", Tech: []string{"spring"}},
			{Type: "gradle", Tech: []string{"java"}},
		}
		got := computeEcosystems(comps, nil)
		if c := findEco(got, "JVM"); c != 2 {
			t.Fatalf("expected JVM components=2 from typed components, got %d (%v)", c, got)
		}
	})

	t.Run("tech matches only untyped components", func(t *testing.T) {
		comps := []ComponentEntry{
			{Type: "", Tech: []string{"spring"}}, // untyped -> tech signal
			{Type: "", Tech: []string{"java"}},
		}
		got := computeEcosystems(comps, nil)
		if c := findEco(got, "JVM"); c != 2 {
			t.Fatalf("expected JVM components=2 from tech signal, got %d (%v)", c, got)
		}
	})

	t.Run("typed component does not double-count via tech", func(t *testing.T) {
		comps := []ComponentEntry{{Type: "maven", Tech: []string{"spring", "hibernate"}}}
		got := computeEcosystems(comps, nil)
		if c := findEco(got, "JVM"); c != 1 {
			t.Fatalf("expected JVM components=1 (type signal only), got %d (%v)", c, got)
		}
	})

	t.Run("language is a fallback only when not already detected", func(t *testing.T) {
		langs := []types.PrimaryLanguage{{Language: "Java", Pct: 90}}
		got := computeEcosystems(nil, langs)
		if c := findEco(got, "JVM"); c != 1 {
			t.Fatalf("expected JVM components=1 from language fallback, got %d (%v)", c, got)
		}
	})

	t.Run("no matches returns nil", func(t *testing.T) {
		got := computeEcosystems([]ComponentEntry{{Type: "totally-unknown-type"}}, nil)
		if got != nil {
			t.Fatalf("expected nil for no matches, got %v", got)
		}
	})

	t.Run("entries sorted by components desc then name asc", func(t *testing.T) {
		// JVM via two typed components; .NET via one. JVM should sort first.
		comps := []ComponentEntry{
			{Type: "maven"},
			{Type: "gradle"},
			{Type: "dotnet"},
		}
		got := computeEcosystems(comps, nil)
		if len(got) < 2 {
			t.Fatalf("expected at least 2 ecosystems, got %v", got)
		}
		if got[0].Ecosystem != "JVM" {
			t.Fatalf("expected JVM first (highest component count), got %v", got)
		}
	})
}
