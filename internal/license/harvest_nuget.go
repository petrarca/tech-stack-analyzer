package license

import (
	"encoding/xml"
)

// nugetHarvester resolves NuGet package licenses from a global-packages-folder
// layout: <root>/<id>/<version>/<id>.nuspec (all lower-cased).
type nugetHarvester struct {
	normalizer *Normalizer
}

// NewNugetHarvester creates a NuGet license harvester searching the given roots
// (typically the global packages folder, e.g. ~/.nuget/packages).
func NewNugetHarvester(roots HarvestRoots) Harvester {
	return &harvesterWithRoots{
		impl:  &nugetHarvester{normalizer: defaultNormalizer},
		roots: roots,
	}
}

func (h *nugetHarvester) ecosystem() string { return "nuget" }

// licenseAt reads the package's .nuspec from the global-packages layout and
// extracts a normalized SPDX license expression. NuGet stores packages under
// lower-cased id/version directories with a lower-cased "<id>.nuspec".
func (h *nugetHarvester) licenseAt(root, name, version string) string {
	if version == "" {
		return ""
	}
	dir := joinLower(root, name, version)
	nuspec := firstExisting(joinLower(dir, name+".nuspec"))
	if nuspec == "" {
		return ""
	}
	content := readFileLimited(nuspec)
	if content == "" {
		return ""
	}
	if raw := nuspecLicense(content); raw != "" {
		return h.normalizer.Normalize(raw)
	}
	return ""
}

// nuspecMetadata models the license-bearing fields of a .nuspec file.
type nuspecMetadata struct {
	Metadata struct {
		License struct {
			Type string `xml:"type,attr"`
			Text string `xml:",chardata"`
		} `xml:"license"`
		LicenseURL string `xml:"licenseUrl"`
	} `xml:"metadata"`
}

// nuspecLicense extracts the declared license from a .nuspec. The modern form is
// <license type="expression">MIT</license>; only expression-type licenses are a
// usable SPDX id. The deprecated <licenseUrl> is ignored here (URL-to-SPDX
// mapping is handled elsewhere for .csproj and is not reliable from a bare URL).
func nuspecLicense(content string) string {
	var n nuspecMetadata
	if err := xml.Unmarshal([]byte(content), &n); err != nil {
		return ""
	}
	if n.Metadata.License.Type == "expression" && n.Metadata.License.Text != "" {
		return n.Metadata.License.Text
	}
	return ""
}
