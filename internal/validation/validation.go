package validation

import (
	"embed"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed *.json
var schemaFS embed.FS

// ValidationError represents a schema validation error
type ValidationError struct {
	Errors []string
}

func (e ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("validation failed: %s", e.Errors[0])
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(e.Errors, "; "))
}

// ValidateJSON validates a data structure against an embedded JSON schema
// schemaName should be the filename of the schema (e.g., "stack-analyzer-config.json")
// data should be the parsed YAML/JSON data as interface{}
func ValidateJSON(schemaName string, data interface{}) error {
	// Load schema from embedded filesystem
	schemaData, err := schemaFS.ReadFile(schemaName)
	if err != nil {
		return fmt.Errorf("failed to load schema %s: %w", schemaName, err)
	}

	// Compile schema
	schema, err := jsonschema.CompileString(schemaName, string(schemaData))
	if err != nil {
		return fmt.Errorf("failed to compile schema %s: %w", schemaName, err)
	}

	// Validate data
	err = schema.Validate(data)
	if err != nil {
		// Extract validation errors
		var validationErrors []string
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			for _, e := range validationErr.Causes {
				validationErrors = append(validationErrors, e.Message)
			}
			// Add the main error message if there are no causes
			if len(validationErrors) == 0 {
				validationErrors = append(validationErrors, validationErr.Message)
			}
		} else {
			validationErrors = append(validationErrors, err.Error())
		}
		return ValidationError{Errors: validationErrors}
	}

	return nil
}

// ValidateYAML validates YAML content against an embedded JSON schema
// schemaName should be the filename of the schema (e.g., "stack-analyzer-config.json")
// yamlContent should be the raw YAML content as bytes
func ValidateYAML(schemaName string, yamlContent []byte) error {
	// Parse YAML content
	var data interface{}
	if err := yaml.Unmarshal(yamlContent, &data); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return ValidateJSON(schemaName, data)
}

// ValidateYAMLFile validates a YAML file against an embedded JSON schema
// schemaName should be the filename of the schema (e.g., "stack-analyzer-config.json")
// filePath should be the path to the YAML file to validate
func ValidateYAMLFile(schemaName string, filePath string) error {
	// Read file content
	content, err := schemaFS.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return ValidateYAML(schemaName, content)
}

// ValidateStruct validates a Go struct against an embedded JSON schema
// schemaName should be the filename of the schema (e.g., "stack-analyzer-config.json")
// structData should be the Go struct to validate
func ValidateStruct(schemaName string, structData interface{}) error {
	// Convert struct to YAML, then to interface{} for consistent validation
	yamlContent, err := yaml.Marshal(structData)
	if err != nil {
		return fmt.Errorf("failed to marshal struct: %w", err)
	}

	return ValidateYAML(schemaName, yamlContent)
}

// ListAvailableSchemas returns a list of available schema filenames
func ListAvailableSchemas() ([]string, error) {
	entries, err := schemaFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory: %w", err)
	}

	var schemas []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			schemas = append(schemas, entry.Name())
		}
	}

	return schemas, nil
}
