package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitHubActionsParser(t *testing.T) {
	parser := NewGitHubActionsParser()
	assert.NotNil(t, parser, "Should create a new GitHubActionsParser")
	assert.IsType(t, &GitHubActionsParser{}, parser, "Should return correct type")
}

func TestParseWorkflow(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name        string
		content     string
		expectError bool
		expected    *GitHubActionsWorkflow
	}{
		{
			name: "basic workflow",
			content: `name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v3
`,
			expectError: false,
			expected: &GitHubActionsWorkflow{
				Name: "CI",
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
							{Uses: "actions/setup-node@v3"},
						},
					},
				},
			},
		},
		{
			name: "workflow with container and services",
			content: `name: CI with Services
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    container: node:18-alpine
    services:
      postgres:
        image: postgres:13
      redis:
        image: redis:6-alpine
    steps:
      - uses: actions/checkout@v4
`,
			expectError: false,
			expected: &GitHubActionsWorkflow{
				Name: "CI with Services",
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn:    "ubuntu-latest",
						Container: "node:18-alpine",
						Services: map[string]GitHubActionsService{
							"postgres": {Image: "postgres:13"},
							"redis":    {Image: "redis:6-alpine"},
						},
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
						},
					},
				},
			},
		},
		{
			name: "workflow with container object",
			content: `name: CI with Container Object
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    container:
      image: node:18-alpine
      options: --user root
    steps:
      - uses: actions/checkout@v4
`,
			expectError: false,
			expected: &GitHubActionsWorkflow{
				Name: "CI with Container Object",
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Container: map[string]interface{}{
							"image":   "node:18-alpine",
							"options": "--user root",
						},
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
						},
					},
				},
			},
		},
		{
			name:        "invalid YAML",
			content:     `name: CI\ninvalid: [unclosed`,
			expectError: true,
			expected:    nil,
		},
		{
			name: "empty workflow",
			content: `name: Empty
on: push
jobs: {}
`,
			expectError: false,
			expected: &GitHubActionsWorkflow{
				Name: "Empty",
				Jobs: map[string]GitHubActionsJob{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseWorkflow(tt.content)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid content")
				assert.Nil(t, result, "Should return nil result on error")
			} else {
				assert.NoError(t, err, "Should not return error for valid content")
				require.NotNil(t, result, "Should return non-nil result")
				assert.Equal(t, tt.expected.Name, result.Name, "Workflow name should match")

				// Compare jobs
				assert.Equal(t, len(tt.expected.Jobs), len(result.Jobs), "Number of jobs should match")
				for jobName, expectedJob := range tt.expected.Jobs {
					actualJob, exists := result.Jobs[jobName]
					assert.True(t, exists, "Job %s should exist", jobName)
					assert.Equal(t, expectedJob.RunsOn, actualJob.RunsOn, "Runs-on should match")
					assert.Equal(t, expectedJob.Container, actualJob.Container, "Container should match")

					// Compare services
					assert.Equal(t, len(expectedJob.Services), len(actualJob.Services), "Number of services should match")
					for serviceName, expectedService := range expectedJob.Services {
						actualService, exists := actualJob.Services[serviceName]
						assert.True(t, exists, "Service %s should exist", serviceName)
						assert.Equal(t, expectedService.Image, actualService.Image, "Service image should match")
					}

					// Compare steps
					assert.Equal(t, len(expectedJob.Steps), len(actualJob.Steps), "Number of steps should match")
					for i, expectedStep := range expectedJob.Steps {
						if i < len(actualJob.Steps) {
							assert.Equal(t, expectedStep.Uses, actualJob.Steps[i].Uses, "Step uses should match")
						}
					}
				}
			}
		})
	}
}

