package parsers

import (
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
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
			regex := regexp.MustCompile(`group\s*[=]?\s*['"]([^'"]+)['"]`)
			if match := regex.FindStringSubmatch(line); match != nil {
				info.Group = match[1]
			}
		}

		// Match version = '1.0.0' or version = "1.0.0"
		if strings.HasPrefix(line, "version") && !strings.Contains(line, "sourceCompatibility") {
			regex := regexp.MustCompile(`version\s*[=]?\s*['"]([^'"]+)['"]`)
			if match := regex.FindStringSubmatch(line); match != nil {
				info.Version = match[1]
			}
		}

		// Match rootProject.name = 'name' (typically in settings.gradle)
		if strings.Contains(line, "rootProject.name") {
			regex := regexp.MustCompile(`rootProject\.name\s*=\s*['"]([^'"]+)['"]`)
			if match := regex.FindStringSubmatch(line); match != nil {
				info.Name = match[1]
			}
		}
	}

	return info
}

// shouldSkipLine checks if a line should be skipped during parsing
func (p *GradleParser) shouldSkipLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*")
}

// isPotentialDependencyLine does quick validation before expensive regex matching
func (p *GradleParser) isPotentialDependencyLine(line string) bool {
	// Must contain a dependency type and quoted content with colon
	hasDepType := strings.Contains(line, "implementation") ||
		strings.Contains(line, "compile") ||
		strings.Contains(line, "api") ||
		strings.Contains(line, "runtimeOnly") ||
		strings.Contains(line, "compileOnly") ||
		strings.Contains(line, "annotationProcessor") ||
		strings.Contains(line, "testImplementation") ||
		strings.Contains(line, "testRuntimeOnly")

	hasQuotedContent := (strings.Contains(line, "'") || strings.Contains(line, `"`)) && strings.Contains(line, ":")

	return hasDepType && hasQuotedContent
}

// parseGradleDependency parses a single Gradle dependency line
func (p *GradleParser) parseGradleDependency(line string) *types.Dependency {
	// Supported dependency types
	depTypes := []string{
		"implementation", "compile", "testImplementation", "api",
		"compileOnly", "runtimeOnly", "testRuntimeOnly", "annotationProcessor",
	}

	// Extract dependency type
	depTypeRegex := regexp.MustCompile(`^\s*(` + strings.Join(depTypes, "|") + `)`)
	depTypeMatch := depTypeRegex.FindStringSubmatch(line)
	if len(depTypeMatch) < 2 {
		return nil
	}

	// Extract the quoted dependency string
	quotedRegex := regexp.MustCompile(`['"]([^'"]+)['"]`)
	quotedMatch := quotedRegex.FindStringSubmatch(line)
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

	if len(parts) >= 3 && parts[2] != "" {
		version = parts[2]
	}

	dependencyName := group + ":" + artifact

	return &types.Dependency{
		Type:    "gradle",
		Name:    dependencyName,
		Version: version,
	}
}
