package license

// rootHarvester is the per-root lookup an ecosystem harvester implements. The
// shared harvesterWithRoots wrapper handles iterating roots and returning the
// first hit, so each ecosystem only implements a single-root lookup.
type rootHarvester interface {
	ecosystem() string
	// licenseAt returns the normalized license for name@version under a single
	// root, or "" when not found there.
	licenseAt(root, name, version string) string
}

// harvesterWithRoots adapts a rootHarvester to the public Harvester interface by
// searching each configured root in order and returning the first license found.
type harvesterWithRoots struct {
	impl  rootHarvester
	roots HarvestRoots
}

// Ecosystem implements Harvester.
func (h *harvesterWithRoots) Ecosystem() string { return h.impl.ecosystem() }

// License implements Harvester.
func (h *harvesterWithRoots) License(name, version string) string {
	if name == "" {
		return ""
	}
	for _, root := range h.roots.allRoots() {
		if lic := h.impl.licenseAt(root, name, version); lic != "" {
			return lic
		}
	}
	return ""
}
