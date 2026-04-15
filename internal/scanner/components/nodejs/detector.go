package nodejs

import (
	"encoding/json"
	"path/filepath"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
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

		payload := d.processPackageJSON(file, currentPath, basePath, provider, depDetector)
		if payload != nil {
			payloads = append(payloads, payload)
		}
	}

	return payloads
}

// processPackageJSON processes a single package.json file and returns a payload
func (d *Detector) processPackageJSON(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	// Read package.json
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse package.json
	var packageJSON struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		License         string            `json:"license"`
	}

	if err := json.Unmarshal(content, &packageJSON); err != nil {
		return nil
	}

	// Must have a name
	if packageJSON.Name == "" {
		return nil
	}

	// Skip build-tooling-only packages: if a package.json has zero runtime
	// dependencies (only devDependencies), it is not a Node.js application or
	// library -- it is build tooling (e.g., grunt/webpack for a Java or Perl
	// project). Creating a nodejs component for it misrepresents the tech stack.
	if len(packageJSON.Dependencies) == 0 {
		return nil
	}

	// Create payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}

	payload := types.NewPayloadWithPath(packageJSON.Name, relativeFilePath)
	payload.SetComponentType("nodejs")
	payload.AddPrimaryTech("nodejs")

	// Add Node.js package info as component property for inter-component dependencies
	nodejsInfo := make(map[string]interface{})
	nodejsInfo["package_name"] = packageJSON.Name // Package identifier (e.g., "@org/package")
	payload.Properties["nodejs"] = nodejsInfo

	// Process dependencies using priority-based extraction (lock files first)
	d.processDependenciesWithPriority(currentPath, provider, depDetector, payload)

	// Process license
	d.processLicense(&packageJSON, payload)

	return payload
}

// processDependenciesWithPriority handles dependency processing using lock file priority system
// Priority 1: package-lock.json (npm)
// Priority 2: pnpm-lock.yaml (pnpm)
// Priority 3: yarn.lock (yarn)
// Priority 4: package.json (fallback)
func (d *Detector) processDependenciesWithPriority(currentPath string, provider types.Provider, depDetector components.DependencyDetector, payload *types.Payload) {
	dependencies := d.extractDependenciesFromLockFiles(currentPath, provider)

	// Add dependencies to payload
	payload.Dependencies = append(payload.Dependencies, dependencies...)

	// Match dependencies against rules for tech detection
	d.matchAndAddTechs(dependencies, depDetector, payload)
}

// extractDependenciesFromLockFiles tries lock files in priority order and returns dependencies
func (d *Detector) extractDependenciesFromLockFiles(currentPath string, provider types.Provider) []types.Dependency {
	// Check if lock files are enabled
	if !components.UseLockFiles() {
		return d.tryPackageJSON(currentPath, provider)
	}

	// Priority 1: package-lock.json
	if deps := d.tryPackageLock(currentPath, provider); len(deps) > 0 {
		return deps
	}

	// Priority 2: pnpm-lock.yaml
	if deps := d.tryPnpmLock(currentPath, provider); len(deps) > 0 {
		return deps
	}

	// Priority 3: yarn.lock
	if deps := d.tryYarnLock(currentPath, provider); len(deps) > 0 {
		return deps
	}

	// Priority 4: package.json fallback
	return d.tryPackageJSON(currentPath, provider)
}

func (d *Detector) tryPackageLock(currentPath string, provider types.Provider) []types.Dependency {
	lockContent, err := provider.ReadFile(filepath.Join(currentPath, "package-lock.json"))
	if err != nil || len(lockContent) == 0 {
		return nil
	}

	// Read package.json to determine scope information
	packageContent, err := provider.ReadFile(filepath.Join(currentPath, "package.json"))
	var packageJSON *parsers.PackageJSON
	var packageJSONContent []byte
	if err == nil && len(packageContent) > 0 {
		parser := parsers.NewNodeJSParser()
		packageJSON, _ = parser.ParsePackageJSON(packageContent)
		packageJSONContent = packageContent // Pass raw content for peer/optional detection
	}

	return parsers.ParsePackageLockWithOptions(lockContent, packageJSON, packageJSONContent, parsers.ParsePackageLockOptions{})
}

func (d *Detector) tryPnpmLock(currentPath string, provider types.Provider) []types.Dependency {
	pnpmContent, err := provider.ReadFile(filepath.Join(currentPath, "pnpm-lock.yaml"))
	if err != nil || len(pnpmContent) == 0 {
		return nil
	}
	return parsers.ParsePnpmLock(pnpmContent)
}

func (d *Detector) tryYarnLock(currentPath string, provider types.Provider) []types.Dependency {
	yarnContent, err := provider.ReadFile(filepath.Join(currentPath, "yarn.lock"))
	if err != nil || len(yarnContent) == 0 {
		return nil
	}

	packageContent, err := provider.ReadFile(filepath.Join(currentPath, "package.json"))
	if err != nil {
		return nil
	}

	nodejsParser := parsers.NewNodeJSParser()
	pkg, err := nodejsParser.ParsePackageJSON(packageContent)
	if err != nil {
		return nil
	}

	return parsers.ParseYarnLock(yarnContent, pkg)
}

func (d *Detector) tryPackageJSON(currentPath string, provider types.Provider) []types.Dependency {
	packageContent, err := provider.ReadFile(filepath.Join(currentPath, "package.json"))
	if err != nil {
		return nil
	}

	nodejsParser := parsers.NewNodeJSParser()
	pkg, err := nodejsParser.ParsePackageJSON(packageContent)
	if err != nil {
		return nil
	}

	depNames := nodejsParser.ExtractDependencies(pkg)
	dependencies := nodejsParser.CreateDependencies(pkg, depNames)

	for i := range dependencies {
		dependencies[i].SourceFile = "package.json"
	}

	return dependencies
}

func (d *Detector) matchAndAddTechs(dependencies []types.Dependency, depDetector components.DependencyDetector, payload *types.Payload) {
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	matchedTechs := depDetector.MatchDependencies(depNames, "npm")
	for tech, reasons := range matchedTechs {
		for _, reason := range reasons {
			payload.AddTech(tech, reason)
		}
		// Check if this tech should be a primary tech
		depDetector.AddPrimaryTechIfNeeded(payload, tech)
	}
}

// processLicense handles license processing for package.json
func (d *Detector) processLicense(packageJSON *struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	License         string            `json:"license"`
}, payload *types.Payload) {
	licensenormalizer.ProcessLicenseExpression(packageJSON.License, "package.json", payload)
}

func init() {
	// Auto-register this detector
	components.Register(&Detector{})

	// Register npm package provider
	providers.Register(&providers.PackageProvider{
		DependencyType:      "npm",
		ExtractPackageNames: providers.SinglePropertyExtractor("nodejs", "package_name"),
	})
}
