package php

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
	payload.SetComponentType("php")

	// Set tech field to php
	payload.AddPrimaryTech("php")

	// Store package name in properties for inter-component dependency tracking
	payload.SetComponentProperty("php", "package_name", projectName)

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add phpcomposer tech
	payload.AddTech("phpcomposer", "matched file: composer.json")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, parsers.DependencyTypePHP))
		payload.Dependencies = dependencies
	}

	// Attach the dependency graph (no-op unless the mode is on and
	// composer.lock is present).
	components.AttachLockfileGraph(payload, currentPath, provider, lockfileGraphProducers)

	// Add license if present with traceability reasons
	if license != "" {
		d.processLicense(license, payload)
	}

	return payload
}

// lockfileGraphProducers lists this ecosystem's lockfile. PHP/Composer has a
// single lockfile; direct deps come from composer.json (require/require-dev).
var lockfileGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: "composer.lock", Manifest: "composer.json", Parse: parsers.ParseComposerLockGraph},
}

// processLicense handles license processing for composer.json, supporting SPDX expressions
func (d *Detector) processLicense(rawLicense string, payload *types.Payload) {
	licensenormalizer.ProcessLicenseExpression(rawLicense, "composer.json", payload)
}

func init() {
	components.Register(&Detector{})

	// Register composer package provider
	providers.Register(&providers.PackageProvider{
		DependencyType:      parsers.DependencyTypePHP,
		ExtractPackageNames: providers.SinglePropertyExtractor("php", "package_name"),
	})
}
