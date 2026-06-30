package license

import "strings"

// This file classifies SPDX licenses into risk categories and computes the
// effective category of a license expression. Categories follow the common
// compliance taxonomy (also used by the Google license classifier and others);
// the SPDX-id -> category assignments are public facts about each license.
//
// This is a compliance/inventory signal, not vulnerability scanning. It lets a
// consumer answer "does this stack contain a copyleft/forbidden license?".

// Category is a license risk category.
type Category string

const (
	// CategoryForbidden licenses are generally disallowed in proprietary use.
	CategoryForbidden Category = "forbidden"
	// CategoryRestricted licenses impose strong (often copyleft) obligations.
	CategoryRestricted Category = "restricted"
	// CategoryReciprocal licenses require sharing modifications to the file/library.
	CategoryReciprocal Category = "reciprocal"
	// CategoryNotice licenses require attribution/notice only.
	CategoryNotice Category = "notice"
	// CategoryPermissive licenses impose minimal obligations.
	CategoryPermissive Category = "permissive"
	// CategoryUnencumbered licenses are public-domain-equivalent.
	CategoryUnencumbered Category = "unencumbered"
	// CategoryUnknown is used when a license is not classified.
	CategoryUnknown Category = "unknown"
)

// categorySeverity orders categories from most to least restrictive. A higher
// value is more restrictive. Used to fold AND (take max) and OR (take min).
//
// CategoryNotice and CategoryPermissive share severity 2 intentionally: both
// impose only attribution-tier obligations in practice. If a future distinction
// is needed (e.g. categorising BSD vs Beerware differently), give Permissive a
// lower value and add more licenses to that bucket.
var categorySeverity = map[Category]int{
	CategoryForbidden:    6,
	CategoryRestricted:   5,
	CategoryReciprocal:   4,
	CategoryNotice:       2,
	CategoryPermissive:   2,
	CategoryUnencumbered: 1,
	CategoryUnknown:      0,
}

// licenseCategories maps a canonical SPDX id to its risk category. Keys are
// lower-cased for case-insensitive lookup. The list covers the licenses that
// occur in practice across the ecosystems this tool scans; unlisted licenses
// resolve to CategoryUnknown.
var licenseCategories = buildLicenseCategories()

func buildLicenseCategories() map[string]Category {
	groups := map[Category][]string{
		CategoryForbidden: {
			"AGPL-1.0", "AGPL-1.0-only", "AGPL-1.0-or-later",
			"AGPL-3.0", "AGPL-3.0-only", "AGPL-3.0-or-later",
			"CC-BY-NC-1.0", "CC-BY-NC-2.0", "CC-BY-NC-3.0", "CC-BY-NC-4.0",
			"CC-BY-NC-SA-1.0", "CC-BY-NC-SA-2.0", "CC-BY-NC-SA-3.0", "CC-BY-NC-SA-4.0",
			"CC-BY-NC-ND-4.0", "WTFPL", "SSPL-1.0", "BUSL-1.1",
		},
		CategoryRestricted: {
			"GPL-1.0", "GPL-1.0-only", "GPL-1.0-or-later",
			"GPL-2.0", "GPL-2.0-only", "GPL-2.0-or-later",
			"GPL-3.0", "GPL-3.0-only", "GPL-3.0-or-later",
			"LGPL-2.0", "LGPL-2.0-only", "LGPL-2.0-or-later",
			"LGPL-2.1", "LGPL-2.1-only", "LGPL-2.1-or-later",
			"LGPL-3.0", "LGPL-3.0-only", "LGPL-3.0-or-later",
			"GFDL-1.1", "GFDL-1.2", "GFDL-1.3", "OSL-3.0", "QPL-1.0",
			"Sleepycat", "CC-BY-SA-4.0",
		},
		CategoryReciprocal: {
			"MPL-1.0", "MPL-1.1", "MPL-2.0",
			"EPL-1.0", "EPL-2.0", "CDDL-1.0", "CDDL-1.1",
			"CPL-1.0", "Ms-RL", "APSL-2.0", "EUPL-1.1", "EUPL-1.2",
		},
		CategoryNotice: {
			"Apache-1.0", "Apache-1.1", "Apache-2.0",
			"BSD-2-Clause", "BSD-3-Clause", "BSD-3-Clause-Clear", "BSD-4-Clause",
			"MIT", "MIT-0", "ISC", "Zlib", "libpng", "X11",
			"Artistic-1.0", "Artistic-2.0", "AFL-2.1", "AFL-3.0",
			"BSL-1.0", "PostgreSQL", "Python-2.0", "PSF-2.0", "Ruby",
			"OpenSSL", "Ms-PL", "ICU", "NCSA", "Zend-2.0",
			"CC-BY-3.0", "CC-BY-4.0",
		},
		CategoryPermissive: {
			"WTFPL-2.0", "Beerware",
		},
		CategoryUnencumbered: {
			"Unlicense", "CC0-1.0", "0BSD", "Public-Domain", "blessing",
		},
	}

	out := make(map[string]Category)
	for category, ids := range groups {
		for _, id := range ids {
			out[strings.ToLower(id)] = category
		}
	}
	return out
}

// CategoryOf returns the risk category of a single normalized SPDX license id.
func CategoryOf(licenseID string) Category {
	if cat, ok := licenseCategories[strings.ToLower(strings.TrimSpace(licenseID))]; ok {
		return cat
	}
	return CategoryUnknown
}

// CategoryOfExpression returns the effective category of a parsed license
// expression. For a compound expression: AND takes the more restrictive
// (maximum-severity) branch -- you must satisfy both -- while OR takes the less
// restrictive (minimum-severity) branch -- you may pick the looser license.
func CategoryOfExpression(expr Expression) Category {
	switch e := expr.(type) {
	case SimpleExpr:
		return CategoryOf(e.License)
	case CompoundExpr:
		left := CategoryOfExpression(e.Left)
		right := CategoryOfExpression(e.Right)
		return combineCategories(e.Op, left, right)
	default:
		return CategoryUnknown
	}
}

// combineCategories folds two categories under an operator: AND -> max severity,
// OR -> min severity.
func combineCategories(op Operator, left, right Category) Category {
	if op == OpAnd {
		if categorySeverity[left] >= categorySeverity[right] {
			return left
		}
		return right
	}
	// OR: take the less restrictive branch.
	if categorySeverity[left] <= categorySeverity[right] {
		return left
	}
	return right
}

// CategoryForExpressionString parses a license expression string and returns its
// effective risk category. License ids should already be normalized to SPDX.
func CategoryForExpressionString(expr string) Category {
	parsed := ParseExpression(expr)
	if parsed == nil {
		return CategoryUnknown
	}
	return CategoryOfExpression(parsed)
}
