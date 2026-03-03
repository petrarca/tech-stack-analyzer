package deno

import (
	"encoding/json"
	"path/filepath"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "deno"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Scan for relevant files
	var denoLockFile *types.File
	var denoConfigFile *types.File
	for i, file := range files {
		switch file.Name {
		case "deno.lock":
			denoLockFile = &files[i]
		case "deno.json", "deno.jsonc":
			denoConfigFile = &files[i]
		}
	}

	// Process deno.lock for dependencies
	if denoLockFile != nil {
		payload := d.detectDenoLock(*denoLockFile, currentPath, basePath, provider, depDetector)
		if payload != nil {
			// If deno.json is present, extract license from it
			if denoConfigFile != nil {
				d.addDenoConfigLicense(payload, *denoConfigFile, currentPath, provider)
			}
			results = append(results, payload)
		}
	} else if denoConfigFile != nil {
		// No deno.lock but deno.json exists - create a payload from config alone
		payload := d.detectDenoConfig(*denoConfigFile, currentPath, basePath, provider)
		if payload != nil {
			results = append(results, payload)
		}
	}

	return results
}

func (d *Detector) detectDenoLock(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse deno.lock using parser
	denoParser := parsers.NewDenoParser()
	version, dependencies := denoParser.ParseDenoLock(string(content))

	// Must have a version to be valid
	if version == "" {
		return nil
	}

	// Create virtual payload (deno.lock doesn't have project names)
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath("virtual", relativeFilePath)

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "deno")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}

		payload.Dependencies = dependencies
	}

	return payload
}

// detectDenoConfig creates a payload from deno.json/deno.jsonc when no deno.lock is present
func (d *Detector) detectDenoConfig(file types.File, currentPath, basePath string, provider types.Provider) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse deno.json to extract name and license
	var denoConfig struct {
		Name    string `json:"name"`
		License string `json:"license"`
	}
	if err := json.Unmarshal(content, &denoConfig); err != nil {
		return nil
	}

	// Must have a name to create a non-virtual payload, or a license to be useful
	if denoConfig.Name == "" && denoConfig.License == "" {
		return nil
	}

	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	name := denoConfig.Name
	if name == "" {
		name = "virtual"
	}
	payload := types.NewPayloadWithPath(name, relativeFilePath)
	payload.AddPrimaryTech("deno")

	// Process license
	if denoConfig.License != "" {
		licensenormalizer.ProcessLicenseExpression(denoConfig.License, file.Name, payload)
	}

	return payload
}

// addDenoConfigLicense extracts license from deno.json/deno.jsonc and adds it to an existing payload
func (d *Detector) addDenoConfigLicense(payload *types.Payload, configFile types.File, currentPath string, provider types.Provider) {
	content, err := provider.ReadFile(filepath.Join(currentPath, configFile.Name))
	if err != nil {
		return
	}

	var denoConfig struct {
		License string `json:"license"`
	}
	if err := json.Unmarshal(content, &denoConfig); err != nil {
		return
	}

	if denoConfig.License != "" {
		licensenormalizer.ProcessLicenseExpression(denoConfig.License, configFile.Name, payload)
	}
}

func init() {
	components.Register(&Detector{})
}
