package currency

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// mapResolver resolves from a fixed table; missing keys return ErrNotFound.
type mapResolver map[string]LatestInfo

func (m mapResolver) LatestVersion(system, name string) (LatestInfo, error) {
	if info, ok := m[system+"|"+name]; ok {
		return info, nil
	}
	return LatestInfo{}, ErrNotFound
}

func dep(typ, name, ver, scope string, direct bool) types.Dependency {
	return types.Dependency{Type: typ, Name: name, Version: ver, Scope: scope, Direct: direct}
}

func TestResolveClassifiesAndCounts(t *testing.T) {
	deps := []types.Dependency{
		dep("npm", "react", "17.0.2", "prod", true),          // major behind
		dep("npm", "lodash", "4.18.1", "prod", true),         // up to date
		dep("maven", "org.x:y", "1.0.0", "prod", true),       // minor behind
		dep("nuget", "newtonsoft.json", "13.0.4", "", true),  // up to date (numeric path)
		dep("delphi", "SomeNativeLib", "1.0", "", true),      // unsupported ecosystem
		dep("npm", "internal-pkg", "1.0.0", "prod", true),    // unknown (not in table)
		dep("npm", "transitive-dep", "1.0.0", "prod", false), // transitive: skipped
	}
	res := mapResolver{
		"npm|react":             {Latest: "19.3.0"},
		"npm|lodash":            {Latest: "4.18.1"},
		"maven|org.x:y":         {Latest: "1.4.0"},
		"nuget|newtonsoft.json": {Latest: "13.0.4"},
	}

	art := Resolve(deps, res, Options{DirectOnly: true, TTLHours: 24})

	if art.Schema != SchemaID {
		t.Errorf("schema = %q", art.Schema)
	}
	if art.Scope != "direct" {
		t.Errorf("scope = %q, want direct", art.Scope)
	}
	// 7 deps, 1 transitive skipped -> 6 entries.
	if art.Summary.Total != 6 {
		t.Errorf("total = %d, want 6 (transitive excluded)", art.Summary.Total)
	}
	if art.Summary.Major != 1 {
		t.Errorf("major = %d, want 1", art.Summary.Major)
	}
	if art.Summary.Minor != 1 {
		t.Errorf("minor = %d, want 1", art.Summary.Minor)
	}
	if art.Summary.UpToDate != 2 {
		t.Errorf("up_to_date = %d, want 2", art.Summary.UpToDate)
	}
	if art.Summary.Unsupported != 1 {
		t.Errorf("unsupported = %d, want 1", art.Summary.Unsupported)
	}
	if art.Summary.Unknown != 1 {
		t.Errorf("unknown = %d, want 1", art.Summary.Unknown)
	}
	if art.Summary.Resolved != 4 {
		t.Errorf("resolved = %d, want 4 (2 up_to_date + 1 major + 1 minor)", art.Summary.Resolved)
	}

	// Spot-check a specific entry's fields.
	var react *Dependency
	for i := range art.Dependencies {
		if art.Dependencies[i].Name == "react" {
			react = &art.Dependencies[i]
		}
	}
	if react == nil {
		t.Fatal("react entry missing")
	}
	if react.Currency != Major || react.Latest != "19.3.0" || react.System != "npm" {
		t.Errorf("react entry wrong: %+v", react)
	}
	if react.PURL != "pkg:npm/react@17.0.2" {
		t.Errorf("react purl = %q", react.PURL)
	}
}

func TestPurl(t *testing.T) {
	cases := []struct {
		d    types.Dependency
		want string
	}{
		{dep("npm", "react", "17.0.2", "", true), "pkg:npm/react@17.0.2"},
		{dep("maven", "org.springframework:spring-core", "6.0.0", "", true), "pkg:maven/org.springframework/spring-core@6.0.0"},
		{dep("gradle", "com.x:y", "1.0", "", true), "pkg:maven/com.x/y@1.0"},
		{dep("pypi", "requests", "2.0", "", true), "pkg:pypi/requests@2.0"},
	}
	for _, c := range cases {
		if got := purl(c.d); got != c.want {
			t.Errorf("purl(%s) = %q, want %q", c.d.Name, got, c.want)
		}
	}
}
