package java

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"

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

	// Check for pom.xml (Maven) first
	for _, file := range files {
		if file.Name == "pom.xml" {
			payload = d.detectPomXML(file, currentPath, basePath, provider, depDetector)
			break
		}
	}

	// If no pom.xml, check for build.gradle or build.gradle.kts (Gradle)
	if payload == nil {
		gradleRegex := regexp.MustCompile(`^build\.gradle(\.kts)?$`)
		for _, file := range files {
			if gradleRegex.MatchString(file.Name) {
				payload = d.detectGradle(file, currentPath, basePath, provider, depDetector)
				break
			}
		}
	} else {
		// If we found pom.xml, also check for gradle files and add them to the path
		gradleRegex := regexp.MustCompile(`^build\.gradle(\.kts)?$`)
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

	// Extract project name
	projectName := d.extractProjectNameFromPom(string(content))
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

	// Parse pom.xml for dependencies using parser
	mavenParser := parsers.NewMavenParser()
	dependencies := mavenParser.ParsePomXML(string(content))

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

	// Extract project name
	projectName := d.extractProjectNameFromGradle(string(content), currentPath)
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

	// Parse Gradle file for dependencies using parser
	gradleParser := parsers.NewGradleParser()
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

// extractProjectNameFromPom extracts project name from pom.xml
func (d *Detector) extractProjectNameFromPom(content string) string {
	type Project struct {
		GroupId    string `xml:"groupId"`
		ArtifactId string `xml:"artifactId"`
	}

	type MavenProject struct {
		Project Project `xml:"project"`
	}

	var mavenProject MavenProject
	if err := xml.Unmarshal([]byte(content), &mavenProject); err != nil {
		return ""
	}

	project := mavenProject.Project
	if project.ArtifactId != "" {
		if project.GroupId != "" {
			return project.GroupId + ":" + project.ArtifactId
		}
		return project.ArtifactId
	}

	return ""
}

// extractProjectNameFromGradle extracts project name from Gradle files
func (d *Detector) extractProjectNameFromGradle(content, currentPath string) string {
	// Look for rootProject.name or project.name
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Match rootProject.name = 'name' or rootProject.name = "name"
		if strings.Contains(line, "rootProject.name") {
			regex := regexp.MustCompile(`rootProject\.name\s*=\s*['"]([^'"]+)['"]`)
			if match := regex.FindStringSubmatch(line); match != nil {
				return match[1]
			}
		}

		// Match project.name = 'name' or project.name = "name"
		if strings.Contains(line, "project.name") {
			regex := regexp.MustCompile(`project\.name\s*=\s*['"]([^'"]+)['"]`)
			if match := regex.FindStringSubmatch(line); match != nil {
				return match[1]
			}
		}
	}

	return ""
}

func init() {
	components.Register(&Detector{})
}
