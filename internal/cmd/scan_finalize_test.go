package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/petrarca/tech-stack-analyzer/internal/codestats"
	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// stubSubsystemAnalyzer implements codestats.SubsystemAnalyzer for testing.
type stubSubsystemAnalyzer struct {
	stats map[string]*codestats.CodeStats
}

func (s *stubSubsystemAnalyzer) GetSubsystemStats(key string) *codestats.CodeStats {
	return s.stats[key]
}
func (s *stubSubsystemAnalyzer) SubsystemKeys() []string {
	keys := make([]string, 0, len(s.stats))
	for k := range s.stats {
		keys = append(keys, k)
	}
	return keys
}

// stubAnalyzer wraps stubSubsystemAnalyzer and satisfies codestats.Analyzer.
type stubAnalyzer struct {
	stubSubsystemAnalyzer
}

func (s *stubAnalyzer) GetStats() *codestats.CodeStats                    { return nil }
func (s *stubAnalyzer) GetComponentStats(string) *codestats.CodeStats     { return nil }
func (s *stubAnalyzer) IsEnabled() bool                                   { return true }
func (s *stubAnalyzer) ProcessFile(_, _, _ string, _ []byte, _, _ string) {}

// newStubStats returns a minimal CodeStats with a recognisable code line count.
func newStubStats(codeLines int64) *codestats.CodeStats {
	return &codestats.CodeStats{Total: codestats.Stats{Code: codeLines}}
}

// component builds a leaf Payload with the given path, techs, and languages.
func component(path string, techs []string, languages map[string]int) *types.Payload {
	return &types.Payload{
		ID:        path,
		Name:      path,
		Path:      []string{path},
		Techs:     techs,
		Languages: languages,
	}
}

// depthResolver returns the depth-1 prefix of a component path (mirrors the scanner's resolver).
func depthResolver(path string) string {
	return types.DepthPrefix(path, 1)
}

// identityResolver returns the key as-is (used when the test controls exact key strings).
func identityResolver(path string) string { return path }

// ---- Tests ------------------------------------------------------------------

func TestCollectSubsystemComponentData_SingleLevel(t *testing.T) {
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/core/lib/pom.xml", []string{"maven", "java"}, map[string]int{"Java": 10}),
			component("/core/utils/package.json", []string{"nodejs", "npm"}, map[string]int{"TypeScript": 5}),
			component("/services/auth/pom.xml", []string{"maven", "postgresql"}, map[string]int{"Java": 8}),
		},
	}

	data := collectSubsystemComponentData(root, depthResolver)

	if data["/core"] == nil {
		t.Fatal("expected entry for /core, got nil")
	}
	if data["/services"] == nil {
		t.Fatal("expected entry for /services, got nil")
	}

	if got := data["/core"].count; got != 2 {
		t.Errorf("/core component_count: got %d, want 2", got)
	}
	if got := data["/services"].count; got != 1 {
		t.Errorf("/services component_count: got %d, want 1", got)
	}

	wantCoreTechs := map[string]bool{"maven": true, "java": true, "nodejs": true, "npm": true}
	if diff := cmp.Diff(wantCoreTechs, data["/core"].techSet); diff != "" {
		t.Errorf("/core techSet mismatch (-want +got):\n%s", diff)
	}

	wantCoreLangs := map[string]int{"Java": 10, "TypeScript": 5}
	if diff := cmp.Diff(wantCoreLangs, data["/core"].languages); diff != "" {
		t.Errorf("/core languages mismatch (-want +got):\n%s", diff)
	}
}

func TestCollectSubsystemComponentData_NoMatchingComponents(t *testing.T) {
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/frontend/app/package.json", []string{"react"}, map[string]int{"TypeScript": 3}),
		},
	}

	// Resolver always returns empty — no component maps to any subsystem.
	data := collectSubsystemComponentData(root, func(string) string { return "" })

	if len(data) != 0 {
		t.Errorf("expected empty data, got %v", data)
	}
}

func TestCollectSubsystemComponentData_LanguageMerge(t *testing.T) {
	// Two components under the same subsystem with overlapping languages.
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/svc/auth/pom.xml", []string{"java"}, map[string]int{"Java": 10, "XML": 2}),
			component("/svc/billing/pom.xml", []string{"java"}, map[string]int{"Java": 5, "YAML": 1}),
		},
	}

	data := collectSubsystemComponentData(root, depthResolver)

	wantLangs := map[string]int{"Java": 15, "XML": 2, "YAML": 1}
	if diff := cmp.Diff(wantLangs, data["/svc"].languages); diff != "" {
		t.Errorf("/svc languages mismatch (-want +got):\n%s", diff)
	}
}

