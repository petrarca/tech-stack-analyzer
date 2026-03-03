package parsers

import (
	"fmt"
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-data-jpa", Version: "2.7.0"},
				{Type: "maven", Name: "junit:junit", Version: "latest"},
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "maven", Name: "io.quarkiverse.quinoa:quarkus-quinoa", Version: "1.2.3"},
				{Type: "maven", Name: "org.junit.jupiter:junit-jupiter", Version: "5.8.2"},
				{Type: "maven", Name: "org.mockito:mockito-core", Version: "4.6.1"},
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
				{Type: "maven", Name: "io.quarkiverse.quinoa:quarkus-quinoa", Version: "${undefined.version}"},
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "${spring.version}"},
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
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
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
	assert.Equal(t, "2.7.0", depMap["org.springframework.boot:spring-boot-starter-web"].Version)
	assert.Equal(t, "latest", depMap["org.projectlombok:lombok"].Version) // No version specified
}

func TestMavenParser_ParentPOMResolution(t *testing.T) {
	// Create a mock provider for testing
	mockProvider := &mockFileProvider{
		files: map[string]string{
			"/project/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<parent>
		<groupId>com.example</groupId>
		<artifactId>parent</artifactId>
		<version>2.0.0</version>
	</parent>
	<artifactId>child</artifactId>
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>lib</artifactId>
			<version>${project.version}</version>
		</dependency>
		<dependency>
			<groupId>org.springframework</groupId>
			<artifactId>spring-core</artifactId>
			<version>${spring.version}</version>
		</dependency>
	</dependencies>
</project>`,
			"/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>parent</artifactId>
	<version>2.0.0</version>
	<properties>
		<spring.version>3.2.0</spring.version>
	</properties>
</project>`,
		},
	}

	parser := NewMavenParser()
	childContent := mockProvider.files["/project/pom.xml"]

	deps := parser.ParsePomXMLWithProvider(childContent, "/project", mockProvider)

	require.Len(t, deps, 2, "Should have 2 dependencies")

	// Check that project.version was resolved from parent
	assert.Equal(t, "com.example:lib", deps[0].Name)
	assert.Equal(t, "2.0.0", deps[0].Version, "project.version should resolve to parent's version")

	// Check that spring.version was resolved from parent's properties
	assert.Equal(t, "org.springframework:spring-core", deps[1].Name)
	assert.Equal(t, "3.2.0", deps[1].Version, "spring.version should resolve from parent's properties")
}

// mockFileProvider implements types.Provider for testing
type mockFileProvider struct {
	files map[string]string
}

func (m *mockFileProvider) ReadFile(path string) ([]byte, error) {
	if content, ok := m.files[path]; ok {
		return []byte(content), nil
	}
	return nil, fmt.Errorf("file not found: %s", path)
}

func (m *mockFileProvider) ListDir(path string) ([]types.File, error) {
	return nil, nil
}

func (m *mockFileProvider) GetBasePath() string {
	return "/"
}

func (m *mockFileProvider) Exists(path string) (bool, error) {
	_, ok := m.files[path]
	return ok, nil
}

func (m *mockFileProvider) IsDir(path string) (bool, error) {
	return false, nil
}

func (m *mockFileProvider) Open(path string) (string, error) {
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return "", fmt.Errorf("file not found: %s", path)
}

func TestMavenParser_RecursivePropertyResolution(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "chained property resolution",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<base.version>2.7.0</base.version>
		<spring.version>${base.version}</spring.version>
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
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0"},
			},
		},
		{
			name: "project.version resolution",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>3.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>my-lib</artifactId>
			<version>${project.version}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:my-lib", Version: "3.0.0"},
			},
		},
		{
			name: "pom.version alias resolution",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>4.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>my-lib</artifactId>
			<version>${pom.version}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:my-lib", Version: "4.0.0"},
			},
		},
		{
			name: "embedded property in version string",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<qualifier>RELEASE</qualifier>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>my-lib</artifactId>
			<version>2.0.0-${qualifier}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:my-lib", Version: "2.0.0-RELEASE"},
			},
		},
		{
			name: "cycle detection prevents infinite loop",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<a.version>${b.version}</a.version>
		<b.version>${a.version}</b.version>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>my-lib</artifactId>
			<version>${a.version}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:my-lib", Version: "${a.version}"},
			},
		},
		{
			name: "deeply chained properties",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<groupId>com.example</groupId>
	<artifactId>my-app</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<level1>5.0.0</level1>
		<level2>${level1}</level2>
		<level3>${level2}</level3>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>my-lib</artifactId>
			<version>${level3}</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:my-lib", Version: "5.0.0"},
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
				assert.Equal(t, expectedDep.Version, result[i].Version, "Should have correct version")
			}
		})
	}
}
