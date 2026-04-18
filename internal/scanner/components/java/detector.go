package java

import (
	"path/filepath"
	"regexp"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
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

	// Handle inheritance from parent
	if projectInfo.GroupId == "" && projectInfo.Parent.GroupId != "" {
		projectInfo.GroupId = projectInfo.Parent.GroupId
	}
	if projectInfo.Version == "" && projectInfo.Parent.Version != "" {
		projectInfo.Version = projectInfo.Parent.Version
	}

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
	payload.SetComponentType("maven")

	// Mark java as a primary tech for any Maven project (the JVM is the
	// default target). Kotlin, Scala and Groovy are added separately by
	// dependency matches when their plugins/runtimes are declared.
	payload.AddPrimaryTech("java")

	// Extract Maven project info and add as properties
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

	// Process licenses from pom.xml <licenses> section
	d.processLicenses(projectInfo.Licenses, payload)

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
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, "maven"))
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
	payload.SetComponentType("gradle")

	// Every Gradle project targets the JVM by default, so mark java as a
	// primary tech. Kotlin (and Groovy/Scala when used) is added separately
	// by the gradle.plugin match below when the corresponding plugin is
	// declared in plugins{} / buildscript{}.
	payload.AddPrimaryTech("java")

	// Extract Gradle project info and add as properties
	projectInfo = gradleParser.ParseProjectInfo(string(content))
	if projectInfo.Group != "" || projectInfo.Name != "" {
		// Use project name as artifact if not specified
		artifactName := projectInfo.Name
		if artifactName == "" {
			artifactName = projectName
		}
		payload.SetComponentProperties("gradle", map[string]interface{}{
			"group_id":    projectInfo.Group,
			"artifact_id": artifactName,
			"version":     projectInfo.Version,
		})
	}

	dependencies := gradleParser.ParseGradle(string(content))

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add gradle tech
	payload.AddTech("gradle", "matched file: "+file.Name)

	// Extract plugin IDs declared in plugins{} / buildscript{} blocks.
	// These are matched against "gradle.plugin" rules — the authoritative
	// signal for Kotlin, Spring Boot, Quarkus, etc. when no explicit
	// starter coordinates are present in dependencies{}.
	plugins := gradleParser.ParsePlugins(string(content))
	var pluginIDs []string
	for _, p := range plugins {
		pluginIDs = append(pluginIDs, p.ID)
	}

	// Match coordinates and plugin IDs against rules. The dependency
	// detector aliases "gradle" and "maven" so a single
	// MatchDependencies("gradle") call covers both rule types — no need to
	// call both explicitly.
	if len(dependencies) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(depNames, "gradle"))
		payload.Dependencies = dependencies
	}
	if len(pluginIDs) > 0 {
		depDetector.ApplyMatchesToPayload(payload, depDetector.MatchDependencies(pluginIDs, "gradle.plugin"))
	}

	return payload
}

// processLicenses handles license processing for pom.xml <licenses> section
func (d *Detector) processLicenses(mavenLicenses []parsers.MavenLicense, payload *types.Payload) {
	for _, ml := range mavenLicenses {
		if ml.Name == "" {
			continue
		}
		licensenormalizer.ProcessLicenseExpression(ml.Name, "pom.xml", payload)
	}
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

	// Register maven package provider
	providers.Register(&providers.PackageProvider{
		DependencyType:      "maven",
		ExtractPackageNames: providers.GroupArtifactExtractor("maven"),
	})

	// Register gradle package provider (same pattern as maven)
	providers.Register(&providers.PackageProvider{
		DependencyType:      "gradle",
		ExtractPackageNames: providers.GroupArtifactExtractor("gradle"),
	})
}
