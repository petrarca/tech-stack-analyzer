package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMavenPluginDependencies(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "plugin with dependencies",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<build>
		<plugins>
			<plugin>
				<groupId>org.apache.maven.plugins</groupId>
				<artifactId>maven-compiler-plugin</artifactId>
				<version>3.8.1</version>
				<dependencies>
					<dependency>
						<groupId>org.codehaus.plexus</groupId>
						<artifactId>plexus-compiler-javac</artifactId>
						<version>2.8.8</version>
					</dependency>
				</dependencies>
			</plugin>
		</plugins>
	</build>
	
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
				{Type: "maven", Name: "org.codehaus.plexus:plexus-compiler-javac", Version: "2.8.8", Scope: types.ScopeBuild},
			},
		},
		{
			name: "multiple plugins with dependencies",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<build>
		<plugins>
			<plugin>
				<groupId>org.apache.maven.plugins</groupId>
				<artifactId>maven-compiler-plugin</artifactId>
				<version>3.8.1</version>
				<dependencies>
					<dependency>
						<groupId>org.codehaus.plexus</groupId>
						<artifactId>plexus-compiler-javac</artifactId>
						<version>2.8.8</version>
					</dependency>
				</dependencies>
			</plugin>
			<plugin>
				<groupId>org.apache.maven.plugins</groupId>
				<artifactId>maven-surefire-plugin</artifactId>
				<version>2.22.2</version>
				<dependencies>
					<dependency>
						<groupId>org.junit.platform</groupId>
						<artifactId>junit-platform-surefire-provider</artifactId>
						<version>1.3.2</version>
					</dependency>
				</dependencies>
			</plugin>
		</plugins>
	</build>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.codehaus.plexus:plexus-compiler-javac", Version: "2.8.8", Scope: types.ScopeBuild},
				{Type: "maven", Name: "org.junit.platform:junit-platform-surefire-provider", Version: "1.3.2", Scope: types.ScopeBuild},
			},
		},
		{
			name: "plugin without dependencies",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<build>
		<plugins>
			<plugin>
				<groupId>org.apache.maven.plugins</groupId>
				<artifactId>maven-clean-plugin</artifactId>
				<version>3.1.0</version>
			</plugin>
		</plugins>
	</build>
	
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
			name: "no build section",
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

func TestMavenPluginDependenciesWithProperties(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<properties>
		<plexus.version>2.8.8</plexus.version>
	</properties>
	
	<build>
		<plugins>
			<plugin>
				<groupId>org.apache.maven.plugins</groupId>
				<artifactId>maven-compiler-plugin</artifactId>
				<version>3.8.1</version>
				<dependencies>
					<dependency>
						<groupId>org.codehaus.plexus</groupId>
						<artifactId>plexus-compiler-javac</artifactId>
						<version>${plexus.version}</version>
					</dependency>
				</dependencies>
			</plugin>
		</plugins>
	</build>
</project>`

	result := parser.ParsePomXML(content)

	require.Len(t, result, 1, "Should have one plugin dependency")
	assert.Equal(t, "org.codehaus.plexus:plexus-compiler-javac", result[0].Name)
	assert.Equal(t, "2.8.8", result[0].Version, "Should resolve property in plugin dependency version")
	assert.Equal(t, types.ScopeBuild, result[0].Scope, "Plugin dependencies should have build scope")
}
