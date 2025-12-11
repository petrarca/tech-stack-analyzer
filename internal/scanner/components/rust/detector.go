package rust

import (
	"fmt"
	"path/filepath"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "rust"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for Cargo.toml
	for _, file := range files {
		if file.Name == "Cargo.toml" {
			payload := d.detectCargoToml(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectCargoToml(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse Cargo.toml using parser
	rustParser := parsers.NewRustParser()
	projectName, license, dependencies, isWorkspace := rustParser.ParseCargoToml(string(content))

	// Create payload (named if not workspace and has package section, virtual otherwise)
	var payload *types.Payload

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	if !isWorkspace && projectName != "" {
		// Named component for projects with [package] section (not workspace)
		payload = types.NewPayloadWithPath(projectName, relativeFilePath)

		// Set tech field to rust
		payload.AddPrimaryTech("rust")
	} else {
		// Virtual payload for workspace files or files without [package] section
		payload = types.NewPayloadWithPath("virtual", relativeFilePath)
	}

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add cargo tech
	payload.AddTech("cargo", "matched file: Cargo.toml")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "rust")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}

		payload.Dependencies = dependencies
	}

	// Add license if present with traceability reasons
	if license != "" {
		// Try to detect license format with SPDX normalization
		detectedLicense := d.detectLicense(license)
		if detectedLicense != "" {
			// Create structured License object
			licenseObj := types.License{
				LicenseName: detectedLicense,
				SourceFile:  "Cargo.toml",
				Confidence:  1.0,
			}

			// Add traceability reason for license detection
			if detectedLicense == license {
				// License was already in correct format
				licenseObj.DetectionType = "direct"
				reason := fmt.Sprintf("license detected: %s (from Cargo.toml)", detectedLicense)
				payload.AddReason(reason)
			} else {
				// License was normalized to SPDX format
				licenseObj.DetectionType = "normalized"
				licenseObj.OriginalLicense = license
				reason := fmt.Sprintf("license normalized: %q -> %s (from Cargo.toml, SPDX format)", license, detectedLicense)
				payload.AddReason(reason)
			}

			payload.Licenses = append(payload.Licenses, licenseObj)
		} else {
			// License was invalid or empty after processing
			payload.AddReason(fmt.Sprintf("license ignored: %q (invalid format from Cargo.toml)", license))
		}
	}

	return payload
}

// detectLicense normalizes license strings using the shared SPDX-compliant normalizer
func (d *Detector) detectLicense(license string) string {
	if license == "" {
		return ""
	}

	// Use the shared license normalizer
	normalizer := licensenormalizer.NewNormalizer()
	return normalizer.Normalize(license)
}

func init() {
	components.Register(&Detector{})
}
