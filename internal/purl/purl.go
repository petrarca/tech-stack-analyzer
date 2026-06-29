// Package purl constructs Package URLs (PURLs) from dependency coordinates.
// It is the single source of truth for PURL encoding, shared by the SBOM
// producer and the currency artifact.
package purl

import (
	"net/url"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Build assembles a Package URL for a dependency.
// Returns an empty string when no usable PURL can be formed (unknown type).
// Version is included only when resolved (concrete pin, not a range specifier).
func Build(dep types.Dependency) string {
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
	// advisory matching.
	if v := semver.ResolvedVersion(dep.Version); v != "" {
		b.WriteString("@")
		b.WriteString(url.PathEscape(v))
	}
	return b.String()
}

// purlType maps a dependency type to its PURL type identifier.
// Gradle artifacts use Maven coordinates on deps.dev and in PURLs.
func purlType(depType string) string {
	if depType == "gradle" {
		return "maven"
	}
	known := map[string]bool{
		"npm": true, "maven": true, "pypi": true, "nuget": true,
		"cargo": true, "golang": true, "gem": true, "composer": true,
		"pub": true, "conan": true, "docker": true, "golang-direct": true,
	}
	if known[depType] {
		return depType
	}
	return ""
}

// splitNamespace separates a dependency name into its PURL namespace and name
// components. The rules are ecosystem-specific.
func splitNamespace(ptype, rawName string) (namespace, name string) {
	switch ptype {
	case "maven":
		if i := strings.Index(rawName, ":"); i >= 0 {
			return rawName[:i], rawName[i+1:]
		}
		return "", rawName
	case "npm":
		// Scoped packages: "@scope/name" -> namespace="@scope", name="name".
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

// encodeSegment percent-encodes a PURL segment, explicitly encoding '@' (which
// url.PathEscape does not encode, but PURL requires encoded in type/namespace).
func encodeSegment(seg string) string {
	parts := strings.Split(seg, "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(url.PathEscape(p), "@", "%40")
	}
	return strings.Join(parts, "/")
}
