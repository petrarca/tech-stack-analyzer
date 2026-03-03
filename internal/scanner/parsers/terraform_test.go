package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTerraformParser(t *testing.T) {
	parser := NewTerraformParser()
	assert.NotNil(t, parser, "Should create a new TerraformParser")
	assert.IsType(t, &TerraformParser{}, parser, "Should return correct type")
}

func TestParseTerraformLock(t *testing.T) {
	parser := NewTerraformParser()

	tests := []struct {
		name              string
		content           string
		expectedProviders []TerraformProvider
	}{
		{
			name: "basic terraform.lock.hcl",
			content: `provider "registry.terraform.io/hashicorp/aws" {
  version     = "5.0.0"
  constraints = ">= 4.0"
}

provider "registry.terraform.io/hashicorp/google" {
  version     = "4.5.0"
}

provider "registry.terraform.io/azurerm/azurerm" {
  version     = "3.0.0"
}`,
			expectedProviders: []TerraformProvider{
				{Name: "registry.terraform.io/hashicorp/aws", Version: "5.0.0"},
				{Name: "registry.terraform.io/hashicorp/google", Version: "4.5.0"},
				{Name: "registry.terraform.io/azurerm/azurerm", Version: "3.0.0"},
			},
		},
		{
			name: "terraform.lock.hcl without versions",
			content: `provider "registry.terraform.io/hashicorp/aws" {
}

provider "registry.terraform.io/hashicorp/google" {
}

provider "registry.terraform.io/hashicorp/kubernetes" {
}`,
			expectedProviders: []TerraformProvider{
				{Name: "registry.terraform.io/hashicorp/aws", Version: "latest"},
				{Name: "registry.terraform.io/hashicorp/google", Version: "latest"},
				{Name: "registry.terraform.io/hashicorp/kubernetes", Version: "latest"},
			},
		},
		{
			name: "terraform.lock.hcl with complex versions",
			content: `provider "registry.terraform.io/hashicorp/aws" {
  version     = "~> 5.0.0"
  constraints = ">= 4.0, < 6.0"
}

provider "registry.terraform.io/hashicorp/google" {
  version     = ">= 4.5.0"
}

provider "registry.terraform.io/azurerm/azurerm" {
  version     = "3.0.0"
}`,
			expectedProviders: []TerraformProvider{
				{Name: "registry.terraform.io/hashicorp/aws", Version: "~> 5.0.0"},
				{Name: "registry.terraform.io/hashicorp/google", Version: ">= 4.5.0"},
				{Name: "registry.terraform.io/azurerm/azurerm", Version: "3.0.0"},
			},
		},
		{
			name:              "empty terraform.lock.hcl",
			content:           "",
			expectedProviders: []TerraformProvider{},
		},
		{
			name: "invalid HCL syntax",
			content: `provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
  # Missing closing brace`,
			expectedProviders: []TerraformProvider{},
		},
		{
			name: "terraform.lock.hcl with comments",
			content: `# AWS Provider
provider "registry.terraform.io/hashicorp/aws" {
  version     = "5.0.0"  # Latest version
  constraints = ">= 4.0"
}

# Google Cloud Provider
provider "registry.terraform.io/hashicorp/google" {
  version     = "4.5.0"
}

# Azure Provider
provider "registry.terraform.io/azurerm/azurerm" {
  version     = "3.0.0"
}`,
			expectedProviders: []TerraformProvider{
				{Name: "registry.terraform.io/hashicorp/aws", Version: "5.0.0"},
				{Name: "registry.terraform.io/hashicorp/google", Version: "4.5.0"},
				{Name: "registry.terraform.io/azurerm/azurerm", Version: "3.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := parser.ParseTerraformLock(tt.content)

			require.Len(t, providers, len(tt.expectedProviders), "Should return correct number of providers")

			for i, expectedProvider := range tt.expectedProviders {
				assert.Equal(t, expectedProvider.Name, providers[i].Name, "Should return correct provider name")
				assert.Equal(t, expectedProvider.Version, providers[i].Version, "Should return correct provider version")
			}
		})
	}
}

func TestParseTerraformResources(t *testing.T) {
	parser := NewTerraformParser()

	tests := []struct {
		name              string
		content           string
		expectedResources []string
	}{
		{
			name: "basic terraform resources",
			content: `resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "google_compute_instance" "default" {
  name         = "test"
  machine_type = "e2-medium"
}`,
			expectedResources: []string{"aws_instance", "aws_vpc", "google_compute_instance"},
		},
		{
			name: "terraform resources with duplicates (should deduplicate)",
			content: `resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

resource "aws_instance" "db" {
  ami           = "ami-87654321"
  instance_type = "t3.small"
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_vpc" "secondary" {
  cidr_block = "10.1.0.0/16"
}`,
			expectedResources: []string{"aws_instance", "aws_instance", "aws_vpc", "aws_vpc"},
		},
		{
			name: "terraform resources with data sources",
			content: `data "aws_ami" "ubuntu" {
  most_recent = true
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }
}

resource "aws_instance" "web" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = "t3.micro"
}

resource "aws_security_group" "web" {
  name = "web-sg"
}`,
			expectedResources: []string{"aws_instance", "aws_security_group"},
		},
		{
			name: "terraform resources with modules",
			content: `module "vpc" {
  source = "./modules/vpc"
}

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

module "security_group" {
  source = "./modules/security_group"
}

resource "aws_subnet" "public" {
  vpc_id     = aws_vpc.main.id
  cidr_block = "10.0.1.0/24"
}`,
			expectedResources: []string{"aws_instance", "aws_subnet"},
		},
		{
			name: "terraform resources with providers",
			content: `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}`,
			expectedResources: []string{"aws_instance"},
		},
		{
			name: "terraform resources with variables",
			content: `variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.micro"
}

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = var.instance_type
}

resource "aws_instance" "db" {
  ami           = "ami-87654321"
  instance_type = var.instance_type
}`,
			expectedResources: []string{"aws_instance", "aws_instance"},
		},
		{
			name: "terraform resources with outputs",
			content: `output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.web.id
}

resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

output "instance_public_ip" {
  description = "Public IP address of the EC2 instance"
  value       = aws_instance.web.public_ip
}`,
			expectedResources: []string{"aws_instance"},
		},
		{
			name:              "empty terraform file",
			content:           "",
			expectedResources: []string{},
		},
		{
			name: "invalid HCL syntax",
			content: `resource "aws_instance" "web" {
  ami = "ami-12345678"
  # Missing closing brace`,
			expectedResources: []string{},
		},
		{
			name: "terraform resources with comments",
			content: `# Web server instance
resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
}

# VPC configuration
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"  # Main VPC CIDR
}

# Database instance
resource "aws_instance" "db" {
  ami           = "ami-87654321"
  instance_type = "t3.small"
}`,
			expectedResources: []string{"aws_instance", "aws_vpc", "aws_instance"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := parser.ParseTerraformResources(tt.content)

			require.Len(t, resources, len(tt.expectedResources), "Should return correct number of resources")

			for i, expectedResource := range tt.expectedResources {
				assert.Equal(t, expectedResource, resources[i].Type, "Should return correct resource type")
			}
		})
	}
}

func TestTerraformParser_Integration(t *testing.T) {
	parser := NewTerraformParser()

	// Test realistic terraform.lock.hcl
	lockFileContent := `# This file is maintained automatically by "terraform init".
# Manual edits may be lost in future updates.

provider "registry.terraform.io/hashicorp/aws" {
  version     = "5.0.0"
  constraints = ">= 4.0"
  hashes = [
    "sha256:abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567stu890vwx1",
  ]
}

provider "registry.terraform.io/hashicorp/kubernetes" {
  version     = "2.20.0"
  constraints = ">= 2.10.0"
  hashes = [
    "sha256:def789ghi012jkl345mno678pqr901vwx234yz567stu890vwx123yz4567stu8",
  ]
}

provider "registry.terraform.io/hashicorp/random" {
  version     = "3.5.1"
  constraints = ">= 3.1.0"
  hashes = [
    "sha256:ghi012jkl345mno678pqr901vwx234yz567stu890vwx123yz4567stu890vwx123",
  ]
}

provider "registry.terraform.io/cloudflare/cloudflare" {
  version     = "4.0.0"
  constraints = ">= 3.0"
  hashes = [
    "sha256:jkl345mno678pqr901vwx234yz567stu890vwx123yz4567stu890vwx123yz4567",
  ]
}`

	providers := parser.ParseTerraformLock(lockFileContent)

	assert.Len(t, providers, 4)

	// Create provider map for verification
	providerMap := make(map[string]TerraformProvider)
	for _, provider := range providers {
		providerMap[provider.Name] = provider
	}

	// Verify AWS provider
	assert.Contains(t, providerMap["registry.terraform.io/hashicorp/aws"].Name, "hashicorp/aws")
	assert.Equal(t, "5.0.0", providerMap["registry.terraform.io/hashicorp/aws"].Version)

	// Verify Kubernetes provider
	assert.Equal(t, "2.20.0", providerMap["registry.terraform.io/hashicorp/kubernetes"].Version)

	// Verify Random provider
	assert.Equal(t, "3.5.1", providerMap["registry.terraform.io/hashicorp/random"].Version)

	// Verify Cloudflare provider
	assert.Equal(t, "4.0.0", providerMap["registry.terraform.io/cloudflare/cloudflare"].Version)

	// Test realistic terraform configuration
	terraformConfig := `terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.20"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

provider "kubernetes" {
  config_path = "~/.kube/config"
}

# VPC Module
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "my-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-west-2a", "us-west-2b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway = true
  enable_vpn_gateway = false

  tags = {
    Terraform   = "true"
    Environment = "dev"
  }
}

# Security Group
resource "aws_security_group" "web" {
  name        = "web-sg"
  description = "Allow web traffic"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "web-sg"
  }
}

# EC2 Instance
resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
  subnet_id     = module.vpc.private_subnets[0]

  vpc_security_group_ids = [aws_security_group.web.id]

  tags = {
    Name = "web-server"
  }
}

# Kubernetes Namespace
resource "kubernetes_namespace" "example" {
  metadata {
    name = "example-namespace"
  }
}

# Kubernetes Deployment
resource "kubernetes_deployment" "example" {
  metadata {
    name = "example-deployment"
    namespace = kubernetes_namespace.example.metadata.0.name
  }

  spec {
    replicas = 3

    selector {
      match_labels = {
        app = "example"
      }
    }

    template {
      metadata {
        labels = {
          app = "example"
        }
      }

      spec {
        container {
          image = "nginx:1.21"
          name  = "nginx"

          port {
            container_port = 80
          }
        }
      }
    }
  }
}

# Random Password
resource "random_password" "db_password" {
  length  = 16
  special = true
}

# Outputs
output "vpc_id" {
  description = "The ID of the VPC"
  value       = module.vpc.vpc_id
}

output "web_instance_id" {
  description = "The ID of the web instance"
  value       = aws_instance.web.id
}`

	resources := parser.ParseTerraformResources(terraformConfig)

	assert.Len(t, resources, 5) // aws_security_group, aws_instance, kubernetes_namespace, kubernetes_deployment, random_password

	// Create resource set for verification
	resourceSet := make(map[string]bool)
	for _, resource := range resources {
		resourceSet[resource.Type] = true
	}

	// Verify key resources
	assert.True(t, resourceSet["aws_security_group"], "Should detect aws_security_group")
	assert.True(t, resourceSet["aws_instance"], "Should detect aws_instance")
	assert.True(t, resourceSet["kubernetes_namespace"], "Should detect kubernetes_namespace")
	assert.True(t, resourceSet["kubernetes_deployment"], "Should detect kubernetes_deployment")
	assert.True(t, resourceSet["random_password"], "Should detect random_password")
}

func TestTerraformParser_EdgeCases(t *testing.T) {
	parser := NewTerraformParser()

	// Test with malformed HCL - HCL parser fails on any syntax error
	t.Run("malformed HCL", func(t *testing.T) {
		content := `provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
}

provider "registry.terraform.io/hashicorp/google" {
  version = "4.5.0"
  # Missing closing brace

resource "aws_instance" "web" {
  ami = "ami-12345678"
}`

		providers := parser.ParseTerraformLock(content)
		// HCL parser fails on any syntax error, returns empty
		assert.Len(t, providers, 0)
	})

	// Test with provider blocks without labels
	t.Run("provider without labels", func(t *testing.T) {
		content := `provider {
  version = "5.0.0"
}

provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
}`

		providers := parser.ParseTerraformLock(content)
		// Should only parse the provider with label
		assert.Len(t, providers, 1)
		assert.Equal(t, "registry.terraform.io/hashicorp/aws", providers[0].Name)
	})

	// Test with resource blocks without labels
	t.Run("resource without labels", func(t *testing.T) {
		content := `resource {
  ami = "ami-12345678"
}

resource "aws_instance" "web" {
  ami = "ami-12345678"
}`

		resources := parser.ParseTerraformResources(content)
		// Should only parse the resource with labels
		assert.Len(t, resources, 1)
		assert.Equal(t, "aws_instance", resources[0].Type)
	})

	// Test with complex provider names
	t.Run("complex provider names", func(t *testing.T) {
		content := `provider "registry.terraform.io/hashicorp/aws" {
  version = "5.0.0"
}

provider "registry.terraform.io/integrations/github" {
  version = "5.0.0"
}

provider "registry.terraform.io/grafana/grafana" {
  version = "2.0.0"
}`

		providers := parser.ParseTerraformLock(content)
		assert.Len(t, providers, 3)

		providerMap := make(map[string]TerraformProvider)
		for _, provider := range providers {
			providerMap[provider.Name] = provider
		}

		assert.Contains(t, providerMap, "registry.terraform.io/hashicorp/aws")
		assert.Contains(t, providerMap, "registry.terraform.io/integrations/github")
		assert.Contains(t, providerMap, "registry.terraform.io/grafana/grafana")
	})
}
