package ruby

import (
	"path/filepath"
	"regexp"
	"strings"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "ruby"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for Gemfile and .gemspec files
	var gemfileExists, gemfileLockExists bool
	var gemspecFile *types.File
	for i, file := range files {
		switch {
		case file.Name == "Gemfile":
			gemfileExists = true
		case file.Name == "Gemfile.lock":
			gemfileLockExists = true
		case strings.HasSuffix(file.Name, ".gemspec"):
			gemspecFile = &files[i]
		}
	}

	// Process Gemfile if it exists
	if gemfileExists {
		for _, file := range files {
			if file.Name == "Gemfile" {
				payload := d.detectGemfile(file, currentPath, basePath, provider, depDetector, gemfileLockExists)
				if payload != nil {
					// If a .gemspec file is present, extract license from it
					if gemspecFile != nil {
						d.addGemspecLicense(payload, *gemspecFile, currentPath, provider)
					}
					results = append(results, payload)
				}
			}
		}
	}

	return results
}

func (d *Detector) detectGemfile(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector, gemfileLockExists bool) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Extract project name ( fallback to folder name)
	projectName := d.extractProjectName(string(content))
	if projectName == "" {
		projectName = filepath.Base(currentPath)
	}

	// Create named payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(projectName, relativeFilePath)
	payload.SetComponentType("ruby")

	// Set tech field to ruby
	payload.AddPrimaryTech("ruby")

	// Store gem name in properties for inter-component dependency tracking
	payload.SetComponentProperty("ruby", "gem_name", projectName)

	var dependencies []types.Dependency

	// Prefer Gemfile.lock for exact versions if available
	if gemfileLockExists {
		lockContent, err := provider.ReadFile(filepath.Join(currentPath, "Gemfile.lock"))
		if err == nil {
			lockParser := parsers.NewGemfileLockParser()
			dependencies = lockParser.ParseGemfileLock(string(lockContent))
		}
	}

	// Fallback to Gemfile if no lockfile or lockfile parsing failed
	if len(dependencies) == 0 {
		rubyParser := parsers.NewRubyParser()
		dependencies = rubyParser.ParseGemfile(string(content))
	}

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add bundler tech
	payload.AddTech("bundler", "matched file: Gemfile")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "ruby")
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

// extractProjectName attempts to extract a project name from Gemfile
// Gemfiles typically don't have project names, so this returns empty string
// to trigger folder name fallback
func (d *Detector) extractProjectName(content string) string {
	// Gemfiles don't have a standard project name field like pyproject.toml
	// We could try to parse comments or specific patterns, but for now
	// return empty to use folder name as project name
	return ""
}

// addGemspecLicense extracts license information from a .gemspec file and adds it
// to the payload. Gemspec files use patterns like:
//
//	spec.license  = "MIT"
//	spec.licenses = ["MIT", "GPL-2.0"]
//	s.license     = 'Apache-2.0'
func (d *Detector) addGemspecLicense(payload *types.Payload, gemspecFile types.File, currentPath string, provider types.Provider) {
	content, err := provider.ReadFile(filepath.Join(currentPath, gemspecFile.Name))
	if err != nil {
		return
	}

	licenses := extractGemspecLicenses(string(content))
	for _, lic := range licenses {
		licensenormalizer.ProcessLicenseExpression(lic, gemspecFile.Name, payload)
	}
}

// extractGemspecLicenses extracts license strings from .gemspec content.
// Handles both single license (spec.license = "MIT") and
// multiple licenses (spec.licenses = ["MIT", "GPL-2.0"]).
func extractGemspecLicenses(content string) []string {
	var licenses []string

	// Match spec.licenses = ["MIT", "GPL-2.0"] (array form)
	arrayRe := regexp.MustCompile(`(?m)\.\s*licenses\s*=\s*\[([^\]]+)\]`)
	if match := arrayRe.FindStringSubmatch(content); len(match) > 1 {
		// Extract individual license strings from the array
		itemRe := regexp.MustCompile(`['"]([^'"]+)['"]`)
		items := itemRe.FindAllStringSubmatch(match[1], -1)
		for _, item := range items {
			if len(item) > 1 {
				licenses = append(licenses, strings.TrimSpace(item[1]))
			}
		}
		if len(licenses) > 0 {
			return licenses
		}
	}

	// Match spec.license = "MIT" (single form)
	singleRe := regexp.MustCompile(`(?m)\.\s*license\s*=\s*['"]([^'"]+)['"]`)
	if match := singleRe.FindStringSubmatch(content); len(match) > 1 {
		licenses = append(licenses, strings.TrimSpace(match[1]))
	}

	return licenses
}

func init() {
	components.Register(&Detector{})

	// Register rubygems package provider
	providers.Register(&providers.PackageProvider{
		DependencyType:      "rubygems",
		ExtractPackageNames: providers.SinglePropertyExtractor("ruby", "gem_name"),
	})
}
