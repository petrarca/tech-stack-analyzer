package semver

import "strings"

// IsResolved reports whether v is a concrete, resolved release version (e.g.
// "1.2.3", "v1.2.3") as opposed to an unresolved form: an empty value, a tag
// or placeholder ("latest", "RELEASE"), a range or operator expression
// ("^1.2.0", ">=1"), a wildcard, an unresolved property reference ("${x}"), or
// a non-version source reference ("git:...", "file:...").
//
// This is the single source of truth for "do we have a version we can trust as
// a release identity". It is used both when emitting PURLs (a versionless PURL
// breaks advisory matching, so only resolved versions are appended) and when
// backfilling versions from authoritative sources (a resolved value must never
// be overwritten).
func IsResolved(v string) bool {
	return ResolvedVersion(v) != ""
}

// ResolvedVersion returns v when it is a concrete release, or an empty string
// when it is unresolved. The returned value is suitable for use as a PURL
// version. See IsResolved for the classification rules.
func ResolvedVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Unresolved placeholders and tags.
	switch v {
	case "latest", "git", "workspace", "local", "tarball", "RELEASE", "LATEST":
		return ""
	}
	// Range and operator characters (npm/composer/pypi/cargo), wildcards, and
	// Gradle/Maven property markers ("$", "{", "}"), plus Maven range brackets.
	if strings.ContainsAny(v, "^~><=*|$ {}[](),") {
		return ""
	}
	// Non-version source references.
	for _, prefix := range []string{"git:", "path:", "file:", "link:", "http://", "https://"} {
		if strings.HasPrefix(v, prefix) {
			return ""
		}
	}
	// A concrete version must start with a version character (a digit, or a
	// leading "v"/"V" followed by a digit) -- reject names and tokens.
	if !startsWithVersionChar(v) {
		return ""
	}
	return v
}

// startsWithVersionChar reports whether v begins with a character valid at the
// start of a concrete version: a digit, or a leading "v"/"V" followed by a
// digit (common for Go module versions like "v1.2.3").
func startsWithVersionChar(v string) bool {
	if v == "" {
		return false
	}
	c := v[0]
	if c >= '0' && c <= '9' {
		return true
	}
	if (c == 'v' || c == 'V') && len(v) > 1 && v[1] >= '0' && v[1] <= '9' {
		return true
	}
	return false
}
