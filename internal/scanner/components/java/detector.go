package java

import (
	"path/filepath"
	"regexp"
	"strings"

	licensenormalizer "github.com/petrarca/tech-stack-analyzer/internal/license"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/mavenresolve"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/providers"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/resolver"
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

	// Attach the dependency graph. A committed dependency-tree.json (or
	// CycloneDX) always wins first; otherwise the effective Maven graph source
	// applies -- the repo crawl (--maven-graph-source=repo) or deps.dev. No-op
	// unless the dependency-graph mode is on.
	if payload != nil {
		components.AttachLockfileGraphWithFallback(payload, currentPath, provider,
			mavenGraphProducers, d.mavenGraphFallback(provider))
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
		if !gradleRegex.MatchString(file.Name) {
			continue
		}
		relativeFilePath, _ := filepath.Rel(basePath, filepath.Join(currentPath, file.Name))
		if relativeFilePath != "." {
			payload.AddPath("/" + relativeFilePath)
		}
		payload.AddTech("gradle", "matched file: "+file.Name)

		if gradlePayload := d.detectGradle(file, currentPath, basePath, provider, depDetector); gradlePayload != nil {
			mergeGradleIntoPayload(payload, gradlePayload)
		}
	}
}

// mergeGradleIntoPayload folds a detected Gradle payload's dependencies and
// gradle properties into the Maven payload.
func mergeGradleIntoPayload(payload, gradlePayload *types.Payload) {
	for _, dep := range gradlePayload.Dependencies {
		payload.AddDependency(dep)
	}
	gradleProps, exists := gradlePayload.Properties["gradle"]
	if !exists {
		return
	}
	if payload.Properties == nil {
		payload.Properties = make(map[string]interface{})
	}
	payload.Properties["gradle"] = gradleProps
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
	chain := d.mavenPomChain(provider)
	if chain.Empty() {
		return nil
	}
	return chain.FetchPOM
}

