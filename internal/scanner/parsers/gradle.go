package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// Pre-compiled regexes for Gradle parsing performance
var (
	gradleDepTypeRegex = regexp.MustCompile(`^\s*(testImplementation|testRuntimeOnly|testCompileOnly|testApi|compileOnly|annotationProcessor|runtimeOnly|implementation|compile|api)`)
	gradleQuotedRegex  = regexp.MustCompile(`['"]([^'"]+)['"]`)

	// Plugin DSL patterns — capture (pluginID, version?).
	//
	//   id("org.springframework.boot") version "3.4.3"   Kotlin DSL
	//   id 'org.springframework.boot' version '2.7.0'    Groovy DSL (no parens)
	//   kotlin("jvm") version "2.1.10"                   Kotlin DSL short form
	gradlePluginIDParenRegex  = regexp.MustCompile(`\bid\s*\(\s*["']([^"']+)["']\s*\)(?:\s+version\s+["']([^"']+)["'])?`)
	gradlePluginIDGroovyRegex = regexp.MustCompile(`^\s*id\s+["']([^"']+)["'](?:\s+version\s+["']([^"']+)["'])?`)
	gradlePluginKotlinRegex   = regexp.MustCompile(`\bkotlin\s*\(\s*["']([^"']+)["']\s*\)(?:\s+version\s+["']([^"']+)["'])?`)

	// Project info patterns used by ParseProjectInfo. Pre-compiled to avoid
	// recompiling on every line of every build file scanned.
	gradleGroupRegex           = regexp.MustCompile(`group\s*[=]?\s*['"]([^'"]+)['"]`)
	gradleVersionRegex         = regexp.MustCompile(`version\s*[=]?\s*['"]([^'"]+)['"]`)
	gradleRootProjectNameRegex = regexp.MustCompile(`rootProject\.name\s*=\s*['"]([^'"]+)['"]`)

	// Single regex covering all Gradle dependency configurations. Used as a
	// cheap gate by isPotentialDependencyLine before the full parser runs.
	gradleDepConfigRegex = regexp.MustCompile(`\b(testImplementation|testRuntimeOnly|testCompileOnly|testApi|annotationProcessor|compileOnly|runtimeOnly|implementation|compile|api)\b`)
)

// GradleParser handles Gradle-specific file parsing (build.gradle, build.gradle.kts)
type GradleParser struct{}

// NewGradleParser creates a new Gradle parser
func NewGradleParser() *GradleParser {
	return &GradleParser{}
}

// ParseGradle parses build.gradle or build.gradle.kts and extracts Gradle dependencies
func (p *GradleParser) ParseGradle(content string) []types.Dependency {
	var dependencies []types.Dependency

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if p.shouldSkipLine(line) {
			continue
		}

		// Quick validation - is this even a dependency line?
		if !p.isPotentialDependencyLine(line) {
			continue
		}

		gradleDep := p.parseGradleDependency(line)
		if gradleDep != nil {
			dependencies = append(dependencies, *gradleDep)
		}
	}

	return dependencies
}

// GradleDependency represents a parsed Gradle dependency
type GradleDependency struct {
	Type     string
	Group    string
	Artifact string
	Version  string
}

// GradleProjectInfo holds extracted Gradle project information
type GradleProjectInfo struct {
	Group   string
	Name    string
	Version string
}

// ParseProjectInfo extracts group, name, and version from Gradle build file
func (p *GradleParser) ParseProjectInfo(content string) GradleProjectInfo {
	info := GradleProjectInfo{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments
		if p.shouldSkipLine(line) {
			continue
		}

		// Match group = 'com.example' or group = "com.example" or group 'com.example'
		if strings.HasPrefix(line, "group") {
			if match := gradleGroupRegex.FindStringSubmatch(line); match != nil {
				info.Group = match[1]
			}
		}

		// Match version = '1.0.0' or version = "1.0.0"
		if strings.HasPrefix(line, "version") && !strings.Contains(line, "sourceCompatibility") {
			if match := gradleVersionRegex.FindStringSubmatch(line); match != nil {
				info.Version = match[1]
			}
		}

		// Match rootProject.name = 'name' (typically in settings.gradle)
		if strings.Contains(line, "rootProject.name") {
			if match := gradleRootProjectNameRegex.FindStringSubmatch(line); match != nil {
				info.Name = match[1]
			}
		}
	}

	return info
}

// GradlePlugin represents a plugin declared in a Gradle build file's
// plugins{} or buildscript{} block. ID is the canonical plugin ID used for
// rule matching (e.g. "org.springframework.boot", "org.jetbrains.kotlin.jvm").
// Version is empty when omitted in the build file.
type GradlePlugin struct {
	ID      string
	Version string
}

