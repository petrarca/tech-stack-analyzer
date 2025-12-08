package golang

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
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

	// Parse go.mod for dependencies
	// Format: \t<url> v<version>
	// Match TypeScript: const lineReg = /[\t](.+)\sv(.+)/;
	lineRegex := regexp.MustCompile(`[\t](.+)\sv(.+)`)

	var depNames []string
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		// First check if line matches the pattern
		if !lineRegex.MatchString(line) {
			continue
		}

		// Match TypeScript: const [url, version, comment, ...rest] = line.slice(1).split(' ');
		// Remove first character (tab) and split by spaces
		parts := strings.Split(line[1:], " ")
		if len(parts) < 2 {
			continue
		}

		url := parts[0]
		version := parts[1]
		var comment string
		var rest []string

		if len(parts) > 2 {
			comment = parts[2]
		}
		if len(parts) > 3 {
			rest = parts[3:]
		}

		// Match TypeScript: if (rest.length > 0 || comment) { continue; }
		// Skip false positives and '// indirect'
		if len(rest) > 0 || comment != "" {
			continue
		}

		// Store dependency
		payload.Dependencies = append(payload.Dependencies, types.Dependency{
			Type: "golang",
			Name: url + "@" + version,
		})

		depNames = append(depNames, url)
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
