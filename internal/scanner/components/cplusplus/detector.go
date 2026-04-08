package cplusplus

import (
	"path/filepath"
	"strings"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Detector implements C++ component detection via conanfile.py and .vcxproj
type Detector struct{}

func (d *Detector) Name() string {
	return "cpp"
}

// Detect scans for C++ projects with conanfile.py or .vcxproj files
func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload
	payloads = append(payloads, d.detectConanProjects(files, currentPath, basePath, provider, depDetector)...)
	payloads = append(payloads, d.detectVcxprojProjects(files, currentPath, basePath, provider, depDetector)...)
	return payloads
}

// --- Conan detection ---

func (d *Detector) detectConanProjects(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if file.Name != "conanfile.py" {
			continue
		}

		content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		projectName := d.extractConanProjectName(string(content))
		if projectName == "" {
			projectName = filepath.Base(currentPath)
		}

		payload := types.NewPayloadWithPath(projectName, relativeFilePath(file.Name, currentPath, basePath))
		payload.SetComponentType("cplusplus")
		payload.AddPrimaryTech("cplusplus")

		conanParser := parsers.NewConanParser()
		dependencies := conanParser.ExtractDependenciesFromFiles(string(content), files, currentPath, provider)

		var depNames []string
		for _, dep := range dependencies {
			depNames = append(depNames, dep.Name)
		}

		payload.AddTech("conan", "matched file: conanfile.py")

		if len(dependencies) > 0 {
			matchedTechs := depDetector.MatchDependencies(depNames, "conan")
			for tech, reasons := range matchedTechs {
				for _, reason := range reasons {
					payload.AddTech(tech, reason)
				}
				depDetector.AddPrimaryTechIfNeeded(payload, tech)
			}
			payload.Dependencies = dependencies
		}

		if license := d.extractConanLicense(string(content)); license != "" {
			licensenormalizer.ProcessLicenseExpression(license, "conanfile.py", payload)
		}

		payloads = append(payloads, payload)
	}

	return payloads
}

// --- MSBuild .vcxproj detection ---

func (d *Detector) detectVcxprojProjects(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var payloads []*types.Payload

	for _, file := range files {
		if !strings.HasSuffix(file.Name, ".vcxproj") {
			continue
		}

		content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
		if err != nil {
			continue
		}

		vcxParser := parsers.NewVcxprojParser()
		project := vcxParser.ParseVcxproj(string(content), filepath.Join(currentPath, file.Name))
		if project.Name == "" {
			continue
		}

		payload := types.NewPayloadWithPath(project.Name, relativeFilePath(file.Name, currentPath, basePath))
		payload.SetComponentType("msbuild-cpp")
		payload.AddPrimaryTech("cplusplus")
		payload.AddTech("cplusplus", "matched file: "+file.Name)

		if project.UseOfMfc != "" {
			payload.AddTech("mfc", "UseOfMfc: "+project.UseOfMfc)
		}

		d.setMSBuildCppProperties(payload, project)
		d.addVcxprojDependencies(payload, project)

		payloads = append(payloads, payload)
	}

	return payloads
}

// setMSBuildCppProperties populates the msbuild_cpp properties map on the payload.
func (d *Detector) setMSBuildCppProperties(payload *types.Payload, project parsers.VcxprojProject) {
	props := map[string]interface{}{
		"project_name": project.Name,
	}
	if project.PlatformToolset != "" {
		props["platform_toolset"] = project.PlatformToolset
		if vsVersion := parsers.PlatformToolsetToVSVersion(project.PlatformToolset); vsVersion != "" {
			props["vs_version"] = vsVersion
		}
	}
	if project.ConfigurationType != "" {
		props["configuration_type"] = project.ConfigurationType
	}
	if project.UseOfMfc != "" {
		props["use_of_mfc"] = project.UseOfMfc
	}
	if project.CLRSupport != "" {
		props["clr_support"] = project.CLRSupport
	}
	if project.CharacterSet != "" {
		props["character_set"] = project.CharacterSet
	}
	if project.WindowsTargetPlatformVersion != "" {
		props["windows_sdk"] = project.WindowsTargetPlatformVersion
	}
	payload.Properties["msbuild_cpp"] = props
}

// addVcxprojDependencies adds linked libraries and project references as dependencies.
func (d *Detector) addVcxprojDependencies(payload *types.Payload, project parsers.VcxprojProject) {
	for _, lib := range project.AdditionalDependencies {
		payload.AddDependency(types.Dependency{
			Type:     "native-lib",
			Name:     lib,
			Scope:    "prod",
			Direct:   true,
			Metadata: map[string]interface{}{"source": parsers.MetadataSourceVcxproj},
		})
	}

	for _, projRef := range project.ProjectReferences {
		normalizedPath := strings.ReplaceAll(projRef, "\\", "/")
		projName := filepath.Base(normalizedPath)
		projName = strings.TrimSuffix(projName, filepath.Ext(projName))

		payload.AddDependency(types.Dependency{
			Type:     "vcxproj-ref",
			Name:     projName,
			Scope:    "prod",
			Direct:   true,
			Metadata: map[string]interface{}{"path": normalizedPath},
		})
	}
}

// --- Helpers ---

// relativeFilePath calculates the manifest-relative path for use in NewPayloadWithPath.
// Returns the filename alone when the file is at the base path root.
func relativeFilePath(fileName, currentPath, basePath string) string {
	rel, _ := filepath.Rel(basePath, filepath.Join(currentPath, fileName))
	if rel == "." {
		return fileName
	}
	return rel
}

func (d *Detector) extractConanProjectName(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "class ") && strings.Contains(line, "Recipe") {
			classLine := strings.TrimPrefix(line, "class ")
			recipeIdx := strings.Index(classLine, "Recipe")
			if recipeIdx > 0 {
				if name := strings.TrimSpace(classLine[:recipeIdx]); name != "" {
					return strings.ToLower(name)
				}
			}
		}
	}
	return ""
}

func (d *Detector) extractConanLicense(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "license") {
			continue
		}
		// Match: license = "MIT" or license = 'MIT'
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			continue
		}
		val := strings.TrimSpace(trimmed[eqIdx+1:])
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[0] == val[len(val)-1] {
			return val[1 : len(val)-1]
		}
	}
	return ""
}

func init() {
	components.Register(&Detector{})

	// Register vcxproj-ref provider for inter-component dependency resolution.
	// Matches ProjectReference dependencies against msbuild_cpp project_name properties.
	providers.Register(&providers.PackageProvider{
		DependencyType:      "vcxproj-ref",
		ExtractPackageNames: providers.SinglePropertyExtractor("msbuild_cpp", "project_name"),
		MatchFunc: func(componentPkgName, dependencyName string) bool {
			return strings.EqualFold(componentPkgName, dependencyName)
		},
	})
}
