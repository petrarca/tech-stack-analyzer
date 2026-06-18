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
	"crypto/rand"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/aggregator"
	"github.com/petrarca/tech-stack-analyzer/internal/metadata"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// SpecVersion is the CycloneDX specification version produced.
const SpecVersion = "1.7"

// jsonSchema is the CycloneDX JSON schema URL for SpecVersion, emitted as the
// document's "$schema" field.
const jsonSchema = "http://cyclonedx.org/schema/bom-1.7.schema.json"

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
	"pub":       true,
	"hex":       true,
	"swift":     true,
	"cpan":      true,
	"cran":      true,
	"docker":    true,
}

// BOM is the top-level CycloneDX document. Field order follows the CycloneDX
// JSON schema ($schema, bomFormat, specVersion, serialNumber, version, ...).
type BOM struct {
	JSONSchema   string      `json:"$schema,omitempty"`
	BOMFormat    string      `json:"bomFormat"`
	SpecVersion  string      `json:"specVersion"`
	SerialNumber string      `json:"serialNumber,omitempty"`
	Version      int         `json:"version"`
	Metadata     *Metadata   `json:"metadata,omitempty"`
	Components   []Component `json:"components"`
}

// Metadata holds document-level metadata.
type Metadata struct {
	Timestamp  string     `json:"timestamp,omitempty"`
	Component  *Component `json:"component,omitempty"`
	Properties []Property `json:"properties,omitempty"`
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
//
// When the scan resolved a dependency graph (--dependency-graph full, populating
// each component's DependencyEdges), the transitive nodes discovered there --
// which carry resolved versions from lockfiles/tree-files or deps.dev -- are
// also emitted as components, deduplicated against the declared set. This gives
// the SBOM transitive breadth without a recursive package-manager crawl: the
// resolved graph is the source of truth. Without graph resolution the SBOM
// contains only declared dependencies (the default).
func FromPayload(payload *types.Payload) *BOM {
	agg := aggregator.NewAggregator([]string{"dependencies"})
	out := agg.Aggregate(payload)
	bom := FromDependencies(out.Dependencies, rootName(payload))
	addTransitiveComponents(bom, payload)
	applyUserMetadata(bom, payload)
	return bom
}

// FromPayloadDirect builds a CycloneDX BOM from only the payload's direct
// dependencies (those the project declares in its manifests), without folding
// in any transitive dependency-graph nodes. Use this to emit a strictly
// direct-dependency SBOM even from a scan that captured the full transitive
// graph. This is the direct/transitive axis, distinct from the declared/
// resolved version axis -- the emitted versions are still the resolved ones.
func FromPayloadDirect(payload *types.Payload) *BOM {
	agg := aggregator.NewAggregator([]string{"dependencies"})
	out := agg.Aggregate(payload)
	// Keep only dependencies flagged direct. Some ecosystem parsers (e.g. npm
	// from a lockfile) emit transitive dependencies into the flat list with
	// Direct=false; those must be excluded here, in addition to not folding in
	// the transitive graph nodes.
	direct := make([]types.Dependency, 0, len(out.Dependencies))
	for _, d := range out.Dependencies {
		if d.Direct {
			direct = append(direct, d)
		}
	}
	bom := FromDependencies(direct, rootName(payload))
	applyUserMetadata(bom, payload)
	return bom
}

// applyUserMetadata copies the user-provided scan metadata (the config
// `properties`, e.g. product_key/team/owner) onto the BOM's metadata.properties.
// Only this user-supplied metadata is surfaced -- never internal scan data. The
// keys are namespaced under "stack-analyzer:" to avoid collisions with other
// tools' properties. Values are sorted for deterministic output.
func applyUserMetadata(bom *BOM, payload *types.Payload) {
	props := userMetadataProperties(payload)
	if len(props) == 0 {
		return
	}
	if bom.Metadata == nil {
		bom.Metadata = &Metadata{}
	}

	// The scanner treats `properties` as opaque user key/values -- it defines no
	// canonical "product name" key -- so all user metadata is surfaced verbatim
	// as metadata.properties (the user's own key names, unprefixed) and the SBOM
	// subject name (metadata.component) is left as the scanned root.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		bom.Metadata.Properties = append(bom.Metadata.Properties, Property{
			Name:  k,
			Value: props[k],
		})
	}
}

// userMetadataProperties extracts the user-provided scan-metadata properties
// from the payload as a flat string map. It handles both shapes the metadata
// takes: the typed *metadata.ScanMetadata (a live scan) and the
// map[string]interface{} produced when a saved scan JSON is unmarshaled (the
// sbom command). Only scalar values are emitted (strings/numbers/bools);
// non-scalar values are skipped. Returns nil when there are none.
func userMetadataProperties(payload *types.Payload) map[string]string {
	if payload == nil || payload.Metadata == nil {
		return nil
	}

	var raw map[string]interface{}
	switch m := payload.Metadata.(type) {
	case *metadata.ScanMetadata:
		raw = m.Properties
	case map[string]interface{}:
		if p, ok := m["properties"].(map[string]interface{}); ok {
			raw = p
		}
	}
	if len(raw) == 0 {
		return nil
	}

	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := scalarString(v); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// scalarString renders a scalar metadata value as a string, or returns
// ok=false for non-scalar values (maps, slices) which are not user metadata.
func scalarString(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	default:
		return "", false
	}
}

