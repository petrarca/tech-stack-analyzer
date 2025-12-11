package nodejs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Node.js component detection
type Detector struct{}

// Name returns the detector name
func (d *Detector) Name() string {
	return "nodejs"
}

// Detect scans for Node.js projects (package.json)
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "package.json" {
			continue
		}

		payload := d.processPackageJSON(file, currentPath, basePath, provider, depDetector)
		if payload != nil {
			payloads = append(payloads, payload)
		}
	}

	return payloads
}

// processPackageJSON processes a single package.json file and returns a payload
func (d *Detector) processPackageJSON(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	// Read package.json
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse package.json
	var packageJSON struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		License         string            `json:"license"`
	}

	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return nil
	}

	// Must have a name
	if packageJSON.Name == "" {
		return nil
	}

	// Create payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(packageJSON.Name, relativeFilePath)
	payload.AddPrimaryTech("nodejs")

	// Process dependencies
	d.processDependencies(&packageJSON, payload, depDetector)

	// Process license
	d.processLicense(&packageJSON, payload)

	return payload
}

// processDependencies handles dependency processing for package.json
func (d *Detector) processDependencies(packageJSON *struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	License         string            `json:"license"`
}, payload *types.Payload, depDetector components.DependencyDetector) {
	// Merge dependencies
	allDeps := make(map[string]string)
	for name, version := range packageJSON.Dependencies {
		allDeps[name] = version
	}
	for name, version := range packageJSON.DevDependencies {
		allDeps[name] = version
	}

	// Match dependencies against rules
	var depNames []string
	for name := range allDeps {
		depNames = append(depNames, name)
	}

	matchedTechs := depDetector.MatchDependencies(depNames, "npm")
	for tech, reasons := range matchedTechs {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
	}

	// Convert to dependency array
	for name, version := range allDeps {
		payload.Dependencies = append(payload.Dependencies, types.Dependency{
			Type:    "npm",
			Name:    name,
			Example: version,
		})
	}
}

// processLicense handles license processing for package.json
func (d *Detector) processLicense(packageJSON *struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	License         string            `json:"license"`
}, payload *types.Payload) {
	if packageJSON.License == "" {
		return
	}

	// Use the shared license normalizer
	normalizer := licensenormalizer.NewNormalizer()

	// Try to parse as license expression first (e.g., "MIT OR Apache-2.0")
	licenses := normalizer.ParseLicenseExpression(packageJSON.License)

	if len(licenses) > 0 {
		// Add traceability reason for license expression parsing
		if len(licenses) == 1 {
			// Single license
			license := types.License{
				LicenseName: licenses[0],
				SourceFile:  "package.json",
				Confidence:  1.0,
			}

			if licenses[0] == packageJSON.License {
				license.DetectionType = "direct"
				reason := fmt.Sprintf("license detected: %s (from package.json)", licenses[0])
				payload.AddReason(reason)
			} else {
				license.DetectionType = "normalized"
				license.OriginalLicense = packageJSON.License
				reason := fmt.Sprintf("license normalized: %q -> %s (from package.json, SPDX format)", packageJSON.License, licenses[0])
				payload.AddReason(reason)
			}

			d.addLicenseToPayload(payload, license)
		} else {
			// License expression was parsed into multiple licenses
			reason := fmt.Sprintf("license expression parsed: %q -> [%s] (from package.json, SPDX format)", packageJSON.License, strings.Join(licenses, ", "))

			for _, licenseName := range licenses {
				license := types.License{
					LicenseName:     licenseName,
					DetectionType:   "expression_parsed",
					SourceFile:      "package.json",
					Confidence:      1.0,
					OriginalLicense: packageJSON.License,
				}
				d.addLicenseToPayload(payload, license)
				payload.AddReason(reason)
			}
		}
	} else {
		// License was invalid or empty after processing
		payload.AddReason(fmt.Sprintf("license ignored: %q (invalid expression from package.json)", packageJSON.License))
	}
}

// addLicenseToPayload adds a license to payload avoiding duplicates
func (d *Detector) addLicenseToPayload(payload *types.Payload, license types.License) {
	// Avoid duplicates
	exists := false
	for _, existing := range payload.Licenses {
		if existing.LicenseName == license.LicenseName {
			exists = true
			break
		}
	}
	if !exists {
		payload.Licenses = append(payload.Licenses, license)
	}
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
