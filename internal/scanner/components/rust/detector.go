package rust

import (
	"path/filepath"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
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
	projectName, license, _, isWorkspace := rustParser.ParseCargoToml(string(content))

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
		payload.SetComponentType("rust")

		// Set tech field to rust
		payload.AddPrimaryTech("rust")

		// Store crate name in properties for inter-component dependency tracking
		payload.SetComponentProperty("rust", "crate_name", projectName)
	} else {
		// Virtual payload for workspace files or files without [package] section
		payload = types.NewPayloadWithPath("virtual", relativeFilePath)
	}

	// Extract dependencies using lock file priority system
	dependencies := d.extractDependenciesWithPriority(currentPath, string(content), provider)

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
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}

		payload.Dependencies = dependencies
	}

	// Add license if present with traceability reasons
	if license != "" {
		d.processLicense(license, payload)
	}

	return payload
}

// extractDependenciesWithPriority extracts dependencies using lock file priority system
// Priority 1: Cargo.lock (resolved versions)
// Priority 2: Cargo.toml (version ranges as fallback)
func (d *Detector) extractDependenciesWithPriority(currentPath, cargoTomlContent string, provider types.Provider) []types.Dependency {
	// Check if lock files are enabled
	if !components.UseLockFiles() {
		rustParser := parsers.NewRustParser()
		_, _, dependencies, _ := rustParser.ParseCargoToml(cargoTomlContent)
		for i := range dependencies {
			dependencies[i].SourceFile = "Cargo.toml"
		}
		return dependencies
	}

	// Priority 1: Check for Cargo.lock
	if lockContent, err := provider.ReadFile(filepath.Join(currentPath, "Cargo.lock")); err == nil && len(lockContent) > 0 {
		deps := parsers.ParseCargoLock(lockContent, cargoTomlContent)
		if len(deps) > 0 {
			return deps
		}
	}

	// Priority 2: Fallback to Cargo.toml
	rustParser := parsers.NewRustParser()
	_, _, dependencies, _ := rustParser.ParseCargoToml(cargoTomlContent)

	// Add source file information
	for i := range dependencies {
		dependencies[i].SourceFile = "Cargo.toml"
	}

	return dependencies
}

// processLicense handles license processing for Cargo.toml, supporting SPDX expressions
func (d *Detector) processLicense(rawLicense string, payload *types.Payload) {
	licensenormalizer.ProcessLicenseExpression(rawLicense, "Cargo.toml", payload)
}

func init() {
	components.Register(&Detector{})

	// Register cargo package provider
	providers.Register(&providers.PackageProvider{
		DependencyType:      "cargo",
		ExtractPackageNames: providers.SinglePropertyExtractor("rust", "crate_name"),
	})
}
