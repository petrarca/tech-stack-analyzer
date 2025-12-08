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

	// Check for .NET project files (C#, VB.NET, F#)
	dotnetRegex := regexp.MustCompile(`\.(csproj|vbproj|fsproj)$`)
	for _, file := range files {
		if dotnetRegex.MatchString(file.Name) {
			payload := d.detectDotNetProject(file, currentPath, basePath, provider, depDetector)
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

	// Add NuGet package dependencies
	for _, pkg := range project.Packages {
		dep := types.Dependency{
			Type:    "nuget",
			Name:    pkg.Name,
			Example: pkg.Version,
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

func init() {
	components.Register(&Detector{})
}