// ParsePlugins extracts plugin declarations from a build.gradle or
// build.gradle.kts file. It handles three syntactic forms:
//
//	id("org.springframework.boot") version "3.4.3"   (Kotlin DSL)
//	id 'org.springframework.boot' version '2.7.0'    (Groovy DSL)
//	kotlin("jvm") version "2.1.10"                   (Kotlin DSL short form,
//	                                                  normalised to
//	                                                  org.jetbrains.kotlin.jvm)
//
// Duplicate plugin IDs are deduplicated; the first occurrence wins.
func (p *GradleParser) ParsePlugins(content string) []GradlePlugin {
	var plugins []GradlePlugin
	seen := make(map[string]bool)

	add := func(id, version string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		plugins = append(plugins, GradlePlugin{ID: id, Version: version})
	}

	// id("foo.bar") [version "x"] — Kotlin DSL and Groovy DSL with parens
	for _, m := range gradlePluginIDParenRegex.FindAllStringSubmatch(content, -1) {
		add(m[1], m[2])
	}

	// id 'foo.bar' [version 'x'] — Groovy DSL without parens. The regex is
	// line-anchored (^) so we scan line-by-line to anchor correctly; this
	// avoids false matches like `obj.id 'foo'` in the middle of a line.
	for _, line := range strings.Split(content, "\n") {
		if m := gradlePluginIDGroovyRegex.FindStringSubmatch(line); m != nil {
			add(m[1], m[2])
		}
	}

	// kotlin("sub") [version "x"] — normalised to org.jetbrains.kotlin.<sub>
	// per the Kotlin Gradle plugin accessor convention.
	for _, m := range gradlePluginKotlinRegex.FindAllStringSubmatch(content, -1) {
		add("org.jetbrains.kotlin."+m[1], m[2])
	}

	return plugins
}

// shouldSkipLine checks if a line should be skipped during parsing
func (p *GradleParser) shouldSkipLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*")
}

// isPotentialDependencyLine does quick validation before expensive regex
// matching. Requires a known Gradle dependency configuration keyword and a
// quoted group:artifact-like token.
func (p *GradleParser) isPotentialDependencyLine(line string) bool {
	if !gradleDepConfigRegex.MatchString(line) {
		return false
	}
	return (strings.Contains(line, "'") || strings.Contains(line, `"`)) && strings.Contains(line, ":")
}

// parseGradleDependency parses a single Gradle dependency line
func (p *GradleParser) parseGradleDependency(line string) *types.Dependency {
	// Extract dependency type using pre-compiled regex
	depTypeMatch := gradleDepTypeRegex.FindStringSubmatch(line)
	if len(depTypeMatch) < 2 {
		return nil
	}
	depType := depTypeMatch[1]

	// Extract the quoted dependency string using pre-compiled regex
	quotedMatch := gradleQuotedRegex.FindStringSubmatch(line)
	if len(quotedMatch) < 2 {
		return nil
	}

	// Parse the dependency parts
	depString := quotedMatch[1]
	parts := strings.Split(depString, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}

	group := parts[0]
	artifact := parts[1]
	version := "latest"
	classifier := ""
	extension := ""

	// Handle different parts of the dependency notation
	if len(parts) >= 3 && parts[2] != "" {
		version = parts[2]
	}
	if len(parts) >= 4 && parts[3] != "" {
		classifier = parts[3]
	}
	if len(parts) >= 5 && parts[4] != "" {
		extension = parts[4]
	}

	dependencyName := group + ":" + artifact

	// Map Gradle dependency types to scope constants
	var scope string
	switch depType {
	case "testImplementation", "testRuntimeOnly", "testCompileOnly", "testApi":
		scope = types.ScopeDev
	case "compileOnly", "annotationProcessor":
		scope = types.ScopeBuild
	case "implementation", "compile", "api", "runtimeOnly":
		scope = types.ScopeProd
	default:
		scope = types.ScopeProd
	}

	return &types.Dependency{
		Type:     DependencyTypeGradle,
		Name:     dependencyName,
		Version:  version,
		Scope:    scope,
		Direct:   true, // All Gradle dependencies are direct (from build.gradle)
		Metadata: p.buildGradleMetadata(depType, classifier, extension),
	}
}

// buildGradleMetadata creates metadata map for Gradle dependencies
func (p *GradleParser) buildGradleMetadata(depType, classifier, extension string) map[string]interface{} {
	metadata := types.NewMetadata(MetadataSourceBuildGradle)

	// Add Gradle configuration type (implementation, api, etc.)
	if depType != "" {
		metadata["configuration"] = depType
	}

	// Add classifier if present (e.g., sources, javadoc)
	if classifier != "" {
		metadata["classifier"] = classifier
	}

	// Add extension/type if not default jar
	if extension != "" && extension != "jar" {
		metadata["type"] = extension
	}

	return metadata
}
