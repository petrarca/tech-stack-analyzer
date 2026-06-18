package sbom

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// SPDX document constants.
const (
	spdxVersion      = "SPDX-2.3"
	spdxDataLicense  = "CC0-1.0"
	spdxDocumentID   = "SPDXRef-DOCUMENT"
	spdxNoAssertion  = "NOASSERTION"
	spdxToolCreator  = "Tool: tech-stack-analyzer"
	spdxNamespaceURI = "https://github.com/petrarca/tech-stack-analyzer/spdxdocs/"
)

// SPDXDocument is the top-level SPDX 2.3 JSON document. It carries the same
// package inventory as the CycloneDX BOM, re-expressed in SPDX terms (packages
// with PURL externalRefs and DESCRIBES/CONTAINS relationships) for consumers
// that require SPDX (license/compliance pipelines, GitHub dependency graph).
type SPDXDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	Packages          []SPDXPackage      `json:"packages"`
	Relationships     []SPDXRelationship `json:"relationships"`
	Annotations       []SPDXAnnotation   `json:"annotations,omitempty"`
}

// SPDXAnnotation carries arbitrary document-level metadata (used here to surface
// the user-provided scan properties). AnnotationDate is filled by SPDXStamp.
type SPDXAnnotation struct {
	Annotator      string `json:"annotator"`
	AnnotationDate string `json:"annotationDate,omitempty"`
	AnnotationType string `json:"annotationType"`
	Comment        string `json:"comment"`
}

// SPDXCreationInfo records who/what/when produced the document.
type SPDXCreationInfo struct {
	Creators []string `json:"creators"`
	Created  string   `json:"created"`
}

// SPDXPackage is a single SPDX package entry.
type SPDXPackage struct {
	Name             string            `json:"name"`
	SPDXID           string            `json:"SPDXID"`
	VersionInfo      string            `json:"versionInfo,omitempty"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs,omitempty"`
}

// SPDXExternalRef carries the Package URL that ties an SPDX package to advisory
// databases (the same PURL used by the CycloneDX component).
type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// SPDXRelationship links SPDX elements (document DESCRIBES root; root CONTAINS
// each package).
type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
	RelationshipType   string `json:"relationshipType"`
}

// SPDXFromPayload builds an SPDX 2.3 document from a scan payload. It reuses the
// CycloneDX component derivation (filtering, dedup, transitive fold-in, PURLs)
// so the two SBOM formats describe an identical package set, then maps each
// component to an SPDX package.
func SPDXFromPayload(payload *types.Payload) *SPDXDocument {
	bom := FromPayload(payload)
	return spdxFromBOM(bom, rootName(payload))
}

// SPDXFromPayloadDirect builds an SPDX document from only the payload's direct
// dependencies (those the project declares), excluding transitive graph nodes.
func SPDXFromPayloadDirect(payload *types.Payload) *SPDXDocument {
	bom := FromPayloadDirect(payload)
	return spdxFromBOM(bom, rootName(payload))
}

// spdxFromBOM converts a built CycloneDX BOM into an SPDX document. The
// non-deterministic document namespace is left blank here; SPDXStamp fills it
// (and the timestamp) at output time so the pure builder stays reproducible.
func spdxFromBOM(bom *BOM, name string) *SPDXDocument {
	if name == "" {
		name = "scan"
	}
	rootID := "SPDXRef-Root-" + shortHash(name)

	doc := &SPDXDocument{
		SPDXVersion: spdxVersion,
		DataLicense: spdxDataLicense,
		SPDXID:      spdxDocumentID,
		Name:        name,
		CreationInfo: SPDXCreationInfo{
			Creators: []string{spdxToolCreator},
		},
	}

	// Surface the user-provided metadata (the same name/value pairs as
	// CycloneDX metadata.properties) as document annotations -- SPDX's mechanism
	// for arbitrary key/value metadata. One annotation per property.
	if bom.Metadata != nil {
		for _, p := range bom.Metadata.Properties {
			doc.Annotations = append(doc.Annotations, SPDXAnnotation{
				Annotator:      spdxToolCreator,
				AnnotationType: "OTHER",
				Comment:        p.Name + "=" + p.Value,
			})
		}
	}

	// Root package representing the scanned application.
	doc.Packages = append(doc.Packages, SPDXPackage{
		Name:             name,
		SPDXID:           rootID,
		DownloadLocation: spdxNoAssertion,
		FilesAnalyzed:    false,
		LicenseConcluded: spdxNoAssertion,
		LicenseDeclared:  spdxNoAssertion,
	})
	// Document describes the root package.
	doc.Relationships = append(doc.Relationships, SPDXRelationship{
		SPDXElementID:      spdxDocumentID,
		RelatedSPDXElement: rootID,
		RelationshipType:   "DESCRIBES",
	})

	// One SPDX package per CycloneDX component. SPDXIDs must be unique; derive
	// a stable id from the PURL (or name when no PURL).
	seen := make(map[string]bool)
	for _, c := range bom.Components {
		key := c.PURL
		if key == "" {
			key = c.Name + "@" + c.Version
		}
		id := "SPDXRef-Package-" + shortHash(key)
		if seen[id] {
			continue
		}
		seen[id] = true

		pkg := SPDXPackage{
			Name:             c.Name,
			SPDXID:           id,
			VersionInfo:      c.Version,
			DownloadLocation: spdxNoAssertion,
			FilesAnalyzed:    false,
			LicenseConcluded: spdxNoAssertion,
			LicenseDeclared:  spdxNoAssertion,
		}
		if c.PURL != "" {
			pkg.ExternalRefs = []SPDXExternalRef{{
				ReferenceCategory: "PACKAGE-MANAGER",
				ReferenceType:     "purl",
				ReferenceLocator:  c.PURL,
			}}
		}
		doc.Packages = append(doc.Packages, pkg)

		doc.Relationships = append(doc.Relationships, SPDXRelationship{
			SPDXElementID:      rootID,
			RelatedSPDXElement: id,
			RelationshipType:   "CONTAINS",
		})
	}

	return doc
}

// SPDXStamp sets the document's per-emission identity: a unique
// documentNamespace and the creation timestamp (RFC 3339, UTC). Like
// CycloneDX Stamp, these are applied at output time to keep the builder
// reproducible. A no-op for a nil document.
func SPDXStamp(doc *SPDXDocument) {
	if doc == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	doc.CreationInfo.Created = now
	for i := range doc.Annotations {
		doc.Annotations[i].AnnotationDate = now
	}
	if id := newUUIDv4(); id != "" {
		doc.DocumentNamespace = spdxNamespaceURI + doc.Name + "-" + id
	}
}

// shortHash returns a short, stable hex digest of s for use in SPDX element
// identifiers (which must be unique and match [a-zA-Z0-9.-]). The component
// list it is derived from is already sorted by PURL, so package/relationship
// order is deterministic without an extra sort.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
