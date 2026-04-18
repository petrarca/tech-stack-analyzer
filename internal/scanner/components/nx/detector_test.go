package nx

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// mockProvider implements types.Provider for testing
type mockProvider struct {
	files map[string][]byte
}

func (m *mockProvider) ReadFile(path string) ([]byte, error) {
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return nil, nil
}

func (m *mockProvider) Exists(path string) (bool, error) {
	_, ok := m.files[path]
	return ok, nil
}

func (m *mockProvider) ListDir(_ string) ([]types.File, error) { return nil, nil }
func (m *mockProvider) Open(_ string) (string, error)          { return "", nil }
func (m *mockProvider) IsDir(_ string) (bool, error)           { return false, nil }
func (m *mockProvider) GetBasePath() string                    { return "" }

// mockDepDetector implements components.DependencyDetector for testing
type mockDepDetector struct{}

func (m *mockDepDetector) MatchDependencies(_ []string, _ string) map[string][]string {
	return nil
}

func (m *mockDepDetector) AddPrimaryTechIfNeeded(_ *types.Payload, _ string) {}

func (m *mockDepDetector) ApplyMatchesToPayload(_ *types.Payload, _ map[string][]string) {}

func TestDetect_Library(t *testing.T) {
	d := &Detector{}
	provider := &mockProvider{
		files: map[string][]byte{
			"/workspace/libs/calendar/project.json": []byte(`{
				"name": "calendar",
				"projectType": "library",
				"sourceRoot": "libs/calendar/src"
			}`),
		},
	}
	files := []types.File{{Name: "project.json"}}
	payloads := d.Detect(files, "/workspace/libs/calendar", "/workspace", provider, &mockDepDetector{})

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Name != "calendar" {
		t.Errorf("expected name 'calendar', got %q", payloads[0].Name)
	}
	if payloads[0].ComponentType != "nx" {
		t.Errorf("expected component type 'nx', got %q", payloads[0].ComponentType)
	}
	if len(payloads[0].Path) == 0 || payloads[0].Path[0] != "/libs/calendar/project.json" {
		t.Errorf("unexpected path: %v", payloads[0].Path)
	}
}

func TestDetect_Application(t *testing.T) {
	d := &Detector{}
	provider := &mockProvider{
		files: map[string][]byte{
			"/workspace/apps/cdp/project.json": []byte(`{
				"name": "cdp",
				"projectType": "application"
			}`),
		},
	}
	files := []types.File{{Name: "project.json"}}
	payloads := d.Detect(files, "/workspace/apps/cdp", "/workspace", provider, &mockDepDetector{})

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Name != "cdp" {
		t.Errorf("expected name 'cdp', got %q", payloads[0].Name)
	}
}

func TestDetect_SkipsUnknownProjectType(t *testing.T) {
	d := &Detector{}
	provider := &mockProvider{
		files: map[string][]byte{
			"/workspace/project.json": []byte(`{
				"name": "something",
				"projectType": "unknown-type"
			}`),
		},
	}
	files := []types.File{{Name: "project.json"}}
	payloads := d.Detect(files, "/workspace", "/workspace", provider, &mockDepDetector{})

	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads for unknown projectType, got %d", len(payloads))
	}
}

func TestDetect_SkipsMissingProjectType(t *testing.T) {
	d := &Detector{}
	provider := &mockProvider{
		files: map[string][]byte{
			"/workspace/project.json": []byte(`{"name": "something"}`),
		},
	}
	files := []types.File{{Name: "project.json"}}
	payloads := d.Detect(files, "/workspace", "/workspace", provider, &mockDepDetector{})

	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads without projectType, got %d", len(payloads))
	}
}

func TestDetect_SkipsMissingName(t *testing.T) {
	d := &Detector{}
	provider := &mockProvider{
		files: map[string][]byte{
			"/workspace/libs/foo/project.json": []byte(`{"projectType": "library"}`),
		},
	}
	files := []types.File{{Name: "project.json"}}
	payloads := d.Detect(files, "/workspace/libs/foo", "/workspace", provider, &mockDepDetector{})

	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads without name, got %d", len(payloads))
	}
}

func TestDetect_NoProjectJSON(t *testing.T) {
	d := &Detector{}
	files := []types.File{{Name: "package.json"}, {Name: "tsconfig.json"}}
	payloads := d.Detect(files, "/workspace", "/workspace", &mockProvider{files: nil}, &mockDepDetector{})

	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads when no project.json, got %d", len(payloads))
	}
}
