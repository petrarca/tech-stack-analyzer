package java

import (
	"path/filepath"
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

type Detector struct{}

func (d *Detector) Name() string {
	return "java"
}

func (d *Detector) Detect(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) []*types.Payload {
	var results []*types.Payload
	var payload *types.Payload

	// Check for Maven first
	payload = d.detectMaven(files, currentPath, basePath, provider, depDetector)

	// If no Maven found, check for Gradle
	if payload == nil {
		payload = d.detectGradleOnly(files, currentPath, basePath, provider, depDetector)
	} else {
		// Maven found - also add Gradle info if present
		d.addGradleInfoToMaven(payload, files, currentPath, basePath, provider, depDetector)
	}

	if payload != nil {
		results = append(results, payload)
	}

	return results
}

// detectMaven looks for pom.xml and creates a Maven payload
func (d *Detector) detectMaven(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	var payload *types.Payload
	var dependencyListFile *types.File

	// Look for pom.xml and dependency-list.txt
	for i := range files {
		if files[i].Name == "pom.xml" {
			payload = d.detectPomXML(files[i], currentPath, basePath, provider, depDetector)
		}
		if files[i].Name == "dependency-list.txt" {
			dependencyListFile = &files[i]
		}
	}

	// If we have dependency-list.txt, use it for resolved versions
	if payload != nil && dependencyListFile != nil {
		d.mergeDependencyList(payload, *dependencyListFile, currentPath, provider)
	}

	return payload
}

// detectGradleOnly looks for Gradle files when no Maven was found
func (d *Detector) detectGradleOnly(files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	gradleRegex := regexp.MustCompile(`^build\.gradle(\.kts)?$`)
	for _, file := range files {
		if gradleRegex.MatchString(file.Name) {
			return d.detectGradle(file, currentPath, basePath, provider, depDetector)
		}
	}
	return nil
}

// addGradleInfoToMaven adds Gradle file paths and dependencies to an existing Maven payload
func (d *Detector) addGradleInfoToMaven(payload *types.Payload, files []types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) {
	gradleRegex := regexp.MustCompile(`^build\.gradle(\.kts)?$`)

	for _, file := range files {
		if gradleRegex.MatchString(file.Name) {
			relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
			if relativeFilePath != "." {
				relativeFilePath = "/" + relativeFilePath
				payload.AddPath(relativeFilePath)
			}
			// Add gradle tech
			payload.AddTech("gradle", "matched file: "+file.Name)

			// Parse and merge gradle dependencies
			if gradlePayload := d.detectGradle(file, currentPath, basePath, provider, depDetector); gradlePayload != nil {
				for _, dep := range gradlePayload.Dependencies {
					payload.AddDependency(dep)
				}
				// Merge gradle properties if they exist
				if gradleProps, exists := gradlePayload.Properties["gradle"]; exists {
					if payload.Properties == nil {
						payload.Properties = make(map[string]interface{})
					}
					payload.Properties["gradle"] = gradleProps
				}
			}
		}
	}
}

// mergeDependencyList merges dependency list data into the payload
func (d *Detector) mergeDependencyList(payload *types.Payload, listFile types.File, currentPath string, provider types.Provider) {
	content, err := provider.ReadFile(filepath.Join(currentPath, listFile.Name))
	if err != nil {
		return
	}

	listParser := parsers.NewMavenDependencyListParser()
	// Only include direct dependencies for now (includeTransitive=false)
	// This can be changed later to support transitive dependencies
	listDeps := listParser.ParseDependencyList(string(content), false)

	if len(listDeps) == 0 {
		return
	}

	// Create a map of existing dependencies for quick lookup
	existingDeps := make(map[string]int)
	for i, dep := range payload.Dependencies {
		existingDeps[dep.Name] = i
	}

	// Update versions for direct dependencies from pom.xml
	for _, listDep := range listDeps {
		if idx, exists := existingDeps[listDep.Name]; exists {
			// This is a direct dependency from pom.xml - update its version
			originalMetadata := payload.Dependencies[idx].Metadata
			payload.Dependencies[idx].Version = listDep.Version

			// Add source marker to indicate dependency list source
			if originalMetadata == nil {
				originalMetadata = make(map[string]interface{})
			}
			originalMetadata["source"] = "dependency-list"
			payload.Dependencies[idx].Metadata = originalMetadata
		}
	}
}

func (d *Detector) detectPomXML(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Extract project name using parser
	mavenParser := parsers.NewMavenParser()
	projectInfo := mavenParser.ExtractProjectInfo(string(content))
	projectName := d.formatProjectName(projectInfo.GroupId, projectInfo.ArtifactId)
	if projectName == "" {
		projectName = filepath.Base(currentPath)
	}

	// Create named payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(projectName, relativeFilePath)

	// Set tech field to java (covers both Java and Kotlin projects)
	payload.AddPrimaryTech("java")

	// Extract Maven project info and add as properties
	projectInfo = mavenParser.ExtractProjectInfo(string(content))
	if projectInfo.GroupId != "" || projectInfo.ArtifactId != "" {
		mavenInfo := map[string]interface{}{
			"group_id":    projectInfo.GroupId,
			"artifact_id": projectInfo.ArtifactId,
			"version":     projectInfo.Version,
		}

		// Add packaging if not default jar
		if projectInfo.Packaging != "" && projectInfo.Packaging != "jar" {
			mavenInfo["packaging"] = projectInfo.Packaging
		}

		// Add parent POM info if exists
		if projectInfo.Parent.GroupId != "" {
			mavenInfo["parent"] = map[string]string{
				"group_id":    projectInfo.Parent.GroupId,
				"artifact_id": projectInfo.Parent.ArtifactId,
				"version":     projectInfo.Parent.Version,
			}
		}

		payload.Properties["maven"] = mavenInfo
	}

	dependencies := mavenParser.ParsePomXMLWithProvider(string(content), currentPath, provider)

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add maven tech
	payload.AddTech("maven", "matched file: pom.xml")

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "maven")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}

		payload.Dependencies = dependencies
	}

	return payload
}

