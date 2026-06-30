package license

import "github.com/petrarca/tech-stack-analyzer/internal/types"

// dependencyLicenseKey is the metadata key under which a harvested per-dependency
// license is recorded. Stored in metadata to keep the dependency JSON array
// shape unchanged (schema-safe, additive).
const dependencyLicenseKey = "license"

// ApplyHarvesters annotates every dependency in the payload tree with a license
// resolved by the matching ecosystem harvester, writing it into the
// dependency's metadata under "license". Dependencies that already carry a
// license, or for which no harvester/license is found, are left unchanged.
// Returns the number of licenses added.
func ApplyHarvesters(payload *types.Payload, harvesters []Harvester) int {
	if payload == nil || len(harvesters) == 0 {
		return 0
	}
	byEcosystem := make(map[string]Harvester, len(harvesters))
	for _, h := range harvesters {
		byEcosystem[h.Ecosystem()] = h
	}
	return applyHarvestersWalk(payload, byEcosystem)
}

// applyHarvestersWalk recurses the component tree applying harvesters to each
// component's dependencies.
func applyHarvestersWalk(p *types.Payload, byEcosystem map[string]Harvester) int {
	added := 0
	for i := range p.Dependencies {
		if annotateDependencyLicense(&p.Dependencies[i], byEcosystem) {
			added++
		}
	}
	for _, child := range p.Children {
		added += applyHarvestersWalk(child, byEcosystem)
	}
	return added
}

// annotateDependencyLicense resolves and records a license for one dependency.
// Returns true when a license was added.
func annotateDependencyLicense(dep *types.Dependency, byEcosystem map[string]Harvester) bool {
	if dependencyHasLicense(dep) {
		return false
	}
	h, ok := byEcosystem[dep.Type]
	if !ok {
		return false
	}
	lic := h.License(dep.Name, dep.Version)
	if lic == "" {
		return false
	}
	if dep.Metadata == nil {
		dep.Metadata = make(map[string]interface{})
	}
	dep.Metadata[dependencyLicenseKey] = lic
	return true
}

// dependencyHasLicense reports whether a dependency already carries a license in
// its metadata.
func dependencyHasLicense(dep *types.Dependency) bool {
	if dep.Metadata == nil {
		return false
	}
	v, ok := dep.Metadata[dependencyLicenseKey]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && s != ""
}

// DependencyLicense returns the harvested license recorded on a dependency, or
// "" when none. Used by the SBOM builder to surface per-component licenses.
func DependencyLicense(dep types.Dependency) string {
	if dep.Metadata == nil {
		return ""
	}
	if s, ok := dep.Metadata[dependencyLicenseKey].(string); ok {
		return s
	}
	return ""
}