// mavenPomChain composes the POM-source chain shared by version resolution
// (BOM/parent fetch) and the transitive graph crawl: in-repo source index ->
// local ~/.m2 -> settings.xml repos / --maven-repo-url -> optional Maven
// Central. Returns an empty chain when nothing is available.
func (d *Detector) mavenPomChain(provider types.Provider) *mavenresolve.Chain {
	if provider == nil {
		return mavenresolve.NewChain()
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

	// Tiers 3+: remote repositories, in precedence order:
	//   settings.xml <repositories> -> --maven-repo-url -> Maven Central.
	//
	// An EXPLICITLY configured repository -- settings.xml <repositories> or
	// --maven-repo-url -- is the user's deliberate opt-in, so it is always
	// used; configuring it IS the consent to reach it. Such repositories are
	// typically internal mirrors/virtual repos that already proxy public
	// artifacts.
	//
	// Maven Central is the PUBLIC fallback, contacted only under --maven-central
	// (default off). It is appended last, so it is consulted only for
	// coordinates the higher tiers could not resolve. It may coexist with a
	// configured private repo: when that repo does not proxy Central, public
	// BOMs/POMs still resolve, while private artifacts resolve from the private
	// repo first.
	//
	// Scan-wide cache: shared so a POM is fetched at most once across all
	// components, not re-fetched per module.
	cache := components.GetGraphCache(provider)

	if repos := settings.RemoteSources(nil, cache); len(repos) > 0 {
		sources = append(sources, repos...)
	}
	if url := components.MavenRepoURL(); url != "" {
		opts := mavenresolve.RemoteOptions{BaseURL: url, Cache: cache}
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
	// Maven Central is opt-in via --maven-central (default off). When enabled it
	// is appended as the lowest-priority source, so it serves public BOMs/POMs
	// even alongside a configured private repo that does not proxy Central
	// (private artifacts still resolve from the private repo first).
	if components.UseMavenCentral() {
		sources = append(sources, mavenresolve.NewRemoteSource(mavenresolve.RemoteOptions{Cache: cache}))
	}

	return mavenresolve.NewChain(sources...)
}

// mavenGraphFallback returns the transitive-graph resolver to try after a
// committed tree (which always wins first), per the effective Maven graph
// source:
//
//   - "repo"     -> pure repository crawl; never contacts deps.dev (privacy).
//   - "deps-dev" -> deps.dev for the public set, with a repository-crawl
//     fallback for the coordinates deps.dev cannot resolve (private artifacts),
//     when a repository chain is available. Without a repo chain it returns nil
//     and the chain's own deps.dev resolver handles it.
//   - "none"     -> nil (offline; only a committed tree applies).
func (d *Detector) mavenGraphFallback(provider types.Provider) resolver.DependencyResolver {
	switch components.MavenEffectiveGraphSource() {
	case "repo":
		if repo := d.mavenRepoGraphResolver(provider); repo != nil {
			return repo
		}
		return nil
	case "deps-dev":
		repo := d.mavenRepoGraphResolver(provider)
		if repo == nil {
			return nil // no repo chain: let the chain's deps.dev resolver run
		}
		// Hybrid: deps.dev public + repo-crawl fallback for private artifacts.
		return mavenresolve.NewHybridResolver(components.NewDepsDevResolver(provider), repo)
	default:
		return nil
	}
}

// mavenRepoGraphResolver builds the repository-crawl transitive resolver over
// the POM source chain, or nil when no source is available.
func (d *Detector) mavenRepoGraphResolver(provider types.Provider) *mavenresolve.GraphResolver {
	chain := d.mavenPomChain(provider)
	if chain.Empty() {
		return nil
	}
	// Share the child memo scan-wide so each coordinate's subtree resolves once
	// across all components, not per component.
	return mavenresolve.NewGraphResolver(chain, components.GetMavenChildMemo(provider))
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

	// Extract plugin IDs declared in plugins{} / buildscript{} blocks.
	// These are matched against "gradle.plugin" rules — the authoritative
	// signal for Kotlin, Spring Boot, Quarkus, etc. when no explicit
	// starter coordinates are present in dependencies{}. Parsed before version
	// resolution because some plugins (e.g. Spring Boot) imply a managed BOM.
	plugins := gradleParser.ParsePlugins(string(content))

	// Resolve versions managed by a platform()/enforcedPlatform() BOM and by
	// version-managing plugins (e.g. Spring Boot), then backfill any dependency
	// declared without a version (the Gradle analogue of Maven
	// dependencyManagement-import resolution).
	d.applyGradlePlatformVersions(dependencies, plugins, provider)

	// Prefer a committed gradle.lockfile: it holds fully resolved versions for
	// this module, superseding build-script + BOM resolution. Matched by
	// suffix because Gradle names locks per project (e.g. "gradle.lockfile",
	// "settings-gradle.lockfile").
	if locked := d.gradleLockfileDependencies(currentPath, provider); len(locked) > 0 {
		dependencies = mergeGradleLockedVersions(dependencies, locked)
	}

	// Extract dependency names for tech matching
	var depNames []string
	for _, dep := range dependencies {
		depNames = append(depNames, dep.Name)
	}

	// Always add gradle tech
	payload.AddTech("gradle", "matched file: "+file.Name)

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

// applyGradlePlatformVersions resolves every platform()/enforcedPlatform() BOM
// declared in the dependency list (scope=import) and backfills versions onto
// dependencies declared without one. It reuses the Maven POM source chain and
// dependencyManagement collector, since a Gradle platform is a pom-packaged BOM
// identical to a Maven imported BOM.
func (d *Detector) applyGradlePlatformVersions(deps []types.Dependency, plugins []parsers.GradlePlugin, provider types.Provider) {
	bomResolver := d.newBomResolver(provider)
	if bomResolver == nil {
		return
	}

	// Collect BOM coordinates to resolve: explicit platform()/enforcedPlatform()
	// imports plus the implicit BOMs contributed by version-managing plugins.
	type bomCoord struct{ group, artifact, version string }
	var boms []bomCoord
	for _, dep := range deps {
		if dep.Scope != types.ScopeImport {
			continue
		}
		if g, a, ok := splitGradleCoordinate(dep.Name); ok {
			boms = append(boms, bomCoord{g, a, dep.Version})
		}
	}
	for _, p := range gradlePluginManagedBoms(plugins) {
		boms = append(boms, bomCoord{p.group, p.artifact, p.version})
	}

	managed := make(map[string]string)
	for _, b := range boms {
		for ga, version := range parsers.CollectBomManagedVersions(b.group, b.artifact, b.version, provider, bomResolver) {
			if _, exists := managed[ga]; !exists {
				managed[ga] = version
			}
		}
	}
	parsers.ApplyManagedVersions(deps, managed)
}

// gradlePluginManagedBoms maps version-managing Gradle plugins to the BOM they
// implicitly import. The Spring Boot plugin (with io.spring.dependency-management)
// imports org.springframework.boot:spring-boot-dependencies at the plugin's own
// version, supplying managed versions for spring-boot-starter-* and friends
// declared without an explicit version.
func gradlePluginManagedBoms(plugins []parsers.GradlePlugin) []struct{ group, artifact, version string } {
	var boms []struct{ group, artifact, version string }
	for _, p := range plugins {
		if p.ID == "org.springframework.boot" && p.Version != "" {
			boms = append(boms, struct{ group, artifact, version string }{
				"org.springframework.boot", "spring-boot-dependencies", p.Version,
			})
		}
	}
	return boms
}

// splitGradleCoordinate splits a "group:artifact" dependency name into its
// parts. Returns ok=false when the name is not a valid two-part coordinate.
func splitGradleCoordinate(name string) (group, artifact string, ok bool) {
	i := strings.Index(name, ":")
	if i <= 0 || i == len(name)-1 {
		return "", "", false
	}
	return name[:i], name[i+1:], true
}

// gradleLockfileDependencies reads a committed *.gradle.lockfile in the module
// directory and returns its resolved dependencies, or nil when none exists.
func (d *Detector) gradleLockfileDependencies(currentPath string, provider types.Provider) []types.Dependency {
	files, err := provider.ListDir(currentPath)
	if err != nil {
		return nil
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name, "gradle.lockfile") {
			continue
		}
		content, err := provider.ReadFile(filepath.Join(currentPath, f.Name))
		if err != nil || len(content) == 0 {
			continue
		}
		if locked := parsers.ParseGradleLockfile(string(content)); len(locked) > 0 {
			return locked
		}
	}
	return nil
}

// mergeGradleLockedVersions backfills unresolved versions in the build-script
// dependency list from the lockfile's resolved versions (keyed by
// group:artifact), preserving the build script's scope/metadata. Lockfile
// entries for dependencies not declared in the build script are not added: the
// build script defines the project's direct dependencies; the lockfile (which
// also contains transitive entries) only supplies versions.
func mergeGradleLockedVersions(deps, locked []types.Dependency) []types.Dependency {
	versions := make(map[string]string, len(locked))
	for _, l := range locked {
		versions[l.Name] = l.Version
	}
	parsers.ApplyManagedVersions(deps, versions)
	return deps
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

	// Expose the Maven graph-fallback builder so off-scan resolution (the sbom
	// command) can resolve Maven/Gradle transitive graphs from coordinates via
	// the same repo-crawl/hybrid resolver, without importing this package.
	components.RegisterMavenGraphFallback(func(provider types.Provider) resolver.DependencyResolver {
		return (&Detector{}).mavenGraphFallback(provider)
	})

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
