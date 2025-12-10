package golang

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "golang"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Check for go.mod (component - creates named payload)
	for _, file := range files {
		if file.Name == "go.mod" {
			payload := d.detectGoMod(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}

	// Check for main.go (component - creates named payload)
	mainGoRegex := regexp.MustCompile(`^main\.go$`)
	for _, file := range files {
		if mainGoRegex.MatchString(file.Name) {
			folderName := filepath.Base(currentPath)
			relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
			if relativeFilePath == "." {
				relativeFilePath = "/"
			} else {
				relativeFilePath = "/" + relativeFilePath
			}
			payload := types.NewPayloadWithPath(folderName, relativeFilePath)
			payload.AddPrimaryTech("golang")
			results = append(results, payload)
			break
		}
	}

	return results
}

func (d *Detector) detectGoMod(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Create named payload with folder name as project name
	folderName := filepath.Base(currentPath)
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(folderName, relativeFilePath)

	// Set tech field to golang
	payload.AddPrimaryTech("golang")

	// Parse go.mod for dependencies using parser
	goParser := parsers.NewGolangParser()
	dependencies := goParser.ParseGoMod(string(content))

	// Add dependencies to payload
	for _, dep := range dependencies {
		payload.AddDependency(dep)
	}

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		// Remove version suffix for tech matching
		name := strings.Split(dep.Name, "@")[0]
		depNames = append(depNames, name)
	}

	// Match dependencies against rules
	if len(depNames) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "golang")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}
	}

	return payload
}

func init() {
	components.Register(&Detector{})
}
