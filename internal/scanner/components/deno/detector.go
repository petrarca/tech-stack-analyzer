package deno

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "deno"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for deno.lock
	for _, file := range files {
		if file.Name == "deno.lock" {
			payload := d.detectDenoLock(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectDenoLock(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse deno.lock using parser
	denoParser := parsers.NewDenoParser()
	version, dependencies := denoParser.ParseDenoLock(string(content))

	// Must have a version to be valid
	if version == "" {
		return nil
	}

	// Create virtual payload (deno.lock doesn't have project names)
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "deno")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}

		payload.Dependencies = dependencies
	}

	return payload
}

func init() {
	components.Register(&Detector{})
}