func (d *Detector) detectGradle(file types.File, currentPath, basePath string, provider types.Provider, depDetector components.DependencyDetector) *types.Payload {
	content, err := provider.ReadFile(filepath.Join(currentPath, file.Name))
	if err != nil {
		return nil
	}

	// Extract project name using parser
	gradleParser := parsers.NewGradleParser()
	projectInfo := gradleParser.ParseProjectInfo(string(content))
	projectName := projectInfo.Name
	if projectName == "" {
		projectName = filepath.Base(currentPath)
	}

	// Create named payload with specific file path
	relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
	if relativeFilePath == "." {
		relativeFilePath = "/"
	} else {
		relativeFilePath = "/" + relativeFilePath
	}
	payload := types.NewPayloadWithPath(projectName, relativeFilePath)

	// Set tech field to java (covers both Java and Kotlin projects)
	payload.AddPrimaryTech("java")

	// Extract Gradle project info and add as properties
	projectInfo = gradleParser.ParseProjectInfo(string(content))
	if projectInfo.Group != "" || projectInfo.Name != "" {
		// Use project name as artifact if not specified
		artifactName := projectInfo.Name
		if artifactName == "" {
			artifactName = projectName
		}
		payload.Properties["gradle"] = map[string]string{
			"group_id":    projectInfo.Group,
			"artifact_id": artifactName,
			"version":     projectInfo.Version,
		}
	}

	dependencies := gradleParser.ParseGradle(string(content))

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add gradle tech
	payload.AddTech("gradle", "matched file: "+file.Name)

	// Match dependencies against rules
	if len(dependencies) > 0 {
		matchedTechs := depDetector.MatchDependencies(depNames, "gradle")
		for tech, reasons := range matchedTechs {
			for _, reason := range reasons {
				payload.AddTech(tech, reason)
			}
			depDetector.AddPrimaryTechIfNeeded(payload, tech)
		}

		payload.Dependencies = dependencies
	}

	return payload
}

// formatProjectName formats project name from groupId and artifactId
func (d *Detector) formatProjectName(groupId, artifactId string) string {
	if artifactId != "" {
		if groupId != "" {
			return groupId + ":" + artifactId
		}
		return artifactId
	}
	return ""
}

func init() {
	components.Register(&Detector{})
}
