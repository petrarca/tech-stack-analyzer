package scanner

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestComponentRegistry(t *testing.T) {
	registry := NewComponentRegistry()

	// Create .NET library component
	lib := types.NewPayloadWithPath("MyLib", "/lib")
	lib.ID = "lib-id"
	lib.Properties = map[string]interface{}{
		"dotnet": map[string]interface{}{
			"assembly_name": "MyCompany.SharedLib",
			"package_id":    "MyCompany.SharedLib",
			"framework":     "net8.0",
		},
	}

	// Register library
	registry.Register(lib)

	// Create app component with dependency
	app := types.NewPayloadWithPath("MyApp", "/app")
	app.ID = "app-id"
	app.Properties = map[string]interface{}{
		"dotnet": map[string]interface{}{
			"assembly_name": "MyCompany.WebApp",
			"framework":     "net8.0",
		},
	}
	app.Dependencies = []types.Dependency{
		{
			Type:    "nuget",
			Name:    "MyCompany.SharedLib",
			Version: "1.0.0",
		},
	}

	// Test lookup via byDependencyType map
	found := registry.byDependencyType["nuget"]["MyCompany.SharedLib"]
	if found == nil {
		t.Fatal("Expected to find MyCompany.SharedLib in registry")
	}
	if found.ID != "lib-id" {
		t.Errorf("Expected lib-id, got %s", found.ID)
	}

	// Test findMatchingComponent
	scanner := &Scanner{}
	matched := scanner.findMatchingComponent(app.Dependencies[0], registry)
	if matched == nil {
		t.Fatal("Expected to find matching component for dependency")
	}
	if matched.ID != "lib-id" {
		t.Errorf("Expected lib-id, got %s", matched.ID)
	}
}

func TestComponentRegistryMaven(t *testing.T) {
	registry := NewComponentRegistry()

	// Create Maven library component
	lib := types.NewPayloadWithPath("MyLib", "/lib")
	lib.ID = "lib-id"
	lib.Properties = map[string]interface{}{
		"maven": map[string]string{
			"group_id":    "com.example",
			"artifact_id": "my-library",
			"version":     "1.0.0",
		},
	}

	// Register library
	registry.Register(lib)

	// Test lookup with groupId:artifactId via byDependencyType map
	found := registry.byDependencyType["maven"]["com.example:my-library"]
	if found == nil {
		t.Fatal("Expected to find com.example:my-library in registry")
	}
	if found.ID != "lib-id" {
		t.Errorf("Expected lib-id, got %s", found.ID)
	}
}

func TestComponentRegistryNpmNodejs(t *testing.T) {
	registry := NewComponentRegistry()

	// Create Node.js package component
	pkg := types.NewPayloadWithPath("my-package", "/pkg")
	pkg.ID = "pkg-id"
	pkg.Properties = map[string]interface{}{
		"nodejs": map[string]string{
			"package_name": "my-awesome-package",
		},
	}

	// Register package
	registry.Register(pkg)

	// Test lookup via byDependencyType map
	found := registry.byDependencyType["npm"]["my-awesome-package"]
	if found == nil {
		t.Fatal("Expected to find my-awesome-package in registry")
	}
	if found.ID != "pkg-id" {
		t.Errorf("Expected pkg-id, got %s", found.ID)
	}
}
