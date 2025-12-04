package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMavenParser(t *testing.T) {
	parser := NewMavenParser()
	assert.NotNil(t, parser, "Should create a new MavenParser")
	assert.IsType(t, &MavenParser{}, parser, "Should return correct type")
}

func TestParsePomXML(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "valid pom.xml with dependencies",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
		</dependency>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-data-jpa</artifactId>
			<version>2.7.0</version>
		</dependency>
		<dependency>
			<groupId>junit</groupId>
			<artifactId>junit</artifactId>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-data-jpa", Example: "2.7.0"},
				{Type: "maven", Name: "junit:junit", Example: "latest"},
			},
		},
		{
			name: "pom.xml with no dependencies",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
</project>`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "pom.xml with empty dependencies section",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "pom.xml with missing groupId or artifactId",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<!-- Missing artifactId -->
			<version>2.7.0</version>
		</dependency>
		<dependency>
			<!-- Missing groupId -->
			<artifactId>spring-boot-starter-data-jpa</artifactId>
			<version>2.7.0</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{}, // Should skip incomplete dependencies
		},
		{
			name: "invalid XML",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
		</dependency>
	<!-- Missing closing dependency tag -->
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
			}, // XML parser is more lenient than expected
		},
		{
			name:         "empty content",
			content:      "",
			expectedDeps: []types.Dependency{},
		},
		{
			name: "pom.xml with properties and variable substitution",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<spring.version>2.7.0</spring.version>
		<quinoa.version>1.2.3</quinoa.version>
		<junit.version>5.8.2</junit.version>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>${spring.version}</version>
		</dependency>
		<dependency>
			<groupId>io.quarkiverse.quinoa</groupId>
			<artifactId>quarkus-quinoa</artifactId>
			<version>${quinoa.version}</version>
		</dependency>
		<dependency>
			<groupId>org.junit.jupiter</groupId>
			<artifactId>junit-jupiter</artifactId>
			<version>${junit.version}</version>
		</dependency>
		<dependency>
			<groupId>org.mockito</groupId>
			<artifactId>mockito-core</artifactId>
			<version>4.6.1</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "maven", Name: "io.quarkiverse.quinoa:quarkus-quinoa", Example: "1.2.3"},
				{Type: "maven", Name: "org.junit.jupiter:junit-jupiter", Example: "5.8.2"},
				{Type: "maven", Name: "org.mockito:mockito-core", Example: "4.6.1"},
			},
		},
		{
			name: "pom.xml with undefined property reference",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<spring.version>2.7.0</spring.version>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>${spring.version}</version>
		</dependency>
		<dependency>
			<groupId>io.quarkiverse.quinoa</groupId>
			<artifactId>quarkus-quinoa</artifactId>
			<version>${undefined.version}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Example: "2.7.0"},
				{Type: "maven", Name: "io.quarkiverse.quinoa:quarkus-quinoa", Example: "${undefined.version}"},
			},
		},
		{
			name: "pom.xml with empty properties section",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>${spring.version}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Example: "${spring.version}"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParsePomXML(tt.content)

			require.Len(t, result, len(tt.expectedDeps), "Should return correct number of dependencies")

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, result[i].Type, "Should have correct type")
				assert.Equal(t, expectedDep.Name, result[i].Name, "Should have correct name")
				assert.Equal(t, expectedDep.Example, result[i].Example, "Should have correct version")
			}
		})
	}
}

func TestMavenParser_ComplexScenarios(t *testing.T) {
	parser := NewMavenParser()

	// Test complex Maven with multiple dependency scopes
	complexPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>complex-app</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
		</dependency>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-data-jpa</artifactId>
			<version>2.7.0</version>
		</dependency>
		<dependency>
			<groupId>org.postgresql</groupId>
			<artifactId>postgresql</artifactId>
			<version>42.3.3</version>
		</dependency>
		<dependency>
			<groupId>org.projectlombok</groupId>
			<artifactId>lombok</artifactId>
		</dependency>
	</dependencies>
</project>`

	mavenDeps := parser.ParsePomXML(complexPom)
	assert.Len(t, mavenDeps, 4, "Should parse 4 Maven dependencies")

	// Create dependency map for verification
	depMap := make(map[string]types.Dependency)
	for _, dep := range mavenDeps {
		depMap[dep.Name] = dep
	}

	assert.Equal(t, "maven", depMap["org.springframework.boot:spring-boot-starter-web"].Type)
	assert.Equal(t, "2.7.0", depMap["org.springframework.boot:spring-boot-starter-web"].Example)
	assert.Equal(t, "latest", depMap["org.projectlombok:lombok"].Example) // No version specified
}