func TestAttachSubsystemStats_DepthMode(t *testing.T) {
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/core/lib/pom.xml", []string{"maven", "java"}, map[string]int{"Java": 10}),
			component("/services/auth/pom.xml", []string{"maven", "postgresql"}, map[string]int{"Java": 8}),
		},
	}

	analyzer := &stubAnalyzer{stubSubsystemAnalyzer{stats: map[string]*codestats.CodeStats{
		"/core":     newStubStats(1000),
		"/services": newStubStats(800),
	}}}

	attachSubsystemStats(root, analyzer, depthResolver, nil)

	if got := len(root.SubsystemStats); got != 2 {
		t.Fatalf("expected 2 subsystem entries, got %d", got)
	}

	// Sorted by code lines descending: /core (1000) before /services (800).
	core := root.SubsystemStats[0]
	if core.Path != "/core" {
		t.Errorf("first entry path: got %q, want %q", core.Path, "/core")
	}
	if core.ComponentCount != 1 {
		t.Errorf("/core component_count: got %d, want 1", core.ComponentCount)
	}
	if diff := cmp.Diff([]string{"java", "maven"}, core.Techs); diff != "" {
		t.Errorf("/core techs mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int{"Java": 10}, core.Languages); diff != "" {
		t.Errorf("/core languages mismatch (-want +got):\n%s", diff)
	}
	// Depth mode: no paths or description.
	if core.Paths != nil {
		t.Errorf("/core paths: expected nil in depth mode, got %v", core.Paths)
	}
	if core.Description != "" {
		t.Errorf("/core description: expected empty in depth mode, got %q", core.Description)
	}
}

func TestAttachSubsystemStats_NamedGroups(t *testing.T) {
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/core/lib/pom.xml", []string{"java", "maven"}, map[string]int{"Java": 10}),
			component("/svc-auth/pom.xml", []string{"java", "postgresql"}, map[string]int{"Java": 8}),
			component("/svc-billing/requirements.txt", []string{"python"}, map[string]int{"Python": 5}),
		},
	}

	// Named group resolver: maps each component path to its group name.
	groupResolver := func(path string) string {
		switch types.DepthPrefix(path, 1) {
		case "/core":
			return "platform"
		case "/svc-auth", "/svc-billing":
			return "business"
		}
		return ""
	}

	groups := map[string]config.SubsystemGroup{
		"platform": {Paths: []string{"/core"}, Description: "Core platform"},
		"business": {Paths: []string{"/svc-auth", "/svc-billing"}, Description: "Business services"},
	}

	analyzer := &stubAnalyzer{stubSubsystemAnalyzer{stats: map[string]*codestats.CodeStats{
		"platform": newStubStats(1000),
		"business": newStubStats(1500),
	}}}

	attachSubsystemStats(root, analyzer, groupResolver, groups)

	if got := len(root.SubsystemStats); got != 2 {
		t.Fatalf("expected 2 subsystem entries, got %d", got)
	}

	// Sorted by code lines descending: business (1500) before platform (1000).
	biz := root.SubsystemStats[0]
	if biz.Path != "business" {
		t.Errorf("first entry path: got %q, want %q", biz.Path, "business")
	}
	if biz.ComponentCount != 2 {
		t.Errorf("business component_count: got %d, want 2", biz.ComponentCount)
	}
	if diff := cmp.Diff([]string{"/svc-auth", "/svc-billing"}, biz.Paths); diff != "" {
		t.Errorf("business paths mismatch (-want +got):\n%s", diff)
	}
	if biz.Description != "Business services" {
		t.Errorf("business description: got %q, want %q", biz.Description, "Business services")
	}
	if diff := cmp.Diff([]string{"java", "postgresql", "python"}, biz.Techs); diff != "" {
		t.Errorf("business techs mismatch (-want +got):\n%s", diff)
	}
	wantBizLangs := map[string]int{"Java": 8, "Python": 5}
	if diff := cmp.Diff(wantBizLangs, biz.Languages); diff != "" {
		t.Errorf("business languages mismatch (-want +got):\n%s", diff)
	}
}

func TestAttachSubsystemStats_NilSafe_NoMatchingComponents(t *testing.T) {
	// Analyzer has a key but no component resolves to it — data[key] is nil.
	// This is the regression test for the nil pointer dereference bug.
	root := &types.Payload{ID: "root", Name: "root"}

	analyzer := &stubAnalyzer{stubSubsystemAnalyzer{stats: map[string]*codestats.CodeStats{
		"ghost": newStubStats(100),
	}}}

	// Resolver never matches anything.
	attachSubsystemStats(root, analyzer, func(string) string { return "" }, nil)

	// Entry still appears (code_stats comes from analyzer, not component walk),
	// but component_count, techs, and languages are zero/nil.
	if got := len(root.SubsystemStats); got != 1 {
		t.Fatalf("expected 1 subsystem entry, got %d", got)
	}
	entry := root.SubsystemStats[0]
	if entry.ComponentCount != 0 {
		t.Errorf("component_count: got %d, want 0", entry.ComponentCount)
	}
	if entry.Techs != nil {
		t.Errorf("techs: expected nil, got %v", entry.Techs)
	}
	if entry.Languages != nil {
		t.Errorf("languages: expected nil, got %v", entry.Languages)
	}
}

