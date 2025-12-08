package php

import (
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "php"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for composer.json
	for _, file := range files {
		if file.Name == "composer.json" {
			payload := d.detectComposerJSON(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectComposerJSON(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse composer.json using parser
	phpParser := parsers.NewPHPParser()
	projectName, license, dependencies := phpParser.ParseComposerJSON(string(content))

	// Must have a name
	if projectName == "" {
		return nil
	}

	// Create named payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(projectName, relativeFilePath)

	// Set tech field to php
	payload.AddPrimaryTech("php")

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add phpcomposer tech
	payload.AddTech("phpcomposer", "matched file: composer.json")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "php")
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
	switch strings.ToLower(license) {
	case "mit":
		return "MIT"
	case "apache-2.0", "apache", "apache 2.0":
		return "Apache-2.0"
	case "gpl", "gpl-3.0", "gplv3":
		return "GPL-3.0"
	case "bsd":
		return "BSD"
	case "isc":
		return "ISC"
	case "lgpl", "lgpl-3.0", "lgplv3":
		return "LGPL-3.0"
	default:
		// Return as-is for unknown licenses
		return license
	}
}

func init() {
	components.Register(&Detector{})
}
