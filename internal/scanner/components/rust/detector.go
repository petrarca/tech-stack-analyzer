package rust

import (
	"path/filepath"

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

	// Add license if present
	if license != "" {
		// Try to detect license format
		detectedLicense := d.detectLicense(license)
		if detectedLicense != "" {
			payload.Licenses = append(payload.Licenses, detectedLicense)
		}
	}

	return payload
}

// detectLicense attempts to normalize license strings
func (d *Detector) detectLicense(license string) string {
	// Handle different license formats
	switch license {
	case "MIT":
		return "MIT"
	case "Apache-2.0":
		return "Apache-2.0"
	case "GPL-3.0":
		return "GPL-3.0"
	case "BSD-3-Clause":
		return "BSD-3-Clause"
	case "ISC":
		return "ISC"
	default:
		// Return as-is for unknown licenses
		return license
	}
}

func init() {
	components.Register(&Detector{})
}
