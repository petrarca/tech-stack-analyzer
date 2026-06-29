package currency

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name      string
		system    string
		installed string
		latest    string
		want      Bucket
	}{
		// npm (semver parser path)
		{"npm equal", "npm", "1.2.3", "1.2.3", UpToDate},
		{"npm patch", "npm", "1.2.3", "1.2.9", Patch},
		{"npm minor", "npm", "1.2.3", "1.5.0", Minor},
		{"npm major", "npm", "17.0.2", "19.3.0", Major},
		{"npm installed newer", "npm", "2.0.0", "1.9.9", UpToDate},
		{"npm missing trailing == patch via 0-pad", "npm", "1.2", "1.2.4", Patch},

		// maven (semver parser path; group:artifact name is irrelevant to bucketing)
		{"maven major", "maven", "6.0.0", "7.0.8", Major},
		{"maven minor", "maven", "7.0.0", "7.2.0", Minor},

		// nuget/go/rubygems (numeric path, no dedicated parser)
		{"nuget major", "nuget", "12.0.3", "13.0.4", Major},
		{"go minor", "go", "1.20.0", "1.22.0", Minor},
		{"rubygems patch", "rubygems", "2.5.0", "2.5.3", Patch},
		{"nuget installed newer", "nuget", "14.0.0", "13.0.4", UpToDate},

		// cargo: semver.Cargo.Parse is not yet implemented; silently falls
		// through to numeric comparison. This case locks in that behavior so
		// any future implementation of semver.Cargo is caught by a test change.
		{"cargo numeric fallback", "cargo", "1.2.3", "2.0.0", Major},
		{"cargo minor fallback", "cargo", "1.2.3", "1.3.0", Minor},

		// pre-release / build suffix stripped before numeric compare
		{"suffix stripped", "npm", "1.2.3", "2.0.0-rc1", Major},

		// unresolved installed specifier -> Unpinned, consistently across
		// ecosystems (Maven's lenient parser must not report "latest" as a
		// comparable version).
		{"installed latest npm", "npm", "latest", "19.2.7", Unpinned},
		{"installed latest maven", "maven", "latest", "7.0.8", Unpinned},
		{"installed range", "npm", "^6.24.1", "6.24.1", Unpinned},
		{"installed RELEASE maven", "maven", "RELEASE", "7.0.8", Unpinned},

		// unparseable -> Unknown
		{"unparseable latest", "nuget", "1.0.0", "weird", Unknown},
		{"empty installed", "npm", "", "1.0.0", Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classify(c.system, c.installed, c.latest); got != c.want {
				t.Errorf("classify(%q,%q,%q) = %q, want %q", c.system, c.installed, c.latest, got, c.want)
			}
		})
	}
}

func TestNumericComponents(t *testing.T) {
	cases := []struct {
		in   string
		want []int
		ok   bool
	}{
		{"1.2.3", []int{1, 2, 3}, true},
		{"1.2.3-rc1", []int{1, 2, 3}, true},
		{"1.2.3+build", []int{1, 2, 3}, true},
		{"1", []int{1}, true},
		{"v1.2", nil, false}, // leading 'v' is non-numeric
		{"", nil, false},
		{"latest", nil, false},
	}
	for _, c := range cases {
		got, ok := numericComponents(c.in)
		if ok != c.ok {
			t.Errorf("numericComponents(%q) ok=%v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && !equalInts(got, c.want) {
			t.Errorf("numericComponents(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fakeResolver returns canned results for chain tests.
type fakeResolver struct {
	info LatestInfo
	err  error
}

func (f fakeResolver) LatestVersion(system, name string) (LatestInfo, error) {
	return f.info, f.err
}

func TestChainResolver(t *testing.T) {
	hit := fakeResolver{info: LatestInfo{Latest: "2.0.0"}}
	miss := fakeResolver{err: ErrNotFound}

	// first hit wins
	chain := NewChainResolver(hit, miss)
	if info, err := chain.LatestVersion("npm", "x"); err != nil || info.Latest != "2.0.0" {
		t.Errorf("first-hit: got %+v err=%v", info, err)
	}

	// fall through misses to a later hit
	chain = NewChainResolver(miss, hit)
	if info, err := chain.LatestVersion("npm", "x"); err != nil || info.Latest != "2.0.0" {
		t.Errorf("fall-through: got %+v err=%v", info, err)
	}

	// all miss -> ErrNotFound
	chain = NewChainResolver(miss, miss)
	if _, err := chain.LatestVersion("npm", "x"); err != ErrNotFound {
		t.Errorf("all-miss: err=%v, want ErrNotFound", err)
	}
}
