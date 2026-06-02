package types

import (
	"testing"
)

func TestPayload_AddDependency_Deduplication(t *testing.T) {
	payload := NewPayload("test", []string{"test/path"})

	// Add the same dependency multiple times
	dep1 := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: ""}
	dep2 := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: ""}
	dep3 := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: ""}
	dep4 := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: ""}
	dep5 := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: ""}

	payload.AddDependency(dep1)
	payload.AddDependency(dep2)
	payload.AddDependency(dep3)
	payload.AddDependency(dep4)
	payload.AddDependency(dep5)

	// Should only have one dependency due to deduplication
	if len(payload.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(payload.Dependencies))
	}

	// Verify the dependency content
	if payload.Dependencies[0].Type != "docker" {
		t.Errorf("Expected type 'docker', got '%s'", payload.Dependencies[0].Type)
	}
	if payload.Dependencies[0].Name != "registry.example.com/myorg/myteam/activemq-artemis" {
		t.Errorf("Expected name 'registry.example.com/myorg/myteam/activemq-artemis', got '%s'", payload.Dependencies[0].Name)
	}

	// Test adding different dependencies
	depDifferent := Dependency{Type: "docker", Name: "different-image", Version: ""}
	payload.AddDependency(depDifferent)

	if len(payload.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies after adding different one, got %d", len(payload.Dependencies))
	}

	// Test adding dependency with different version
	depWithVersion := Dependency{Type: "docker", Name: "registry.example.com/myorg/myteam/activemq-artemis", Version: "latest"}
	payload.AddDependency(depWithVersion)

	if len(payload.Dependencies) != 3 {
		t.Errorf("Expected 3 dependencies after adding same name with different version, got %d", len(payload.Dependencies))
	}
}

func TestPayload_containsDependency(t *testing.T) {
	payload := NewPayload("test", []string{"test/path"})

	dep1 := Dependency{Type: "docker", Name: "test-image", Version: ""}
	dep2 := Dependency{Type: "docker", Name: "test-image", Version: "latest"}
	dep3 := Dependency{Type: "npm", Name: "test-image", Version: ""}

	// Initially empty
	if payload.containsDependency(dep1) {
		t.Error("Should not contain dependency in empty payload")
	}

	// Add first dependency
	payload.AddDependency(dep1)

	// Should contain exact match
	if !payload.containsDependency(dep1) {
		t.Error("Should contain dependency that was just added")
	}

	// Should not contain dependency with different version
	if payload.containsDependency(dep2) {
		t.Error("Should not contain dependency with different version")
	}

	// Should not contain dependency with different type
	if payload.containsDependency(dep3) {
		t.Error("Should not contain dependency with different type")
	}
}

func TestDependency_SetDeclaredVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		declared  string
		wantInMap bool
		wantValue string
	}{
		{"differs - range vs resolved", "1.2.11", "^1.2.0", true, "^1.2.0"},
		{"differs - property ref", "5.3.20", "${spring.version}", true, "${spring.version}"},
		{"equal - no record", "1.2.3", "1.2.3", false, ""},
		{"empty declared - no record", "1.2.3", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := Dependency{Type: "npm", Name: "lib", Version: tt.version}
			dep.SetDeclaredVersion(tt.declared)
			val, ok := dep.Metadata[MetadataKeyDeclared]
			if ok != tt.wantInMap {
				t.Fatalf("declared present = %v, want %v", ok, tt.wantInMap)
			}
			if tt.wantInMap && val.(string) != tt.wantValue {
				t.Errorf("declared = %v, want %q", val, tt.wantValue)
			}
		})
	}
}
