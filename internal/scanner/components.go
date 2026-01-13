package scanner

import (
	"fmt"
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ComponentDetector handles component-based detection (NodeJS, Python)
type ComponentDetector struct {
	provider     types.Provider
	depDetector  *DependencyDetector
	rules        []types.Rule // Add rules to create implicit components
	pythonParser *parsers.PythonParser
	nodejsParser *parsers.NodeJSParser
}

// NewComponentDetector creates a new component detector
func NewComponentDetector(depDetector *DependencyDetector, provider types.Provider, rules []types.Rule) *ComponentDetector {
	return &ComponentDetector{
		provider:     provider,
		depDetector:  depDetector,
		rules:        rules,
		pythonParser: parsers.NewPythonParser(),
		nodejsParser: parsers.NewNodeJSParser(),
	}
}

// DetectNodeComponent detects Node.js projects from package.json
func (d *ComponentDetector) DetectNodeComponent(files []types.File, currentPath string, basePath string) []*types.Payload {
	var payloads []*types.Payload

	// Process package.json files
	payloads = append(payloads, d.detectPackageJSON(files, currentPath, basePath)...)

	return payloads
}

// detectPackageJSON processes package.json files
func (d *ComponentDetector) detectPackageJSON(files []types.File, currentPath string, basePath string) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "package.json" {
			continue
		}

		// Read package.json
		content, err := d.provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		// Parse package.json using NodeJSParser
		packageJSON, err := d.nodejsParser.ParsePackageJSON(content)
		if err != nil {
			continue
		}

		// Extract all dependency names
		dependencies := d.nodejsParser.ExtractDependencies(packageJSON)

		// Match dependencies against rules
		matchedTechs := d.depDetector.MatchDependencies(dependencies, "npm")
		if len(matchedTechs) == 0 {
			continue
		}

		// Create payload with package name and relative path
		payloadName := d.nodejsParser.GetPackageName(packageJSON)
		relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
		payload := types.NewPayloadWithPath(payloadName, relativeFilePath)

		// Add detected technologies and create implicit components for them
		for tech := range matchedTechs {
			payload.AddTech(tech, "npm dependency matched")
			// Check if this tech should be a primary tech
			d.depDetector.AddPrimaryTechIfNeeded(payload, tech)
			d.createImplicitComponentForTech(payload, tech, currentPath)
		}

		// Add dependencies to payload using NodeJSParser
		deps := d.nodejsParser.CreateDependencies(packageJSON, dependencies)
		for _, dep := range deps {
			payload.AddDependency(dep)
		}

		payloads = append(payloads, payload)
	}

	return payloads
}

// DetectPythonComponent detects Python components (requirements.txt, pyproject.toml)
// Matches TypeScript: returns after first match, processes pyproject.toml first
func (d *ComponentDetector) DetectPythonComponent(files []types.File, currentPath string, basePath string) []*types.Payload {
	var payloads []*types.Payload

	// Process pyproject.toml files
	payloads = append(payloads, d.detectPyprojectToml(files, currentPath, basePath)...)

	// Process requirements.txt files
	payloads = append(payloads, d.detectRequirementsTxt(files, currentPath, basePath)...)

	return payloads
}

// detectPyprojectToml processes pyproject.toml files
func (d *ComponentDetector) detectPyprojectToml(files []types.File, currentPath string, basePath string) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "pyproject.toml" {
			continue
		}

		content, err := d.provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		projectName := d.pythonParser.ExtractProjectName(string(content))
		if projectName == "" {
			continue
		}

		relativeFilePath := d.getRelativeFilePath(basePath, currentPath, file.Name)
		payload := types.NewPayloadWithPath(projectName, relativeFilePath)

		dependencies := d.pythonParser.ParsePyprojectTOML(string(content))
		d.processPythonDependencies(payload, dependencies, currentPath)
		d.pythonParser.DetectLicense(string(content), payload)

		payloads = append(payloads, payload)
	}

	return payloads
}

// detectRequirementsTxt processes requirements.txt files
func (d *ComponentDetector) detectRequirementsTxt(files []types.File, currentPath string, basePath string) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "requirements.txt" {
			continue
		}

		content, err := d.provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		relativePath := d.getRelativeDirPath(basePath, currentPath)
		payload := types.NewPayloadWithPath("virtual", relativePath)

		dependencies := d.pythonParser.ParseRequirementsTxt(string(content))
		d.processPythonDependencies(payload, dependencies, currentPath)

		payloads = append(payloads, payload)
	}

	return payloads
}

// processPythonDependencies handles dependency matching and tech detection
func (d *ComponentDetector) processPythonDependencies(payload *types.Payload, dependencies []types.Dependency, currentPath string) {
	if len(dependencies) == 0 {
		return
	}

	depNames := make([]string, len(dependencies))
	for i, dep := range dependencies {
		depNames[i] = dep.Name
	}

	matchedTechs := d.depDetector.MatchDependencies(depNames, "python")
	for tech := range matchedTechs {
		payload.AddTech(tech, "python dependency matched")
		// Check if this tech should be a primary tech
		d.depDetector.AddPrimaryTechIfNeeded(payload, tech)
		d.createImplicitComponentForTech(payload, tech, currentPath)
	}

	payload.Dependencies = dependencies
}

// getRelativeFilePath returns relative file path for payload
func (d *ComponentDetector) getRelativeFilePath(basePath, currentPath, fileName string) string {
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, fileName))
	if relativeFilePath == "." {
		return "/"
	}
	return "/" + relativeFilePath
}

// getRelativeDirPath returns relative directory path for payload
func (d *ComponentDetector) getRelativeDirPath(basePath, currentPath string) string {
	relativePath, _ := filepath.Rel(basePath, currentPath)
	if relativePath == "." {
		return "/"
	}
	return "/" + relativePath
}

// createImplicitComponentForTech creates a child component for a technology (like TypeScript's findImplicitComponent)
func (d *ComponentDetector) createImplicitComponentForTech(payload *types.Payload, tech string, currentPath string) {
	// Find the rule for this tech
	for _, rule := range d.rules {
		if rule.Tech == tech {
			// Check if this rule should create a component (uses _types.yaml)
			if !ShouldCreateComponent(rule) {
				return
			}

			// Create component payload
			component := types.NewPayload(tech, []string{currentPath})
			component.AddTech(tech, fmt.Sprintf("matched: /^%s$/", tech))
			component.AddReason(fmt.Sprintf("%s matched: /^%s$/", tech, tech))

			// Add the component as a child
			payload.AddChild(component)

			// Add edges if configured to do so
			if ShouldCreateEdges(rule) {
				payload.AddEdges(component)
			}
			return
		}
	}
}
