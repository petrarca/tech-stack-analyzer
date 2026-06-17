package java

import (
	"path/filepath"
	"regexp"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/semver"
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
	var dependencyTreeFile *types.File

	// Look for pom.xml and the pre-generated resolved-version files.
	for i := range files {
		if files[i].Name == "pom.xml" {
			payload = d.detectPomXML(files[i], currentPath, basePath, provider, depDetector)
		}
		if files[i].Name == "dependency-list.txt" {
			dependencyListFile = &files[i]
		}
		if files[i].Name == parsers.MavenTreeFileName {
			dependencyTreeFile = &files[i]
		}
	}

	// Backfill resolved versions from pre-generated files (most authoritative,
	// when present). dependency-list.txt is preferred; dependency-tree.json is
	// the fallback so its resolved versions reach the flat dependency list (and
	// thus the SBOM), not only the graph edges.
	if payload != nil && dependencyListFile != nil {
		d.mergeDependencyList(payload, *dependencyListFile, currentPath, provider)
	} else if payload != nil && dependencyTreeFile != nil {
		d.mergeDependencyTreeVersions(payload, *dependencyTreeFile, currentPath, provider)
	}

	// Attach the dependency graph from a pre-generated dependency-tree.json
	// (mvn dependency:tree -DoutputType=json). No-op unless the dependency-graph
	// mode is on and the file is present; the analyzer never runs Maven.
	if payload != nil {
		components.AttachLockfileGraph(payload, currentPath, provider, mavenGraphProducers)
	}

	return payload
}

// mavenGraphProducers lists pre-generated Maven graph sources in priority
// order: the dependency:tree JSON, then a CycloneDX SBOM's dependency-graph
// section (e.g. cyclonedx-maven-plugin). A resolved Maven graph cannot be
// derived statically from pom.xml, so we read what the user/CI committed.
var mavenGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: parsers.MavenTreeFileName, Parse: parsers.ParseMavenTreeGraph},
	{Lockfile: parsers.CycloneDXFileName, Parse: parsers.ParseCycloneDXGraph},
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

// newBomResolver returns a parsers.BomResolver that locates an imported BOM's
// pom.xml within the scanned tree via the source index. This lets the Maven
// parser follow scope=import BOMs to sibling/ancestor modules in the repo and
// read their managed versions -- offline, without contacting a Maven
// repository. Returns ok=false when the BOM is not in the repo (third-party or
// private BOMs published only to a registry), leaving those versions
// unresolved.
// newBomResolver composes the Maven POM-source chain used to resolve
// scope=import BOMs and parent POMs, in precedence order:
//
//  1. in-repo source index   -- BOMs committed to the scanned tree (offline)
//  2. local ~/.m2 repository -- previously built/downloaded POMs (offline)
//  3. remote Maven repository -- Central or a configured mirror/JFrog (opt-in)
//
// The chain is adapted to parsers.BomResolver. Returns nil when no source is
// available (so the parser skips BOM-import following).
func (d *Detector) newBomResolver(provider types.Provider) parsers.BomResolver {
	if provider == nil {
		return nil
	}
	index := components.GetSourceIndex(provider)

	// Tier 1: BOMs committed to the scanned tree (offline, no network).
	sources := []mavenresolve.PomSource{
		mavenresolve.NewRepoSource(
			func(coordinate string) []string { return index.Lookup("maven", coordinate) },
			provider,
		),
	}

	// Maven settings.xml supplies the local-repo path, repository URLs, and
	// credentials, reused by both the local and remote tiers below.
	settings := components.MavenSettings()

	// Tier 2: local ~/.m2 cache (offline, no credentials). Opt-in -- reads
	// outside the scanned tree. Honors settings.xml <localRepository>.
	if components.UseMavenLocalRepo() {
		dir := components.MavenLocalRepoDir()
		if dir == "" && settings != nil {
			dir = settings.LocalRepository
		}
		sources = append(sources, mavenresolve.NewLocalRepoSource(dir))
	}

	// Tiers 3+: remote repositories (opt-in via online resolution). settings.xml
	// repos (with their credentials) first, then any explicitly configured URL,
	// then Maven Central as the public fallback. Distinct from the deps.dev
	// resolve-online endpoint.
	if components.ResolveOnline() {
		sources = append(sources, settings.RemoteSources(nil)...)
		if url := components.MavenRepoURL(); url != "" {
			opts := mavenresolve.RemoteOptions{BaseURL: url}
			// With a username, the token is the Basic-auth password (JFrog
			// reference token); without one, it is a Bearer token.
			if user := components.MavenRepoUser(); user != "" {
				opts.Username = user
				opts.Password = components.MavenRepoToken()
			} else {
				opts.Token = components.MavenRepoToken()
			}
			sources = append(sources, mavenresolve.NewRemoteSource(opts))
		}
		// Maven Central public fallback.
		sources = append(sources, mavenresolve.NewRemoteSource(mavenresolve.RemoteOptions{}))
	}

	chain := mavenresolve.NewChain(sources...)
	if chain.Empty() {
		return nil
	}
	return chain.FetchPOM
}