func TestAttachSubsystemStats_NoSubsystemAnalyzer(t *testing.T) {
	// When the analyzer does not implement SubsystemAnalyzer, nothing is attached.
	root := &types.Payload{ID: "root", Name: "root"}

	attachSubsystemStats(root, &noopSubsystemAnalyzer{}, identityResolver, nil)

	if root.SubsystemStats != nil {
		t.Errorf("expected nil SubsystemStats, got %v", root.SubsystemStats)
	}
}

func TestAttachSubsystemStats_SortTieBreak(t *testing.T) {
	// Two subsystems with identical code line counts — should be sorted by path alphabetically.
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			component("/beta/pom.xml", []string{"java"}, map[string]int{"Java": 1}),
			component("/alpha/pom.xml", []string{"java"}, map[string]int{"Java": 1}),
		},
	}

	analyzer := &stubAnalyzer{stubSubsystemAnalyzer{stats: map[string]*codestats.CodeStats{
		"/alpha": newStubStats(500),
		"/beta":  newStubStats(500),
	}}}

	attachSubsystemStats(root, analyzer, depthResolver, nil)

	if got := len(root.SubsystemStats); got != 2 {
		t.Fatalf("expected 2 entries, got %d", got)
	}
	if root.SubsystemStats[0].Path != "/alpha" {
		t.Errorf("tie-break sort: got %q first, want %q", root.SubsystemStats[0].Path, "/alpha")
	}
	if root.SubsystemStats[1].Path != "/beta" {
		t.Errorf("tie-break sort: got %q second, want %q", root.SubsystemStats[1].Path, "/beta")
	}
}

func TestCollectSubsystemComponentData_NestedChildren(t *testing.T) {
	// Components nested two levels deep in the payload tree should still
	// be counted under their subsystem.
	inner := component("/core/lib/pom.xml", []string{"java"}, map[string]int{"Java": 5})
	outer := &types.Payload{
		ID:        "/core",
		Name:      "core",
		Path:      []string{"/core/build.gradle"},
		Techs:     []string{"gradle"},
		Languages: map[string]int{"Groovy": 2},
		Children:  []*types.Payload{inner},
	}
	root := &types.Payload{
		ID:       "root",
		Name:     "root",
		Children: []*types.Payload{outer},
	}

	data := collectSubsystemComponentData(root, depthResolver)

	if data["/core"] == nil {
		t.Fatal("expected /core entry, got nil")
	}
	// Both outer (/core/build.gradle) and inner (/core/lib/pom.xml) map to /core.
	if got := data["/core"].count; got != 2 {
		t.Errorf("/core component_count: got %d, want 2", got)
	}
	wantTechs := map[string]bool{"java": true, "gradle": true}
	if diff := cmp.Diff(wantTechs, data["/core"].techSet); diff != "" {
		t.Errorf("/core techSet mismatch (-want +got):\n%s", diff)
	}
	wantLangs := map[string]int{"Java": 5, "Groovy": 2}
	if diff := cmp.Diff(wantLangs, data["/core"].languages); diff != "" {
		t.Errorf("/core languages mismatch (-want +got):\n%s", diff)
	}
}

func TestCollectSubsystemComponentData_NilTechsAndLanguages(t *testing.T) {
	// Components with nil Techs / Languages (e.g. virtual or root-like nodes)
	// must not panic and must not contribute spurious entries.
	root := &types.Payload{
		ID:   "root",
		Name: "root",
		Children: []*types.Payload{
			{
				ID:        "/core/virtual",
				Name:      "virtual",
				Path:      []string{"/core/virtual"},
				Techs:     nil,
				Languages: nil,
			},
		},
	}

	data := collectSubsystemComponentData(root, depthResolver)

	if data["/core"] == nil {
		t.Fatal("expected /core entry, got nil")
	}
	if got := data["/core"].count; got != 1 {
		t.Errorf("/core component_count: got %d, want 1", got)
	}
	if got := len(data["/core"].techSet); got != 0 {
		t.Errorf("/core techSet: expected empty, got %v", data["/core"].techSet)
	}
	if got := len(data["/core"].languages); got != 0 {
		t.Errorf("/core languages: expected empty, got %v", data["/core"].languages)
	}
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]bool
		want  []string
	}{
		{"nil map", nil, nil},
		{"empty map", map[string]bool{}, nil},
		{"single entry", map[string]bool{"a": true}, []string{"a"}},
		{"multiple entries sorted", map[string]bool{"c": true, "a": true, "b": true}, []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedKeys(tt.input)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("sortedKeys mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// noopSubsystemAnalyzer satisfies codestats.Analyzer but NOT codestats.SubsystemAnalyzer.
type noopSubsystemAnalyzer struct{}

func (n *noopSubsystemAnalyzer) GetStats() *codestats.CodeStats                    { return nil }
func (n *noopSubsystemAnalyzer) GetComponentStats(string) *codestats.CodeStats     { return nil }
func (n *noopSubsystemAnalyzer) IsEnabled() bool                                   { return true }
func (n *noopSubsystemAnalyzer) ProcessFile(_, _, _ string, _ []byte, _, _ string) {}