// assertDependenciesEqual checks that expected and actual dependencies match (order-independent)
func assertDependenciesEqual(t *testing.T, expected, actual []types.Dependency) {
	t.Helper()
	assert.Equal(t, len(expected), len(actual), "Number of dependencies should match")

	for _, expectedDep := range expected {
		found := false
		for _, actualDep := range actual {
			if actualDep.Type == expectedDep.Type &&
				actualDep.Name == expectedDep.Name &&
				actualDep.Version == expectedDep.Version {
				assert.Equal(t, expectedDep.Scope, actualDep.Scope, "Dependency scope should match")
				assert.Equal(t, expectedDep.Direct, actualDep.Direct, "Dependency direct flag should match")
				assert.Equal(t, expectedDep.Metadata, actualDep.Metadata, "Dependency metadata should match")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected dependency not found: %s@%s", expectedDep.Name, expectedDep.Version)
	}
}

// assertActionNamesEqual checks that expected and actual action names match (order-independent)
func assertActionNamesEqual(t *testing.T, expected, actual []string) {
	t.Helper()
	assert.Equal(t, len(expected), len(actual), "Number of action names should match")

	for _, expectedName := range expected {
		assert.Contains(t, actual, expectedName, "Action name should be present: %s", expectedName)
	}
}

// assertContainerAndServicesCase handles the special case for container and services test
func assertContainerAndServicesCase(t *testing.T, dependencies []types.Dependency, names []string) {
	t.Helper()
	assert.Equal(t, 4, len(dependencies), "Should have 4 dependencies (1 action + 1 container + 2 services)")
	assert.Equal(t, 1, len(names), "Should have 1 action name")

	// Check for action dependency
	var foundAction bool
	for _, dep := range dependencies {
		if dep.Type == "githubAction" && dep.Name == "actions/checkout" {
			foundAction = true
			assert.Equal(t, "v4", dep.Version)
			assert.Equal(t, types.ScopeBuild, dep.Scope)
			break
		}
	}
	assert.True(t, foundAction, "Should find actions/checkout dependency")

	// Check for container dependency
	var foundContainer bool
	for _, dep := range dependencies {
		if dep.Type == "docker" && dep.Name == "node" {
			foundContainer = true
			assert.Equal(t, "18-alpine", dep.Version)
			break
		}
	}
	assert.True(t, foundContainer, "Should find node container dependency")

	// Check for service dependencies
	var foundPostgres, foundRedis bool
	for _, dep := range dependencies {
		if dep.Type == "docker" {
			if dep.Name == "postgres" && dep.Version == "13" {
				foundPostgres = true
			} else if dep.Name == "redis" && dep.Version == "6-alpine" {
				foundRedis = true
			}
		}
	}
	assert.True(t, foundPostgres, "Should find postgres service dependency")
	assert.True(t, foundRedis, "Should find redis service dependency")

	assert.Equal(t, []string{"actions/checkout"}, names, "Action names should match")
}

func TestGitHubActionsCreateDependencies(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name          string
		workflow      *GitHubActionsWorkflow
		expectedDeps  []types.Dependency
		expectedNames []string
	}{
		{
			name: "workflow with actions only",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
							{Uses: "actions/setup-node@v3"},
							{Uses: "actions/cache"},
						},
					},
				},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/setup-node", Version: "v3", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/cache", Version: "latest", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout", "actions/setup-node", "actions/cache"},
		},
		{
			name: "workflow with container and services",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn:    "ubuntu-latest",
						Container: "node:18-alpine",
						Services: map[string]GitHubActionsService{
							"postgres": {Image: "postgres:13"},
							"redis":    {Image: "redis:6-alpine"},
						},
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
						},
					},
				},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout"},
		},
		{
			name: "workflow with container object",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Container: map[string]interface{}{
							"image":   "node:18-alpine",
							"options": "--user root",
						},
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
						},
					},
				},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "docker", Name: "node", Version: "18-alpine", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout"},
		},
		{
			name: "empty workflow",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{},
			},
			expectedDeps:  []types.Dependency{},
			expectedNames: []string{},
		},
		{
			name: "workflow with no uses steps",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Steps: []GitHubActionsStep{
							{Name: "Run tests", Run: "npm test"},
							{Name: "Build", Run: "npm run build"},
						},
					},
				},
			},
			expectedDeps:  []types.Dependency{},
			expectedNames: []string{},
		},
		{
			name: "multiple jobs",
			workflow: &GitHubActionsWorkflow{
				Jobs: map[string]GitHubActionsJob{
					"test": {
						RunsOn: "ubuntu-latest",
						Steps: []GitHubActionsStep{
							{Uses: "actions/checkout@v4"},
						},
					},
					"build": {
						RunsOn: "windows-latest",
						Steps: []GitHubActionsStep{
							{Uses: "actions/setup-node@v3"},
						},
					},
				},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/setup-node", Version: "v3", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout", "actions/setup-node"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies, names := parser.CreateDependencies(tt.workflow)

			if tt.name == "workflow with container and services" {
				assertContainerAndServicesCase(t, dependencies, names)
			} else {
				assertDependenciesEqual(t, tt.expectedDeps, dependencies)
				assertActionNamesEqual(t, tt.expectedNames, names)
			}
		})
	}
}

