package semver

import (
	"testing"
)

func TestNPMVersionParsing(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
		canon   string
	}{
		// Basic versions
		{name: "simple version", version: "1.2.3", canon: "1.2.3"},
		{name: "with v prefix", version: "v1.2.3", canon: "1.2.3"},
		{name: "with V prefix", version: "V1.2.3", canon: "1.2.3"},
		{name: "with = prefix", version: "=1.2.3", canon: "1.2.3"},
		{name: "two part", version: "1.2", canon: "1.2.0"},
		{name: "one part", version: "1", canon: "1.0.0"},

		// Pre-releases
		{name: "alpha", version: "1.0.0-alpha", canon: "1.0.0-alpha"},
		{name: "alpha.1", version: "1.0.0-alpha.1", canon: "1.0.0-alpha.1"},
		{name: "beta", version: "1.0.0-beta", canon: "1.0.0-beta"},
		{name: "rc.1", version: "1.0.0-rc.1", canon: "1.0.0-rc.1"},
		{name: "complex prerelease", version: "1.0.0-alpha.beta.1", canon: "1.0.0-alpha.beta.1"},

		// Build metadata
		{name: "with build", version: "1.0.0+20130313144700", canon: "1.0.0+20130313144700"},
		{name: "build metadata", version: "1.0.0+build.1", canon: "1.0.0+build.1"},
		{name: "complex build", version: "1.0.0+exp.sha.5114f85", canon: "1.0.0+exp.sha.5114f85"},

		// Combined
		{name: "pre and build", version: "1.0.0-alpha+001", canon: "1.0.0-alpha+001"},
		{name: "complex", version: "1.0.0-beta.1+build.123", canon: "1.0.0-beta.1+build.123"},

		// Edge cases
		{name: "zeros", version: "0.0.0", canon: "0.0.0"},
		{name: "large numbers", version: "999.999.999", canon: "999.999.999"},

		// Error cases
		{name: "empty", version: "", wantErr: true},
		{name: "invalid", version: "abc", wantErr: true},
		{name: "too many parts", version: "1.2.3.4", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NPM.Parse(tt.version)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.version)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.version, err)
				return
			}

			canon := v.Canon(true)
			if canon != tt.canon {
				t.Errorf("Parse(%q).Canon() = %q, want %q", tt.version, canon, tt.canon)
			}
		})
	}
}

func TestNPMVersionComparison(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		// Basic comparisons
		{name: "equal", v1: "1.0.0", v2: "1.0.0", want: 0},
		{name: "major less", v1: "1.0.0", v2: "2.0.0", want: -1},
		{name: "major greater", v1: "2.0.0", v2: "1.0.0", want: 1},
		{name: "minor less", v1: "1.0.0", v2: "1.1.0", want: -1},
		{name: "minor greater", v1: "1.1.0", v2: "1.0.0", want: 1},
		{name: "patch less", v1: "1.0.0", v2: "1.0.1", want: -1},
		{name: "patch greater", v1: "1.0.1", v2: "1.0.0", want: 1},

		// Pre-releases
		{name: "release > pre", v1: "1.0.0", v2: "1.0.0-alpha", want: 1},
		{name: "pre < release", v1: "1.0.0-alpha", v2: "1.0.0", want: -1},
		{name: "alpha < beta", v1: "1.0.0-alpha", v2: "1.0.0-beta", want: -1},
		{name: "numeric prerelease", v1: "1.0.0-1", v2: "1.0.0-2", want: -1},
		{name: "numeric < string", v1: "1.0.0-1", v2: "1.0.0-alpha", want: -1},
		{name: "longer prerelease", v1: "1.0.0-alpha", v2: "1.0.0-alpha.1", want: -1},

		// Build metadata (ignored in comparison)
		{name: "build ignored", v1: "1.0.0+build1", v2: "1.0.0+build2", want: 0},
		{name: "build ignored 2", v1: "1.0.0", v2: "1.0.0+build", want: 0},

		// Complex
		{name: "pre with build", v1: "1.0.0-alpha+001", v2: "1.0.0-alpha+002", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := NPM.Parse(tt.v1)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.v1, err)
			}

			v2, err := NPM.Parse(tt.v2)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.v2, err)
			}

			got := v1.Compare(v2)
			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestNormalizeNPMVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		// Basic normalization
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2", "1.2.0"},
		{"1", "1.0.0"},

		// Special protocols - following deps.dev patterns
		{"workspace:*", "workspace"},
		{"workspace:^1.0.0", "workspace"},
		{"file:../local", "local"},
		{"git:repo#main", "git:repo#main"}, // Preserve full git URL
		{"git+https://github.com/user/repo.git", "git+https://github.com/user/repo.git"}, // Preserve full git URL
		{"github:user/repo", "github:user/repo"},                                         // Preserve full github reference
		{"http://example.com/package.tgz", "http://example.com/package.tgz"},             // Preserve full HTTP URL
		{"https://example.com/package.tgz", "https://example.com/package.tgz"},           // Preserve full HTTPS URL
		{"link:../local", "link"},

		// npm protocol - extract package@version
		{"npm:package@1.2.3", "package@1.2.3"},
		{"npm:@scope/package@1.2.3", "@scope/package@1.2.3"},

		// Special values
		{"*", "latest"},
		{"latest", "latest"},
		{"", "latest"},

		// Invalid (returns original)
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := NormalizeNPMVersion(tt.version)
			if got != tt.want {
				t.Errorf("NormalizeNPMVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestNPMNormalize(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2", "1.2.0"},
		{"1.0.0-alpha", "1.0.0-alpha"},
		{"1.0.0+build", "1.0.0+build"},
		{"invalid", "invalid"}, // Returns original on error
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := Normalize(NPM, tt.version)
			if got != tt.want {
				t.Errorf("Normalize(NPM, %q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
