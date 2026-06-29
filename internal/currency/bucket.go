package currency

import (
	"strconv"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
)

// Bucket is the currency classification of a dependency: how far behind latest,
// or why it could not be resolved. The unresolved reasons are recorded
// explicitly, never silently dropped.
type Bucket string

const (
	UpToDate        Bucket = "up_to_date"
	Patch           Bucket = "patch"
	Minor           Bucket = "minor"
	Major           Bucket = "major"
	Unsupported     Bucket = "unsupported_ecosystem" // no public registry exists
	Unknown         Bucket = "unknown"               // queried, not found (incl. internal/yanked)
	ResolutionError Bucket = "error"                 // transient lookup failure
)

// semverSystemFor maps a deps.dev system identifier to a semver.System parser
// for ordering comparison. Returns nil when no parser is available (the bucket
// then falls back to a numeric-component comparison only).
func semverSystemFor(depsDevSystem string) semver.System {
	switch depsDevSystem {
	case "npm":
		return semver.NPM
	case "pypi":
		return semver.PyPI
	case "cargo":
		// semver.Cargo.Parse always returns "not yet implemented", so this
		// silently falls through to the numeric-component comparison in classify.
		// When semver.Cargo is implemented the behavior will change; the test
		// case "cargo numeric fallback" in bucket_test.go locks in current
		// behavior so that change is visible.
		return semver.Cargo
	case "maven":
		return semver.Maven
	default:
		// nuget, go, rubygems: no dedicated parser; numeric comparison is used.
		return nil
	}
}

// classify returns the currency bucket for an installed/latest pair under a
// deps.dev system. Both versions are assumed resolvable (the caller handles the
// unresolved cases: unsupported, unknown, error).
//
// Algorithm:
//  1. If a semver parser exists for the system, use Version.Compare to establish
//     ordering. If installed >= latest, the package is up to date (covers equal,
//     and an installed pin ahead of the deps.dev default, e.g. a pre-release).
//  2. Otherwise determine the level by comparing numeric version components
//     (split on '.'): first differing component at index 0 -> major, index 1 ->
//     minor, index 2+ -> patch.
//  3. If either version cannot be parsed at all, return Unknown.
func classify(depsDevSystem, installed, latest string) Bucket {
	installed = strings.TrimSpace(installed)
	latest = strings.TrimSpace(latest)
	if installed == "" || latest == "" {
		return Unknown
	}
	if installed == latest {
		return UpToDate
	}

	// Ordering: never report "behind" when installed is equal-or-newer.
	if sys := semverSystemFor(depsDevSystem); sys != nil {
		iv, ierr := sys.Parse(installed)
		lv, lerr := sys.Parse(latest)
		if ierr == nil && lerr == nil {
			if iv.Compare(lv) >= 0 {
				return UpToDate
			}
			return levelOf(installed, latest)
		}
		// Unparseable under the dedicated system: fall through to numeric compare.
	}

	// No parser (nuget/go/rubygems) or parse failure: numeric-component compare.
	return numericBucket(installed, latest)
}

// numericBucket compares two versions by numeric components without a semver
// parser. If components cannot be parsed as integers, returns Unknown.
func numericBucket(installed, latest string) Bucket {
	ic, iok := numericComponents(installed)
	lc, lok := numericComponents(latest)
	if !iok || !lok {
		return Unknown
	}
	// Ordering check: if installed >= latest numerically, up to date.
	if compareComponents(ic, lc) >= 0 {
		return UpToDate
	}
	return levelFromComponents(ic, lc)
}

// levelOf buckets two ordered version strings (latest strictly newer) by the
// first differing numeric component.
func levelOf(installed, latest string) Bucket {
	ic, iok := numericComponents(installed)
	lc, lok := numericComponents(latest)
	if !iok || !lok {
		return Unknown
	}
	return levelFromComponents(ic, lc)
}

// levelFromComponents returns the bucket from the first differing component.
func levelFromComponents(a, b []int) Bucket {
	switch {
	case component(a, 0) != component(b, 0):
		return Major
	case component(a, 1) != component(b, 1):
		return Minor
	default:
		return Patch
	}
}

// numericComponents parses the leading numeric dotted components of a version
// (e.g. "1.2.3-rc1" -> [1 2 3]). Stops at the first non-numeric component.
// Returns ok=false if there is no leading numeric component at all.
func numericComponents(v string) ([]int, bool) {
	// Drop any build/pre-release suffix after '-' or '+'.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// component returns the i-th component or 0 if absent (missing trailing
// components are treated as 0, so "1.2" == "1.2.0").
func component(c []int, i int) int {
	if i < len(c) {
		return c[i]
	}
	return 0
}

// compareComponents compares two numeric component slices lexicographically,
// padding the shorter with zeros. Returns -1, 0, or 1.
func compareComponents(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ai, bi := component(a, i), component(b, i)
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
