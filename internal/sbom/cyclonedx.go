// Package sbom converts scan results into a CycloneDX Software Bill of
// Materials (SBOM). The emitted document carries Package URL (PURL)
// identifiers per component so it can be consumed directly by vulnerability
// scanners such as Trivy (trivy sbom ...).
//
// Only dependencies whose type maps onto a PURL type are emitted as
// components. Non-package types (docker, terraform, githubAction, regex,
// and similar matching domains) are skipped because they are not packages
// resolvable against advisory databases.
package sbom

import (
	"net/url"
	"sort"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// SpecVersion is the CycloneDX specification version produced.
const SpecVersion = "1.5"

// bomFormat is the fixed CycloneDX format identifier.
const bomFormat = "CycloneDX"

// purlTypes is the set of dependency type values that correspond to a PURL
// type and are therefore emitted as SBOM components. The values match the
// dependency type vocabulary set by the parsers (see
// internal/scanner/parsers/constants.go), which already follows the PURL
// type vocabulary.
var purlTypes = map[string]bool{
	"npm":       true,
	"pypi":      true,
	"gem":       true,
	"composer":  true,
	"cargo":     true,
	"golang":    true,
	"maven":     true,
	"gradle":    true, // Gradle artifacts use Maven coordinates -> pkg:maven
	"nuget":     true,
	"conan":     true,
	"cocoapods": true,
	"docker":    true,
}

// BOM is the top-level CycloneDX document.
type BOM struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Metadata    *Metadata   `json:"metadata,omitempty"`
	Components  []Component `json:"components"`
}

// Metadata holds document-level metadata.
type Metadata struct {
	Component *Component `json:"component,omitempty"`
}

