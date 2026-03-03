package semver

import (
	"testing"
)

func TestPyPIVersionParsing(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
		canon   string
	}{
		// Basic versions
		{name: "simple version", version: "1.0.0", canon: "1.0.0"},
		{name: "two part version", version: "1.0", canon: "1.0"},
		{name: "four part version", version: "1.2.3.4", canon: "1.2.3.4"},

		// Epoch
		{name: "with epoch", version: "1!2.0.0", canon: "1!2.0.0"},
		{name: "epoch zero", version: "0!1.0.0", canon: "1.0.0"},

		// Pre-releases
		{name: "alpha", version: "1.0.0a1", canon: "1.0.0a1"},
		{name: "alpha long", version: "1.0.0alpha1", canon: "1.0.0a1"},
		{name: "beta", version: "1.0.0b2", canon: "1.0.0b2"},
		{name: "beta long", version: "1.0.0beta2", canon: "1.0.0b2"},
		{name: "rc", version: "1.0.0rc3", canon: "1.0.0rc3"},
		{name: "rc short", version: "1.0.0c3", canon: "1.0.0rc3"},
		{name: "alpha no number", version: "1.0.0a", canon: "1.0.0a0"},

		// Post-releases
		{name: "post", version: "1.0.0.post1", canon: "1.0.0.post1"},
		{name: "post short", version: "1.0.0post1", canon: "1.0.0.post1"},
		{name: "post dash", version: "1.0.0-1", canon: "1.0.0.post1"},
		{name: "underscore as separator", version: "1.0.0_1", canon: "1.0.0.1"},

		// Dev releases
		{name: "dev", version: "1.0.0.dev0", canon: "1.0.0.dev0"},
		{name: "dev short", version: "1.0.0dev0", canon: "1.0.0.dev0"},
		{name: "dev no number", version: "1.0.0.dev", canon: "1.0.0.dev0"},

		// Local versions
		{name: "local", version: "1.0.0+local", canon: "1.0.0+local"},
		{name: "local complex", version: "1.0.0+ubuntu.1", canon: "1.0.0+ubuntu.1"},

		// Complex combinations
		{name: "all parts", version: "1!1.0.0a1.post2.dev3+local", canon: "1!1.0.0a1.post2.dev3+local"},
		{name: "pre and post", version: "1.0.0b1.post2", canon: "1.0.0b1.post2"},
		{name: "post and dev", version: "1.0.0.post1.dev0", canon: "1.0.0.post1.dev0"},

		// Case insensitivity
		{name: "uppercase", version: "1.0.0A1", canon: "1.0.0a1"},
		{name: "mixed case", version: "1.0.0Beta2", canon: "1.0.0b2"},

		// Normalization
		{name: "leading zeros", version: "1.02.003", canon: "1.2.3"},
		{name: "trailing dot", version: "1.0.", canon: "1.0"},
		{name: "underscore separators", version: "1_0_0", canon: "1.0.0"},

		// Error cases
		{name: "empty", version: "", wantErr: true},
		{name: "no numbers", version: "abc", wantErr: true},
		{name: "invalid epoch", version: "a!1.0.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := PyPI.Parse(tt.version)

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

func TestPyPIVersionComparison(t *testing.T) {
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
		{name: "patch less", v1: "1.0.0", v2: "1.0.1", want: -1},

		// Length differences
		{name: "shorter less", v1: "1.0", v2: "1.0.0", want: -1},
		{name: "longer greater", v1: "1.0.0", v2: "1.0", want: 1},

		// Epoch
		{name: "epoch greater", v1: "1!1.0.0", v2: "2.0.0", want: 1},
		{name: "epoch less", v1: "1.0.0", v2: "1!1.0.0", want: -1},

		// Pre-releases
		{name: "release > pre", v1: "1.0.0", v2: "1.0.0a1", want: 1},
		{name: "pre < release", v1: "1.0.0a1", v2: "1.0.0", want: -1},
		{name: "alpha < beta", v1: "1.0.0a1", v2: "1.0.0b1", want: -1},
		{name: "beta < rc", v1: "1.0.0b1", v2: "1.0.0rc1", want: -1},
		{name: "alpha number", v1: "1.0.0a1", v2: "1.0.0a2", want: -1},

		// Post-releases
		{name: "no post < post", v1: "1.0.0", v2: "1.0.0.post1", want: -1},
		{name: "post > no post", v1: "1.0.0.post1", v2: "1.0.0", want: 1},
		{name: "post number", v1: "1.0.0.post1", v2: "1.0.0.post2", want: -1},

		// Dev releases
		{name: "release > dev", v1: "1.0.0", v2: "1.0.0.dev0", want: 1},
		{name: "dev < release", v1: "1.0.0.dev0", v2: "1.0.0", want: -1},
		{name: "dev number", v1: "1.0.0.dev1", v2: "1.0.0.dev2", want: -1},

		// Complex
		{name: "pre post", v1: "1.0.0a1", v2: "1.0.0a1.post1", want: -1},
		{name: "post dev", v1: "1.0.0.post1", v2: "1.0.0.post1.dev0", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := PyPI.Parse(tt.v1)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.v1, err)
			}

			v2, err := PyPI.Parse(tt.v2)
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

func TestNormalize(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"1.0.0", "1.0.0"},
		{"1.02.003", "1.2.3"},
		{"1.0.0alpha1", "1.0.0a1"},
		{"1.0.0Beta2", "1.0.0b2"},
		{"1.0.0-1", "1.0.0.post1"},
		{"invalid", "invalid"}, // Returns original on error
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := Normalize(PyPI, tt.version)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