// mergeDependencyTreeVersions backfills resolved versions from a pre-generated
// dependency-tree.json (mvn dependency:tree) into the flat dependency list.
// Maven's tree output carries fully resolved versions, so this fills the gap
// for direct dependencies that pom.xml left versionless (BOM-managed) or
// declared as an unresolved property. It only updates dependencies that do not
// already carry a concrete version, so an authoritative pom.xml/list value is
// never overwritten.
func (d *Detector) mergeDependencyTreeVersions(payload *types.Payload, treeFile types.File, currentPath string, provider types.Provider) {
	content, err := provider.ReadFile(filepath.Join(currentPath, treeFile.Name))
	if err != nil {
		return
	}

	versions := parsers.ParseMavenTreeVersions(content)
	if len(versions) == 0 {
		return
	}

	for i := range payload.Dependencies {
		dep := &payload.Dependencies[i]
		if dep.Type != "maven" && dep.Type != "gradle" {
			continue
		}
		if semver.IsResolved(dep.Version) {
			continue
		}
		resolved, ok := versions[dep.Name]
		if !ok || resolved == "" {
			continue
		}
		declared := dep.Version
		dep.Version = resolved
		dep.SetDeclaredVersion(declared)
		if dep.Metadata == nil {
			dep.Metadata = make(map[string]interface{})
		}
		dep.Metadata["source"] = "dependency-tree"
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

	dependencies := mavenParser.ParsePomXMLWithBomResolver(string(content), currentPath, provider,
		d.newBomResolver(provider))

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

// collectGradleProperties reads gradle.properties from the module directory and
// ancestor directories, merging them so the nearest definition wins. This
// resolves version properties declared in a root gradle.properties.
//
// The climb continues a bounded number of levels even above the scan root
// (basePath): when a sub-module is scanned directly, its build's root
// gradle.properties lives in an ancestor outside the scan root, and the
// (non-sandboxed) provider can still read it from disk. The bound prevents
// reading unrelated files far up the tree.
func (d *Detector) collectGradleProperties(currentPath, basePath string, provider types.Provider) map[string]string {
	const maxDepth = 12 // guard against unbounded climbs
	const maxAboveRoot = 3

	// Collect directories from current up the tree, stopping shortly after the
	// scan root.
	var dirs []string
	dir := filepath.Clean(currentPath)
	root := filepath.Clean(basePath)
	aboveRoot := 0
	for i := 0; i < maxDepth; i++ {
		dirs = append(dirs, dir)
		if dir == "." || dir == string(filepath.Separator) {
			break
		}
		if isAtOrAboveRoot(dir, root) {
			if aboveRoot >= maxAboveRoot {
				break
			}
			aboveRoot++
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Merge from the highest ancestor down to the module so the nearest file
	// overrides ancestors.
	merged := make(map[string]string)
	for i := len(dirs) - 1; i >= 0; i-- {
		content, err := provider.ReadFile(filepath.Join(dirs[i], "gradle.properties"))
		if err != nil || len(content) == 0 {
			continue
		}
		for k, v := range parsers.ParseGradleProperties(string(content)) {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// isAtOrAboveRoot reports whether dir is the scan root or an ancestor of it.
func isAtOrAboveRoot(dir, root string) bool {
	if dir == root {
		return true
	}
	rel, err := filepath.Rel(dir, root)
	if err != nil {
		return false
	}
	// root is under dir (dir is an ancestor) when the relative path does not
	// need to climb out of dir.
	return rel == "." || (rel != ".." && !startsWithDotDot(rel))
}

// startsWithDotDot reports whether a relative path begins with "..".
func startsWithDotDot(rel string) bool {
	return rel == ".." || (len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && rel[2] == filepath.Separator)
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

	// Collect gradle.properties for version resolution, climbing from the
	// module directory up to the scan root. In multi-module builds the version
	// properties live in the root gradle.properties, so the nearest file wins
	// but ancestors fill in anything it does not define.
	gradleProps := d.collectGradleProperties(currentPath, basePath, provider)

	dependencies := gradleParser.ParseGradleWithProperties(string(content), gradleProps)

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

	// Attach the dependency graph from a pre-generated `gradle dependencies`
	// tree. No-op unless the mode is on and the file is present; the analyzer
	// never runs gradle.
	components.AttachLockfileGraph(payload, currentPath, provider, gradleGraphProducers)

	return payload
}

// gradleGraphProducers lists the pre-generated Gradle graph file. A resolved
// Gradle graph cannot be derived from build.gradle, so we read the
// machine-generated `gradle dependencies` output the operator/CI committed.
var gradleGraphProducers = []components.LockfileGraphProducer{
	{Lockfile: parsers.GradleTreeFileName, Parse: parsers.ParseGradleTreeGraph},
	{Lockfile: parsers.CycloneDXFileName, Parse: parsers.ParseCycloneDXGraph},
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
