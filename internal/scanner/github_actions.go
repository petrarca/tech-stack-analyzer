package scanner

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"

	"gopkg.in/yaml.v3"
)

// GitHubActionsWorkflow represents a GitHub Actions workflow file structure
type GitHubActionsWorkflow struct {
	Name string                      `yaml:"name"`
	On   map[string]interface{}      `yaml:"on"`
	Jobs map[string]GitHubActionsJob `yaml:"jobs"`
}

type GitHubActionsJob struct {
	RunsOn         string                          `yaml:"runs-on"`
	TimeoutMinutes int                             `yaml:"timeout-minutes"`
	Container      interface{}                     `yaml:"container"` // Can be string or object
	Services       map[string]GitHubActionsService `yaml:"services"`
	Steps          []GitHubActionsStep             `yaml:"steps"`
}

type GitHubActionsService struct {
	Image string `yaml:"image"`
}

type GitHubActionsStep struct {
	Name string                 `yaml:"name"`
	Uses string                 `yaml:"uses"`
	Run  string                 `yaml:"run"`
	With map[string]interface{} `yaml:"with"`
}

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

	workflow, err := d.readWorkflow(fullPath)
	if err != nil || len(workflow.Jobs) == 0 {
		return nil
	}

	relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	dependencies, actionNames := d.extractDependenciesFromWorkflow(workflow)
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

func (d *ComponentDetector) readWorkflow(fullPath string) (*GitHubActionsWorkflow, error) {
	content, err := d.provider.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	var workflow GitHubActionsWorkflow
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (d *ComponentDetector) extractDependenciesFromWorkflow(workflow *GitHubActionsWorkflow) ([]types.Dependency, []string) {
	var dependencies []types.Dependency
	var actionNames []string

	for _, job := range workflow.Jobs {
		deps, names := d.extractFromJob(job)
		dependencies = append(dependencies, deps...)
		actionNames = append(actionNames, names...)
	}

	return dependencies, actionNames
}

func (d *ComponentDetector) extractFromJob(job GitHubActionsJob) ([]types.Dependency, []string) {
	var dependencies []types.Dependency
	var actionNames []string

	// Extract from steps
	stepDeps, stepNames := d.extractFromSteps(job.Steps)
	dependencies = append(dependencies, stepDeps...)
	actionNames = append(actionNames, stepNames...)

	// Extract from container
	if containerDep := d.extractFromContainer(job.Container); containerDep != nil {
		dependencies = append(dependencies, *containerDep)
	}

	// Extract from services
	serviceDeps := d.extractFromServices(job.Services)
	dependencies = append(dependencies, serviceDeps...)

	return dependencies, actionNames
}

func (d *ComponentDetector) extractFromSteps(steps []GitHubActionsStep) ([]types.Dependency, []string) {
	var dependencies []types.Dependency
	var actionNames []string

	for _, step := range steps {
		if step.Uses == "" {
			continue
		}

		name, version := d.parseActionReference(step.Uses)
		dependencies = append(dependencies, types.Dependency{
			Type:    "githubAction",
			Name:    name,
			Version: version,
		})
		actionNames = append(actionNames, name)
	}

	return dependencies, actionNames
}

func (d *ComponentDetector) parseActionReference(uses string) (string, string) {
	parts := strings.Split(uses, "@")
	name := parts[0]
	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}

func (d *ComponentDetector) extractFromContainer(container interface{}) *types.Dependency {
	if container == nil {
		return nil
	}

	imageName := d.extractImageName(container)
	if imageName == "" {
		return nil
	}

	name, version := d.parseImageReference(imageName)
	return &types.Dependency{
		Type:    "docker",
		Name:    name,
		Version: version,
	}
}

func (d *ComponentDetector) extractImageName(container interface{}) string {
	switch v := container.(type) {
	case string:
		return v
	case map[string]interface{}:
		if img, ok := v["image"].(string); ok {
			return img
		}
	}
	return ""
}

func (d *ComponentDetector) parseImageReference(image string) (string, string) {
	parts := strings.Split(image, ":")
	name := parts[0]
	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}

func (d *ComponentDetector) extractFromServices(services map[string]GitHubActionsService) []types.Dependency {
	var dependencies []types.Dependency

	for _, service := range services {
		if service.Image == "" {
			continue
		}

		name, version := d.parseImageReference(service.Image)
		dependencies = append(dependencies, types.Dependency{
			Type:    "docker",
			Name:    name,
			Version: version,
		})
	}

	return dependencies
}

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
