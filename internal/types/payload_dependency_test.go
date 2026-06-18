package types

import (
	"encoding/json"
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

func TestDependency_JSONRoundTrip(t *testing.T) {
	cases := []Dependency{
		{Type: "maven", Name: "com.example:lib", Version: "1.2.3", Scope: "prod", Direct: true,
			Metadata: map[string]interface{}{"source": "pom.xml", "declared": "${lib.version}"}},
		{Type: "npm", Name: "@scope/pkg", Version: "4.5.6", Scope: "dev", Direct: false},
		{Type: "pypi", Name: "requests", Version: "2.32.3", Scope: "prod", Direct: true},
	}
	for _, in := range cases {
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal %s: %v", in.Name, err)
		}
		var out Dependency
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("unmarshal %s: %v (json=%s)", in.Name, err, data)
		}
		if out.Type != in.Type || out.Name != in.Name || out.Version != in.Version ||
			out.Scope != in.Scope || out.Direct != in.Direct {
			t.Errorf("round-trip mismatch for %s:\n in=%+v\nout=%+v", in.Name, in, out)
		}
		// declared metadata must survive when present
		if in.Metadata["declared"] != nil && out.Metadata["declared"] != in.Metadata["declared"] {
			t.Errorf("declared metadata lost for %s: %v", in.Name, out.Metadata)
		}
	}
}

func TestDependency_UnmarshalJSON_ObjectForm(t *testing.T) {
	// The object (struct) form must also decode, for forward compatibility.
	var d Dependency
	if err := json.Unmarshal([]byte(`{"type":"npm","name":"lodash","version":"4.17.21","direct":true}`), &d); err != nil {
		t.Fatalf("unmarshal object form: %v", err)
	}
	if d.Type != "npm" || d.Name != "lodash" || d.Version != "4.17.21" || !d.Direct {
		t.Errorf("object form decoded wrong: %+v", d)
	}
}
