package license

import (
	"testing"
)

func TestNormalizer_Normalize(t *testing.T) {
	normalizer := NewNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// MIT variations
		{"MIT lowercase", "mit", "MIT"},
		{"MIT uppercase", "MIT", "MIT"},
		{"MIT with spaces", "mit license", "MIT"},
		{"MIT expat", "expat", "MIT"},

		// Apache variations
		{"Apache lowercase", "apache", "Apache-2.0"},
		{"Apache-2.0", "apache-2.0", "Apache-2.0"},
		{"Apache with spaces", "apache 2.0", "Apache-2.0"},
		{"Apache uppercase", "Apache-2.0", "Apache-2.0"},

		// GPL variations
		{"GPL lowercase", "gpl", "GPL-3.0"},
		{"GPL-3.0", "gpl-3.0", "GPL-3.0"},
		{"GPLv3", "gplv3", "GPL-3.0"},
		{"GPL-2.0", "gpl-2.0", "GPL-2.0"},

		// BSD variations
		{"BSD simple", "bsd", "BSD-3-Clause"},
		{"BSD-3-Clause", "bsd-3-clause", "BSD-3-Clause"},
		{"BSD-2-Clause", "bsd-2-clause", "BSD-2-Clause"},

		// Other licenses
		{"ISC lowercase", "isc", "ISC"},
		{"ISC uppercase", "ISC", "ISC"},
		{"Unlicense", "unlicense", "Unlicense"},
		{"CC0", "cc0", "CC0-1.0"},
		{"MPL", "mpl", "MPL-2.0"},

		// Already SPDX (should pass through)
		{"Already SPDX MIT", "MIT", "MIT"},
		{"Already SPDX Apache", "Apache-2.0", "Apache-2.0"},

		// AGPL variations
		{"AGPL lowercase", "agpl", "AGPL-3.0-only"},
		{"AGPL-3.0", "AGPL-3.0", "AGPL-3.0-only"},
		{"AGPL-3.0-only", "AGPL-3.0-only", "AGPL-3.0-only"},
		{"AGPL-3.0-or-later", "AGPL-3.0-or-later", "AGPL-3.0-or-later"},

		// GPL -only/-or-later variants
		{"GPL-3.0-only", "GPL-3.0-only", "GPL-3.0-only"},
		{"GPL-3.0-or-later", "GPL-3.0-or-later", "GPL-3.0-or-later"},
		{"GPL-2.0-only", "GPL-2.0-only", "GPL-2.0-only"},
		{"GPL-2.0-or-later", "GPL-2.0-or-later", "GPL-2.0-or-later"},

		// EPL
		{"EPL simple", "epl", "EPL-2.0"},
		{"EPL-1.0", "EPL-1.0", "EPL-1.0"},
		{"EPL-2.0", "EPL-2.0", "EPL-2.0"},

		// CDDL
		{"CDDL simple", "cddl", "CDDL-1.0"},
		{"CDDL-1.0", "CDDL-1.0", "CDDL-1.0"},

		// Artistic
		{"Artistic", "artistic", "Artistic-2.0"},
		{"Artistic-2.0", "Artistic-2.0", "Artistic-2.0"},

		// Zlib
		{"Zlib lowercase", "zlib", "Zlib"},

		// 0BSD
		{"0BSD lowercase", "0bsd", "0BSD"},
		{"0BSD uppercase", "0BSD", "0BSD"},

		// BSL (Boost)
		{"BSL", "bsl-1.0", "BSL-1.0"},
		{"Boost", "boost", "BSL-1.0"},

		// EUPL
		{"EUPL", "eupl-1.2", "EUPL-1.2"},

		// PostgreSQL
		{"PostgreSQL", "postgresql", "PostgreSQL"},

		// Edge cases
		{"Empty string", "", ""},
		{"Unknown license", "CustomLicense-1.0", "CustomLicense-1.0"},
		{"With quotes", `"MIT"`, "MIT"},
		{"With single quotes", `'MIT'`, "MIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizer_ParseTOMLLicense(t *testing.T) {
	normalizer := NewNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Simple string format
		{"Simple quoted", `"MIT"`, "MIT"},
		{"Simple single quoted", `'MIT'`, "MIT"},
		{"Simple unquoted", `MIT`, "MIT"},

		// TOML object format
		{"TOML object text", `{text = "MIT"}`, "MIT"},
		{"TOML object single quoted", `{text = 'MIT'}`, "MIT"},
		{"TOML object with spaces", `{ text = "MIT" }`, "MIT"},

		// Complex TOML
		{"TOML with file", `{file = "LICENSE"}`, "{file = \"LICENSE\"}"},
		{"TOML mixed", `{text = "Apache-2.0"}`, "Apache-2.0"},

		// Edge cases
		{"Empty string", "", ""},
		{"Just braces", `{}`, "{}"},
		{"Invalid TOML", `{invalid}`, "{invalid}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.ParseTOMLLicense(tt.input)
			if result != tt.expected {
				t.Errorf("ParseTOMLLicense(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizer_ParseLicenseExpression(t *testing.T) {
	normalizer := NewNormalizer()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// Single license
		{"Single MIT", "MIT", []string{"MIT"}},
		{"Single Apache", "Apache-2.0", []string{"Apache-2.0"}},

		// OR expressions
		{"MIT OR Apache", "MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"Apache OR GPL", "Apache-2.0 OR GPL-3.0", []string{"Apache-2.0", "GPL-3.0"}},
		{"Lowercase OR", "mit or apache-2.0", []string{"MIT", "Apache-2.0"}},

		// AND expressions
		{"MIT AND Apache", "MIT AND Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"Lowercase AND", "mit and apache-2.0", []string{"MIT", "Apache-2.0"}},

		// Symbolic operators
		{"Double pipe", "MIT || Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"Double ampersand", "MIT && Apache-2.0", []string{"MIT", "Apache-2.0"}},

		// Edge cases
		{"Empty string", "", nil},
		{"Just operator", "OR", nil},
		{"Unknown license", "UnknownLicense", []string{"UnknownLicense"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.ParseLicenseExpression(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseLicenseExpression(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, val := range result {
				if val != tt.expected[i] {
					t.Errorf("ParseLicenseExpression(%q)[%d] = %q, want %q", tt.input, i, val, tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizer_IsSPDXValid(t *testing.T) {
	normalizer := NewNormalizer()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid MIT", "MIT", true},
		{"Valid Apache", "Apache-2.0", true},
		{"Valid GPL", "GPL-3.0", true},
		{"Valid BSD", "BSD-3-Clause", true},
		{"Valid LGPL", "LGPL-2.1", true},
		{"Valid CC0", "CC0-1.0", true},
		{"Valid MPL", "MPL-2.0", true},

		{"Invalid unknown", "UnknownLicense-1.0", false},
		{"Invalid custom", "MyCustomLicense", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.IsSPDXValid(tt.input)
			if result != tt.expected {
				t.Errorf("IsSPDXValid(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
