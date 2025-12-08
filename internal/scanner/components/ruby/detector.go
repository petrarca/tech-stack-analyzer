package ruby

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "ruby"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for Gemfile (component - creates named payload)
	for _, file := range files {
		if file.Name == "Gemfile" {
			payload := d.detectGemfile(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	return results
}

func (d *Detector) detectGemfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Extract project name ( fallback to folder name)
	projectName := d.extractProjectName(string(content))
	if projectName == "" {
		projectName = filepath.Base(currentPath)
	}

	// Create named payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(projectName, relativeFilePath)

	// Set tech field to ruby
	payload.AddPrimaryTech("ruby")

	// Parse Gemfile for dependencies using parser
	rubyParser := parsers.NewRubyParser()
	dependencies := rubyParser.ParseGemfile(string(content))

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add bundler tech
	payload.AddTech("bundler", "matched file: Gemfile")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "ruby")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}

		payload.Dependencies = dependencies
	}

	return payload
}

// extractProjectName attempts to extract a project name from Gemfile
// Gemfiles typically don't have project names, so this returns empty string
// to trigger folder name fallback
func (d *Detector) extractProjectName(content string) string {
	// Gemfiles don't have a standard project name field like pyproject.toml
	// We could try to parse comments or specific patterns, but for now
	// return empty to use folder name as project name
	return ""
}

func init() {
	components.Register(&Detector{})
}
