package nodejs

import (
	"encoding/json"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements Node.js component detection
type Detector struct{}

// Name returns the detector name
func (d *Detector) Name() string {
	return "nodejs"
}

// Detect scans for Node.js projects (package.json)
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "package.json" {
			continue
		}

		// Read package.json
		content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		// Parse package.json
		var packageJSON struct {
			Name            string            `json:"name"`
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
			License         string            `json:"license"`
		}

		if err := json.Unmarshal(content, &packageJSON); err != nil {
			continue
		}

		// Must have a name
		if packageJSON.Name == "" {
			continue
		}

		// Create payload with specific file path (like TypeScript: folderPath: file.fp)
		relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
		if relativeFilePath == "." {
			relativeFilePath = "/"
		} else {
			relativeFilePath = "/" + relativeFilePath
		}

		payload := types.NewPayloadWithPath(packageJSON.Name, relativeFilePath)

		// Set tech field to nodejs
		payload.AddPrimaryTech("nodejs")

		// Merge dependencies
		allDeps := make(map[string]string)
		for name, version := range packageJSON.Dependencies {
			allDeps[name] = version
		}
		for name, version := range packageJSON.DevDependencies {
			allDeps[name] = version
		}

		// Match dependencies against rules
		var depNames []string
		for name := range allDeps {
			depNames = append(depNames, name)
		}

		matchedTechs := depDetector.MatchDependencies(depNames, "npm")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}

		// Convert to dependency array
		for name, version := range allDeps {
			payload.Dependencies = append(payload.Dependencies, types.Dependency{
				Type:    "npm",
				Name:    name,
				Example: version,
			})
		}

		// Add license if present
		if packageJSON.License != "" {
			payload.Licenses = append(payload.Licenses, packageJSON.License)
		}

		// Add to payloads array instead of returning immediately
		payloads = append(payloads, payload)
	}

	return payloads
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})
}
