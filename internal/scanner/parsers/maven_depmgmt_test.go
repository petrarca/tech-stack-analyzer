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

func TestMavenParser_ImportedBomDependencyManagement(t *testing.T) {
	// fhir-server has no <dependencyManagement> of its own; its parent chain
	// reaches an ancestor that imports a BOM (scope=import) whose POM holds the
	// managed version. The resolver locates that BOM in the "repo".
	childPath := "/repo/backend/platform-services/fhir-server/pom.xml"
	files := map[string]string{
		childPath: `<?xml version="1.0"?>
<project>
	<parent>
		<groupId>com.example.app</groupId>
		<artifactId>platform-services</artifactId>
		<version>1.0</version>
	</parent>
	<artifactId>fhir-server</artifactId>
	<dependencies>
		<dependency>
			<groupId>ca.uhn.hapi.fhir</groupId>
			<artifactId>hapi-fhir-base</artifactId>
		</dependency>
	</dependencies>
</project>`,
		"/repo/backend/platform-services/pom.xml": `<?xml version="1.0"?>
<project>
	<parent>
		<groupId>com.example.app</groupId>
		<artifactId>backend</artifactId>
		<version>1.0</version>
	</parent>
	<artifactId>platform-services</artifactId>
</project>`,
		"/repo/backend/pom.xml": `<?xml version="1.0"?>
<project>
	<groupId>com.example.app</groupId>
	<artifactId>backend</artifactId>
	<version>1.0</version>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>com.example.app</groupId>
				<artifactId>bom</artifactId>
				<version>1.0</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`,
	}
	bomPOM := `<?xml version="1.0"?>
<project>
	<groupId>com.example.app</groupId>
	<artifactId>bom</artifactId>
	<version>1.0</version>
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
</project>`

	provider := &mockFileProvider{files: files}
	// Resolver maps the imported BOM coordinate to its POM content.
	resolver := func(groupID, artifactID, _ string) ([]byte, string, bool) {
		if groupID == "com.example.app" && artifactID == "bom" {
			return []byte(bomPOM), "/repo/backend/bom", true
		}
		return nil, "", false
	}

	deps := NewMavenParser().ParsePomXMLWithBomResolver(files[childPath], "/repo/backend/platform-services/fhir-server", provider, resolver)
	byName := depByName(deps)

	hapi, ok := byName["ca.uhn.hapi.fhir:hapi-fhir-base"]
	require.True(t, ok)
	assert.Equal(t, "7.6.0", hapi.Version, "version should resolve from imported BOM via property")
	assert.Equal(t, "dependency-management", hapi.Metadata["source"])
}

func TestMavenParser_ImportedBom_NotInRepoStaysVersionless(t *testing.T) {
	// A versionless dep managed only by a BOM that the resolver cannot find
	// (third-party/private) stays unresolved.
	pom := `<?xml version="1.0"?>
<project>
	<groupId>com.example.app</groupId>
	<artifactId>app</artifactId>
	<version>1.0</version>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.thirdparty</groupId>
				<artifactId>thirdparty-bom</artifactId>
				<version>3.0.0</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
	<dependencies>
		<dependency>
			<groupId>org.thirdparty</groupId>
			<artifactId>some-lib</artifactId>
		</dependency>
	</dependencies>
</project>`
	// Resolver finds nothing.
	resolver := func(_, _, _ string) ([]byte, string, bool) { return nil, "", false }

	deps := NewMavenParser().ParsePomXMLWithBomResolver(pom, "", nil, resolver)
	byName := depByName(deps)
	lib, ok := byName["org.thirdparty:some-lib"]
	require.True(t, ok)
	assert.Equal(t, "latest", lib.Version)
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

// TestCollectBomManagedVersions_GradlePlatform exercises the exported helpers
// the Gradle detector uses to resolve a platform()/enforcedPlatform() BOM and
// backfill versionless dependencies. The BOM (a pom-packaged artifact) supplies
// managed versions, including one via a property and one from its parent chain.
func TestCollectBomManagedVersions_GradlePlatform(t *testing.T) {
	bomPOM := `<?xml version="1.0"?>
<project>
	<parent>
		<groupId>com.example</groupId>
		<artifactId>parent-bom</artifactId>
		<version>1.0.0</version>
	</parent>
	<groupId>com.example</groupId>
	<artifactId>my-bom</artifactId>
	<version>3.36.2</version>
	<properties>
		<lib.version>2.5.0</lib.version>
	</properties>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>com.example</groupId>
				<artifactId>managed-lib</artifactId>
				<version>${lib.version}</version>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`
	parentBOM := `<?xml version="1.0"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>parent-bom</artifactId>
	<version>1.0.0</version>
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>com.example</groupId>
				<artifactId>inherited-lib</artifactId>
				<version>9.9.9</version>
			</dependency>
		</dependencies>
	</dependencyManagement>
</project>`
	resolver := func(groupID, artifactID, _ string) ([]byte, string, bool) {
		switch {
		case groupID == "com.example" && artifactID == "my-bom":
			return []byte(bomPOM), "/repo/my-bom", true
		case groupID == "com.example" && artifactID == "parent-bom":
			return []byte(parentBOM), "/repo/parent-bom", true
		}
		return nil, "", false
	}

	managed := CollectBomManagedVersions("com.example", "my-bom", "3.36.2", nil, resolver)
	if managed["com.example:managed-lib"] != "2.5.0" {
		t.Errorf("managed-lib: got %q, want 2.5.0 (via property)", managed["com.example:managed-lib"])
	}
	if managed["com.example:inherited-lib"] != "9.9.9" {
		t.Errorf("inherited-lib: got %q, want 9.9.9 (via parent chain)", managed["com.example:inherited-lib"])
	}

	// ApplyManagedVersions backfills only unresolved versions.
	deps := []types.Dependency{
		{Type: DependencyTypeGradle, Name: "com.example:managed-lib", Version: "latest"},
		{Type: DependencyTypeGradle, Name: "com.example:pinned-lib", Version: "1.0.0"},
	}
	ApplyManagedVersions(deps, managed)
	if deps[0].Version != "2.5.0" {
		t.Errorf("managed-lib not backfilled: got %q", deps[0].Version)
	}
	if deps[1].Version != "1.0.0" {
		t.Errorf("pinned-lib must not be overwritten: got %q", deps[1].Version)
	}
}
