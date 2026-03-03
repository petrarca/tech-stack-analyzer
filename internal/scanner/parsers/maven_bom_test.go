package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMavenBOMImport(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "BOM import with scope=import and type=pom",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.springframework.boot</groupId>
				<artifactId>spring-boot-dependencies</artifactId>
				<version>2.7.0</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>main-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:main-dep", Version: "1.0.0", Scope: types.ScopeProd},
				{Type: "maven", Name: "org.springframework.boot:spring-boot-dependencies", Version: "2.7.0", Scope: types.ScopeImport},
			},
		},
		{
			name: "multiple BOM imports",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.springframework.boot</groupId>
				<artifactId>spring-boot-dependencies</artifactId>
				<version>2.7.0</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
			<dependency>
				<groupId>org.springframework.cloud</groupId>
				<artifactId>spring-cloud-dependencies</artifactId>
				<version>2021.0.3</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-dependencies", Version: "2.7.0", Scope: types.ScopeImport},
				{Type: "maven", Name: "org.springframework.cloud:spring-cloud-dependencies", Version: "2021.0.3", Scope: types.ScopeImport},
			},
		},
		{
			name: "BOM with property version",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<spring.boot.version>2.7.0</spring.boot.version>
	</properties>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.springframework.boot</groupId>
				<artifactId>spring-boot-dependencies</artifactId>
				<version>${spring.boot.version}</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-dependencies", Version: "2.7.0", Scope: types.ScopeImport},
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
				assert.Equal(t, expectedDep.Scope, result[i].Scope, "Should have correct scope (import for BOM)")
			}
		})
	}
}

func TestMavenDependencyManagementWithoutImport(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>com.managed</groupId>
				<artifactId>managed-dep</artifactId>
				<version>1.0.0</version>
			</dependency>
			<dependency>
				<groupId>com.managed</groupId>
				<artifactId>test-managed</artifactId>
				<version>2.0.0</version>
				<scope>test</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>main-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`

	result := parser.ParsePomXML(content)

	// Following Maven semantics: dependencyManagement without scope=import doesn't add dependencies
	// It only manages versions for dependencies declared in <dependencies>
	require.Len(t, result, 1, "Should only include main dep, not managed deps without import scope")

	assert.Equal(t, "com.example:main-dep", result[0].Name, "Should have main dependency")
	assert.Equal(t, types.ScopeProd, result[0].Scope, "Should have prod scope")
}

func TestMavenBOMInProfile(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>spring-profile</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencyManagement>
				<dependencies>
					<dependency>
						<groupId>org.springframework.boot</groupId>
						<artifactId>spring-boot-dependencies</artifactId>
						<version>2.7.0</version>
						<type>pom</type>
						<scope>import</scope>
					</dependency>
				</dependencies>
			</dependencyManagement>
		</profile>
	</profiles>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>main-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`

	result := parser.ParsePomXML(content)

	require.Len(t, result, 2, "Should include main dep and BOM from profile")

	// Verify BOM import is present
	bomFound := false
	for _, dep := range result {
		if dep.Name == "org.springframework.boot:spring-boot-dependencies" {
			bomFound = true
			assert.Equal(t, types.ScopeImport, dep.Scope, "Should have import scope")
			assert.Equal(t, "2.7.0", dep.Version, "Should have correct version")
		}
	}

	assert.True(t, bomFound, "Should find BOM import from active profile")
}

func TestMavenBOMWithMixedDependencies(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.springframework.boot</groupId>
				<artifactId>spring-boot-dependencies</artifactId>
				<version>2.7.0</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
			<dependency>
				<groupId>com.managed</groupId>
				<artifactId>regular-managed</artifactId>
				<version>1.0.0</version>
			</dependency>
			<dependency>
				<groupId>com.managed</groupId>
				<artifactId>provided-managed</artifactId>
				<version>2.0.0</version>
				<scope>provided</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>main-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`

	result := parser.ParsePomXML(content)

	// Following Maven semantics: only BOM imports (scope=import) are included from dependencyManagement
	// Regular managed dependencies are just version constraints, not actual dependencies
	require.Len(t, result, 2, "Should include main dep and BOM import only")

	// Verify we have the BOM import and main dependency
	bomFound := false
	mainFound := false
	for _, dep := range result {
		if dep.Name == "org.springframework.boot:spring-boot-dependencies" && dep.Scope == types.ScopeImport {
			bomFound = true
		}
		if dep.Name == "com.example:main-dep" && dep.Scope == types.ScopeProd {
			mainFound = true
		}
	}

	assert.True(t, bomFound, "Should have BOM import")
	assert.True(t, mainFound, "Should have main dependency")
}