// addTransitiveComponents folds dependency-graph nodes into the BOM as
// components. Edge nodes are bare "name@version" strings with no ecosystem tag,
// so the owning component's ComponentType (which is known per payload) supplies
// the PURL type. Components already present (by PURL) are not duplicated;
// added ones are marked scope "" and carry no declared metadata.
func addTransitiveComponents(bom *BOM, payload *types.Payload) {
	existing := make(map[string]bool, len(bom.Components))
	for _, c := range bom.Components {
		if c.PURL != "" {
			existing[c.PURL] = true
		}
	}

	var added []Component
	var walk func(p *types.Payload)
	walk = func(p *types.Payload) {
		depType := componentTypeToDepType(p.ComponentType)
		if depType != "" {
			for _, node := range graphNodes(p.DependencyEdges) {
				name, version := splitNodeVersion(node)
				if name == "" {
					continue
				}
				c := toComponent(types.Dependency{Type: depType, Name: name, Version: version})
				if c.PURL == "" || existing[c.PURL] {
					continue
				}
				existing[c.PURL] = true
				added = append(added, c)
			}
		}
		for _, child := range p.Children {
			walk(child)
		}
	}
	walk(payload)
	if len(added) == 0 {
		return
	}

	bom.Components = append(bom.Components, added...)
	sort.Slice(bom.Components, func(i, j int) bool {
		if bom.Components[i].PURL != bom.Components[j].PURL {
			return bom.Components[i].PURL < bom.Components[j].PURL
		}
		return bom.Components[i].Name < bom.Components[j].Name
	})
}

// graphNodes returns the unique node identities appearing in edges (both
// endpoints), excluding the synthetic root marker ".".
func graphNodes(edges []types.DependencyEdge) []string {
	seen := make(map[string]bool)
	var nodes []string
	for _, e := range edges {
		for _, n := range []string{e.From, e.To} {
			if n == "" || n == "." || seen[n] {
				continue
			}
			seen[n] = true
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// splitNodeVersion splits a "name@version" node identity into its name and
// version. For npm scoped packages ("@scope/name@version") only the final "@"
// separates the version. Returns an empty name when no version delimiter is
// present (an unresolved node is not a useful component).
func splitNodeVersion(node string) (name, version string) {
	i := strings.LastIndex(node, "@")
	if i <= 0 { // no "@", or leading "@" only (scoped name without version)
		return "", ""
	}
	return node[:i], node[i+1:]
}

// componentTypeToDepType maps a payload ComponentType (ecosystem) to the
// dependency type vocabulary used to build PURLs. Returns "" for ecosystems
// that do not map to a PURL type.
func componentTypeToDepType(componentType string) string {
	switch componentType {
	case "nodejs":
		return "npm"
	case "python":
		return "pypi"
	case "ruby":
		return "gem"
	case "php":
		return "composer"
	case "rust":
		return "cargo"
	default:
		// maven, gradle, golang, nuget, etc. already use the PURL-type name.
		if purlTypes[componentType] {
			return componentType
		}
		return ""
	}
}

// FromDependencies builds a CycloneDX BOM from an already-aggregated set of
// dependencies. rootName, when non-empty, becomes the metadata component name.
func FromDependencies(deps []types.Dependency, rootName string) *BOM {
	components := make([]Component, 0, len(deps))
	for _, dep := range deps {
		if !purlTypes[dep.Type] {
			continue
		}
		// Maven BOM imports (scope=import) are version-management entries, not
		// packages: they declare no artifact of their own. The parser keeps
		// them as a tech-detection signal, but they must not appear as SBOM
		// components (they have no resolvable artifact to scan). This matches
		// Maven semantics and Trivy, which never emits import-scope entries.
		if dep.Scope == types.ScopeImport {
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
		JSONSchema:  jsonSchema,
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
	// field still carries the original value for human inspection). The
	// classification lives in semver.ResolvedVersion (single source of truth).
	if v := semver.ResolvedVersion(dep.Version); v != "" {
		b.WriteString("@")
		b.WriteString(url.PathEscape(v))
	}
	return b.String()
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

// Stamp sets the document's per-emission identity: a unique serialNumber
// (urn:uuid) and the metadata timestamp (RFC 3339, UTC). These are
// non-deterministic by nature, so they are applied at output time rather than
// in the pure builders -- keeping FromPayload/FromDependencies reproducible for
// tests and diffs. A no-op for a nil BOM.
func Stamp(bom *BOM) {
	if bom == nil {
		return
	}
	if id := newUUIDv4(); id != "" {
		bom.SerialNumber = "urn:uuid:" + id
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	if bom.Metadata == nil {
		bom.Metadata = &Metadata{}
	}
	bom.Metadata.Timestamp = ts
}

// newUUIDv4 returns a random RFC 4122 version-4 UUID, or "" if the system
// random source is unavailable (in which case serialNumber is simply omitted).
func newUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
