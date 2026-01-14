package dotnet

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
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

	// Check if there are any .csproj/.vbproj/.fsproj files in this directory
	dotnetRegex := regexp.MustCompile(`\.(csproj|vbproj|fsproj)$`)
	hasDotNetProject := false
	for _, file := range files {
		if dotnetRegex.MatchString(file.Name) {
			hasDotNetProject = true
			break
		}
	}

	// Detect .NET project files
	projectPayloads := d.detectProjectFiles(files, currentPath, basePath, provider, depDetector, centralVersions)
	results = append(results, projectPayloads...)

	// Only detect standalone packages.config if there's no .csproj file in this directory
	// (if .csproj exists, it will handle packages.config itself)
	if !hasDotNetProject {
		legacyPayloads := d.detectPackagesConfigFiles(files, currentPath, basePath, provider, depDetector)
		results = append(results, legacyPayloads...)
	}

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
			payload := d.detectDotNetProject(file, files, currentPath, basePath, provider, depDetector)
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

func (d *Detector) detectDotNetProject(file types.File, files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	project := d.parseDotNetProject(file, files, currentPath, provider)
	if project == nil {
		return nil
	}

	payload := d.createDotNetPayload(project, file, currentPath, basePath)
	d.addProjectReferences(payload, project.ProjectReferences)
	d.addNuGetDependencies(payload, project.Packages, depDetector)

	return payload
}

func (d *Detector) parseDotNetProject(file types.File, files []types.File, currentPath string, provider types.Provider) *parsers.DotNetProject {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	dotnetParser := parsers.NewDotNetParser()
	project := dotnetParser.ParseCsproj(string(content), filepath.Join(currentPath, file.Name))
	if project.Name == "" {
		return nil
	}

	// Merge packages.config if .csproj has no packages (legacy .NET Framework)
	if len(project.Packages) == 0 {
		d.mergeLegacyPackages(&project, files, currentPath, provider, dotnetParser)
	}

	return &project
}

func (d *Detector) mergeLegacyPackages(project *parsers.DotNetProject, files []types.File, currentPath string, provider types.Provider, parser *parsers.DotNetParser) {
	for _, f := range files {
		if f.Name == "packages.config" {
			if pkgContent, err := provider.ReadFile(filepath.Join(currentPath, f.Name)); err == nil {
				legacyDeps := parser.ParsePackagesConfig(string(pkgContent))
				for _, dep := range legacyDeps {
					project.Packages = append(project.Packages, parsers.DotNetPackage{
						Name:     dep.Name,
						Version:  dep.Version,
						Scope:    dep.Scope,
						Metadata: dep.Metadata,
					})
				}
			}
			break
		}
	}
}

func (d *Detector) createDotNetPayload(project *parsers.DotNetProject, file types.File, currentPath, basePath string) *types.Payload {
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = file.Name
	}

	payload := types.NewPayloadWithPath(project.Name, relativeFilePath)
	payload.SetComponentType("dotnet")

	languageTech := d.getLanguageTech(file.Name)
	payload.AddPrimaryTech(languageTech)

	if project.Framework != "" {
		payload.AddTech(languageTech, "framework: "+project.Framework)
	} else {
		payload.AddTech(languageTech, "matched file: "+file.Name)
	}

	d.setDotNetProperties(payload, project)
	return payload
}

func (d *Detector) getLanguageTech(fileName string) string {
	switch {
	case strings.HasSuffix(fileName, ".csproj"):
		return "dotnet"
	case strings.HasSuffix(fileName, ".vbproj"):
		return "vbnet"
	case strings.HasSuffix(fileName, ".fsproj"):
		return "fsharp"
	default:
		return "dotnet"
	}
}

func (d *Detector) setDotNetProperties(payload *types.Payload, project *parsers.DotNetProject) {
	dotnetInfo := map[string]interface{}{
		"assembly_name": project.Name,
		"package_id":    project.PackageId,
	}
	if dotnetInfo["package_id"] == "" {
		dotnetInfo["package_id"] = project.Name
	}
	if project.Framework != "" {
		dotnetInfo["framework"] = project.Framework
	}
	payload.Properties["dotnet"] = dotnetInfo
}

func (d *Detector) addProjectReferences(payload *types.Payload, projectReferences []string) {
	for _, projRef := range projectReferences {
		normalizedPath := strings.ReplaceAll(projRef, "\\", "/")
		projName := filepath.Base(normalizedPath)
		projName = strings.TrimSuffix(projName, filepath.Ext(projName))

		payload.AddDependency(types.Dependency{
			Type:     "dotnet-ref",
			Name:     projName,
			Version:  "",
			Scope:    "prod",
			Direct:   true,
			Metadata: map[string]interface{}{"path": projRef},
		})
	}
}

func (d *Detector) addNuGetDependencies(payload *types.Payload, packages []parsers.DotNetPackage, depDetector components.DependencyDetector) {
	for _, pkg := range packages {
		payload.AddDependency(types.Dependency{
			Type:     "nuget",
			Name:     pkg.Name,
			Version:  pkg.Version,
			Scope:    pkg.Scope,
			Direct:   true,
			Metadata: pkg.Metadata,
		})

		matchedTechs := depDetector.MatchDependencies([]string{pkg.Name}, "nuget")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}
	}
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
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
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

	// Register .NET package provider for component dependency resolution
	providers.Register(&providers.PackageProvider{
		DependencyType:      "nuget",
		ExtractPackageNames: providers.SinglePropertyExtractor("dotnet", "package_id"),
		MatchFunc: func(componentPkgName, dependencyName string) bool {
			// NuGet package names are case-insensitive
			return strings.EqualFold(componentPkgName, dependencyName)
		},
	})

	// Register dotnet-ref provider for inter-component dependencies
	providers.Register(&providers.PackageProvider{
		DependencyType:      "dotnet-ref",
		ExtractPackageNames: providers.SinglePropertyExtractor("dotnet", "assembly_name"),
		MatchFunc: func(componentName, dependencyName string) bool {
			// Match by assembly name (case-insensitive)
			return strings.EqualFold(componentName, dependencyName)
		},
	})
}
