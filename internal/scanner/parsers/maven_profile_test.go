package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMavenProfileActiveByDefault(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "profile with activeByDefault=true",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>default-profile</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>profile-dep</artifactId>
					<version>2.0.0</version>
				</dependency>
			</dependencies>
		</profile>
	</profiles>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>main-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.profile:profile-dep", Version: "2.0.0", Scope: types.ScopeProd},
				{Type: "maven", Name: "com.example:main-dep", Version: "1.0.0", Scope: types.ScopeProd},
			},
		},
		{
			name: "profile with activeByDefault=false (not activated)",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>inactive-profile</id>
			<activation>
				<activeByDefault>false</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>should-not-appear</artifactId>
					<version>2.0.0</version>
				</dependency>
			</dependencies>
		</profile>
	</profiles>
	
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
			},
		},
		{
			name: "no profiles",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
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
				assert.Equal(t, expectedDep.Scope, result[i].Scope, "Should have correct scope")
			}
		})
	}
}

func TestMavenProfileMultipleDefaults(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>profile-one</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>dep-one</artifactId>
					<version>1.0.0</version>
				</dependency>
			</dependencies>
		</profile>
		<profile>
			<id>profile-two</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>dep-two</artifactId>
					<version>2.0.0</version>
				</dependency>
			</dependencies>
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

	// Should include dependencies from both active profiles plus main dependencies
	require.Len(t, result, 3, "Should return dependencies from both profiles and main")

	// Verify all expected dependencies are present
	depNames := make(map[string]bool)
	for _, dep := range result {
		depNames[dep.Name] = true
	}

	assert.True(t, depNames["com.profile:dep-one"], "Should include dep-one from profile-one")
	assert.True(t, depNames["com.profile:dep-two"], "Should include dep-two from profile-two")
	assert.True(t, depNames["com.example:main-dep"], "Should include main-dep")
}

func TestMavenProfileWithDependencyManagement(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>managed-profile</id>
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
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>profile-dep</artifactId>
					<version>2.0.0</version>
				</dependency>
			</dependencies>
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

	// Should include profile dependencies, main dependencies, and BOM import from profile dependencyManagement
	require.Len(t, result, 3, "Should include dependencies from profile, main, and BOM import")

	depNames := make(map[string]bool)
	for _, dep := range result {
		depNames[dep.Name] = true
	}

	assert.True(t, depNames["com.profile:profile-dep"], "Should include profile-dep")
	assert.True(t, depNames["com.example:main-dep"], "Should include main-dep")
	assert.True(t, depNames["org.springframework.boot:spring-boot-dependencies"], "Should include BOM import from profile dependencyManagement")
}

func TestMavenProfileWithScopes(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>test-profile</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>test-dep</artifactId>
					<version>1.0.0</version>
					<scope>test</scope>
				</dependency>
				<dependency>
					<groupId>com.profile</groupId>
					<artifactId>provided-dep</artifactId>
					<version>2.0.0</version>
					<scope>provided</scope>
				</dependency>
			</dependencies>
		</profile>
	</profiles>
</project>`

	result := parser.ParsePomXML(content)

	require.Len(t, result, 2, "Should return both profile dependencies")

	// Check scopes are correctly mapped
	for _, dep := range result {
		if dep.Name == "com.profile:test-dep" {
			assert.Equal(t, types.ScopeDev, dep.Scope, "test scope should map to dev")
		}
		if dep.Name == "com.profile:provided-dep" {
			assert.Equal(t, types.ScopeProd, dep.Scope, "provided scope should map to prod")
		}
	}
}
