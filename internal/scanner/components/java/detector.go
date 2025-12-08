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

	// Define gradle regex once
	gradleRegex := regexp.MustCompile(`^build\.gradle(\.kts)?$`)

	// Check for pom.xml (Maven) first
	for _, file := range files {
		if file.Name == "pom.xml" {
			payload = d.detectPomXML(file, currentPath, basePath, provider, depDetector)
			break
		}
	}

	// If no pom.xml, check for build.gradle or build.gradle.kts (Gradle)
	if payload == nil {
		for _, file := range files {
			if gradleRegex.MatchString(file.Name) {
				payload = d.detectGradle(file, currentPath, basePath, provider, depDetector)
				break
			}
		}
	} else {
		// If we found pom.xml, also check for gradle files and add them to the path
		for _, file := range files {
			if gradleRegex.MatchString(file.Name) {
				relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
				if relativeFilePath != "." {
					relativeFilePath = "/" + relativeFilePath
					payload.AddPath(relativeFilePath)
				}
				// Also add gradle tech
				payload.AddTech("gradle", "matched file: "+file.Name)
			}
		}
	}

	if payload != nil {
		results = append(results, payload)
	}

	return results
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
		payload.Properties["maven"] = map[string]string{
			"group_id":    projectInfo.GroupId,
			"artifact_id": projectInfo.ArtifactId,
		}
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
