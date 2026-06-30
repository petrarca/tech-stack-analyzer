package license

import "testing"

func TestCategoryOf(t *testing.T) {
	tests := []struct {
		license string
		want    Category
	}{
		{"MIT", CategoryNotice},
		{"Apache-2.0", CategoryNotice},
		{"BSD-3-Clause", CategoryNotice},
		{"GPL-3.0-only", CategoryRestricted},
		{"LGPL-2.1", CategoryRestricted},
		{"AGPL-3.0", CategoryForbidden},
		{"MPL-2.0", CategoryReciprocal},
		{"EPL-2.0", CategoryReciprocal},
		{"Unlicense", CategoryUnencumbered},
		{"CC0-1.0", CategoryUnencumbered},
		{"mit", CategoryNotice}, // case-insensitive
		{"  Apache-2.0  ", CategoryNotice},
		{"NoSuchLicense", CategoryUnknown},
		{"", CategoryUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			if got := CategoryOf(tt.license); got != tt.want {
				t.Errorf("CategoryOf(%q) = %q, want %q", tt.license, got, tt.want)
			}
		})
	}
}

func TestCategoryForExpressionString_Fold(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want Category
	}{
		// AND -> more restrictive branch wins.
		{"AND takes restricted over notice", "MIT AND GPL-3.0-only", CategoryRestricted},
		{"AND takes forbidden over notice", "Apache-2.0 AND AGPL-3.0", CategoryForbidden},
		// OR -> less restrictive branch wins.
		{"OR takes notice over restricted", "MIT OR GPL-3.0-only", CategoryNotice},
		{"OR takes notice over forbidden", "AGPL-3.0 OR Apache-2.0", CategoryNotice},
		// Simple.
		{"single restricted", "GPL-2.0-only", CategoryRestricted},
		// Mixed precedence: MIT OR (Apache-2.0 AND GPL-3.0-only)
		// AND-branch = restricted; OR picks the looser (MIT = notice).
		{"mixed", "MIT OR Apache-2.0 AND GPL-3.0-only", CategoryNotice},
		// Parenthesized: (MIT OR GPL-3.0) AND AGPL-3.0
		// left OR = notice (MIT); AND with forbidden -> forbidden.
		{"parens AND forbidden", "(MIT OR GPL-3.0-only) AND AGPL-3.0", CategoryForbidden},
		{"empty", "", CategoryUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryForExpressionString(tt.expr); got != tt.want {
				t.Errorf("CategoryForExpressionString(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}
