// Package githubactions implements GitHub Actions workflow detection as a plugin-based component detector.
package githubactions

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

var workflowRegex = regexp.MustCompile(`\.github/workflows/.+\.ya?ml$`)

// Detector implements GitHub Actions component detection.
type Detector struct{}

// Name returns the detector name.
func (d *Detector) Name() string {
	return "githubactions"
}

// Detect scans for GitHub Actions workflow files (.github/workflows/*.yml)
// and extracts action dependencies, container images, and service images.
// Returns a virtual component (merged into parent) when dependencies are found.
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	for _, file := range files {
		fullPath := filepath.Join(currentPath, file.Name)
		if !workflowRegex.MatchString(fullPath) && !workflowRegex.MatchString(file.Name) {
			continue
		}

		content, err := provider.ReadFile(fullPath)
		if err != nil {
			continue
		}

		parser := parsers.NewGitHubActionsParser()
		workflow, err := parser.ParseWorkflow(string(content))
		if err != nil || len(workflow.Jobs) == 0 {
			continue
		}

		relativeFilePath := relativePath(basePath, currentPath, file.Name)
		payload := types.NewPayloadWithPath("virtual", relativeFilePath)

		dependencies, actionNames := parser.CreateDependencies(workflow)
		matchActionsToTechs(actionNames, payload, depDetector)
		for _, dep := range dependencies {
			payload.AddDependency(dep)
		}

		if len(dependencies) > 0 {
			return []*types.Payload{payload}
		}
	}

	return nil
}

// matchActionsToTechs matches GitHub Action names against rules to detect technologies.
func matchActionsToTechs(actionNames []string, payload *types.Payload, depDetector components.DependencyDetector) {
	if len(actionNames) == 0 {
		return
	}

	matchedTechs := depDetector.MatchDependencies(actionNames, "githubAction")
	for tech := range matchedTechs {
		payload.AddTech(tech, "github action matched")
	}
}

// relativePath computes the relative file path for payload display.
func relativePath(basePath, currentPath, fileName string) string {
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, fileName))
	if relativeFilePath == "." {
		return "/"
	}
	return "/" + relativeFilePath
}

func init() {
	components.Register(&Detector{})
}
