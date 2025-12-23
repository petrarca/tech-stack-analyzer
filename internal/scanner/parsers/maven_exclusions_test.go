package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMavenDependencyExclusions(t *testing.T) {
	parser := NewMavenParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "dependency with exclusions",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
			<exclusions>
				<exclusion>
					<groupId>org.springframework.boot</groupId>
					<artifactId>spring-boot-starter-tomcat</artifactId>
				</exclusion>
			</exclusions>
		</dependency>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>other-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0", Scope: types.ScopeProd},
				{Type: "maven", Name: "com.example:other-dep", Version: "1.0.0", Scope: types.ScopeProd},
			},
		},
		{
			name: "dependency with multiple exclusions",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
			<exclusions>
				<exclusion>
					<groupId>org.springframework.boot</groupId>
					<artifactId>spring-boot-starter-tomcat</artifactId>
				</exclusion>
				<exclusion>
					<groupId>org.springframework.boot</groupId>
					<artifactId>spring-boot-starter-logging</artifactId>
				</exclusion>
			</exclusions>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0", Scope: types.ScopeProd},
			},
		},
		{
			name: "wildcard exclusion",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>org.springframework.boot</groupId>
			<artifactId>spring-boot-starter-web</artifactId>
			<version>2.7.0</version>
			<exclusions>
				<exclusion>
					<groupId>*</groupId>
					<artifactId>*</artifactId>
				</exclusion>
			</exclusions>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "org.springframework.boot:spring-boot-starter-web", Version: "2.7.0", Scope: types.ScopeProd},
			},
		},
		{
			name: "dependency without exclusions",
			content: `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<dependencies>
		<dependency>
			<groupId>com.example</groupId>
			<artifactId>simple-dep</artifactId>
			<version>1.0.0</version>
		</dependency>
	</dependencies>
</project>`,
			expectedDeps: []types.Dependency{
				{Type: "maven", Name: "com.example:simple-dep", Version: "1.0.0", Scope: types.ScopeProd},
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

func TestMavenExclusionsInProfiles(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	
	<profiles>
		<profile>
			<id>exclude-profile</id>
			<activation>
				<activeByDefault>true</activeByDefault>
			</activation>
			<dependencies>
				<dependency>
					<groupId>org.springframework.boot</groupId>
					<artifactId>spring-boot-starter-web</artifactId>
					<version>2.7.0</version>
					<exclusions>
						<exclusion>
							<groupId>org.springframework.boot</groupId>
							<artifactId>spring-boot-starter-tomcat</artifactId>
						</exclusion>
					</exclusions>
				</dependency>
			</dependencies>
		</profile>
	</profiles>
</project>`

	result := parser.ParsePomXML(content)

	require.Len(t, result, 1, "Should have one dependency from active profile")
	assert.Equal(t, "org.springframework.boot:spring-boot-starter-web", result[0].Name)
	assert.Equal(t, "2.7.0", result[0].Version)
}

func TestMavenExclusionsInPlugins(t *testing.T) {
	parser := NewMavenParser()

	content := `<?xml version="1.0"?>
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
						<exclusions>
							<exclusion>
								<groupId>org.codehaus.plexus</groupId>
								<artifactId>plexus-utils</artifactId>
							</exclusion>
						</exclusions>
					</dependency>
				</dependencies>
			</plugin>
		</plugins>
	</build>
</project>`

	result := parser.ParsePomXML(content)

	require.Len(t, result, 1, "Should have one plugin dependency")
	assert.Equal(t, "org.codehaus.plexus:plexus-compiler-javac", result[0].Name)
	assert.Equal(t, types.ScopeBuild, result[0].Scope)
}
