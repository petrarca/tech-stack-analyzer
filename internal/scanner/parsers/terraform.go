package parsers

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// TerraformParser handles Terraform-specific file parsing (.tf and .terraform.lock.hcl)
type TerraformParser struct{}

// NewTerraformParser creates a new Terraform parser
func NewTerraformParser() *TerraformParser {
	return &TerraformParser{}
}

// TerraformProvider represents a provider in terraform.lock.hcl
type TerraformProvider struct {
	Name    string
	Version string
}

// TerraformResource represents a parsed Terraform resource
type TerraformResource struct {
	Type     string // e.g., "aws_instance", "google_storage_bucket"
	Name     string // e.g., "web_server", "app_bucket"
	Provider string // e.g., "aws", "google", "azurerm"
	Category string // e.g., "compute", "storage", "database"
}

// TerraformInfo represents aggregated information from a Terraform file
type TerraformInfo struct {
	File                string         `json:"file,omitempty"`
	Providers           []string       `json:"providers,omitempty"`
	ResourcesByProvider map[string]int `json:"resources_by_provider,omitempty"`
	ResourcesByCategory map[string]int `json:"resources_by_category,omitempty"`
	TotalResources      int            `json:"total_resources,omitempty"`
}

// ParseTerraformLock parses .terraform.lock.hcl and extracts providers
func (p *TerraformParser) ParseTerraformLock(content string) []TerraformProvider {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(content), "terraform.lock.hcl")
	if diags.HasErrors() {
		return nil
	}

	contentBody := file.Body
	providers := []TerraformProvider{}

	// Extract provider blocks
	if contentBody != nil {
		// Try to parse as blocks
		content, _ := contentBody.Content(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type:       "provider",
					LabelNames: []string{"name"},
				},
			},
		})

		for _, block := range content.Blocks.OfType("provider") {
			if len(block.Labels) > 0 {
				providerName := block.Labels[0]
				version := "latest"

				// Extract version from attributes
				attrs, _ := block.Body.JustAttributes()
				if versionAttr, exists := attrs["version"]; exists {
					if val, diags := versionAttr.Expr.Value(nil); !diags.HasErrors() && val.Type() == cty.String {
						version = val.AsString()
					}
				}

				providers = append(providers, TerraformProvider{
					Name:    providerName,
					Version: version,
				})
			}
		}
	}

	return providers
}

// ParseTerraformResources parses .tf files and extracts full resource information
func (p *TerraformParser) ParseTerraformResources(content string) []TerraformResource {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(content), "resource.tf")
	if diags.HasErrors() {
		return nil
	}

	contentBody := file.Body
	var resources []TerraformResource

	// Extract resource blocks
	if contentBody != nil {
		content, _ := contentBody.Content(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type:       "resource",
					LabelNames: []string{"type", "name"},
				},
			},
		})

		for _, block := range content.Blocks.OfType("resource") {
			if len(block.Labels) >= 2 {
				resourceType := block.Labels[0] // e.g., "aws_instance"
				resourceName := block.Labels[1] // e.g., "web"

				resource := TerraformResource{
					Type:     resourceType,
					Name:     resourceName,
					Provider: extractProvider(resourceType),
					Category: categorizeResource(resourceType),
				}

				resources = append(resources, resource)
			}
		}
	}

	return resources
}

// extractProvider extracts provider from resource type
// Resource types follow pattern: PROVIDER_SERVICE_RESOURCE
// Examples: aws_instance, google_compute_instance, azurerm_virtual_machine
func extractProvider(resourceType string) string {
	parts := strings.Split(resourceType, "_")
	if len(parts) > 0 {
		return parts[0] // "aws", "google", "azurerm"
	}
	return "unknown"
}

// categorizeResource maps resource types to categories
func categorizeResource(resourceType string) string {
	// Map common resource types to categories
	categories := map[string][]string{
		"compute": {
			"aws_instance", "aws_autoscaling_group", "aws_launch_template",
			"google_compute_instance", "google_compute_instance_group",
			"azurerm_virtual_machine", "azurerm_linux_virtual_machine", "azurerm_windows_virtual_machine",
		},
		"storage": {
			"aws_s3_bucket", "aws_ebs_volume", "aws_efs_file_system",
			"google_storage_bucket", "google_compute_disk",
			"azurerm_storage_account", "azurerm_storage_blob", "azurerm_storage_container",
		},
		"database": {
			"aws_db_instance", "aws_rds_cluster", "aws_dynamodb_table",
			"google_sql_database_instance", "google_bigtable_instance",
			"azurerm_sql_database", "azurerm_cosmosdb_account", "azurerm_mysql_server",
		},
		"networking": {
			"aws_vpc", "aws_subnet", "aws_security_group", "aws_lb", "aws_route_table",
			"google_compute_network", "google_compute_firewall", "google_compute_subnetwork",
			"azurerm_virtual_network", "azurerm_network_security_group", "azurerm_subnet",
		},
		"container": {
			"aws_ecs_cluster", "aws_eks_cluster", "aws_ecr_repository",
			"google_container_cluster", "google_cloud_run_service", "google_container_registry",
			"azurerm_kubernetes_cluster", "azurerm_container_registry", "azurerm_container_group",
		},
		"serverless": {
			"aws_lambda_function", "aws_api_gateway_rest_api",
			"google_cloudfunctions_function", "google_cloud_run_service",
			"azurerm_function_app", "azurerm_app_service",
		},
	}

	for category, types := range categories {
		for _, t := range types {
			if t == resourceType {
				return category
			}
		}
	}

	return "other"
}

// AggregateTerraformResources aggregates resources into TerraformInfo
func (p *TerraformParser) AggregateTerraformResources(resources []TerraformResource) *TerraformInfo {
	if len(resources) == 0 {
		return nil
	}

	info := &TerraformInfo{
		ResourcesByProvider: make(map[string]int),
		ResourcesByCategory: make(map[string]int),
		TotalResources:      len(resources),
	}

	// Track unique providers (pre-allocate with reasonable capacity)
	providerSet := make(map[string]bool, 4) // Most projects use 1-4 providers

	for _, resource := range resources {
		// Count by provider
		if resource.Provider != "" {
			info.ResourcesByProvider[resource.Provider]++
			providerSet[resource.Provider] = true
		}

		// Count by category
		if resource.Category != "" {
			info.ResourcesByCategory[resource.Category]++
		}
	}

	// Convert provider set to slice (pre-allocate with exact size)
	info.Providers = make([]string, 0, len(providerSet))
	for provider := range providerSet {
		info.Providers = append(info.Providers, provider)
	}

	return info
}