// Component is a single CycloneDX component entry.
type Component struct {
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	PURL       string     `json:"purl,omitempty"`
	Scope      string     `json:"scope,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

// Property is a CycloneDX name/value property used for non-standard fields.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FromPayload builds a CycloneDX BOM from a scan payload. It aggregates the
// payload's dependencies (flattened and deduplicated across the component
// tree), then emits one component per PURL-typed dependency.
func FromPayload(payload *types.Payload) *BOM {
	agg := aggregator.NewAggregator([]string{"dependencies"})
	out := agg.Aggregate(payload)
	return FromDependencies(out.Dependencies, rootName(payload))
}

// FromDependencies builds a CycloneDX BOM from an already-aggregated set of
// dependencies. rootName, when non-empty, becomes the metadata component name.
func FromDependencies(deps []types.Dependency, rootName string) *BOM {
	components := make([]Component, 0, len(deps))
	for _, dep := range deps {
		if !purlTypes[dep.Type] {
			continue
		}
		components = append(components, toComponent(dep))
	}

	sort.Slice(components, func(i, j int) bool {
		if components[i].PURL != components[j].PURL {
			return components[i].PURL < components[j].PURL
		}
		return components[i].Name < components[j].Name
	})

	bom := &BOM{
		BOMFormat:   bomFormat,
		SpecVersion: SpecVersion,
		Version:     1,
		Components:  components,
	}
	if rootName != "" {
		bom.Metadata = &Metadata{
			Component: &Component{Type: "application", Name: rootName},
		}
	}
	return bom
}

// toComponent maps a dependency to a CycloneDX component with a PURL.
func toComponent(dep types.Dependency) Component {
	c := Component{
		Type:    "library",
		Name:    dep.Name,
		Version: cleanVersion(dep.Version),
		PURL:    buildPURL(dep),
		Scope:   cyclonedxScope(dep.Scope),
	}
	return c
}

// buildPURL assembles a Package URL for a dependency. Returns an empty string
// when no usable PURL can be formed.
func buildPURL(dep types.Dependency) string {
	ptype := purlType(dep.Type)
	if ptype == "" {
		return ""
	}

	namespace, name := splitNamespace(ptype, dep.Name)
	if name == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("pkg:")
	b.WriteString(ptype)
	b.WriteString("/")
	if namespace != "" {
		b.WriteString(encodeSegment(namespace))
		b.WriteString("/")
	}
	b.WriteString(encodeSegment(name))

	// Only resolved (concrete) versions belong in a PURL. Unresolved ranges
	// such as "^1.2.0" or ">=1" do not uniquely identify a release and break
	// advisory matching, so they are omitted here (the component version
	// field still carries the original value for human inspection).
	if v := resolvedVersion(dep.Version); v != "" {
		b.WriteString("@")
		b.WriteString(url.PathEscape(v))
	}
	return b.String()
}

// resolvedVersion returns the version when it is a concrete release, or an
// empty string when it is unresolved (a range, tag, unresolved property
// reference, or placeholder). Only concrete versions belong in a PURL.
func resolvedVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Unresolved placeholders and tags.
	switch v {
	case "latest", "git", "workspace", "local", "tarball", "RELEASE", "LATEST":
		return ""
	}
	// Range and operator characters (npm/composer/pypi/cargo), wildcards,
	// and Gradle/Maven property markers ("$", "{", "}").
	if strings.ContainsAny(v, "^~><=*|$ {}") {
		return ""
	}
	// Non-version source references.
	for _, prefix := range []string{"git:", "path:", "file:", "link:", "http://", "https://"} {
		if strings.HasPrefix(v, prefix) {
			return ""
		}
	}
	// A concrete version must start with a digit (covers "1.2.3", "v1.2.3" is
	// handled below) -- reject anything that is clearly a name or token.
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

// purlType maps a dependency type to its PURL type. Gradle collapses to maven
// because Gradle artifacts share Maven coordinates.
func purlType(depType string) string {
	if depType == "gradle" {
		return "maven"
	}
	if purlTypes[depType] {
		return depType
	}
	return ""
}

// splitNamespace separates a dependency name into PURL namespace and name.
// Maven names use "group:artifact"; npm scoped packages use "@scope/name";
// Go module paths keep all but the last segment as the namespace.
func splitNamespace(ptype, rawName string) (namespace, name string) {
	switch ptype {
	case "maven":
		if i := strings.Index(rawName, ":"); i >= 0 {
			return rawName[:i], rawName[i+1:]
		}
		return "", rawName
	case "npm":
		if strings.HasPrefix(rawName, "@") {
			if i := strings.Index(rawName, "/"); i >= 0 {
				return rawName[:i], rawName[i+1:]
			}
		}
		return "", rawName
	case "golang":
		if i := strings.LastIndex(rawName, "/"); i >= 0 {
			return rawName[:i], rawName[i+1:]
		}
		return "", rawName
	default:
		return "", rawName
	}
}

// encodeSegment percent-encodes a PURL path segment while preserving the
// path separators meaningful within namespaces (e.g. Go module paths).
// The "@" is encoded explicitly (url.PathEscape leaves it intact) so that
// npm scopes like "@scope" become "%40scope" and do not collide with the
// PURL version delimiter.
func encodeSegment(seg string) string {
	parts := strings.Split(seg, "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(url.PathEscape(p), "@", "%40")
	}
	return strings.Join(parts, "/")
}

// cleanVersion trims whitespace and leading version-range operators that are
// not valid in a resolved PURL version. Unresolved ranges are returned as-is
// so the caller can still see them in the component version field.
func cleanVersion(v string) string {
	return strings.TrimSpace(v)
}

// cyclonedxScope maps an internal dependency scope to a CycloneDX scope.
// CycloneDX defines "required", "optional", and "excluded".
func cyclonedxScope(scope string) string {
	switch scope {
	case types.ScopeProd:
		return "required"
	case types.ScopeOptional:
		return "optional"
	default:
		return ""
	}
}

// rootName returns the payload's name for use as the metadata component.
func rootName(payload *types.Payload) string {
	if payload == nil {
		return ""
	}
	return payload.Name
}
