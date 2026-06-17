package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// depByName indexes a dependency slice by name for assertions.
func depByName(deps []types.Dependency) map[string]types.Dependency {
	m := make(map[string]types.Dependency, len(deps))
	for _, d := range deps {
		m[d.Name] = d
	}
	return m
}

func TestMavenParser_IntraPomDependencyManagement(t *testing.T) {
	// A versionless <dependency> takes its version from <dependencyManagement>
	// in the same POM, including via a property reference.
	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>app</artifactId>
	<version>1.0.0</version>
	<properties>
		<hibernate.version>6.4.1.Final</hibernate.version>
	</properties>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.hibernate</groupId>
				<artifactId>hibernate-core</artifactId>
				<version>${hibernate.version}</version>
			</dependency>
		</dependencies>
	</dependencyManagement>
	<dependencies>
		<dependency>
			<groupId>org.hibernate</groupId>
			<artifactId>hibernate-core</artifactId>
		</dependency>
	</dependencies>
</project>`

	deps := NewMavenParser().ParsePomXML(pom)
	byName := depByName(deps)

	hib, ok := byName["org.hibernate:hibernate-core"]
	require.True(t, ok)
	assert.Equal(t, "6.4.1.Final", hib.Version, "version should come from dependencyManagement")
	assert.Equal(t, "dependency-management", hib.Metadata["source"])
	assert.Equal(t, "latest", hib.Metadata[types.MetadataKeyDeclared])
}

func TestMavenParser_CrossPomDependencyManagement(t *testing.T) {
	// The child declares a versionless dependency; its managed version lives in
	// a parent BOM POM, resolved through a property defined in that parent.
	provider := &mockFileProvider{
		files: map[string]string{
			"/project/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<parent>
		<groupId>com.example</groupId>
		<artifactId>parent-bom</artifactId>
		<version>2.0.0</version>
	</parent>
	<artifactId>child</artifactId>
	<dependencies>
		<dependency>
			<groupId>ca.uhn.hapi.fhir</groupId>
			<artifactId>hapi-fhir-base</artifactId>
		</dependency>
	</dependencies>
</project>`,
			"/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>parent-bom</artifactId>
	<version>2.0.0</version>
	<properties>
		<fhir.version>7.6.0</fhir.version>
	</properties>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>ca.uhn.hapi.fhir</groupId>
				<artifactId>hapi-fhir-base</artifactId>
				<version>${fhir.version}</version>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`,
		},
	}

	childContent := provider.files["/project/pom.xml"]
	deps := NewMavenParser().ParsePomXMLWithProvider(childContent, "/project", provider)
	byName := depByName(deps)

	hapi, ok := byName["ca.uhn.hapi.fhir:hapi-fhir-base"]
	require.True(t, ok)
	assert.Equal(t, "7.6.0", hapi.Version, "version should resolve from parent BOM dependencyManagement")
	assert.Equal(t, "dependency-management", hapi.Metadata["source"])
}

func TestMavenParser_DependencyManagement_DoesNotOverwriteConcrete(t *testing.T) {
	// A concrete inline version must win over a managed version.
	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>app</artifactId>
	<version>1.0.0</version>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.hibernate</groupId>
				<artifactId>hibernate-core</artifactId>
				<version>6.4.1.Final</version>
			</dependency>
		</dependencies>
	</dependencyManagement>
	<dependencies>
		<dependency>
			<groupId>org.hibernate</groupId>
			<artifactId>hibernate-core</artifactId>
			<version>5.6.0.Final</version>
		</dependency>
	</dependencies>
</project>`

	deps := NewMavenParser().ParsePomXML(pom)
	byName := depByName(deps)

	hib, ok := byName["org.hibernate:hibernate-core"]
	require.True(t, ok)
	assert.Equal(t, "5.6.0.Final", hib.Version, "inline concrete version must not be overwritten")
	assert.Nil(t, hib.Metadata["source"])
}

func TestMavenParser_DependencyManagement_UnresolvableStaysVersionless(t *testing.T) {
	// A versionless dependency with no managed entry anywhere stays unresolved
	// (e.g. a private artifact whose BOM is not in the repo).
	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>app</artifactId>
	<version>1.0.0</version>
	<dependencies>
		<dependency>
			<groupId>com.example.internal</groupId>
			<artifactId>private-lib</artifactId>
		</dependency>
	</dependencies>
</project>`

	deps := NewMavenParser().ParsePomXML(pom)
	byName := depByName(deps)

	lib, ok := byName["com.example.internal:private-lib"]
	require.True(t, ok)
	assert.Equal(t, "latest", lib.Version, "unmanaged versionless dep stays unresolved")
}
