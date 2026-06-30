package license

import (
	"encoding/json"
	"path/filepath"
)

// npmHarvester resolves npm package licenses from node_modules-style roots.
type npmHarvester struct {
	normalizer *Normalizer
}

// NewNpmHarvester creates an npm license harvester searching the given roots.
// Each root is treated as a directory that contains package directories (e.g. a
// node_modules folder, or a package cache root).
func NewNpmHarvester(roots HarvestRoots) Harvester {
	return &harvesterWithRoots{
		impl:  &npmHarvester{normalizer: defaultNormalizer},
		roots: roots,
	}
}

func (h *npmHarvester) ecosystem() string { return "npm" }

// licenseAt reads <root>/<name>/package.json and extracts a normalized license.
// name may be a scoped package ("@scope/pkg"); filepath.Join handles the slash.
func (h *npmHarvester) licenseAt(root, name, _ string) string {
	manifest := readFileLimited(filepath.Join(root, name, "package.json"))
	if manifest == "" {
		return ""
	}
	raw := npmLicenseField(manifest)
	if raw == "" {
		return ""
	}
	return h.normalizer.Normalize(raw)
}

// npmPackageManifest models the license-bearing fields of package.json. The
// "license" field is normally a string; the deprecated "licenses" field is an
// array of objects with a "type".
type npmPackageManifest struct {
	License  json.RawMessage `json:"license"`
	Licenses []struct {
		Type string `json:"type"`
	} `json:"licenses"`
}

// npmLicenseField extracts the raw license string from a package.json manifest,
// supporting both the modern string form and the legacy array form.
func npmLicenseField(manifest string) string {
	var m npmPackageManifest
	if err := json.Unmarshal([]byte(manifest), &m); err != nil {
		return ""
	}
	// Modern form: a plain string.
	var s string
	if err := json.Unmarshal(m.License, &s); err == nil && s != "" {
		return s
	}
	// Legacy form: { "type": "MIT" } object under "license".
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(m.License, &obj); err == nil && obj.Type != "" {
		return obj.Type
	}
	// Deprecated array form: take the first entry.
	if len(m.Licenses) > 0 {
		return m.Licenses[0].Type
	}
	return ""
}
