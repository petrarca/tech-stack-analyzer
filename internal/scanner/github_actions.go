package scanner

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// DetectGitHubActionsComponent detects GitHub Actions workflows and extracts action dependencies
// Matches TypeScript: detectGithubActionsComponent in component.ts
func (d *ComponentDetector) DetectGitHubActionsComponent(files []types.File, currentPath string, basePath string) *types.Payload {
	workflowRegex := regexp.MustCompile(`\.github/workflows/.+\.ya?ml$`)

	for _, file := range files {
		if payload := d.processWorkflowFile(file, currentPath, basePath, workflowRegex); payload != nil {
			return payload
		}
	}
	return nil
}

func (d *ComponentDetector) processWorkflowFile(file types.File, currentPath, basePath string, workflowRegex *regexp.Regexp) *types.Payload {
	fullPath := filepath.Join(currentPath, file.Name)
	if !d.isWorkflowFile(fullPath, file.Name, workflowRegex) {
		return nil
	}

	// Read workflow file content
	content, err := d.provider.ReadFile(fullPath)
	if err != nil {
		return nil
	}

	// Parse workflow using parser
	parser := parsers.NewGitHubActionsParser()
	workflow, err := parser.ParseWorkflow(string(content))
	if err != nil || len(workflow.Jobs) == 0 {
		return nil
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	// Create dependencies using parser
	dependencies, actionNames := parser.CreateDependencies(workflow)
	d.matchActionsToTechs(actionNames, payload)
	d.addDependenciesToPayload(dependencies, payload)

	if len(dependencies) > 0 {
		return payload
	}
	return nil
}

func (d *ComponentDetector) isWorkflowFile(fullPath, fileName string, regex *regexp.Regexp) bool {
	return regex.MatchString(fullPath) || regex.MatchString(fileName)
}

// Note: Workflow parsing and dependency extraction methods moved to parsers/github_actions.go

func (d *ComponentDetector) matchActionsToTechs(actionNames []string, payload *types.Payload) {
	if len(actionNames) == 0 {
		return
	}

	matchedTechs := d.depDetector.MatchDependencies(actionNames, "githubAction")
	for tech := range matchedTechs {
		payload.AddTech(tech, "github action matched")
	}
}

func (d *ComponentDetector) addDependenciesToPayload(dependencies []types.Dependency, payload *types.Payload) {
	for _, dep := range dependencies {
		payload.AddDependency(dep)
	}
}
