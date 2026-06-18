package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// writeSampleScanJSON writes a minimal native scan output with one direct and
// one transitive npm dependency (plus a graph edge), as the scan command would.
func writeSampleScanJSON(t *testing.T) string {
	t.Helper()
	p := types.NewPayload("sample-app", nil)
	p.SetComponentType("nodejs")
	p.Dependencies = []types.Dependency{
		{Type: "npm", Name: "express", Version: "4.18.2", Scope: "prod", Direct: true},
		{Type: "npm", Name: "accepts", Version: "1.3.8", Scope: "prod", Direct: false},
	}
	p.DependencyEdges = []types.DependencyEdge{
		{From: ".", To: "express@4.18.2"},
		{From: "express@4.18.2", To: "body-parser@1.20.1"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	path := filepath.Join(t.TempDir(), "scan.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write scan json: %v", err)
	}
	return path
}

func sbomPURLs(t *testing.T, out []byte, isSPDX bool) map[string]bool {
	t.Helper()
	m := map[string]bool{}
	if isSPDX {
		var doc struct {
			Packages []struct {
				ExternalRefs []struct {
					ReferenceLocator string `json:"referenceLocator"`
				} `json:"externalRefs"`
			} `json:"packages"`
		}
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatalf("parse spdx: %v", err)
		}
		for _, p := range doc.Packages {
			for _, r := range p.ExternalRefs {
				m[r.ReferenceLocator] = true
			}
		}
		return m
	}
	var bom struct {
		Components []struct {
			PURL string `json:"purl"`
		} `json:"components"`
	}
	if err := json.Unmarshal(out, &bom); err != nil {
		t.Fatalf("parse cyclonedx: %v", err)
	}
	for _, c := range bom.Components {
		m[c.PURL] = true
	}
	return m
}

func TestSBOMCommand_CycloneDXFullVsDirect(t *testing.T) {
	path := writeSampleScanJSON(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p types.Payload
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}

	full, err := marshalSBOM(&p, "cyclonedx", false, false)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := marshalSBOM(&p, "cyclonedx", true, false)
	if err != nil {
		t.Fatal(err)
	}

	fp := sbomPURLs(t, full, false)
	dp := sbomPURLs(t, direct, false)

	if !fp["pkg:npm/express@4.18.2"] || !fp["pkg:npm/accepts@1.3.8"] || !fp["pkg:npm/body-parser@1.20.1"] {
		t.Errorf("full SBOM missing expected components: %v", fp)
	}
	if !dp["pkg:npm/express@4.18.2"] {
		t.Errorf("direct SBOM must include express: %v", dp)
	}
	if dp["pkg:npm/accepts@1.3.8"] || dp["pkg:npm/body-parser@1.20.1"] {
		t.Errorf("direct SBOM must exclude transitive deps: %v", dp)
	}
}

func TestSBOMCommand_SPDXFormat(t *testing.T) {
	path := writeSampleScanJSON(t)
	data, _ := os.ReadFile(path)
	var p types.Payload
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}

	out, err := marshalSBOM(&p, "spdx", false, false)
	if err != nil {
		t.Fatal(err)
	}
	purls := sbomPURLs(t, out, true)
	if !purls["pkg:npm/express@4.18.2"] || !purls["pkg:npm/accepts@1.3.8"] {
		t.Errorf("SPDX SBOM missing expected PURLs: %v", purls)
	}
}
