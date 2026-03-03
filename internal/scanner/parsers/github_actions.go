package parsers

import (
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"gopkg.in/yaml.v3"
)

// GitHubActionsParser handles GitHub Actions workflow file parsing
type GitHubActionsParser struct{}

// NewGitHubActionsParser creates a new GitHub Actions parser
func NewGitHubActionsParser() *GitHubActionsParser {
	return &GitHubActionsParser{}
}

// GitHubActionsWorkflow represents a GitHub Actions workflow file structure
type GitHubActionsWorkflow struct {
	Name string                      `yaml:"name"`
	On   interface{}                 `yaml:"on"`
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

// ParseWorkflow parses a GitHub Actions workflow YAML file
func (p *GitHubActionsParser) ParseWorkflow(content string) (*GitHubActionsWorkflow, error) {
	var workflow GitHubActionsWorkflow
	if err := yaml.Unmarshal([]byte(content), &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

// CreateDependencies creates dependency objects from a GitHub Actions workflow
func (p *GitHubActionsParser) CreateDependencies(workflow *GitHubActionsWorkflow) ([]types.Dependency, []string) {
	dependencies := make([]types.Dependency, 0)
	actionNames := make([]string, 0)

	for _, job := range workflow.Jobs {
		deps, names := p.extractFromJob(job)
		dependencies = append(dependencies, deps...)
		actionNames = append(actionNames, names...)
	}

	return dependencies, actionNames
}

func (p *GitHubActionsParser) extractFromJob(job GitHubActionsJob) ([]types.Dependency, []string) {
	dependencies := make([]types.Dependency, 0)
	actionNames := make([]string, 0)

	// Extract from steps
	stepDeps, stepNames := p.extractFromSteps(job.Steps)
	dependencies = append(dependencies, stepDeps...)
	actionNames = append(actionNames, stepNames...)

	// Extract from container
	if containerDep := p.extractFromContainer(job.Container); containerDep != nil {
		dependencies = append(dependencies, *containerDep)
	}

	// Extract from services
	serviceDeps := p.extractFromServices(job.Services)
	dependencies = append(dependencies, serviceDeps...)

	return dependencies, actionNames
}

func (p *GitHubActionsParser) extractFromSteps(steps []GitHubActionsStep) ([]types.Dependency, []string) {
	dependencies := make([]types.Dependency, 0)
	actionNames := make([]string, 0)

	for _, step := range steps {
		if step.Uses == "" {
			continue
		}

		name, version := p.parseActionReference(step.Uses)
		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeGitHubAction,
			Name:     name,
			Version:  version,
			Scope:    types.ScopeBuild,
			Direct:   true,
			Metadata: types.NewMetadata(MetadataSourceGitHubWorkflow),
		})
		actionNames = append(actionNames, name)
	}

	return dependencies, actionNames
}

func (p *GitHubActionsParser) parseActionReference(uses string) (string, string) {
	parts := strings.Split(uses, "@")
	name := parts[0]
	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}

func (p *GitHubActionsParser) extractFromContainer(container interface{}) *types.Dependency {
	if container == nil {
		return nil
	}

	imageName := p.extractImageName(container)
	if imageName == "" {
		return nil
	}

	name, version := p.parseImageReference(imageName)
	return &types.Dependency{
		Type:     DependencyTypeDocker,
		Name:     name,
		Version:  version,
		Scope:    types.ScopeBuild,
		Direct:   true,
		Metadata: types.NewMetadata(MetadataSourceGitHubWorkflow),
	}
}

func (p *GitHubActionsParser) extractImageName(container interface{}) string {
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

func (p *GitHubActionsParser) parseImageReference(image string) (string, string) {
	parts := strings.Split(image, ":")
	name := parts[0]
	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}

func (p *GitHubActionsParser) extractFromServices(services map[string]GitHubActionsService) []types.Dependency {
	var dependencies []types.Dependency

	for _, service := range services {
		if service.Image == "" {
			continue
		}

		name, version := p.parseImageReference(service.Image)
		dependencies = append(dependencies, types.Dependency{
			Type:     DependencyTypeDocker,
			Name:     name,
			Version:  version,
			Scope:    types.ScopeBuild,
			Direct:   true,
			Metadata: types.NewMetadata(MetadataSourceGitHubWorkflow),
		})
	}

	return dependencies
}
