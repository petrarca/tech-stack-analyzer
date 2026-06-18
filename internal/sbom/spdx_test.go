package sbom

import (
	"strings"
	"testing"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestSPDXFromPayload_Structure(t *testing.T) {
	payload := types.NewPayload("myapp", nil)
	payload.Dependencies = []types.Dependency{
		{Type: "npm", Name: "mylib", Version: "1.2.3", Scope: types.ScopeProd},
		{Type: "maven", Name: "com.example:other", Version: "4.5.6", Scope: types.ScopeProd},
		// import-scoped BOM must be excluded (same as CycloneDX)
		{Type: "maven", Name: "com.example:platform-bom", Version: "2.0.0", Scope: types.ScopeImport},
	}

	doc := SPDXFromPayload(payload)

	if doc.SPDXVersion != "SPDX-2.3" {
		t.Errorf("spdxVersion = %q, want SPDX-2.3", doc.SPDXVersion)
	}
	if doc.DataLicense != "CC0-1.0" {
		t.Errorf("dataLicense = %q, want CC0-1.0", doc.DataLicense)
	}
	if doc.SPDXID != "SPDXRef-DOCUMENT" {
		t.Errorf("SPDXID = %q", doc.SPDXID)
	}
	if doc.Name != "myapp" {
		t.Errorf("name = %q, want myapp", doc.Name)
	}

	// Root package + 2 dependency packages (import excluded).
	if len(doc.Packages) != 3 {
		t.Fatalf("packages = %d, want 3 (root + 2 deps): %+v", len(doc.Packages), doc.Packages)
	}

	// Packages keyed by name for assertions.
	byName := map[string]SPDXPackage{}
	for _, p := range doc.Packages {
		byName[p.Name] = p
	}
	if _, ok := byName["com.example:platform-bom"]; ok {
		t.Error("import-scoped BOM must not be an SPDX package")
	}

	lib := byName["mylib"]
	if lib.VersionInfo != "1.2.3" {
		t.Errorf("mylib versionInfo = %q", lib.VersionInfo)
	}
	if len(lib.ExternalRefs) != 1 ||
		lib.ExternalRefs[0].ReferenceType != "purl" ||
		lib.ExternalRefs[0].ReferenceLocator != "pkg:npm/mylib@1.2.3" {
		t.Errorf("mylib externalRefs = %+v", lib.ExternalRefs)
	}
	if lib.DownloadLocation != "NOASSERTION" || lib.LicenseConcluded != "NOASSERTION" {
		t.Errorf("mylib NOASSERTION fields wrong: %+v", lib)
	}

	// Every SPDXID is unique.
	ids := map[string]bool{}
	for _, p := range doc.Packages {
		if ids[p.SPDXID] {
			t.Errorf("duplicate SPDXID %q", p.SPDXID)
		}
		ids[p.SPDXID] = true
	}

	// Relationships: 1 DESCRIBES (doc->root) + 1 CONTAINS per dependency package.
	var describes, contains int
	for _, r := range doc.Relationships {
		switch r.RelationshipType {
		case "DESCRIBES":
			describes++
			if r.SPDXElementID != "SPDXRef-DOCUMENT" {
				t.Errorf("DESCRIBES from %q, want SPDXRef-DOCUMENT", r.SPDXElementID)
			}
		case "CONTAINS":
			contains++
		}
	}
	if describes != 1 {
		t.Errorf("DESCRIBES count = %d, want 1", describes)
	}
	if contains != 2 {
		t.Errorf("CONTAINS count = %d, want 2", contains)
	}
}

func TestSPDXStamp_SetsNamespaceAndTimestamp(t *testing.T) {
	payload := types.NewPayload("myapp", nil)
	payload.Dependencies = []types.Dependency{{Type: "npm", Name: "mylib", Version: "1.0.0"}}
	doc := SPDXFromPayload(payload)

	// Pure builder is deterministic: no namespace/timestamp before Stamp.
	if doc.DocumentNamespace != "" || doc.CreationInfo.Created != "" {
		t.Errorf("builder must not set namespace/created before Stamp: ns=%q created=%q",
			doc.DocumentNamespace, doc.CreationInfo.Created)
	}

	SPDXStamp(doc)

	if !strings.Contains(doc.DocumentNamespace, "myapp-") {
		t.Errorf("documentNamespace = %q, want it to contain app name and uuid", doc.DocumentNamespace)
	}
	if _, err := time.Parse(time.RFC3339, doc.CreationInfo.Created); err != nil {
		t.Errorf("created not RFC3339: %q (%v)", doc.CreationInfo.Created, err)
	}

	// Unique namespace per emission.
	other := SPDXFromPayload(payload)
	SPDXStamp(other)
	if other.DocumentNamespace == doc.DocumentNamespace {
		t.Error("documentNamespace should be unique per emission")
	}

	// Nil-safe.
	SPDXStamp(nil)
}

func TestSPDXFromPayload_UserMetadataAnnotations(t *testing.T) {
	p := types.NewPayload("app", nil)
	p.Dependencies = []types.Dependency{{Type: "npm", Name: "lodash", Version: "4.17.21", Direct: true}}
	p.Metadata = map[string]interface{}{
		"properties": map[string]interface{}{"product_key": "myproduct"},
	}
	doc := SPDXFromPayload(p)
	found := false
	for _, a := range doc.Annotations {
		if a.Comment == "product_key=myproduct" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected user-metadata annotation, got %+v", doc.Annotations)
	}
}
