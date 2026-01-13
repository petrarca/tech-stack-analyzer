package delphi

import (
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "delphi"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for .dproj files
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Name), ".dproj") {
			payload := d.detectDelphiProject(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectDelphiProject(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse .dproj file
	delphiParser := parsers.NewDelphiParser()
	project := delphiParser.ParseDproj(string(content), file.Name)

	if project.Name == "" {
		return nil
	}

	// Create component payload
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(project.Name, relativeFilePath)

	// Set tech to delphi
	payload.AddPrimaryTech("delphi")
	payload.AddTech("delphi", "matched file: "+file.Name)

	// Add framework info (VCL or FMX)
	if project.Framework != "" {
		frameworkLower := strings.ToLower(project.Framework)
		payload.AddTech(frameworkLower, "framework: "+project.Framework)
	}

	// Create dependencies using parser
	dependencies := delphiParser.CreateDependencies(project)

	// Match dependencies against rules
	if len(dependencies) > 0 {
		payload.Dependencies = dependencies
		matchedTechs := depDetector.MatchDependencies(project.Packages, "delphi")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}
	}

	return payload
}

func init() {
	components.Register(&Detector{})
}
