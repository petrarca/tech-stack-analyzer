package dotnet

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
	return "dotnet"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	// Detect central package management
	centralVersions := d.detectCentralPackageVersions(files, currentPath, provider)

	// Detect .NET project files
	projectPayloads := d.detectProjectFiles(files, currentPath, basePath, provider, depDetector, centralVersions)
	results = append(results, projectPayloads...)

	// Detect packages.config files
	legacyPayloads := d.detectPackagesConfigFiles(files, currentPath, basePath, provider, depDetector)
	results = append(results, legacyPayloads...)

	return results
}

// detectCentralPackageVersions checks for Directory.Packages.props and returns central package versions
func (d *Detector) detectCentralPackageVersions(files []types.File, currentPath string, provider types.Provider) map[string]string {
	for _, file := range files {
		if file.Name == "Directory.Packages.props" {
			content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
			if err == nil {
				dotnetParser := parsers.NewDotNetParser()
				return dotnetParser.ParseDirectoryPackagesProps(string(content))
			}
			break
		}
	}
	return make(map[string]string)
}

// detectProjectFiles handles .csproj, .vbproj, .fsproj files
func (d *Detector) detectProjectFiles(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector, centralVersions map[string]string) []*types.Payload {
	var results []*types.Payload
	dotnetRegex := regexp.MustCompile(`\.(csproj|vbproj|fsproj)$`)

	for _, file := range files {
		if dotnetRegex.MatchString(file.Name) {
			payload := d.detectDotNetProject(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				// Apply central package versions if available
				if len(centralVersions) > 0 {
					d.applyCentralPackageVersions(payload, centralVersions)
				}
				results = append(results, payload)
			}
		}
	}
	return results
}

// detectPackagesConfigFiles handles packages.config files
func (d *Detector) detectPackagesConfigFiles(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload

	for _, file := range files {
		if file.Name == "packages.config" {
			payload := d.detectPackagesConfig(file, currentPath, basePath, provider, depDetector)
			if payload != nil {
				results = append(results, payload)
			}
		}
	}
	return results
}

func (d *Detector) detectDotNetProject(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse .NET project file using parser
	dotnetParser := parsers.NewDotNetParser()
	project := dotnetParser.ParseCsproj(string(content), filepath.Join(currentPath, file.Name))

	if project.Name == "" {
		return nil
	}

	// Create component payload
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = file.Name
	}

	payload := types.NewPayloadWithPath(project.Name, relativeFilePath)

	// Determine language based on file extension
	var languageTech string
	switch {
	case strings.HasSuffix(file.Name, ".csproj"):
		languageTech = "dotnet"
	case strings.HasSuffix(file.Name, ".vbproj"):
		languageTech = "vbnet"
	case strings.HasSuffix(file.Name, ".fsproj"):
		languageTech = "fsharp"
	default:
		languageTech = "dotnet" // fallback
	}

	// Set primary tech based on language
	payload.AddPrimaryTech(languageTech)

	// Add framework info to techs
	if project.Framework != "" {
		payload.AddTech(languageTech, "framework: "+project.Framework)
	} else {
		payload.AddTech(languageTech, "matched file: "+file.Name)
	}

	// Add framework version as component property (following Maven/Go pattern)
	if project.Framework != "" {
		dotnetInfo := make(map[string]interface{})
		dotnetInfo["framework"] = project.Framework
		payload.Properties["dotnet"] = dotnetInfo
	}

	// Add NuGet package dependencies
	for _, pkg := range project.Packages {
		dep := types.Dependency{
			Type:     "nuget",
			Name:     pkg.Name,
			Version:  pkg.Version,
			Scope:    pkg.Scope,    // prod, dev, build (aligned with deps.dev)
			Direct:   true,         // All NuGet deps in .csproj are direct
			Metadata: pkg.Metadata, // Framework, source, conditions
		}
		payload.AddDependency(dep)

		// Match package name against dependency rules
		matchedTechs := depDetector.MatchDependencies([]string{pkg.Name}, "nuget")

		// Add matched techs to parent payload (identical to Java detector behavior)
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}
	}

	return payload
}

func (d *Detector) detectPackagesConfig(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Parse packages.config using parser
	dotnetParser := parsers.NewDotNetParser()
	dependencies := dotnetParser.ParsePackagesConfig(string(content))

	if len(dependencies) == 0 {
		return nil
	}

	// Create component payload
	folderName := filepath.Base(currentPath)
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = file.Name
	}

	payload := types.NewPayloadWithPath(folderName, relativeFilePath)

	// Set primary tech to dotnet
	payload.AddPrimaryTech("dotnet")
	payload.AddTech("dotnet", "matched file: "+file.Name)

	// Add dependencies to payload
	for _, dep := range dependencies {
		payload.AddDependency(dep)
	}

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Match dependencies against rules
	if len(depNames) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "nuget")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
		}
	}

	return payload
}

// applyCentralPackageVersions updates dependencies with versions from Directory.Packages.props
func (d *Detector) applyCentralPackageVersions(payload *types.Payload, centralVersions map[string]string) {
	for i := range payload.Dependencies {
		dep := &payload.Dependencies[i]
		// If dependency has no version, try to get it from central package management
		if dep.Version == "" {
			if version, exists := centralVersions[dep.Name]; exists {
				dep.Version = version
				// Add metadata to indicate version came from central management
				if dep.Metadata == nil {
					dep.Metadata = make(map[string]interface{})
				}
				dep.Metadata["central_package_management"] = true
			}
		}
	}
}

func init() {
	components.Register(&Detector{})
}