func TestParseActionReference(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name     string
		uses     string
		expected string
		version  string
	}{
		{"action with version", "actions/checkout@v4", "actions/checkout", "v4"},
		{"action with latest", "actions/setup-node@latest", "actions/setup-node", "latest"},
		{"action without version", "actions/cache", "actions/cache", "latest"},
		{"action with complex version", "actions/upload-artifact@v3.1.2", "actions/upload-artifact", "v3.1.2"},
		{"action with SHA", "actions/checkout@a81bbbf8298c0fa03ea29dc474d359e3b4a1a8c0", "actions/checkout", "a81bbbf8298c0fa03ea29dc474d359e3b4a1a8c0"},
		{"local action", "./.github/actions/my-action@v1", "./.github/actions/my-action", "v1"},
		{"docker action", "docker://ubuntu:20.04", "docker://ubuntu:20.04", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := parser.parseActionReference(tt.uses)
			assert.Equal(t, tt.expected, name, "Action name should match")
			assert.Equal(t, tt.version, version, "Action version should match")
		})
	}
}

func TestExtractImageName(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name      string
		container interface{}
		expected  string
	}{
		{"string container", "node:18-alpine", "node:18-alpine"},
		{"empty string", "", ""},
		{"container object", map[string]interface{}{"image": "postgres:13"}, "postgres:13"},
		{"container object with options", map[string]interface{}{"image": "node:18-alpine", "options": "--user root"}, "node:18-alpine"},
		{"container object without image", map[string]interface{}{"options": "--user root"}, ""},
		{"nil container", nil, ""},
		{"wrong type", 123, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractImageName(tt.container)
			assert.Equal(t, tt.expected, result, "Image name extraction should match")
		})
	}
}

func TestParseImageReference(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name     string
		image    string
		expected string
		version  string
	}{
		{"image with tag", "node:18-alpine", "node", "18-alpine"},
		{"image with version", "postgres:13", "postgres", "13"},
		{"image without tag", "ubuntu", "ubuntu", "latest"},
		{"image with complex tag", "redis:6.2.6-alpine", "redis", "6.2.6-alpine"},
		{"image with SHA", "myimage@sha256:abc123", "myimage@sha256", "abc123"},
		{"empty image", "", "", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := parser.parseImageReference(tt.image)
			assert.Equal(t, tt.expected, name, "Image name should match")
			assert.Equal(t, tt.version, version, "Image version should match")
		})
	}
}

func TestExtractFromContainer(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name      string
		container interface{}
		expected  *types.Dependency
	}{
		{
			name:      "string container",
			container: "node:18-alpine",
			expected: &types.Dependency{
				Type:     "docker",
				Name:     "node",
				Version:  "18-alpine",
				Scope:    types.ScopeBuild,
				Direct:   true,
				Metadata: types.NewMetadata(".github/workflows"),
			},
		},
		{
			name:      "container object",
			container: map[string]interface{}{"image": "postgres:13"},
			expected: &types.Dependency{
				Type:     "docker",
				Name:     "postgres",
				Version:  "13",
				Scope:    types.ScopeBuild,
				Direct:   true,
				Metadata: types.NewMetadata(".github/workflows"),
			},
		},
		{
			name:      "nil container",
			container: nil,
			expected:  nil,
		},
		{
			name:      "empty string",
			container: "",
			expected:  nil,
		},
		{
			name:      "container without image",
			container: map[string]interface{}{"options": "--user root"},
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractFromContainer(tt.container)

			if tt.expected == nil {
				assert.Nil(t, result, "Should return nil for invalid container")
			} else {
				require.NotNil(t, result, "Should return dependency for valid container")
				assert.Equal(t, tt.expected.Type, result.Type, "Dependency type should match")
				assert.Equal(t, tt.expected.Name, result.Name, "Dependency name should match")
				assert.Equal(t, tt.expected.Version, result.Version, "Dependency version should match")
				assert.Equal(t, tt.expected.Scope, result.Scope, "Dependency scope should match")
				assert.Equal(t, tt.expected.Direct, result.Direct, "Dependency direct flag should match")
				assert.Equal(t, tt.expected.Metadata, result.Metadata, "Dependency metadata should match")
			}
		})
	}
}

