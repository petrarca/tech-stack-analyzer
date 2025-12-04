package cplusplus

import (
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements C++ component detection with Conan dependency parsing
type Detector struct{}

// Name returns the detector name
func (d *Detector) Name() string {
	return "cpp"
}

// Detect scans for C++ projects with conanfile.py
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "conanfile.py" {
			continue
		}

		// Read conanfile.py
		content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		// Extract project name
		projectName := d.extractProjectName(string(content))
		if projectName == "" {
			projectName = filepath.Base(currentPath)
		}

		// Create payload with specific file path
		relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
		if relativeFilePath == "." {
			relativeFilePath = "/"
		} else {
			relativeFilePath = "/" + relativeFilePath
		}

		payload := types.NewPayloadWithPath(projectName, relativeFilePath)

		// Set tech field to cplusplus
		payload.AddPrimaryTech("cplusplus")

		// Parse dependencies using parser (handles both conanfile.py and packages*.txt)
		conanParser := parsers.NewConanParser()
		dependencies := conanParser.ExtractDependenciesFromFiles(string(content), files, currentPath, provider)

		// Extract dependency names for tech matching
		var depNames []string
		for _, dep := range dependencies {
			depNames = append(depNames, dep.Name)
		}

		// Always add conan tech
		payload.AddTech("conan", "matched file: conanfile.py")

		// Match dependencies against rules
		if len(dependencies) > 0 {
			matchedTechs := depDetector.MatchDependencies(depNames, "conan")
			for tech, reasons := range matchedTechs {
				for _, reason := range reasons {
					payload.AddTech(tech, reason)
				}
			}

			payload.Dependencies = dependencies
		}

		payloads = append(payloads, payload)
	}

	return payloads
}

// extractProjectName extracts the project name from conanfile.py
func (d *Detector) extractProjectName(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for class definition (standard Python class naming)
		if strings.HasPrefix(line, "class ") && strings.Contains(line, "Recipe") {
			// Extract class name between "class " and "Recipe"
			classLine := strings.TrimPrefix(line, "class ")
			classLine = strings.TrimSpace(classLine)

			// Find "Recipe" in the string
			recipeIndex := strings.Index(classLine, "Recipe")
			if recipeIndex > 0 {
				className := classLine[:recipeIndex]
				className = strings.TrimSpace(className)

				if className != "" {
					// Convert from CamelCase to lowercase
					return strings.ToLower(className)
				}
			}
		}
	}

	return ""
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
