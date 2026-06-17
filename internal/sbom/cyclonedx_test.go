package sbom

import (
	"strings"
	"testing"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestBuildPURL(t *testing.T) {
	tests := []struct {
		name string
		dep  types.Dependency
		want string
	}{
		{
			name: "npm simple",
			dep:  types.Dependency{Type: "npm", Name: "mylib", Version: "1.2.3"},
			want: "pkg:npm/mylib@1.2.3",
		},
		{
			name: "npm scoped encodes at-sign",
			dep:  types.Dependency{Type: "npm", Name: "@myorg/mylib", Version: "1.2.3"},
			want: "pkg:npm/%40myorg/mylib@1.2.3",
		},
		{
			name: "npm range version omitted",
			dep:  types.Dependency{Type: "npm", Name: "mylib", Version: "^1.2.3"},
			want: "pkg:npm/mylib",
		},
		{
			name: "pypi simple",
			dep:  types.Dependency{Type: "pypi", Name: "mypkg", Version: "2.0.0"},
			want: "pkg:pypi/mypkg@2.0.0",
		},
		{
			name: "pypi latest omitted",
			dep:  types.Dependency{Type: "pypi", Name: "mypkg", Version: "latest"},
			want: "pkg:pypi/mypkg",
		},
		{
			name: "gem simple",
			dep:  types.Dependency{Type: "gem", Name: "mygem", Version: "3.1.0"},
			want: "pkg:gem/mygem@3.1.0",
		},
		{
			name: "composer vendor package",
			dep:  types.Dependency{Type: "composer", Name: "myorg/mypkg", Version: "6.0.0"},
			want: "pkg:composer/myorg/mypkg@6.0.0",
		},
		{
			name: "cargo simple",
			dep:  types.Dependency{Type: "cargo", Name: "mycrate", Version: "0.4.0"},
			want: "pkg:cargo/mycrate@0.4.0",
		},
		{
			name: "maven group artifact split",
			dep:  types.Dependency{Type: "maven", Name: "com.example:mylib", Version: "4.13.2"},
			want: "pkg:maven/com.example/mylib@4.13.2",
		},
		{
			name: "gradle collapses to maven type",
			dep:  types.Dependency{Type: "gradle", Name: "com.example:mylib", Version: "1.0.0"},
			want: "pkg:maven/com.example/mylib@1.0.0",
		},
		{
			name: "golang module path namespace",
			dep:  types.Dependency{Type: "golang", Name: "example.com/myorg/mymod", Version: "v1.2.3"},
			want: "pkg:golang/example.com/myorg/mymod@v1.2.3",
		},
		{
			name: "nuget simple",
			dep:  types.Dependency{Type: "nuget", Name: "MyPkg", Version: "5.0.0"},
			want: "pkg:nuget/MyPkg@5.0.0",
		},
		{
			name: "git version omitted",
			dep:  types.Dependency{Type: "cargo", Name: "mycrate", Version: "git:https://example.com/repo.git#main"},
			want: "pkg:cargo/mycrate",
		},
		{
			name: "non-package type yields no purl",
			dep:  types.Dependency{Type: "terraform", Name: "myprovider", Version: "1.0.0"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPURL(tt.dep)
			if got != tt.want {
				t.Errorf("buildPURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromDependencies_FiltersNonPackageTypes(t *testing.T) {
	deps := []types.Dependency{
		{Type: "npm", Name: "mylib", Version: "1.0.0"},
		{Type: "terraform", Name: "myprovider", Version: "1.0.0"},
		{Type: "regex", Name: "something", Version: ""},
		{Type: "pypi", Name: "mypkg", Version: "2.0.0"},
	}

	bom := FromDependencies(deps, "myapp")

	if got := len(bom.Components); got != 2 {
		t.Fatalf("expected 2 package components, got %d", got)
	}
	if bom.BOMFormat != "CycloneDX" || bom.SpecVersion != SpecVersion {
		t.Errorf("unexpected BOM header: format=%s spec=%s", bom.BOMFormat, bom.SpecVersion)
	}
	if bom.Metadata == nil || bom.Metadata.Component == nil || bom.Metadata.Component.Name != "myapp" {
		t.Errorf("expected metadata component name 'myapp'")
	}
}

func TestFromDependencies_CycloneDX17Header(t *testing.T) {
	bom := FromDependencies([]types.Dependency{{Type: "npm", Name: "mylib", Version: "1.0.0"}}, "myapp")

	if bom.SpecVersion != "1.7" {
		t.Errorf("specVersion = %q, want 1.7", bom.SpecVersion)
	}
	if bom.JSONSchema != "http://cyclonedx.org/schema/bom-1.7.schema.json" {
		t.Errorf("$schema = %q", bom.JSONSchema)
	}
	// Pure builders are deterministic: no serialNumber/timestamp until Stamp.
	if bom.SerialNumber != "" {
		t.Errorf("serialNumber should be empty before Stamp, got %q", bom.SerialNumber)
	}
	if bom.Metadata != nil && bom.Metadata.Timestamp != "" {
		t.Errorf("timestamp should be empty before Stamp, got %q", bom.Metadata.Timestamp)
	}
}

func TestStamp_SetsSerialNumberAndTimestamp(t *testing.T) {
	bom := FromDependencies([]types.Dependency{{Type: "npm", Name: "mylib", Version: "1.0.0"}}, "myapp")
	Stamp(bom)

	if !strings.HasPrefix(bom.SerialNumber, "urn:uuid:") || len(bom.SerialNumber) != len("urn:uuid:")+36 {
		t.Errorf("serialNumber not a urn:uuid: %q", bom.SerialNumber)
	}
	if bom.Metadata == nil || bom.Metadata.Timestamp == "" {
		t.Fatal("Stamp must set metadata timestamp")
	}
	if _, err := time.Parse(time.RFC3339, bom.Metadata.Timestamp); err != nil {
		t.Errorf("timestamp not RFC3339: %q (%v)", bom.Metadata.Timestamp, err)
	}

	// Two stamps yield distinct serial numbers.
	other := FromDependencies(nil, "myapp")
	Stamp(other)
	if other.SerialNumber == bom.SerialNumber {
		t.Error("serialNumber should be unique per emission")
	}

	// Stamp on a nil BOM is a no-op (must not panic).
	Stamp(nil)
}

func TestFromDependencies_ExcludesImportScopedBOMs(t *testing.T) {
	// A Maven BOM import (scope=import) is a version-management entry, not a
	// package, and must not become an SBOM component.
	deps := []types.Dependency{
		{Type: "maven", Name: "org.example:lib", Version: "1.0.0", Scope: types.ScopeProd},
		{Type: "maven", Name: "org.example:platform-bom", Version: "2.0.0", Scope: types.ScopeImport},
	}

	bom := FromDependencies(deps, "myapp")

	if got := len(bom.Components); got != 1 {
		t.Fatalf("expected 1 component (BOM import excluded), got %d", got)
	}
	if bom.Components[0].Name != "org.example:lib" {
		t.Errorf("unexpected component: %q", bom.Components[0].Name)
	}
}

func TestFromPayload_FoldsTransitiveGraphNodes(t *testing.T) {
	// A maven component with one declared dep and a resolved graph that adds
	// transitive nodes. The transitive nodes should appear as components.
	p := &types.Payload{
		Name:          "app",
		ComponentType: "maven",
		Dependencies: []types.Dependency{
			{Type: "maven", Name: "io.quarkus:quarkus-core", Version: "3.36.0", Scope: types.ScopeProd},
		},
		DependencyEdges: []types.DependencyEdge{
			{From: ".", To: "io.quarkus:quarkus-core@3.36.0"},
			{From: "io.quarkus:quarkus-core@3.36.0", To: "io.smallrye.common:smallrye-common-annotation@2.17.0"},
			{From: "io.quarkus:quarkus-core@3.36.0", To: "io.quarkus:quarkus-fs-util@1.3.0"},
		},
	}

	bom := FromPayload(p)

	purls := make(map[string]bool)
	for _, c := range bom.Components {
		purls[c.PURL] = true
	}
	// Declared dep present.
	if !purls["pkg:maven/io.quarkus/quarkus-core@3.36.0"] {
		t.Error("declared quarkus-core should be present")
	}
	// Transitive nodes folded in.
	if !purls["pkg:maven/io.smallrye.common/smallrye-common-annotation@2.17.0"] {
		t.Error("transitive smallrye-common-annotation should be folded in")
	}
	if !purls["pkg:maven/io.quarkus/quarkus-fs-util@1.3.0"] {
		t.Error("transitive quarkus-fs-util should be folded in")
	}
	// No duplicate of the declared dep (edge node == declared).
	count := 0
	for _, c := range bom.Components {
		if c.PURL == "pkg:maven/io.quarkus/quarkus-core@3.36.0" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("quarkus-core should appear once, got %d", count)
	}
}

func TestFromPayload_NoGraphMeansDeclaredOnly(t *testing.T) {
	// Without edges, only declared deps are emitted (default behavior).
	p := &types.Payload{
		Name:          "app",
		ComponentType: "maven",
		Dependencies: []types.Dependency{
			{Type: "maven", Name: "org.example:lib", Version: "1.0.0", Scope: types.ScopeProd},
		},
	}
	bom := FromPayload(p)
	if len(bom.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(bom.Components))
	}
}

func TestFromDependencies_ComponentFields(t *testing.T) {
	deps := []types.Dependency{
		{Type: "npm", Name: "mylib", Version: "^1.2.3", Scope: types.ScopeProd},
	}
	bom := FromDependencies(deps, "")

	if len(bom.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(bom.Components))
	}
	c := bom.Components[0]
	// Component version retains the original (unresolved) value for human inspection.
	if c.Version != "^1.2.3" {
		t.Errorf("component version = %q, want %q", c.Version, "^1.2.3")
	}
	// PURL omits the unresolved range.
	if c.PURL != "pkg:npm/mylib" {
		t.Errorf("component purl = %q, want %q", c.PURL, "pkg:npm/mylib")
	}
	if c.Scope != "required" {
		t.Errorf("component scope = %q, want %q", c.Scope, "required")
	}
	if c.Type != "library" {
		t.Errorf("component type = %q, want %q", c.Type, "library")
	}
}

// Note: the resolved-version classification is tested in
// internal/scanner/semver (TestResolvedVersion), where the logic now lives.
