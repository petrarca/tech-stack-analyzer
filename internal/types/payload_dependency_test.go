package types

import (
	"testing"
)

func TestPayload_AddDependency_Deduplication(t *testing.T) {
	payload := NewPayload("test", []string{"test/path"})

	// Add the same dependency multiple times
	dep1 := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: ""}
	dep2 := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: ""}
	dep3 := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: ""}
	dep4 := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: ""}
	dep5 := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: ""}

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
	if payload.Dependencies[0].Name != "naisacrname.azurecr.io/nais/vromero/activemq-artemis" {
		t.Errorf("Expected name 'naisacrname.azurecr.io/nais/vromero/activemq-artemis', got '%s'", payload.Dependencies[0].Name)
	}

	// Test adding different dependencies
	depDifferent := Dependency{Type: "docker", Name: "different-image", Version: ""}
	payload.AddDependency(depDifferent)

	if len(payload.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies after adding different one, got %d", len(payload.Dependencies))
	}

	// Test adding dependency with different version
	depWithVersion := Dependency{Type: "docker", Name: "naisacrname.azurecr.io/nais/vromero/activemq-artemis", Version: "latest"}
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