func TestExtractFromServices(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name     string
		services map[string]GitHubActionsService
		expected []types.Dependency
	}{
		{
			name: "multiple services",
			services: map[string]GitHubActionsService{
				"postgres": {Image: "postgres:13"},
				"redis":    {Image: "redis:6-alpine"},
			},
			expected: []types.Dependency{
				{Type: "docker", Name: "postgres", Version: "13", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "docker", Name: "redis", Version: "6-alpine", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
		},
		{
			name:     "empty services",
			services: map[string]GitHubActionsService{},
			expected: []types.Dependency{},
		},
		{
			name: "service with empty image",
			services: map[string]GitHubActionsService{
				"empty": {Image: ""},
			},
			expected: []types.Dependency{},
		},
		{
			name: "service without version",
			services: map[string]GitHubActionsService{
				"ubuntu": {Image: "ubuntu"},
			},
			expected: []types.Dependency{
				{Type: "docker", Name: "ubuntu", Version: "latest", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractFromServices(tt.services)
			assert.Equal(t, len(tt.expected), len(result), "Number of dependencies should match")

			// Build lookup by name since map iteration order is non-deterministic
			resultByName := make(map[string]types.Dependency, len(result))
			for _, dep := range result {
				resultByName[dep.Name] = dep
			}
			for _, expectedDep := range tt.expected {
				actualDep, found := resultByName[expectedDep.Name]
				if assert.True(t, found, "Expected dependency %q not found in results", expectedDep.Name) {
					assert.Equal(t, expectedDep.Type, actualDep.Type, "Dependency type should match for %s", expectedDep.Name)
					assert.Equal(t, expectedDep.Version, actualDep.Version, "Dependency version should match for %s", expectedDep.Name)
					assert.Equal(t, expectedDep.Scope, actualDep.Scope, "Dependency scope should match for %s", expectedDep.Name)
					assert.Equal(t, expectedDep.Direct, actualDep.Direct, "Dependency direct flag should match for %s", expectedDep.Name)
					assert.Equal(t, expectedDep.Metadata, actualDep.Metadata, "Dependency metadata should match for %s", expectedDep.Name)
				}
			}
		})
	}
}

func TestExtractFromSteps(t *testing.T) {
	parser := NewGitHubActionsParser()

	tests := []struct {
		name          string
		steps         []GitHubActionsStep
		expectedDeps  []types.Dependency
		expectedNames []string
	}{
		{
			name: "steps with actions",
			steps: []GitHubActionsStep{
				{Uses: "actions/checkout@v4"},
				{Uses: "actions/setup-node@v3"},
				{Uses: "actions/cache"},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/setup-node", Version: "v3", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/cache", Version: "latest", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout", "actions/setup-node", "actions/cache"},
		},
		{
			name: "steps without uses",
			steps: []GitHubActionsStep{
				{Name: "Run tests", Run: "npm test"},
				{Name: "Build", Run: "npm run build"},
			},
			expectedDeps:  []types.Dependency{},
			expectedNames: []string{},
		},
		{
			name: "mixed steps",
			steps: []GitHubActionsStep{
				{Uses: "actions/checkout@v4"},
				{Name: "Run tests", Run: "npm test"},
				{Uses: "actions/setup-node@v3"},
			},
			expectedDeps: []types.Dependency{
				{Type: "githubAction", Name: "actions/checkout", Version: "v4", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
				{Type: "githubAction", Name: "actions/setup-node", Version: "v3", Scope: types.ScopeBuild, Direct: true, Metadata: types.NewMetadata(".github/workflows")},
			},
			expectedNames: []string{"actions/checkout", "actions/setup-node"},
		},
		{
			name:          "empty steps",
			steps:         []GitHubActionsStep{},
			expectedDeps:  []types.Dependency{},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies, names := parser.extractFromSteps(tt.steps)
			assert.Equal(t, len(tt.expectedDeps), len(dependencies), "Number of dependencies should match")
			assert.Equal(t, len(tt.expectedNames), len(names), "Number of action names should match")

			for i, expectedDep := range tt.expectedDeps {
				if i < len(dependencies) {
					assert.Equal(t, expectedDep.Type, dependencies[i].Type, "Dependency type should match")
					assert.Equal(t, expectedDep.Name, dependencies[i].Name, "Dependency name should match")
					assert.Equal(t, expectedDep.Version, dependencies[i].Version, "Dependency version should match")
					assert.Equal(t, expectedDep.Scope, dependencies[i].Scope, "Dependency scope should match")
					assert.Equal(t, expectedDep.Direct, dependencies[i].Direct, "Dependency direct flag should match")
					assert.Equal(t, expectedDep.Metadata, dependencies[i].Metadata, "Dependency metadata should match")
				}
			}

			assert.Equal(t, tt.expectedNames, names, "Action names should match")
		})
	}
}

func TestGitHubActionsParserIntegration(t *testing.T) {
	// Integration test with a realistic GitHub Actions workflow
	parser := NewGitHubActionsParser()

	realWorkflow := `name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        node-version: [16.x, 18.x, 20.x]
    
    services:
      postgres:
        image: postgres:13
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:6-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: \${{ matrix.node-version }}
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Run tests
        run: npm test

      - name: Upload coverage
        uses: codecov/codecov-action@v3

  build:
    runs-on: ubuntu-latest
    container: node:18-alpine
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build application
        run: npm run build

      - name: Build Docker image
        run: docker build -t myapp .

  deploy:
    runs-on: ubuntu-latest
    needs: [test, build]
    if: github.ref == 'refs/heads/main'
    
    steps:
      - name: Deploy to production
        uses: ./.github/actions/deploy@v1
`

	// Parse the workflow
	workflow, err := parser.ParseWorkflow(realWorkflow)
	require.NoError(t, err, "Should parse workflow without error")
	require.NotNil(t, workflow, "Workflow should not be nil")

	// Verify basic structure
	assert.Equal(t, "CI/CD Pipeline", workflow.Name, "Workflow name should match")
	assert.Equal(t, 3, len(workflow.Jobs), "Should have 3 jobs")

	// Extract dependencies
	dependencies, actionNames := parser.CreateDependencies(workflow)

	// Should have actions from all jobs
	expectedActionCount := 5 // checkout (2 times) + setup-node + codecov + deploy
	expectedDepsCount := 8   // 5 actions + postgres + redis services + node container
	assert.Equal(t, expectedDepsCount, len(dependencies), "Should extract correct number of dependencies")
	assert.Equal(t, expectedActionCount, len(actionNames), "Should have correct number of action names")

	// Verify specific dependencies
	var checkoutDeps, setupNodeDeps, codecovDeps, deployDeps, postgresDeps, redisDeps, nodeDeps int

	for _, dep := range dependencies {
		switch dep.Name {
		case "actions/checkout":
			checkoutDeps++
			assert.Equal(t, "githubAction", dep.Type)
			assert.Equal(t, types.ScopeBuild, dep.Scope)
			assert.Equal(t, true, dep.Direct)
			assert.Equal(t, types.NewMetadata(".github/workflows"), dep.Metadata)
		case "actions/setup-node":
			setupNodeDeps++
			assert.Equal(t, "v3", dep.Version)
		case "codecov/codecov-action":
			codecovDeps++
			assert.Equal(t, "v3", dep.Version)
		case "./.github/actions/deploy":
			deployDeps++
			assert.Equal(t, "v1", dep.Version)
		case "postgres":
			postgresDeps++
			assert.Equal(t, "docker", dep.Type)
			assert.Equal(t, "13", dep.Version)
		case "redis":
			redisDeps++
			assert.Equal(t, "docker", dep.Type)
			assert.Equal(t, "6-alpine", dep.Version)
		case "node":
			nodeDeps++
			assert.Equal(t, "docker", dep.Type)
			assert.Equal(t, "18-alpine", dep.Version)
		}
	}

	assert.Equal(t, 2, checkoutDeps, "Should have 2 checkout actions")
	assert.Equal(t, 1, setupNodeDeps, "Should have 1 setup-node action")
	assert.Equal(t, 1, codecovDeps, "Should have 1 codecov action")
	assert.Equal(t, 1, deployDeps, "Should have 1 deploy action")
	assert.Equal(t, 1, postgresDeps, "Should have 1 postgres service")
	assert.Equal(t, 1, redisDeps, "Should have 1 redis service")
	assert.Equal(t, 1, nodeDeps, "Should have 1 node container")

	// Verify action names include all expected actions
	expectedNames := []string{
		"actions/checkout", "actions/setup-node", "codecov/codecov-action",
		"actions/checkout", "./.github/actions/deploy",
	}

	for _, expectedName := range expectedNames {
		assert.Contains(t, actionNames, expectedName, "Should contain action name: %s", expectedName)
	}
}
