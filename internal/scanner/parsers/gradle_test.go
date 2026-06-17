package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGradleParser(t *testing.T) {
	parser := NewGradleParser()
	assert.NotNil(t, parser, "Should create a new GradleParser")
	assert.IsType(t, &GradleParser{}, parser, "Should return correct type")
}

func TestParseGradle(t *testing.T) {
	parser := NewGradleParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "standard Gradle dependencies",
			content: `plugins {
	id 'java'
	id 'org.springframework.boot' version '2.7.0'
}

dependencies {
	implementation 'org.springframework.boot:spring-boot-starter-web:2.7.0'
	implementation 'org.springframework.boot:spring-boot-starter-data-jpa:2.7.0'
	compile 'junit:junit:4.13.2'
	testImplementation 'org.mockito:mockito-core:4.6.1'
	api 'com.google.guava:guava:31.1-jre'
	compileOnly 'org.projectlombok:lombok:1.18.24'
	runtimeOnly 'mysql:mysql-connector-java:8.0.29'
	testRuntimeOnly 'org.junit.jupiter:junit-jupiter-engine:5.8.2'
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-data-jpa", Version: "2.7.0"},
				{Type: "gradle", Name: "junit:junit", Version: "4.13.2"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Version: "4.6.1"},
				{Type: "gradle", Name: "com.google.guava:guava", Version: "31.1-jre"},
				{Type: "gradle", Name: "org.projectlombok:lombok", Version: "1.18.24"},
				{Type: "gradle", Name: "mysql:mysql-connector-java", Version: "8.0.29"},
				{Type: "gradle", Name: "org.junit.jupiter:junit-jupiter-engine", Version: "5.8.2"},
			},
		},
		{
			name: "Gradle with parentheses notation",
			content: `dependencies {
	implementation("org.springframework.boot:spring-boot-starter-web:2.7.0")
	compile("junit:junit:4.13.2")
	testImplementation("org.mockito:mockito-core:4.6.1")
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "gradle", Name: "junit:junit", Version: "4.13.2"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Version: "4.6.1"},
			},
		},
		{
			name: "Gradle dependencies without versions",
			content: `dependencies {
	implementation 'org.springframework.boot:spring-boot-starter-web'
	compile 'junit:junit'
	testImplementation 'org.mockito:mockito-core'
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "latest"},
				{Type: "gradle", Name: "junit:junit", Version: "latest"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Version: "latest"},
			},
		},
		{
			name: "Gradle with comments and empty lines",
			content: `// Spring Boot dependencies
dependencies {
	implementation 'org.springframework.boot:spring-boot-starter-web:2.7.0'
	
	/* Test dependencies */
	testImplementation 'org.mockito:mockito-core:4.6.1'
	// JUnit for testing
	compile 'junit:junit:4.13.2'
	
	* Another comment
	api 'com.google.guava:guava:31.1-jre'
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Version: "4.6.1"},
				{Type: "gradle", Name: "junit:junit", Version: "4.13.2"},
				{Type: "gradle", Name: "com.google.guava:guava", Version: "31.1-jre"},
			},
		},
		{
			name: "Gradle with no dependencies",
			content: `plugins {
	id 'java'
}

repositories {
	mavenCentral()
}`,
			expectedDeps: []types.Dependency{},
		},
		{
			name:         "empty Gradle file",
			content:      "",
			expectedDeps: []types.Dependency{},
		},
		{
			name: "Gradle with invalid dependency format",
			content: `dependencies {
	implementation 'invalid-dependency-format'
	compile 'another-invalid'
}`,
			expectedDeps: []types.Dependency{}, // Should not match invalid format
		},
		{
			name: "Kotlin DSL (build.gradle.kts)",
			content: `plugins {
	java
	id("org.springframework.boot") version "2.7.0"
}

dependencies {
	implementation("org.springframework.boot:spring-boot-starter-web:2.7.0")
	testImplementation("org.junit.jupiter:junit-jupiter:5.8.2")
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "gradle", Name: "org.junit.jupiter:junit-jupiter", Version: "5.8.2"},
			},
		},
		{
			name: "Gradle with annotationProcessor",
			content: `dependencies {
	implementation 'org.springframework.boot:spring-boot-starter-web:2.7.0'
	annotationProcessor 'org.projectlombok:lombok:1.18.24'
	testAnnotationProcessor 'org.projectlombok:lombok:1.18.24'
}`,
			expectedDeps: []types.Dependency{
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "gradle", Name: "org.projectlombok:lombok", Version: "1.18.24"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseGradle(tt.content)

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
			}
		})
	}
}

func TestGradleParser_ComplexScenarios(t *testing.T) {
	parser := NewGradleParser()

	// Test complex Gradle with various configurations
	complexGradle := `plugins {
	id 'java'
	id 'org.springframework.boot' version '2.7.0'
}

dependencies {
	// Spring Boot starters
	implementation('org.springframework.boot:spring-boot-starter-web:2.7.0')
	implementation('org.springframework.boot:spring-boot-starter-data-jpa:2.7.0')
	
	// Database
	runtimeOnly 'org.postgresql:postgresql:42.3.3'
	
	// Testing
	testImplementation 'org.springframework.boot:spring-boot-starter-test:2.7.0'
	testImplementation 'org.mockito:mockito-core:4.6.1'
	
	// Utilities
	compileOnly 'org.projectlombok:lombok:1.18.24'
	annotationProcessor 'org.projectlombok:lombok:1.18.24'
	
	// API dependencies
	api 'com.google.guava:guava:31.1-jre'
}`

	gradleDeps := parser.ParseGradle(complexGradle)
	assert.Len(t, gradleDeps, 8, "Should parse 8 Gradle dependencies including annotationProcessor")

	// Verify specific dependencies
	gradleDepMap := make(map[string]types.Dependency)
	for _, dep := range gradleDeps {
		gradleDepMap[dep.Name] = dep
	}

	assert.Equal(t, "gradle", gradleDepMap["org.postgresql:postgresql"].Type)
	assert.Equal(t, "42.3.3", gradleDepMap["org.postgresql:postgresql"].Version)
	assert.Equal(t, "gradle", gradleDepMap["org.projectlombok:lombok"].Type)
	assert.Equal(t, "1.18.24", gradleDepMap["org.projectlombok:lombok"].Version)
}

func TestParsePlugins(t *testing.T) {
	parser := NewGradleParser()

	// pluginMap builds an id->version map from a slice for order-independent assertions.
	pluginMap := func(plugins []GradlePlugin) map[string]string {
		m := make(map[string]string, len(plugins))
		for _, p := range plugins {
			m[p.ID] = p.Version
		}
		return m
	}

	tests := []struct {
		name     string
		content  string
		expected map[string]string // id -> version
	}{
		{
			name: "Kotlin DSL with id() and version",
			content: `plugins {
    kotlin("jvm") version "2.1.10"
    kotlin("plugin.spring") version "2.1.10"
    id("org.springframework.boot") version "3.4.3"
    id("io.spring.dependency-management") version "1.1.7"
}`,
			expected: map[string]string{
				"org.jetbrains.kotlin.jvm":           "2.1.10",
				"org.jetbrains.kotlin.plugin.spring": "2.1.10",
				"org.springframework.boot":           "3.4.3",
				"io.spring.dependency-management":    "1.1.7",
			},
		},
		{
			name: "Groovy DSL without parens",
			content: `plugins {
    id 'org.springframework.boot' version '2.7.0'
    id 'java'
}`,
			expected: map[string]string{
				"org.springframework.boot": "2.7.0",
				"java":                     "",
			},
		},
		{
			name: "kotlin() without version (version catalog)",
			content: `plugins {
    kotlin("jvm")
    id("io.quarkus")
}`,
			expected: map[string]string{
				"org.jetbrains.kotlin.jvm": "",
				"io.quarkus":               "",
			},
		},
		{
			name:     "no plugins block",
			content:  `dependencies { implementation("com.example:lib:1.0") }`,
			expected: map[string]string{},
		},
		{
			name: "duplicate plugin IDs deduplicated",
			content: `plugins {
    id("org.springframework.boot") version "3.4.3"
    id("org.springframework.boot") version "3.4.3"
}`,
			expected: map[string]string{
				"org.springframework.boot": "3.4.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pluginMap(parser.ParsePlugins(tt.content))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseGradleProperties(t *testing.T) {
	content := "# comment\nguavaVersion=31.1-jre\n  spring.version = 5.3.20  \n! bang comment\nempty=\n"
	props := ParseGradleProperties(content)
	assert.Equal(t, "31.1-jre", props["guavaVersion"])
	assert.Equal(t, "5.3.20", props["spring.version"])
	assert.Equal(t, "", props["empty"])
	if _, ok := props["# comment"]; ok {
		t.Errorf("comment line should not be parsed as property")
	}
}

func TestExtractGradleInlineProperties(t *testing.T) {
	content := `ext {
    jacksonVersion = "2.15.0"
}
ext.kotlinVersion = "1.9.0"
val ktorVersion = "2.3.0"
def jjwtVersion = "0.11.5"
`
	props := ExtractGradleInlineProperties(content)
	assert.Equal(t, "2.15.0", props["jacksonVersion"])
	assert.Equal(t, "1.9.0", props["kotlinVersion"])
	assert.Equal(t, "2.3.0", props["ktorVersion"])
	assert.Equal(t, "0.11.5", props["jjwtVersion"])
}

func TestParseGradleWithProperties_Interpolation(t *testing.T) {
	content := `ext {
    jacksonVersion = "2.15.0"
}
val ktorVersion = "2.3.0"
dependencies {
    implementation 'org.springframework:spring-core:5.3.20'
    implementation "com.google.guava:guava:$guavaVersion"
    implementation "io.ktor:ktor-server:$ktorVersion"
    implementation "com.fasterxml.jackson.core:jackson-databind:${jacksonVersion}"
    implementation "com.example:unresolved:$missingVersion"
}
`
	extProps := map[string]string{"guavaVersion": "31.1-jre"}
	deps := NewGradleParser().ParseGradleWithProperties(content, extProps)

	got := map[string]string{}
	for _, d := range deps {
		got[d.Name] = d.Version
	}
	assert.Equal(t, "5.3.20", got["org.springframework:spring-core"])
	assert.Equal(t, "31.1-jre", got["com.google.guava:guava"])
	assert.Equal(t, "2.3.0", got["io.ktor:ktor-server"])
	assert.Equal(t, "2.15.0", got["com.fasterxml.jackson.core:jackson-databind"])
	// Unresolved references are left intact (the SBOM emitter omits them).
	assert.Equal(t, "$missingVersion", got["com.example:unresolved"])
}

// TestParseGradle_PlatformBom verifies that platform()/enforcedPlatform()
// declarations are recognized as BOM imports (ScopeImport) while the
// dependencies they manage are emitted without a version (to be backfilled by
// the detector). Fixture uses fictional coordinates.
func TestParseGradle_PlatformBom(t *testing.T) {
	content := `dependencies {
    implementation(enforcedPlatform("com.example:my-bom:3.36.2"))
    implementation(platform("org.example:other-bom:1.0.0"))
    implementation("com.example:managed-lib")
    implementation("com.example:pinned-lib:9.9.9")
    testImplementation("com.example:test-lib")
}
`
	deps := NewGradleParser().ParseGradle(content)
	byName := map[string]types.Dependency{}
	for _, d := range deps {
		byName[d.Name] = d
	}

	// Platform BOMs are marked ScopeImport and keep their coordinate version.
	if d := byName["com.example:my-bom"]; d.Scope != types.ScopeImport || d.Version != "3.36.2" {
		t.Errorf("enforcedPlatform: got scope=%q version=%q, want import/3.36.2", d.Scope, d.Version)
	}
	if d := byName["org.example:other-bom"]; d.Scope != types.ScopeImport || d.Version != "1.0.0" {
		t.Errorf("platform: got scope=%q version=%q, want import/1.0.0", d.Scope, d.Version)
	}
	// Managed (versionless) dep keeps the "latest" placeholder for backfill.
	if d := byName["com.example:managed-lib"]; d.Scope == types.ScopeImport {
		t.Errorf("managed-lib should not be ScopeImport, got %q", d.Scope)
	}
	if got := byName["com.example:managed-lib"].Version; got != "latest" {
		t.Errorf("managed-lib version: got %q, want latest (unresolved)", got)
	}
	// Pinned dep keeps its explicit version and is not an import.
	if d := byName["com.example:pinned-lib"]; d.Version != "9.9.9" || d.Scope == types.ScopeImport {
		t.Errorf("pinned-lib: got version=%q scope=%q", d.Version, d.Scope)
	}
}

// TestParseGradleLockfile verifies parsing of a Gradle dependency-lock file:
// resolved coordinates, dev-only classification by configuration, and skipping
// of comments and the trailing "empty=" line. Fixture uses fictional names.
func TestParseGradleLockfile(t *testing.T) {
	content := `# This is a Gradle generated file for dependency locking.
# Manual edits can break the build and are not advised.
com.example:lib-a:1.2.3=compileClasspath,runtimeClasspath
com.example:lib-b:4.5.6=testCompileClasspath,testRuntimeClasspath
org.example:tool:7.8.9=runtimeClasspath
empty=annotationProcessor,kapt
`
	deps := ParseGradleLockfile(content)
	byName := map[string]types.Dependency{}
	for _, d := range deps {
		byName[d.Name] = d
	}

	if len(deps) != 3 {
		t.Fatalf("got %d deps, want 3: %v", len(deps), byName)
	}
	if d := byName["com.example:lib-a"]; d.Version != "1.2.3" || d.Scope != types.ScopeProd {
		t.Errorf("lib-a: got version=%q scope=%q, want 1.2.3/prod", d.Version, d.Scope)
	}
	// Appears only in test configurations -> dev.
	if d := byName["com.example:lib-b"]; d.Version != "4.5.6" || d.Scope != types.ScopeDev {
		t.Errorf("lib-b: got version=%q scope=%q, want 4.5.6/dev", d.Version, d.Scope)
	}
	if d := byName["org.example:tool"]; d.Version != "7.8.9" {
		t.Errorf("tool: got version=%q, want 7.8.9", d.Version)
	}
	if _, ok := byName["empty"]; ok {
		t.Error("the empty= line must not produce a dependency")
	}
}
