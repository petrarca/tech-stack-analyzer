package terraform

import (
	"os"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider implements types.Provider for testing
type MockProvider struct {
	files map[string]string
}

func (m *MockProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockProvider) ListDir(path string) ([]types.File, error) {
	return nil, nil
}

func (m *MockProvider) Open(path string) (string, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return "", os.ErrNotExist
}

func (m *MockProvider) Exists(path string) (bool, error) {
	_, exists := m.files[path]
	return exists, nil
}

func (m *MockProvider) IsDir(path string) (bool, error) {
	return false, nil
}

func (m *MockProvider) GetBasePath() string {
	return "/mock"
}

// MockDependencyDetector implements components.DependencyDetector for testing
type MockDependencyDetector struct {
	matchedTechs map[string][]string
}

func (m *MockDependencyDetector) MatchDependencies(dependencies []string, depType string) map[string][]string {
	return m.matchedTechs
}

func (m *MockDependencyDetector) AddPrimaryTechIfNeeded(payload *types.Payload, tech string) {
	// Mock implementation - do nothing
}

func TestDetector_Name(t *testing.T) {
	detector := &Detector{}
	assert.Equal(t, "terraform", detector.Name())
}

func TestDetector_Detect_TerraformLock(t *testing.T) {
	detector := &Detector{}

	// Create mock .terraform.lock.hcl content
	lockContent := `# This file is maintained automatically by "terraform init".
# Manual edits may be lost in future updates.

provider "registry.terraform.io/hashicorp/aws" {
  version     = "5.0.0"
  constraints = ">= 4.0.0"
  hashes = [
    "h1:abc123",
    "zh1:def456",
  ]
}

provider "registry.terraform.io/hashicorp/kubernetes" {
  version     = "2.20.0"
  constraints = ">= 2.0.0"
  hashes = [
    "h1:ghi789",
    "zh1:jkl012",
  ]
}

provider "registry.terraform.io/cloudflare/cloudflare" {
  version     = "4.0.0"
  hashes = [
    "h1:mno345",
    "zh1:pqr678",
  ]
}
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/.terraform.lock.hcl": lockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"registry.terraform.io/hashicorp/aws":        {"matched provider: aws"},
			"registry.terraform.io/hashicorp/kubernetes": {"matched provider: kubernetes"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/.terraform.lock.hcl"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Terraform project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name) // Virtual payload for lock files
	assert.Equal(t, "/.terraform.lock.hcl", payload.Path[0])

	// Check dependencies - should include all providers
	assert.Len(t, payload.Dependencies, 3, "Should have 3 provider dependencies")

	depNames := make(map[string]bool)
	for _, dep := range payload.Dependencies {
		depNames[dep.Name] = true
		assert.Equal(t, "terraform", dep.Type, "All dependencies should be terraform type")
	}

	assert.True(t, depNames["registry.terraform.io/hashicorp/aws"], "Should have AWS provider")
	assert.True(t, depNames["registry.terraform.io/hashicorp/kubernetes"], "Should have Kubernetes provider")
	assert.True(t, depNames["registry.terraform.io/cloudflare/cloudflare"], "Should have Cloudflare provider")

	// Check child components - note: all providers get children created, not just matched ones
	assert.Len(t, payload.Children, 3, "Should have 3 child components (all providers get children)")

	childNames := make(map[string]*types.Payload)
	for _, child := range payload.Children {
		childNames[child.Name] = child
	}

	// Verify AWS child
	awsChild := childNames["registry.terraform.io/hashicorp/aws"]
	require.NotNil(t, awsChild, "Should have AWS child component")
	assert.NotEmpty(t, awsChild.Tech, "AWS child should have matched tech")
	assert.Len(t, awsChild.Dependencies, 1, "AWS child should have 1 dependency")
	assert.Equal(t, "registry.terraform.io/hashicorp/aws", awsChild.Dependencies[0].Name)

	// Verify Kubernetes child
	k8sChild := childNames["registry.terraform.io/hashicorp/kubernetes"]
	require.NotNil(t, k8sChild, "Should have Kubernetes child component")
	assert.NotEmpty(t, k8sChild.Tech, "Kubernetes child should have matched tech")
	assert.Len(t, k8sChild.Dependencies, 1, "Kubernetes child should have 1 dependency")
	assert.Equal(t, "registry.terraform.io/hashicorp/kubernetes", k8sChild.Dependencies[0].Name)
}

func TestDetector_Detect_TerraformResources(t *testing.T) {
	detector := &Detector{}

	// Create mock .tf content
	tfContent := `provider "aws" {
  region = "us-west-2"
}

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

resource "kubernetes_deployment" "app" {
  metadata {
    name = "my-app"
  }
  
  spec {
    replicas = 3
  }
}

resource "azurerm_resource_group" "main" {
  name     = "example-resources"
  location = "West Europe"
}
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/main.tf": tfContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"aws_instance":          {"matched resource: aws_instance"},
			"kubernetes_deployment": {"matched resource: kubernetes_deployment"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: "main.tf", Path: "/project/main.tf"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Terraform project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name) // Virtual payload for resource files
	assert.Equal(t, "/main.tf", payload.Path[0])

	// Check child components - note: all resources get children created, not just matched ones
	assert.Len(t, payload.Children, 3, "Should have 3 child components (all resources get children)")

	childNames := make(map[string]*types.Payload)
	for _, child := range payload.Children {
		childNames[child.Name] = child
	}

	// Verify AWS instance child
	awsChild := childNames["aws_instance"]
	require.NotNil(t, awsChild, "Should have aws_instance child component")
	assert.NotEmpty(t, awsChild.Tech, "AWS instance child should have matched tech")
	assert.Len(t, awsChild.Dependencies, 1, "AWS instance child should have 1 dependency")
	assert.Equal(t, "aws_instance", awsChild.Dependencies[0].Name)
	assert.Equal(t, "terraform-resource", awsChild.Dependencies[0].Type)
	assert.Equal(t, "web", awsChild.Dependencies[0].Version, "Should have resource name as Example")

	// Verify Kubernetes deployment child
	k8sChild := childNames["kubernetes_deployment"]
	require.NotNil(t, k8sChild, "Should have kubernetes_deployment child component")
	assert.NotEmpty(t, k8sChild.Tech, "Kubernetes deployment child should have matched tech")
	assert.Len(t, k8sChild.Dependencies, 1, "Kubernetes deployment child should have 1 dependency")
	assert.Equal(t, "kubernetes_deployment", k8sChild.Dependencies[0].Name)
	assert.Equal(t, "terraform-resource", k8sChild.Dependencies[0].Type)
	assert.Equal(t, "app", k8sChild.Dependencies[0].Version, "Should have resource name as Example")
}

func TestDetector_Detect_BothLockAndTfFiles(t *testing.T) {
	detector := &Detector{}

	// Create mock content for both files
	lockContent := `provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
  hashes = ["h1:abc123"]
}`

	tfContent := `resource "aws_instance" "web" {
  ami = "ami-12345678"
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/.terraform.lock.hcl": lockContent,
			"/project/main.tf":             tfContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"registry.terraform.io/hashicorp/aws": {"matched provider: aws"},
			"aws_instance":                        {"matched resource: aws_instance"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/.terraform.lock.hcl"},
		{Name: "main.tf", Path: "/project/main.tf"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results - should detect both files as separate components
	require.Len(t, results, 2, "Should detect two Terraform components")

	// First should be lock file component
	lockPayload := results[0]
	assert.Equal(t, "virtual", lockPayload.Name)
	assert.Equal(t, "/.terraform.lock.hcl", lockPayload.Path[0])
	assert.Len(t, lockPayload.Children, 1, "Lock file should have 1 child component")

	// Second should be resource file component
	tfPayload := results[1]
	assert.Equal(t, "virtual", tfPayload.Name)
	assert.Equal(t, "/main.tf", tfPayload.Path[0])
	assert.Len(t, tfPayload.Children, 1, "Resource file should have 1 child component")
}

func TestDetector_Detect_EmptyTerraformLock(t *testing.T) {
	detector := &Detector{}

	// Create empty lock file content
	lockContent := `# Empty lock file
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/.terraform.lock.hcl": lockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/.terraform.lock.hcl"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to no providers
	assert.Empty(t, results, "Should not detect project with no providers")
}

func TestDetector_Detect_EmptyTfFile(t *testing.T) {
	detector := &Detector{}

	// Create empty .tf content
	tfContent := `# Empty terraform file
`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/main.tf": tfContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "main.tf", Path: "/project/main.tf"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to no resources
	assert.Empty(t, results, "Should not detect project with no resources")
}

func TestDetector_Detect_NoTerraformFiles(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list with no Terraform files
	files := []types.File{
		{Name: "app.py", Path: "/project/app.py"},
		{Name: "package.json", Path: "/project/package.json"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any Terraform components without .tf or .terraform.lock.hcl files")
}

func TestDetector_Detect_EmptyFilesList(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Test with empty files list
	results := detector.Detect([]types.File{}, "/project", "/project", provider, depDetector)

	// Verify no results
	assert.Empty(t, results, "Should not detect any components from empty file list")
}

func TestDetector_Detect_FileReadError(t *testing.T) {
	detector := &Detector{}

	// Setup mock provider that returns error (empty files map)
	provider := &MockProvider{
		files: map[string]string{},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/.terraform.lock.hcl"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file read error
	assert.Empty(t, results, "Should not detect components when file read fails")
}

func TestDetector_Detect_RelativePathHandling(t *testing.T) {
	detector := &Detector{}

	// Create mock terraform lock content
	lockContent := `provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
  hashes = ["h1:abc123"]
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/subdir/.terraform.lock.hcl": lockContent,
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{
			"registry.terraform.io/hashicorp/aws": {"matched provider: aws"},
		},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/subdir/.terraform.lock.hcl"},
	}

	// Test detection with nested path
	results := detector.Detect(files, "/project/subdir", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Terraform project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Equal(t, "/subdir/.terraform.lock.hcl", payload.Path[0], "Should handle relative paths correctly")
}

func TestDetector_Detect_NoMatchingProviders(t *testing.T) {
	detector := &Detector{}

	// Create lock file with providers that don't match any tech
	lockContent := `provider "registry.terraform.io/hashicorp/unknown" {
  version = "1.0.0"
  hashes = ["h1:abc123"]
}

provider "registry.terraform.io/unknown/unknown" {
  version = "2.0.0"
  hashes = ["h1:def456"]
}`

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/.terraform.lock.hcl": lockContent,
		},
	}

	// Setup mock dependency detector with no matches
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: ".terraform.lock.hcl", Path: "/project/.terraform.lock.hcl"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify results
	require.Len(t, results, 1, "Should detect one Terraform project")

	payload := results[0]
	assert.Equal(t, "virtual", payload.Name)
	assert.Len(t, payload.Dependencies, 2, "Should have 2 dependencies")
	assert.Empty(t, payload.Children, "Should have no child components when no matches")
}

func TestDetector_Detect_LargeTfFile(t *testing.T) {
	detector := &Detector{}

	// Create a large .tf file (over 500KB limit)
	largeContent := make([]byte, 600_000)
	for i := range largeContent {
		largeContent[i] = '#'
	}

	// Setup mock provider
	provider := &MockProvider{
		files: map[string]string{
			"/project/large.tf": string(largeContent),
		},
	}

	// Setup mock dependency detector
	depDetector := &MockDependencyDetector{
		matchedTechs: map[string][]string{},
	}

	// Create file list
	files := []types.File{
		{Name: "large.tf", Path: "/project/large.tf"},
	}

	// Test detection
	results := detector.Detect(files, "/project", "/project", provider, depDetector)

	// Verify no results due to file size limit
	assert.Empty(t, results, "Should not detect large files over 500KB")
}
