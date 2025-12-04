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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-data-jpa", Example: "2.7.0"},
				{Type: "gradle", Name: "junit:junit", Example: "4.13.2"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Example: "4.6.1"},
				{Type: "gradle", Name: "com.google.guava:guava", Example: "31.1-jre"},
				{Type: "gradle", Name: "org.projectlombok:lombok", Example: "1.18.24"},
				{Type: "gradle", Name: "mysql:mysql-connector-java", Example: "8.0.29"},
				{Type: "gradle", Name: "org.junit.jupiter:junit-jupiter-engine", Example: "5.8.2"},
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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "gradle", Name: "junit:junit", Example: "4.13.2"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Example: "4.6.1"},
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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "latest"},
				{Type: "gradle", Name: "junit:junit", Example: "latest"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Example: "latest"},
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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "gradle", Name: "org.mockito:mockito-core", Example: "4.6.1"},
				{Type: "gradle", Name: "junit:junit", Example: "4.13.2"},
				{Type: "gradle", Name: "com.google.guava:guava", Example: "31.1-jre"},
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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "gradle", Name: "org.junit.jupiter:junit-jupiter", Example: "5.8.2"},
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
				{Type: "gradle", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "gradle", Name: "org.projectlombok:lombok", Example: "1.18.24"},
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
				assert.Equal(t, expectedDep.Example, result[i].Example, "Should have correct version")
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
	assert.Equal(t, "42.3.3", gradleDepMap["org.postgresql:postgresql"].Example)
	assert.Equal(t, "gradle", gradleDepMap["org.projectlombok:lombok"].Type)
	assert.Equal(t, "1.18.24", gradleDepMap["org.projectlombok:lombok"].Example)
}
